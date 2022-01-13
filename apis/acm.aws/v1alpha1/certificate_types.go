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

// CertificateSpec defines the desired state of Certificate
type CertificateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// DomainName is the fully qualified domain name (fqdn) used to create the AWS ACM Certificate.
	// +kubebuilder:validation:Required
	DomainName string `json:"domainName"`

	// Tags defines the tags to apply to the resource.
	// +kubebuilder:validation:Optional
	Tags map[string]string `json:"tags,omitempty"`

	// Validation defines the certificate validation strategy to use.
	// +kubebuilder:validation:Optional
	Validation *CertificateValidation `json:"validation,omitempty"`
}

// CertificateValidation defines the certificate validation strategy
type CertificateValidation struct {
	// Dns defines the dns certificate validation strategy
	// +kubebuilder:validation:Optional
	Dns *CertificateDnsValidation `json:"dns,omitempty"`
}

// CertificateDnsValidation defines the DNS validation strategy
type CertificateDnsValidation struct {
	// HostedZoneId defines the id of the hosted zone to put DNS validation records in
	// +kubebuilder:validation:Required
	HostedZonedId string `json:"hostedZoneId"`
}

// CertificateStatus defines the observed state of Certificate
type CertificateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Arn is the ARN of the AWS ACM Certificate.
	Arn string `json:"arn,omitempty"`

	// State is the current status of the AWS ACM Certificate.
	State string `json:"state,omitempty"`

	// InUse declares if the AWS ACM Certificate is in use.
	InUse bool `json:"inUse,omitempty"`

	// Ready is true when the resource is created and valid
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="DomainName",priority=0,type=string,JSONPath=`.spec.domainName`
//+kubebuilder:printcolumn:name="State",priority=1,type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="InUse",priority=1,type=boolean,JSONPath=`.status.inUse`
//+kubebuilder:printcolumn:name="Arn",priority=1,type=string,JSONPath=`.status.arn`
//+kubebuilder:printcolumn:name="Ready",priority=0,type=boolean,JSONPath=`.status.ready`

// Certificate is the Schema for the certificates API
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateSpec   `json:"spec,omitempty"`
	Status CertificateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CertificateList contains a list of Certificate
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

// NamespacedName returns a namespaced name for the custom resource
func (c Certificate) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

func init() {
	SchemeBuilder.Register(&Certificate{}, &CertificateList{})
}
