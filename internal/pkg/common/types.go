package common

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type ResourceIdentifier struct {
	NamespacedName   types.NamespacedName
	GroupVersionKind schema.GroupVersionKind
}
