package tenant

import (
	"fmt"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/convert"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/template"
	"github.com/kristofferahl/aeto/internal/pkg/util"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewResourceGenerator(ctx reconcile.Context, services ResourceGeneratoreServices) ResourceGenerator {
	return ResourceGenerator{
		ctx:      ctx,
		services: services,
	}
}

type ResourceGenerator struct {
	services ResourceGeneratoreServices
	ctx      reconcile.Context
	state    State
}

type ResourceGeneratoreServices struct {
	kubernetes.Client
}

type ResourceGenerationResult struct {
	ResourceGroups ResourceGroupList
	Sum            string
}

type GenerateError struct {
	Errors []error
}

func (e *GenerateError) Error() string {
	return fmt.Sprintf("failed to generate resources from blueprint, encountered %d error(s)", len(e.Errors))
}

type sum struct {
	BlueprintName string
	Resources     []resourceSum
}

type resourceSum struct {
	Id    string
	Order int
	Sum   string
}

func (r *ResourceGenerator) Generate(state State, blueprint corev1alpha1.Blueprint) (result ResourceGenerationResult, err error) {
	r.state = state

	result.ResourceGroups = make([]ResourceGroup, 0)
	errors := make([]error, 0)
	resourceIndex := 0

	for _, resourceGroup := range blueprint.Spec.Resources {
		group := ResourceGroup{
			Name:           resourceGroup.Name,
			SourceTemplate: resourceGroup.Template,
			Resources:      make([]Resource, 0),
		}

		unstructured, err := r.generateFromResourceGroup(resourceGroup, blueprint, result.ResourceGroups)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		for _, resource := range unstructured {
			resourceIndex++
			bytes, err := resource.MarshalJSON()
			if err != nil {
				errors = append(errors, err)
				continue
			}

			gvk := resource.GroupVersionKind()
			uid := gvk.Group + "/* , Kind=" + gvk.Kind + " " + resource.GetNamespace() + "/" + resource.GetName()
			id, err := util.AsSha256(uid)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			group.Resources = append(group.Resources, Resource{
				Id:    id,
				Order: resourceIndex,
				Sum:   util.Sha256Sum(bytes),
				Embedded: EmbeddedResource{
					RawExtension: runtime.RawExtension{
						Raw: bytes,
					},
				},
			})
		}

		result.ResourceGroups = append(result.ResourceGroups, group)
	}

	sumObj := sum{
		BlueprintName: blueprint.Name,
		Resources:     make([]resourceSum, 0),
	}
	for _, r := range result.ResourceGroups.Resources() {
		sumObj.Resources = append(sumObj.Resources, resourceSum{
			Id:    r.Id,
			Order: r.Order,
			Sum:   r.Sum,
		})
	}

	sum, err := util.AsSha256(sumObj)
	if err != nil {
		errors = append(errors, err)
	} else {
		result.Sum = sum
	}

	if len(errors) > 0 {
		err = &GenerateError{
			Errors: errors,
		}
	}

	return result, err
}

func (r *ResourceGenerator) generateFromResourceGroup(resourceGroup corev1alpha1.BlueprintResourceGroup, blueprint corev1alpha1.Blueprint, resourceGroups []ResourceGroup) ([]*unstructured.Unstructured, error) {
	rtRef := types.NamespacedName{
		Namespace: config.Operator.Namespace,
		Name:      resourceGroup.Template,
	}
	rt, err := r.getResourceTemplate(r.ctx, rtRef)
	if err != nil {
		return nil, err
	}

	resolver := ValueResolver{
		TenantName:        r.state.TenantPrefixedName,
		TenantNamespace:   r.state.TenantPrefixedNamespace,
		OperatorNamespace: config.Operator.Namespace,
		ResourceGroups:    resourceGroups,
		Client:            r.services.Client,
		Context:           r.ctx,
	}

	r.ctx.Log.V(1).Info("applying parameter overrides")
	err = rt.Spec.Parameters.SetValues(resourceGroup.Parameters, resolver.Func)
	if err != nil {
		return nil, err
	}

	err = rt.Spec.Parameters.Validate()
	if err != nil {
		return nil, err
	}

	templateData := r.newTemplateData(blueprint, rt.Spec.Parameters)

	r.ctx.Log.V(1).Info("building resources from resource template", "template", resourceGroup.Template)
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

	r.ctx.Log.V(1).Info("applying name, namespace and common labels/annotations to resources", "template", resourceGroup.Template)
	for _, resource := range allResources {
		resourceName := ""
		resourceNamespace := ""

		switch rt.Spec.Rules.Namespace {
		case corev1alpha1.ResourceNamespaceOperator:
			resourceNamespace = config.Operator.Namespace
		case corev1alpha1.ResourceNamespaceTenant:
			resourceNamespace = r.state.TenantPrefixedNamespace
		case corev1alpha1.ResourceNamespaceKeep:
			resourceNamespace = resource.GetNamespace()
			if resource.GroupVersionKind() == namespaceGVK {
				resourceNamespace = resource.GetName()
			}
		}

		switch rt.Spec.Rules.Name {
		case corev1alpha1.ResourceNameTenant:
			resourceName = r.state.TenantPrefixedName
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
		labels := merge(resource.GetLabels(), r.state.Labels)
		annotations := merge(resource.GetAnnotations(), r.state.Annotations)
		resource.SetLabels(labels)
		resource.SetAnnotations(annotations)

		var content interface{} = resource.UnstructuredContent()
		if util.SliceContainsString(config.NonLoggableKinds(), resource.GetKind()) {
			content = resource.GroupVersionKind().String()
		}
		r.ctx.Log.V(1).Info("all changes applied to resource", "template", resourceGroup.Template, "resource", content)
	}

	return allResources, nil
}

func (r *ResourceGenerator) newTemplateData(blueprint corev1alpha1.Blueprint, parameters []*corev1alpha1.Parameter) template.Data {
	return template.Data{
		Name:         r.state.TenantName,
		PrefixedName: r.state.TenantPrefixedName,
		DisplayName:  r.state.TenantDisplayName,
		Namespaces: template.Namespaces{
			Tenant:   r.state.TenantPrefixedNamespace,
			Operator: config.Operator.Namespace,
		},
		Labels:      r.state.Labels,
		Annotations: r.state.Annotations,
		Parameters:  parameters,
		Utils:       template.UtilityFunctions{},
	}
}

func (r *ResourceGenerator) getResourceTemplate(ctx reconcile.Context, nn types.NamespacedName) (corev1alpha1.ResourceTemplate, error) {
	var rt corev1alpha1.ResourceTemplate
	if err := r.services.Client.Get(ctx, client.ObjectKey{
		Name:      nn.Name,
		Namespace: nn.Namespace,
	}, &rt); err != nil {
		return corev1alpha1.ResourceTemplate{}, err
	}
	return rt, nil
}

func (r *ResourceGenerator) generateUnstructureResources(json string, templateData template.Data) ([]*unstructured.Unstructured, error) {
	allResources := make([]*unstructured.Unstructured, 0)

	tmpl, err := template.NewYamlTemplate(json, template.InputFormatJson)
	if err != nil {
		return nil, err
	}

	templated, err := tmpl.Execute(templateData)
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

func merge(m1 map[string]string, m2 map[string]string) map[string]string {
	m := map[string]string{}
	for k, v := range m1 {
		m[k] = v
	}

	for k, v := range m2 {
		m[k] = v
	}
	return m
}
