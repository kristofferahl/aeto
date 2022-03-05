package tenant

import (
	"encoding/json"
	"fmt"

	"github.com/PaesslerAG/jsonpath"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/dynamic"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type ValueResolver struct {
	TenantName        string
	TenantNamespace   string
	OperatorNamespace string
	ResourceGroups    []ResourceGroup
	Dynamic           dynamic.Clients
	Context           reconcile.Context
}

func (r ValueResolver) Func(vr corev1alpha1.ValueRef) (string, error) {
	resolvers := make([]func(r ValueResolver, vr corev1alpha1.ValueRef) (string, error), 0)

	if vr.Blueprint != nil {
		resolvers = append(resolvers, resolveFromBlueprint)
	}

	if vr.Resource != nil {
		resolvers = append(resolvers, resolveFromResource)
	}

	if len(resolvers) == 0 {
		return "", fmt.Errorf("invalid value reference, blueprint or resource is required")
	}

	if len(resolvers) > 1 {
		return "", fmt.Errorf("invalid value reference, multiple references not allowed")
	}

	resolve := resolvers[0]
	return resolve(r, vr)
}

// Group returns a resource group matching the specified name
func (r *ValueResolver) Group(name string) (*ResourceGroup, error) {
	for _, group := range r.ResourceGroups {
		if group.Name == name {
			return &group, nil
		}
	}
	return nil, fmt.Errorf("resource group %s not found", name)
}

func resolveFromBlueprint(r ValueResolver, vr corev1alpha1.ValueRef) (string, error) {
	ref := vr.Blueprint
	group, err := r.Group(ref.ResourceGroup)
	if err != nil {
		return "", fmt.Errorf("invalid value refrence, resource \"%s\" not found in resource set", ref.ResourceGroup)
	}

	value, err := group.JsonPath(ref.JsonPath)
	if err != nil {
		return "", fmt.Errorf("invalid value refrence for \"%s\" %s, jsonpath error: %v", ref.ResourceGroup, ref.JsonPath, err)
	}

	return value, nil
}

func resolveFromResource(r ValueResolver, vr corev1alpha1.ValueRef) (string, error) {
	ref := vr.Resource
	resourceRef := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	resourceGvk := schema.FromAPIVersionAndKind(ref.ApiVersion, ref.Kind)

	if resourceRef.Name == "$TENANT_NAME" {
		resourceRef.Name = r.TenantName
	}

	if resourceRef.Namespace == "$TENANT_NAMESPACE" {
		resourceRef.Namespace = r.TenantNamespace
	}
	if resourceRef.Namespace == "$OPERATOR_NAMESPACE" {
		resourceRef.Namespace = r.OperatorNamespace
	}

	resource, err := r.Dynamic.Get(r.Context, resourceRef, resourceGvk)
	if err != nil {
		return "", fmt.Errorf("invalid value refrence, error fetching resource \"%s\" %s: %v", resourceRef.String(), resourceGvk.String(), err)
	}

	if resource == nil {
		return "", fmt.Errorf("invalid value refrence, resource \"%s\" %s not found", resourceRef.String(), resourceGvk.String())
	}

	bytes, _ := resource.MarshalJSON()
	v := interface{}(nil)
	json.Unmarshal(bytes, &v)

	value, err := jsonpath.Get(fmt.Sprintf("$%s", ref.JsonPath), v)
	if err != nil {
		return "", fmt.Errorf("invalid value refrence for \"%s\" %s, jsonpath error: %v", resourceRef.String(), resourceGvk.String(), err)
	}

	return fmt.Sprintf("%s", value), nil
}
