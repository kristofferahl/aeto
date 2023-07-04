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
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EventStreamChunkSpec defines the desired state of EventStreamChunk
type EventStreamChunkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// StreamId defines the ID of the stream
	StreamId string `json:"id"`

	// StreamVersion is the version of the stream at the point when it chunk was created
	StreamVersion int64 `json:"version"`

	// Timestamp is point in time when the chunk was created
	Timestamp string `json:"ts"`

	// Events holds the events of the stream chunk
	Events []EventRecord `json:"events"`
}

// EventRecord defines an event
type EventRecord struct {
	// Raw defines the raw data of the event
	Raw string `json:"raw"`
}

// EventStreamChunkStatus defines the observed state of EventStreamChunk
type EventStreamChunkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Id",priority=0,type=string,JSONPath=`.spec.id`
//+kubebuilder:printcolumn:name="Version",priority=0,type=string,JSONPath=`.spec.version`
//+kubebuilder:printcolumn:name="Timestamp",priority=1,type=string,JSONPath=`.spec.ts`

// EventStreamChunk is the Schema for the eventstreamchunks API
type EventStreamChunk struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EventStreamChunkSpec   `json:"spec,omitempty"`
	Status EventStreamChunkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EventStreamChunkList contains a list of EventStreamChunk
type EventStreamChunkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventStreamChunk `json:"items"`
}

func (l EventStreamChunkList) Sort() []EventStreamChunk {
	sort.Slice(l.Items[:], func(i, j int) bool {
		return l.Items[i].Spec.StreamVersion < l.Items[j].Spec.StreamVersion
	})
	return l.Items
}

// NamespacedName returns a namespaced name for the custom resource
func (esc EventStreamChunk) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: esc.Namespace,
		Name:      esc.Name,
	}
}

func init() {
	SchemeBuilder.Register(&EventStreamChunk{}, &EventStreamChunkList{})
}
