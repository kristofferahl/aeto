/*
Copyright 2021.

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

package controllers

import (
	"bytes"
	"context"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/kristofferahl/aeto/api/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/convert"
)

var (
	namespaceGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}

	operatorNamespace   = "default" // TODO: Get operator namespace (from config?)
	blueprintNamePrefix = "prefix-" // TODO: Get from blueprint
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
	rctx := NewReconcileContext("tenant", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	// TODO: Review log levels

	tenant, err := r.getTenant(rctx)
	if err != nil {
		rctx.Log.Info("Tenant not found")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	rctx.Log.V(1).Info("found Tenant")

	resources := make([]*unstructured.Unstructured, 0)

	// TODO: How do we keep track of the tenant namespace name?
	res, err := r.createResourcesFromTemplate(rctx, tenant, "default-namespace-template")
	if err != nil {
		return ctrl.Result{}, err
	}
	resources = append(resources, res...)

	res, err = r.createResourcesFromTemplate(rctx, tenant, "default-network-policy-template")
	if err != nil {
		return ctrl.Result{}, err
	}
	resources = append(resources, res...)

	rctx.Log.Info("finished generating resources", "count", len(resources))

	return ctrl.Result{}, nil
}

func (r *TenantReconciler) createResourcesFromTemplate(rctx ReconcileContext, tenant corev1alpha1.Tenant, resourceTemplateName string) ([]*unstructured.Unstructured, error) {
	rt, err := r.getResourceTemplate(rctx, operatorNamespace, resourceTemplateName)
	if err != nil {
		rctx.Log.Info("ResourceTemplate not found")
		return nil, err
	}
	rctx.Log.V(1).Info("found ResourceTemplate")

	// TODO: Copy labels and annotations from Tenant?
	commonLabels := map[string]string{
		"app.net/tenant": tenant.Name,
	}
	commonAnnotations := map[string]string{
		"app.net/controlled": "true",
	}

	type Namespaces struct {
		Tenant   string
		Operator string
	}

	tmplData := struct {
		Key         string
		Name        string
		Namespaces  Namespaces
		Labels      map[string]string
		Annotations map[string]string
	}{
		Key:  tenant.Name,
		Name: tenant.Spec.Name,
		Namespaces: Namespaces{
			Tenant:   blueprintNamePrefix + tenant.Name,
			Operator: operatorNamespace,
		},
		Labels:      commonLabels,
		Annotations: commonAnnotations,
	}

	allResources := make([]*unstructured.Unstructured, 0)

	rctx.Log.V(1).Info("building resources from resource template", "template", resourceTemplateName)

	for _, raw := range rt.Spec.Raw {
		templated, err := executeTemplate(rctx, raw, tmplData)
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
	}

	for _, resource := range rt.Spec.Resources {
		templated, err := executeTemplate(rctx, string(resource.Raw), tmplData)
		if err != nil {
			return nil, err
		}

		templatedResources, err := convert.YamlToUnstructuredSlice(templated)
		if err != nil {
			return nil, err
		}

		allResources = append(allResources, templatedResources...)
	}

	// TODO: Set manager field
	// TODO: Set ownerReference

	rctx.Log.V(1).Info("applying name, namespace and common labels/annotations to resources", "template", resourceTemplateName)
	for _, resource := range allResources {
		resourceName := ""
		resourceNamespace := ""

		switch rt.Spec.Rules.Namespace {
		case corev1alpha1.ResourceNamespaceOperator:
			resourceNamespace = operatorNamespace
		case corev1alpha1.ResourceNamespaceTenant:
			resourceNamespace = blueprintNamePrefix + tenant.Name
		case corev1alpha1.ResourceNamespaceKeep:
			resourceNamespace = resource.GetNamespace()
			if resource.GroupVersionKind() == namespaceGVK {
				resourceNamespace = resource.GetName()
			}
		}

		switch rt.Spec.Rules.Name {
		case corev1alpha1.ResourceNameTenant:
			resourceName = blueprintNamePrefix + tenant.Name
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
		resource.SetLabels(commonLabels)
		resource.SetAnnotations(commonAnnotations)

		rctx.Log.V(1).Info("all changes applied to resource", "template", resourceTemplateName, "resource", resource.UnstructuredContent())
	}

	return allResources, nil
}

func executeTemplate(ctx ReconcileContext, tmpl string, data interface{}) (string, error) {
	t, err := template.New("resource").Parse(tmpl)
	if err != nil {
		ctx.Log.Error(err, "failed to parse template")
		return "", err
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, data)
	if err != nil {
		ctx.Log.Error(err, "failed to execute template")
		return "", err
	}

	str := buf.String()
	ctx.Log.V(1).Info("finished executing template", "output", str)
	return str, nil
}

func (r *TenantReconciler) getTenant(ctx ReconcileContext) (corev1alpha1.Tenant, error) {
	var tenant corev1alpha1.Tenant
	if err := r.Get(ctx.Context, ctx.Request.NamespacedName, &tenant); err != nil {
		return corev1alpha1.Tenant{}, err
	}
	return tenant, nil
}

func (r *TenantReconciler) getResourceTemplate(ctx ReconcileContext, namespace string, name string) (corev1alpha1.ResourceTemplate, error) {
	var rt corev1alpha1.ResourceTemplate
	if err := r.Get(ctx.Context, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &rt); err != nil {
		return corev1alpha1.ResourceTemplate{}, err
	}
	return rt, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Tenant{}).
		Complete(r)
}
