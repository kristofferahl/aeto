package kubernetes

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/kristofferahl/aeto/internal/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const FieldManagerName = "aeto"

var (
	defaultCreateOptions = client.CreateOptions{
		FieldManager: FieldManagerName,
	}
	defaultUpdateOptions = client.UpdateOptions{
		FieldManager: FieldManagerName,
	}
	defaultDeleteOptions = client.DeleteOptions{}
)

// Clients wraps Kubernetes clients with common behaviors.
type Client struct {
	client    client.Client
	dynamic   dynamic.Interface
	discovery *discovery.DiscoveryClient
	mapper    *restmapper.DeferredDiscoveryRESTMapper
}

// NewClient returns a new Client.
func NewClient(client client.Client, dynamic dynamic.Interface, discovery *discovery.DiscoveryClient) Client {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discovery))
	return Client{
		client:    client,
		dynamic:   dynamic,
		discovery: discovery,
		mapper:    mapper,
	}
}

// GetClient returns the underlying client.Client
func (c Client) GetClient() client.Client {
	return c.client
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server.
func (c Client) Get(ctx reconcile.Context, key client.ObjectKey, obj client.Object) error {
	okt := ObjectKeyType(key, obj)
	c.logDebug(ctx, "fetching %s", okt)
	if err := c.client.Get(ctx.Context, key, obj); err != nil {
		if errors.IsNotFound(err) {
			c.logDebug(ctx, "%s not found", okt)
		} else {
			c.logError(ctx, err, "failed to fetch %s", okt)
		}
		return err
	}
	return nil
}

func (c Client) List(ctx reconcile.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.client.List(ctx.Context, list, opts...); err != nil {
		return err
	}
	return nil
}

// Create saves the object obj in the Kubernetes cluster.
func (c Client) Create(ctx reconcile.Context, obj client.Object) error {
	okt := ObjectKeyType(client.ObjectKeyFromObject(obj), obj)
	c.logDebug(ctx, "creating %s", okt)
	if err := c.client.Create(ctx.Context, obj, &defaultCreateOptions); err != nil {
		c.logError(ctx, err, "failed to create %s", okt)
		return err
	}
	return nil
}

// Update updates the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (c Client) Update(ctx reconcile.Context, obj client.Object) error {
	okt := ObjectKeyType(client.ObjectKeyFromObject(obj), obj)
	c.logDebug(ctx, "updating %s", okt)
	if err := c.client.Update(ctx.Context, obj, &defaultUpdateOptions); err != nil {
		c.logError(ctx, err, "failed to update %s", okt)
		return err
	}
	return nil
}

// Update updates the fields corresponding to the status subresource for the
// given obj. obj must be a struct pointer so that obj can be updated
// with the content returned by the Server.
func (c Client) UpdateStatus(ctx reconcile.Context, obj client.Object) error {
	okt := ObjectKeyType(client.ObjectKeyFromObject(obj), obj)
	c.logDebug(ctx, "updating %s status", okt)
	if err := c.client.Status().Update(ctx.Context, obj, &defaultUpdateOptions); err != nil {
		c.logError(ctx, err, "failed to update %s status", okt)
		return err
	}
	return nil
}

// Delete deletes the given obj from Kubernetes cluster.
func (c Client) Delete(ctx reconcile.Context, obj client.Object) error {
	okt := ObjectKeyType(client.ObjectKeyFromObject(obj), obj)
	c.logDebug(ctx, "deleting %s", okt)
	if err := c.client.Delete(ctx.Context, obj, &defaultDeleteOptions); err != nil {
		c.logError(ctx, err, "failed to delete %s", okt)
		return err
	}
	return nil
}

