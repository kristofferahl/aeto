/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/convert"
	"github.com/kristofferahl/aeto/internal/pkg/dynamic"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	templating "github.com/kristofferahl/aeto/internal/pkg/template"
)

var (
	namespaceGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Dynamic  dynamic.Clients
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core.aeto.net,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.aeto.net,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.aeto.net,resources=tenants/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Tenant object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := reconcile.NewContext("tenant", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	tenant, err := r.getTenant(rctx)
	if err != nil {
		rctx.Log.Info("Tenant not found")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	rctx.Log.V(1).Info("found Tenant")

	blueprintRef := types.NamespacedName{
		Namespace: config.Operator.Namespace,
		Name:      tenant.Blueprint(),
	}
	blueprint, err := r.getBlueprint(rctx, blueprintRef)
	if err != nil {
		rctx.Log.Info("Blueprint not found", "blueprint", blueprintRef.String())
		return ctrl.Result{}, err
	}

	results := make([]reconcile.Result, 0)

	resourceSet, errs := r.newResourceSet(rctx, tenant, blueprint)
	if len(resourceSet.Spec.Groups) > 0 {
		rs, result := r.applyResourceSet(rctx, resourceSet)
		if result.Error == nil {
			resourceSet = *rs
		}
		results = append(results, result)
	}

	if len(errs) > 0 {
		for _, err = range errs {
			r.Recorder.Event(&tenant, "Warning", "ResourceSet", fmt.Sprintf("Failed to generate resource set for tenant; %s", err.Error()))
		}
		results = append(results, rctx.RequeueIn(15))
	}

	// Handle update of status
	results = append(results, r.updateStatus(rctx, tenant, blueprint, resourceSet))

	return rctx.Complete(results...)
}

func (r *TenantReconciler) newResourceSet(ctx reconcile.Context, tenant corev1alpha1.Tenant, blueprint corev1alpha1.Blueprint) (resourceSet corev1alpha1.ResourceSet, errors []error) {
	resourceSet = corev1alpha1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        tenant.Name,
			Namespace:   config.Operator.Namespace,
			Labels:      blueprint.CommonLabels(tenant),
			Annotations: blueprint.CommonAnnotations(tenant),
		},
		Spec: corev1alpha1.ResourceSetSpec{
			Groups: make([]corev1alpha1.ResourceSetResourceGroup, 0),
		},
	}

	for _, resourceGroup := range blueprint.Spec.Resources {
		rsrg := corev1alpha1.ResourceSetResourceGroup{
			Name:           resourceGroup.Name,
			SourceTemplate: resourceGroup.Template,
			Resources:      make([]corev1alpha1.EmbeddedResource, 0),
		}

		resources, err := r.generateResourcesFromBlueprintResourceGroup(ctx, resourceGroup, tenant, blueprint, resourceSet)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		for _, resource := range resources {
			json, err := resource.MarshalJSON()
			if err != nil {
				errors = append(errors, err)
				continue
			}

			rsrg.Resources = append(rsrg.Resources, corev1alpha1.EmbeddedResource{
				RawExtension: runtime.RawExtension{
					Raw: json,
				},
			})
		}

		resourceSet.Spec.Groups = append(resourceSet.Spec.Groups, rsrg)
	}

	ctx.Log.V(1).Info("generated new resource set", "resources", resourceSet.Spec.Groups)
	return resourceSet, errors
}

