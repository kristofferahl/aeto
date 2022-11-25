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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	SavingsPolicyTerminating string = "Terminating"
	SavingsPolicyError       string = "Error"
)

const (
	ConditionTypeSuspended string = "Suspended"
)

// SavingsPolicySpec defines the desired state of SavingsPolicy
type SavingsPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Suspended contains a list of day and time entries for when the SavingsPolicy is suspended.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Suspended []string `json:"suspended"`

	// Targets contains a list of kubenetes resource selectors that the SavingsPolicy applies to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Targets []SavingsPolicyTarget `json:"targets"`
}

// SavingsPolicyTarget defines the a target for a SavingsPolicy
type SavingsPolicyTarget struct {
	// +kubebuilder:validation:Required
	ApiVersion string `json:"apiVersion"`

	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Optional
	Ignore bool `json:"ignore,omitempty"`
}

// SavingsPolicyStatus defines the observed state of SavingsPolicy
type SavingsPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Status is the current status of the AWS ACM Certificate.
	Status string `json:"status,omitempty"`

	// Conditions represent the latest available observations of the ResourceSet state.
	Conditions []metav1.Condition `json:"conditions"`

	// ActiveDuration is the amount of time the SavingsPolicy has been active
	ActiveDuration string `json:"activeDuration,omitempty"`

	// SuspendedDuration is the amount of time the SavingsPolicy has been suspended
	SuspendedDuration string `json:"suspendedDuration,omitempty"`

	// Savings is the percent of time the SavingsPolicy has been active
	Savings string `json:"savings,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",priority=0,type=string,JSONPath=`.status.status`
//+kubebuilder:printcolumn:name="Suspended",priority=0,type="string",JSONPath=`.status.conditions[?(@.type == "Suspended")].status`
//+kubebuilder:printcolumn:name="LastTransition",priority=1,type="string",JSONPath=`.status.conditions[?(@.type == "Suspended")].lastTransitionTime`
//+kubebuilder:printcolumn:name="ActiveDuration",priority=1,type="string",JSONPath=`.status.activeDuration`
//+kubebuilder:printcolumn:name="SuspendedDuration",priority=1,type="string",JSONPath=`.status.suspendedDuration`
//+kubebuilder:printcolumn:name="Savings",priority=0,type="string",JSONPath=`.status.savings`

// SavingsPolicy is the Schema for the savingspolicies API
type SavingsPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SavingsPolicySpec   `json:"spec,omitempty"`
	Status SavingsPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SavingsPolicyList contains a list of SavingsPolicy
type SavingsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SavingsPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SavingsPolicy{}, &SavingsPolicyList{})
}
