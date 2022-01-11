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

// BlueprintSpec defines the desired state of Blueprint
type BlueprintSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ResourceNamePrefix defines the prefix to use when naming resources
	// +kubebuilder:validation:Required
	ResourceNamePrefix string `json:"resourceNamePrefix"`

	// Resources defines the resources groups used when generating tenant resource sets
	// +kubebuilder:validation:Required
	Resources []BlueprintResourceGroup `json:"resources"`
}

// BlueprintResourceGroup defines a group of resources used when generating tenant resource sets
type BlueprintResourceGroup struct {
	// Name defines the name of the resource group
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Template defines the namespace/name of the template used to generate resources
	// +kubebuilder:validation:Required
	Template string `json:"template"`

	// Parameters defines the parameters that applies to the template
	// +kubebuilder:validation:Optional
	Parameters []ParameterValue `json:"parameters,omitempty"`
}

// BlueprintStatus defines the observed state of Blueprint
type BlueprintStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Blueprint is the Schema for the blueprints API
type Blueprint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BlueprintSpec   `json:"spec,omitempty"`
	Status BlueprintStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BlueprintList contains a list of Blueprint
type BlueprintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Blueprint `json:"items"`
}

// CommonLabels merges labels from tenant and blueprint with default labels to create a common set
func (b Blueprint) CommonLabels(tenant Tenant) map[string]string {
	labels := map[string]string{}

	for k, v := range tenant.Labels {
		labels[k] = v
	}

	for k, v := range b.Labels {
		labels[k] = v
	}

	labels["aeto.net/tenant"] = tenant.Name

	return labels
}

// CommonAnnotations merges annotations from tenant and blueprint with default annotations to create a common set
func (b Blueprint) CommonAnnotations(tenant Tenant) map[string]string {
	annotations := map[string]string{}

	for k, v := range tenant.Annotations {
		annotations[k] = v
	}

	for k, v := range b.Annotations {
		annotations[k] = v
	}

	annotations["aeto.net/controlled"] = "true"

	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")

	return annotations
}

// NamespacedName returns a namespaced name for the custom resource
func (b Blueprint) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: b.Namespace,
		Name:      b.Name,
	}
}

func init() {
	SchemeBuilder.Register(&Blueprint{}, &BlueprintList{})
}
