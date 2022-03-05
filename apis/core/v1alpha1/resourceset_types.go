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

	// ResourceSetPaused means the resource set reconciliation has been paused
	ResourceSetPaused ResourceSetPhase = "Paused"
)

// ResourceSetSpec defines the desired state of ResourceSet
type ResourceSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Active defines the state of the ResourceSet. When active, desired state will be reconciled and deleting the ResourceSet will cause cleanup of resources defined by the ResourceSet. There should only ever be a single active ResourceSet per tenant.
	Active bool `json:"active"`

	// Resources contains embedded resources
	// +kubebuilder:validation:Required
	Resources ResourceSetResourceList `json:"resources"`
}

// ResourceSetResourceList defines a list of resources in a ResourceSet
type ResourceSetResourceList []ResourceSetResource

// ResourceSetResource defines a resource in a ResourceSet
type ResourceSetResource struct {
	// Id holds the unique identifier of the resource
	// +kubebuilder:validation:Required
	Id string `json:"id"`

	// Order holds the desired order in which the resource should be applied
	// +kubebuilder:validation:Required
	Order int `json:"order"`

	// Embedded holds an embedded kubernetes resource
	// +kubebuilder:validation:Required
	Embedded EmbeddedResource `json:"embedded"`
}

// ResourceSetStatus defines the observed state of ResourceSet
type ResourceSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase is the current lifecycle phase of the ResourceSet.
	Phase ResourceSetPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the ResourceSet state.
	Conditions []metav1.Condition `json:"conditions"`

	// ObservedGeneration is the last reconciled generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ResourceVersion is the last reconciled resource version.
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

type ResourceSetPhase string

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Active",priority=0,type=string,JSONPath=`.status.conditions[?(@.type == "Active")].status`
//+kubebuilder:printcolumn:name="Phase",priority=0,type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Ready",priority=0,type=string,JSONPath=`.status.conditions[?(@.type == "Ready")].message`
//+kubebuilder:printcolumn:name="ObservedGeneration",priority=1,type=string,JSONPath=`.status.observedGeneration`
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

// NamespacedName returns a namespaced name for the custom resource
func (rs ResourceSet) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: rs.Namespace,
		Name:      rs.Name,
	}
}

// Find returns the first matching resource and it's index in the list matching the specified id
func (rl ResourceSetResourceList) Find(id string) (index int, existing *ResourceSetResource) {
	index = -1
	for i, r := range rl {
		idEqual := r.Id == id
		if idEqual {
			existing = &r
			index = i
			break
		}
	}
	return
}

func init() {
	SchemeBuilder.Register(&ResourceSet{}, &ResourceSetList{})
}