func (r *TenantReconciler) generateResourcesFromBlueprintResourceGroup(rctx reconcile.Context, resourceGroup corev1alpha1.BlueprintResourceGroup, tenant corev1alpha1.Tenant, blueprint corev1alpha1.Blueprint, resourceSet corev1alpha1.ResourceSet) ([]*unstructured.Unstructured, error) {
	rtRef := types.NamespacedName{
		Namespace: config.Operator.Namespace,
		Name:      resourceGroup.Template,
	}
	rt, err := r.getResourceTemplate(rctx, rtRef)
	if err != nil {
		rctx.Log.Info("ResourceTemplate not found")
		return nil, err
	}
	rctx.Log.V(1).Info("found ResourceTemplate")

	resolver := ValueResolver{
		TenantName:        blueprint.Spec.ResourceNamePrefix + tenant.Name, // TODO: This should probably be done in a single place
		TenantNamespace:   blueprint.Spec.ResourceNamePrefix + tenant.Name, // TODO: This should probably be done in a single place
		OperatorNamespace: config.Operator.Namespace,
		ResourceSet:       resourceSet,
		Dynamic:           r.Dynamic,
		Context:           rctx,
	}

	rctx.Log.V(1).Info("applying parameter overrides")
	err = rt.Spec.Parameters.SetValues(resourceGroup.Parameters, resolver.Func)
	if err != nil {
		return nil, err
	}

	err = rt.Spec.Parameters.Validate()
	if err != nil {
		return nil, err
	}

	templateData := newTemplateData(tenant, blueprint, rt.Spec.Parameters)

	rctx.Log.V(1).Info("building resources from resource template", "template", resourceGroup.Template)
	allResources := make([]*unstructured.Unstructured, 0)

	for _, raw := range rt.Spec.Raw {
		resources, err := r.generateUnstructureResources(raw, templateData)
		if err != nil {
			return nil, err
		}
		allResources = append(allResources, resources...)
	}

	for _, resource := range rt.Spec.Resources {
		resources, err := r.generateUnstructureResources(string(resource.Raw), templateData)
		if err != nil {
			return nil, err
		}
		allResources = append(allResources, resources...)
	}

	// TODO: Set field manager for ResourceSet?

	rctx.Log.V(1).Info("applying name, namespace and common labels/annotations to resources", "template", resourceGroup.Template)
	for _, resource := range allResources {
		resourceName := ""
		resourceNamespace := ""

		switch rt.Spec.Rules.Namespace {
		case corev1alpha1.ResourceNamespaceOperator:
			resourceNamespace = config.Operator.Namespace
		case corev1alpha1.ResourceNamespaceTenant:
			resourceNamespace = blueprint.Spec.ResourceNamePrefix + tenant.Name
		case corev1alpha1.ResourceNamespaceKeep:
			resourceNamespace = resource.GetNamespace()
			if resource.GroupVersionKind() == namespaceGVK {
				resourceNamespace = resource.GetName()
			}
		}

		switch rt.Spec.Rules.Name {
		case corev1alpha1.ResourceNameTenant:
			resourceName = blueprint.Spec.ResourceNamePrefix + tenant.Name
		case corev1alpha1.ResourceNameKeep:
			resourceName = resource.GetName()
		}

		if resource.GroupVersionKind() == namespaceGVK {
			resource.SetName(resourceNamespace)
		} else {
			resource.SetName(resourceName)
			resource.SetNamespace(resourceNamespace)
		}

		// TODO: Override existing labels and annotations if they exist?
		resource.SetLabels(blueprint.CommonLabels(tenant))
		resource.SetAnnotations(blueprint.CommonAnnotations(tenant))

		rctx.Log.V(1).Info("all changes applied to resource", "template", resourceGroup.Template, "resource", resource.UnstructuredContent())
	}

	return allResources, nil
}

func newTemplateData(tenant corev1alpha1.Tenant, blueprint corev1alpha1.Blueprint, parameters []*corev1alpha1.Parameter) templating.Data {
	return templating.Data{
		Key:                tenant.Name,
		ResourceNamePrefix: blueprint.Spec.ResourceNamePrefix,
		Name:               tenant.Spec.Name,
		Namespaces: templating.Namespaces{
			Tenant:   blueprint.Spec.ResourceNamePrefix + tenant.Name,
			Operator: config.Operator.Namespace,
		},
		Labels:      blueprint.CommonLabels(tenant),
		Annotations: blueprint.CommonAnnotations(tenant),
		Parameters:  parameters,
	}
}

