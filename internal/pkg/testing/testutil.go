package testing

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func MustConvert(obj runtime.Object) []byte {
	setType(obj)

	o, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return o
}

func setType(obj runtime.Object) runtime.Object {
	gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
	if err != nil {
		panic(err)
	}

	// Set the type correctly because we are to lazy to set it in the test
	accessor, err := meta.TypeAccessor(obj)
	if err != nil {
		panic(err)
	}
	accessor.SetAPIVersion(gvk.GroupVersion().String())
	accessor.SetKind(gvk.Kind)

	return obj
}
