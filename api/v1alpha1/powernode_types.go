/*


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

// PowerNodeSpec defines the desired state of PowerNode
type PowerNodeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// NodeName is the name of the node
	NodeName string `json:"nodeName,omitempty"`
}

// PowerNodeStatus defines the observed state of PowerNode
type PowerNodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SharedPool     []string          `json:"sharedPool,omitempty"`
	GuaranteedPods []GuaranteedPod `json:"guaranteedPods,omitempty"`
}

type GuaranteedPod struct {
	Name       string      `json:"name,omitempty"`
	UID        string      `json:"uid,omitempty"`
	Containers []Container `json:"containers,omitempty"`
	Profile    Profile     `json:"profile,omitempty"`
}

type Container struct {
	Name          string   `json:"name,omitempty"`
	ID            string   `json:"id,omitempty"`
	ExclusiveCPUs []string `json:"exclusiveCpus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PowerNode is the Schema for the powernodes API
type PowerNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PowerNodeSpec   `json:"spec,omitempty"`
	Status PowerNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PowerNodeList contains a list of PowerNode
type PowerNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PowerNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PowerNode{}, &PowerNodeList{})
}
