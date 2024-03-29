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

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the full name of the tenant
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Blueprint contains the name of the Blueprint to use for the tenant
	// +kubebuilder:validation:Optional
	Blueprint string `json:"blueprint,omitempty"`
}

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Namespace is the namespace for the Tenant.
	Namespace string `json:"namespace,omitempty"`

	// Blueprint is the namespace/name of the Blueprint in use by the Tenant.
	Blueprint string `json:"blueprint,omitempty"`

	// ResourceSet is the the namespace/name of the ResourceSet in use by the Tenant.
	ResourceSet string `json:"resourceSet,omitempty"`

	// Events is the number of events produced for the Tenant.
	Events int `json:"events,omitempty"`

	// Status is the current lifecycle phase of the Tenant.
	Status string `json:"status,omitempty"`

	// Conditions represent the latest available observations of the Tenants state.
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Tenant",priority=0,type="string",JSONPath=".spec.name",description="The display name of the tenant"
//+kubebuilder:printcolumn:name="Namespace",priority=0,type="string",JSONPath=".status.namespace",description="Tenant namespace"
//+kubebuilder:printcolumn:name="Blueprint",priority=1,type="string",JSONPath=".status.blueprint",description="Blueprint name"
//+kubebuilder:printcolumn:name="ResourceSet",priority=1,type="string",JSONPath=".status.resourceSet",description="ResourceSet name"
//+kubebuilder:printcolumn:name="Events",priority=1,type="string",JSONPath=".status.events",description="Events produced for tenant"
//+kubebuilder:printcolumn:name="Status",priority=0,type="string",JSONPath=".status.status",description="Tenant lifecycle phase"
//+kubebuilder:printcolumn:name="Ready",priority=0,type="string",JSONPath=`.status.conditions[?(@.type == "Ready")].status`,description="Tenant ready"

// Tenant is the Schema for the tenants API
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

// Blueprint returns the name of the blueprint to use for generating tenant resources
func (t *Tenant) Blueprint() string {
	if t.Spec.Blueprint != "" {
		return t.Spec.Blueprint
	}
	return "default"
}

// NamespacedName returns a namespaced name for the custom resource
func (t Tenant) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: t.Namespace,
		Name:      t.Name,
	}
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
