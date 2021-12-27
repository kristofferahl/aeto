package v1alpha1

import runtime "k8s.io/apimachinery/pkg/runtime"

// EmbeddedResource holds a kubernetes resource
// +kubebuilder:validation:XPreserveUnknownFields
// +kubebuilder:validation:XEmbeddedResource
type EmbeddedResource struct {
	runtime.RawExtension `json:",inline"`
}
