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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

/*
type ProfileNode struct {
	NodeName string `json:"name,omitempty"`

	Max int `json:"max,omitempty"`

	Min int `json:"min,omitempty"`
}
*/

// PowerProfileSpec defines the desired state of PowerProfile
type PowerProfileSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the PowerProfile
	Name string `json:"name"`

	//ProfileNodes []ProfileNode `json:"profileNodes,omitempty"`
	Max int `json:"max,omitempty"`

	Min int `json:"min,omitempty"`

	// The priority value associated with this Power Profile
	Epp string `json:"epp"`
}

// PowerProfileStatus defines the observed state of PowerProfile
type PowerProfileStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The ID given to the power profile
	ID int `json:"id"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PowerProfile is the Schema for the powerprofiles API
type PowerProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PowerProfileSpec   `json:"spec,omitempty"`
	Status PowerProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PowerProfileList contains a list of PowerProfile
type PowerProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PowerProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PowerProfile{}, &PowerProfileList{})
}
