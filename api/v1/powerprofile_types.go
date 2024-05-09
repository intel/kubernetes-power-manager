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

// PowerProfileSpec defines the desired state of PowerProfile
type PowerProfileSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the PowerProfile
	Name string `json:"name"`

	Shared bool `json:"shared,omitempty"`
	// Max frequency cores can run at
	Max int `json:"max,omitempty"`

	// Min frequency cores can run at
	Min int `json:"min,omitempty"`

	// The priority value associated with this Power Profile
	Epp string `json:"epp,omitempty"`

	// Governor to be used
	//+kubebuilder:default=powersave
	Governor string `json:"governor,omitempty"`
}

// PowerProfileStatus defines the observed state of PowerProfile
type PowerProfileStatus struct {
	// The ID given to the power profile
	ID           int `json:"id,omitempty"`
	StatusErrors `json:",inline,omitempty"`
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

func (prfl *PowerProfile) SetStatusErrors(errs *[]string) {
	prfl.Status.Errors = *errs
}
func (prfl *PowerProfile) GetStatusErrors() *[]string {
	return &prfl.Status.Errors
}

func init() {
	SchemeBuilder.Register(&PowerProfile{}, &PowerProfileList{})
}
