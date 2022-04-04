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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	ConnectorTypeAlbIngressController ConnectorType = "alb.ingress.kubernetes.io"
)

// CertificateConnectorSpec defines the desired state of CertificateConnector
type CertificateConnectorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ingress describes the ingress resources to update.
	// +kubebuilder:validation:Optional
	Ingress *IngressSpec `json:"ingress,omitempty"`

	// Certificates describes the certificates to connect.
	// +kubebuilder:validation:Required
	Certificates CertificatesSpec `json:"certificates,omitempty"`
}

// IngressSpec defines the Ingress resource connection
type IngressSpec struct {
	// Connector defines the type of connector to use.
	// +kubebuilder:validation:Required
	Connector ConnectorType `json:"connector"`

	// Selector specifies the selectors that need to match on a resource.
	// +kubebuilder:validation:Required
	Selector SelectorSpec `json:"selector"`
}

// Connector defines a type of connector
type ConnectorType string

// CertificatesSpec defines the Certificate resource selection
type CertificatesSpec struct {
	// Selector specifies the selectors that need to match on a resource.
	// +kubebuilder:validation:Required
	Selector SelectorSpec `json:"selector"`
}

// SelectorSpec defines the selectors that need to match on a resource
type SelectorSpec struct {
	// Namespaces specifies the namespaces to include resources from.
	// +kubebuilder:validation:Required
	Namespaces []string `json:"namespaces"`

	// Labels specifies the required labels on a resource.
	// +kubebuilder:validation:Required
	Labels map[string]string `json:"labels"`

	// Annotations specifies the required annotations on a resource.
	// +kubebuilder:validation:Required
	Annotations map[string]string `json:"annotations"`
}

// CertificateConnectorStatus defines the observed state of CertificateConnector
type CertificateConnectorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions represent the latest available observations of the ResourceSet state.
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="LastUpdated",priority=1,type="string",JSONPath=`.status.conditions[?(@.type == "InSync")].lastTransitionTime`
//+kubebuilder:printcolumn:name="InSync",priority=1,type="string",JSONPath=`.status.conditions[?(@.type == "InSync")].status`

// CertificateConnector is the Schema for the certificateconnectors API
type CertificateConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateConnectorSpec   `json:"spec,omitempty"`
	Status CertificateConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CertificateConnectorList contains a list of CertificateConnector
type CertificateConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CertificateConnector `json:"items"`
}

// NamespacedName returns a namespaced name for the custom resource
func (cc CertificateConnector) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: cc.Namespace,
		Name:      cc.Name,
	}
}

// Match returns true when the item matches the selector annotations and namespaces
func (selector SelectorSpec) Match(item metav1.ObjectMeta) bool {
	matchesNamespace := true
	if len(selector.Namespaces) > 1 {
		matchesNamespace = false
		for _, namespace := range selector.Namespaces {
			if namespace == item.Namespace {
				matchesNamespace = true
			}
		}
	}

	if !matchesNamespace {
		return false
	}

	if len(selector.Annotations) == 0 {
		return true
	}

	for key, value := range selector.Annotations {
		if v, ok := item.Annotations[key]; ok {
			if v != value {
				continue
			}

			return true
		}
	}

	return false
}

// ListOptions returns client.ListOptions bases on the selector
func (selector SelectorSpec) ListOptions() *client.ListOptions {
	options := client.ListOptions{}
	options.LabelSelector = labels.SelectorFromSet(selector.Labels)

	if len(selector.Namespaces) == 1 { // Single namespace
		options.Namespace = selector.Namespaces[0]
	}

	return &options
}

func init() {
	SchemeBuilder.Register(&CertificateConnector{}, &CertificateConnectorList{})
}
