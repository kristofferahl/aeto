package core

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	namespaceGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}
)