// DynamicGet returns an unstructured object given a namespaced name and group, version, kind.
func (c Client) DynamicGet(ctx reconcile.Context, namespacedName types.NamespacedName, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	ctx.Log.V(1).Info("fetching resource", "resource", namespacedName.String(), "gvk", gvk.String())

	// Find REST mapping
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		ctx.Log.Error(err, "failed to fetch resource, REST mapping error", "resource", namespacedName.String(), "gvk", gvk.String())
		return nil, err
	}

	// Get resource
	dri := c.dynamic.Resource(mapping.Resource).Namespace(namespacedName.Namespace)
	resource, err := dri.Get(ctx.Context, namespacedName.Name, metav1.GetOptions{})
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			ctx.Log.V(1).Info("resource not found", "resource", namespacedName.String(), "gvk", gvk.String())
			return nil, nil
		}

		ctx.Log.Error(err, "failed to fetch resource", "resource", namespacedName.String(), "gvk", gvk.String())
		return nil, err
	}

	return resource, nil
}

// DynamicApply performs a patch in a similar manner to kubectl apply.
func (c Client) DynamicApply(ctx reconcile.Context, namespacedName types.NamespacedName, manifest string) error {
	obj := &unstructured.Unstructured{}

	// Decode YAML into unstructured.Unstructured
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := dec.Decode([]byte(manifest), nil, obj)
	if err != nil {
		return err
	}

	// Find REST mapping
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	dri := c.dynamic.Resource(mapping.Resource).Namespace(namespacedName.Namespace)

	// Apply resource
	ctx.Log.V(1).Info("applying resource", "resource", namespacedName.String(), "gvk", obj.GroupVersionKind().String())

	data, err := json.Marshal(obj)
	if err != nil {
		ctx.Log.Error(err, "marshalling resource json failed", "resource", namespacedName.String(), "gvk", obj.GroupVersionKind().String())
		return err
	}

	force := true
	_, err = dri.Patch(ctx.Context, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: FieldManagerName,
		Force:        &force,
	})
	if err != nil {
		ctx.Log.Error(err, "failed to apply resource", "resource", namespacedName.String(), "gvk", obj.GroupVersionKind().String())
		return err
	}

	return nil
}

// DynamicDelete removes an object given a namespaced name and group, version, kind.
func (c Client) DynamicDelete(ctx reconcile.Context, namespacedName types.NamespacedName, gvk schema.GroupVersionKind) error {
	ctx.Log.V(1).Info("deleting resource", "resource", namespacedName.String(), "gvk", gvk.String())

	// Find REST mapping
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		ctx.Log.Error(err, "failed to delete resource, REST mapping error", "resource", namespacedName.String(), "gvk", gvk.String())
		return err
	}

	// Delete resource
	var dri dynamic.ResourceInterface

	if namespacedName.Namespace == "" {
		dri = c.dynamic.Resource(mapping.Resource)
	} else {
		dri = c.dynamic.Resource(mapping.Resource).Namespace(namespacedName.Namespace)
	}

	deletePolicy := metav1.DeletePropagationBackground
	err = dri.Delete(ctx.Context, namespacedName.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			ctx.Log.V(1).Info("resource not found", "resource", namespacedName.String(), "gvk", gvk.String())
			return nil
		}

		ctx.Log.Error(err, "failed to delete resource", "resource", namespacedName.String(), "gvk", gvk.String())
		return err
	}

	return nil
}

func (c Client) logDebug(ctx reconcile.Context, format string, okt objectKeyType) {
	ctx.Log.V(1).Info(fmt.Sprintf(format, okt.TypeName), "object", okt)
}

func (c Client) logInfo(ctx reconcile.Context, format string, okt objectKeyType) {
	ctx.Log.Info(fmt.Sprintf(format, okt.TypeName), "object", okt)
}

func (c Client) logError(ctx reconcile.Context, err error, format string, okt objectKeyType) {
	ctx.Log.Error(err, fmt.Sprintf(format, okt.TypeName), "object", okt)
}

func ObjectKeyType(key client.ObjectKey, obj client.Object) objectKeyType {
	t := reflectObjectType(obj)
	return objectKeyType{
		TypeName: t.Name(),
		Type:     t.String(),
		Key:      key.String(),
	}
}

type objectKeyType struct {
	TypeName string `json:"-"`
	Type     string `json:"type"`
	Key      string `json:"key"`
}

func reflectObjectType(obj client.Object) reflect.Type {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}
