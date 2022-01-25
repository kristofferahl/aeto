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
	"encoding/json"
	"fmt"

	"github.com/PaesslerAG/jsonpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// These are the valid phases of a namespace.
const (
	// ResourceSetReconciling means the resource set is reconciling
	ResourceSetReconciling ResourceSetPhase = "Reconciling"

	// ResourceSetTerminating means the resource set is undergoing graceful termination
	ResourceSetTerminating ResourceSetPhase = "Terminating"
)

// ResourceSetSpec defines the desired state of ResourceSet
type ResourceSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Groups contains grouped resources
	// +kubebuilder:validation:Required
	Groups []ResourceSetResourceGroup `json:"groups"`
}

// ResourceSetResourceGroup defines a grouped set of resources, generated from a resource template
type ResourceSetResourceGroup struct {
	// Name holds the name of the resource group
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// SourceTemplate contains a reference to the template used to genererate the resource group
	// +kubebuilder:validation:Required
	SourceTemplate string `json:"sourceTemplate"`

	// Resources contains embedded resources
	// +kubebuilder:validation:Required
	Resources []EmbeddedResource `json:"resources"`
}

// ResourceSetStatus defines the observed state of ResourceSet
type ResourceSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase is the current lifecycle phase of the resource set.
	Phase ResourceSetPhase `json:"phase,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ResourceVersion is the last reconciled resource version.
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

type ResourceSetPhase string

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",priority=0,type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Generation",priority=1,type=string,JSONPath=`.status.observedGeneration`
//+kubebuilder:printcolumn:name="ResourceVersion",priority=1,type=string,JSONPath=`.status.resourceVersion`

// ResourceSet is the Schema for the resourcesets API
type ResourceSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSetSpec   `json:"spec,omitempty"`
	Status ResourceSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceSetList contains a list of ResourceSet
type ResourceSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceSet `json:"items"`
}

// Group returns a resource group matching the specified name
func (rs ResourceSet) Group(name string) (*ResourceSetResourceGroup, error) {
	for _, group := range rs.Spec.Groups {
		if group.Name == name {
			return &group, nil
		}
	}
	return nil, fmt.Errorf("resource group %s not found", name)
}

// JsonPath returns a value from the resource group JSON representation using the given path
func (g *ResourceSetResourceGroup) JsonPath(path string) (string, error) {
	bytes, err := json.Marshal(g)
	if err != nil {
		return "", err
	}

	v := interface{}(nil)
	json.Unmarshal(bytes, &v)

	value, err := jsonpath.Get(fmt.Sprintf("$%s", path), v)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s", value), nil
}

// NamespacedName returns a namespaced name for the custom resource
func (rs ResourceSet) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: rs.Namespace,
		Name:      rs.Name,
	}
}

func init() {
	SchemeBuilder.Register(&ResourceSet{}, &ResourceSetList{})
}
