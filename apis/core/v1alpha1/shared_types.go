package v1alpha1

import (
	"fmt"

	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EmbeddedResource holds a kubernetes resource
// +kubebuilder:validation:XPreserveUnknownFields
// +kubebuilder:validation:XEmbeddedResource
type EmbeddedResource struct {
	runtime.RawExtension `json:",inline"`
}

// Parameter defines a template parameter
type Parameter struct {
	// Name defines the name of the parameter
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Required make the parameter required
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	Required bool `json:"required,omitempty"`

	// Default holds the default value for the parameter
	// +kubebuilder:validation:Optional
	Default string `json:"default,omitempty"`

	// Value holds the value of the parameter
	value string `json:"-"`
}

// ParameterValue defines a template parameter
type ParameterValue struct {
	// Name defines the name of the parameter
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Value holds a value for the parameter
	// +kubebuilder:validation:Optional
	Value string `json:"value,omitempty"`

	// ValueFrom holds a value for the parameter
	// +kubebuilder:validation:Optional
	ValueFrom *ValueRef `json:"valueFrom,omitempty"`
}

// ValueRef defines a reference to a value
type ValueRef struct {
	// Blueprint defines a reference to a value from a blueprint resource group
	// +kubebuilder:validation:Optional
	Blueprint *BlueprintValueRef `json:"blueprint,omitempty"`

	// Resource defines a reference to a value from a kubernetes resource
	// +kubebuilder:validation:Optional
	Resource *ResourceValueRef `json:"resource,omitempty"`
}

// BlueprintValueRef defines a reference to a value in a blueprint resource group
type BlueprintValueRef struct {
	// ResourceGroup defines the resource group
	// +kubebuilder:validation:Required
	ResourceGroup string `json:"resourceGroup"`

	// JsonPath holds a path expression for the desired value
	// +kubebuilder:validation:Required
	JsonPath string `json:"jsonPath"`
}

// ResourceValueRef defines a reference to a value in a kubernetes resource
type ResourceValueRef struct {
	// ApiVersion defines the api version of the kubernetes resource
	// +kubebuilder:validation:Required
	ApiVersion string `json:"apiVersion"`

	// Kind defines the kind of the kubernetes resource
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name defines the name of the kubernetes resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace defines the namespace of the kubernetes resource
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// JsonPath holds a path expression for the desired value
	// +kubebuilder:validation:Required
	JsonPath string `json:"jsonPath"`
}

// Value returns the value of a named string parameter
func (p Parameter) Value() (string, error) {
	if p.value != "" {
		return p.value, nil
	}

	if p.Default != "" {
		return p.Default, nil
	}

	if p.Required {
		return "", fmt.Errorf("required parameter %s has no value", p.Name)
	}

	return "", nil
}
