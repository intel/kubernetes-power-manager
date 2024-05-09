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

// PowerConfigSpec defines the desired state of PowerConfig
type PowerConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The label on the Nodes you the Operator will look for to deploy the Node Agent
	PowerNodeSelector map[string]string `json:"powerNodeSelector,omitempty"`

	// The PowerProfiles that will be created by the Operator
	PowerProfiles []string `json:"powerProfiles,omitempty"`

	// The CustomDevices include alternative devices that represent other resources
	CustomDevices []string `json:"customDevices,omitempty"`
}

// PowerConfigStatus defines the observed state of PowerConfig
type PowerConfigStatus struct {
	// The Nodes that the Node Agent has been deployed to
	Nodes        []string `json:"nodes,omitempty"`
	StatusErrors `json:",inline,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PowerConfig is the Schema for the powerconfigs API
type PowerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PowerConfigSpec   `json:"spec,omitempty"`
	Status PowerConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PowerConfigList contains a list of PowerConfig
type PowerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PowerConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PowerConfig{}, &PowerConfigList{})
}