func (r *TenantReconciler) generateUnstructureResources(json string, templateData templating.Data) ([]*unstructured.Unstructured, error) {
	allResources := make([]*unstructured.Unstructured, 0)

	template, err := templating.NewYamlTemplate(json, templating.InputFormatJson)
	if err != nil {
		return nil, err
	}

	templated, err := template.Execute(templateData)
	if err != nil {
		return nil, err
	}

	docs, err := convert.YamlToStringSlice(templated)
	if err != nil {
		return nil, err
	}

	for _, doc := range docs {
		templatedResources, err := convert.YamlToUnstructuredSlice(doc)
		if err != nil {
			return nil, err
		}

		allResources = append(allResources, templatedResources...)
	}

	return allResources, nil
}

func (r *TenantReconciler) getTenant(ctx reconcile.Context) (corev1alpha1.Tenant, error) {
	var tenant corev1alpha1.Tenant
	if err := r.Get(ctx.Context, ctx.Request.NamespacedName, &tenant); err != nil {
		return corev1alpha1.Tenant{}, err
	}
	return tenant, nil
}

func (r *TenantReconciler) getBlueprint(ctx reconcile.Context, nn types.NamespacedName) (corev1alpha1.Blueprint, error) {
	var blueprint corev1alpha1.Blueprint
	if err := r.Get(ctx.Context, nn, &blueprint); err != nil {
		return corev1alpha1.Blueprint{}, err
	}
	return blueprint, nil
}

func (r *TenantReconciler) getResourceTemplate(ctx reconcile.Context, nn types.NamespacedName) (corev1alpha1.ResourceTemplate, error) {
	var rt corev1alpha1.ResourceTemplate
	if err := r.Get(ctx.Context, client.ObjectKey{
		Name:      nn.Name,
		Namespace: nn.Namespace,
	}, &rt); err != nil {
		return corev1alpha1.ResourceTemplate{}, err
	}
	return rt, nil
}

func (r *TenantReconciler) getResourceSet(ctx reconcile.Context, nn types.NamespacedName) (*corev1alpha1.ResourceSet, error) {
	var rs corev1alpha1.ResourceSet
	if err := r.Get(ctx.Context, client.ObjectKey{
		Name:      nn.Name,
		Namespace: nn.Namespace,
	}, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

func (r *TenantReconciler) applyResourceSet(ctx reconcile.Context, resourceSet corev1alpha1.ResourceSet) (*corev1alpha1.ResourceSet, reconcile.Result) {
	existing, err := r.getResourceSet(ctx, resourceSet.NamespacedName())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, ctx.Error(err)
		}

		ctx.Log.Info("ResourceSet not found, creating", "resourceset", resourceSet.NamespacedName())
		err := r.Create(ctx.Context, &resourceSet)
		if err != nil {
			ctx.Log.Error(err, "failed to create ResourceSet", "resourceset", resourceSet.NamespacedName())
			return nil, ctx.Error(err)
		}

		return &resourceSet, ctx.Done()
	}

	if existing != nil {
		existing.Spec.Groups = resourceSet.Spec.Groups
		ctx.Log.V(1).Info("updating ResourceSet", "resourceset", resourceSet.NamespacedName())
		err := r.Update(ctx.Context, existing)
		if err != nil {
			ctx.Log.Error(err, "failed to update ResourceSet", "resourceset", resourceSet.NamespacedName())
			return nil, ctx.Error(err)
		}
	}

	return existing, ctx.Done()
}

func (r *TenantReconciler) updateStatus(ctx reconcile.Context, tenant corev1alpha1.Tenant, blueprint corev1alpha1.Blueprint, resourceSet corev1alpha1.ResourceSet) reconcile.Result {
	tenant.Status.Blueprint = blueprint.NamespacedName().String() + " v" + blueprint.ResourceVersion
	tenant.Status.ResourceSet = resourceSet.NamespacedName().String() + " v" + resourceSet.ResourceVersion

	ctx.Log.V(1).Info("updating Tenant status")
	if err := r.Status().Update(ctx.Context, &tenant); err != nil {
		ctx.Log.Error(err, "failed to update Tenant status")
		return ctx.Error(err)
	}

	return ctx.Done()
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Tenant{}).
		Complete(r)
}
