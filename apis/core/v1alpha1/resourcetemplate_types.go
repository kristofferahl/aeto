/*
Copyright 2022.

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

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	ResourceNameKeep   ResourceNameRule = "keep"
	ResourceNameTenant ResourceNameRule = "tenant"

	ResourceNamespaceKeep     ResourceNamespaceRule = "keep"
	ResourceNamespaceTenant   ResourceNamespaceRule = "tenant"
	ResourceNamespaceOperator ResourceNamespaceRule = "operator"
)

// ResourceTemplateSpec defines the desired state of ResourceTemplate
type ResourceTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Rules contains embedded resources in go templating format
	// +kubebuilder:validation:Required
	Rules ResourceTemplateRules `json:"rules"`

	// Parameters contains parameters used for templating
	// +kubebuilder:validation:Required
	Parameters ResourceTemplateParameterList `json:"parameters"`

	// Resources contains embedded resources in go templating format
	// +kubebuilder:validation:Optional
	Resources []EmbeddedResource `json:"resources,omitempty"`

	// Raw contains raw yaml documents in go templating format (prefer using Manifests over Raw)
	// +kubebuilder:validation:Optional
	Raw []string `json:"raw,omitempty"`
}

type ResourceTemplateParameterList []*Parameter

// ResourceTemplateRules defines rules of the ResourceTemplate
type ResourceTemplateRules struct {
	// Name defines the naming rule to apply for the resources in the ResourceTemplate
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=tenant
	Name ResourceNameRule `json:"name"`

	// Namespace defines the namespace source to use for the resources in the ResourceTemplate
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=tenant
	Namespace ResourceNamespaceRule `json:"namespace"`
}

type ResourceNamespaceRule string

type ResourceNameRule string

// ResourceTemplateStatus defines the observed state of ResourceTemplate
type ResourceTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ResourceTemplate is the Schema for the resourcetemplates API
type ResourceTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceTemplateSpec   `json:"spec,omitempty"`
	Status ResourceTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceTemplateList contains a list of ResourceTemplate
type ResourceTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceTemplate `json:"items"`
}

// SetValues applies parameter values to the resource template parameter list
func (rtp ResourceTemplateParameterList) SetValues(pvs []ParameterValue, resolverFunc func(ValueRef) (string, error)) (err error) {
	for _, p := range rtp {
		for _, pv := range pvs {
			if p.Name == pv.Name {
				p.value = pv.Value
				if p.value == "" && pv.ValueFrom != nil {
					val, err := resolverFunc(*pv.ValueFrom)
					if err != nil {
						return err
					}
					p.value = val
				}
			}
		}
	}
	return nil
}

// Validate returns an error if the resource template parameter list contains validation errors
func (rtp ResourceTemplateParameterList) Validate() (err error) {
	for _, p := range rtp {
		if _, e := p.Value(); e != nil {
			if err == nil {
				err = e
				continue
			}
			err = fmt.Errorf("%v; %v", err, e)
		}
	}

	if err != nil {
		err = fmt.Errorf("validation failed: %v", err)
	}
	return err
}

// NamespacedName returns a namespaced name for the custom resource
func (rt ResourceTemplate) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: rt.Namespace,
		Name:      rt.Name,
	}
}

func init() {
	SchemeBuilder.Register(&ResourceTemplate{}, &ResourceTemplateList{})
}
