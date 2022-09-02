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

const (
	ConditionTypeReady string = "Ready"

	HostedZoneDeletionPolicyDefault HostedZoneDeletionPolicy = "default"
	HostedZoneDeletionPolicyForce   HostedZoneDeletionPolicy = "force"
)

type HostedZoneDeletionPolicy string

// HostedZoneSpec defines the desired state of HostedZone
type HostedZoneSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the desired name for the AWS Route53 HostedZone.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Tags defines the tags to apply to the resource.
	// +kubebuilder:validation:Optional
	Tags map[string]string `json:"tags,omitempty"`

	// ConnectWith tells the operator to connect the HostedZone with a parent hosted zone by upserting it's NS recordset.
	// +kubebuilder:validation:Optional
	ConnectWith *HostedZoneConnection `json:"connectWith,omitempty"`

	// DeletionPolicy defines the strategy to use when a hosted zone is deleted
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=default
	DeletionPolicy HostedZoneDeletionPolicy `json:"deletionPolicy"`
}

// HostedZoneConnection defines the connection details for the HostedZone
type HostedZoneConnection struct {
	// Name of the AWS Route53 HostedZone to connect with.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// TTL used for the NS recordset created in the specified AWS Route53 HostedZone.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=172800
	TTL int64 `json:"ttl,omitempty"`
}

// HostedZoneStatus defines the observed state of HostedZone
type HostedZoneStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Id string `json:"id,omitempty"`

	Status string `json:"status,omitempty"`

	ConnectedTo string `json:"connectedTo,omitempty"`

	RecordSets *int64 `json:"recordsets,omitempty"`

	// Conditions represent the latest available observations of the ResourceSet state.
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="HostedZone",priority=0,type=string,JSONPath=`.spec.name`
//+kubebuilder:printcolumn:name="Id",priority=1,type=string,JSONPath=`.status.id`
//+kubebuilder:printcolumn:name="Status",priority=1,type=string,JSONPath=`.status.status`
//+kubebuilder:printcolumn:name="ConnectedTo",priority=1,type=string,JSONPath=`.status.connectedTo`
//+kubebuilder:printcolumn:name="RecordSets",priority=1,type=integer,JSONPath=`.status.recordsets`
//+kubebuilder:printcolumn:name="Ready",priority=0,type="string",JSONPath=`.status.conditions[?(@.type == "Ready")].status`

// HostedZone is the Schema for the hostedzones API
type HostedZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostedZoneSpec   `json:"spec,omitempty"`
	Status HostedZoneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HostedZoneList contains a list of HostedZone
type HostedZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostedZone `json:"items"`
}

// NamespacedName returns a namespaced name for the custom resource
func (hz HostedZone) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: hz.Namespace,
		Name:      hz.Name,
	}
}

func init() {
	SchemeBuilder.Register(&HostedZone{}, &HostedZoneList{})
}
