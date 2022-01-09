package dynamic

import (
	"encoding/json"

	"github.com/kristofferahl/aeto/internal/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// Clients wrapper
type Clients struct {
	DynamicClient   dynamic.Interface
	DiscoveryClient *discovery.DiscoveryClient
}

// Apply performs a patch in a similar manner to kubectl apply
func (c *Clients) Apply(ctx reconcile.Context, namespacedName types.NamespacedName, manifest string) error {
	obj := &unstructured.Unstructured{}

	// Decode YAML into unstructured.Unstructured
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := dec.Decode([]byte(manifest), nil, obj)
	if err != nil {
		return err
	}

	// Find REST mapping
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(c.DiscoveryClient))
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	dri := c.DynamicClient.Resource(mapping.Resource).Namespace(namespacedName.Namespace)

	// Apply resource
	ctx.Log.V(1).Info("applying resource", "resource", namespacedName.String(), "gvk", obj.GroupVersionKind().String())

	data, err := json.Marshal(obj)
	if err != nil {
		ctx.Log.Error(err, "marshalling resource json failed", "resource", namespacedName.String(), "gvk", obj.GroupVersionKind().String())
		return err
	}

	force := true
	_, err = dri.Patch(ctx.Context, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "aeto",
		Force:        &force,
	})
	if err != nil {
		ctx.Log.Error(err, "failed to apply resource", "resource", namespacedName.String(), "gvk", obj.GroupVersionKind().String())
		return err
	}

	ctx.Log.V(1).Info("applied resource", "resource", namespacedName.String(), "gvk", obj.GroupVersionKind().String())
	return nil
}

// Get returns an unstructured object given a namespaced name and group, version, kind
func (c *Clients) Get(ctx reconcile.Context, namespacedName types.NamespacedName, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	ctx.Log.V(1).Info("fetching resource", "resource", namespacedName.String(), "gvk", gvk.String())

	// Find REST mapping
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(c.DiscoveryClient))
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	// Get resource
	dri := c.DynamicClient.Resource(mapping.Resource).Namespace(namespacedName.Namespace)
	resource, err := dri.Get(ctx.Context, namespacedName.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ctx.Log.V(1).Info("fetched resource", "resource", namespacedName.String(), "gvk", gvk.String())

	return resource, nil
}