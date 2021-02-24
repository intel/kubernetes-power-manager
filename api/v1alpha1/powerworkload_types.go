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

// PowerWorkloadSpec defines the desired state of PowerWorkload
type PowerWorkloadSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// AllCores determines if the Workload is to be applied to all cores (i.e. use the Default Workload)
	AllCores bool `json:"allCores,omitempty"`

	ReservedCPUs []int `json:"reservedCPUs,omitempty"`

	PowerNodeSelector map[string]string `json:"powerNodeSelector,omitempty"`

	// Nodes indicates the nodes with Pods using this PowerWorload
	Nodes []string `json:"nodes,omitempty"`

	// CpuIds indicates the CPUs affected by this PowerWorload, across all nodes
	CpuIds []int `json:"cpuids,omitempty"`

	// PowerProfile is the Profile that this PowerWorkload is based on
	//PowerProfile `json:"powerprofile,omitempty"`
	PowerProfile int `json:"powerProfile,omitempty"`
}

// PowerWorkloadStatus defines the observed state of PowerWorkload
type PowerWorkloadStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PowerWorkload is the Schema for the powerworkloads API
type PowerWorkload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PowerWorkloadSpec   `json:"spec,omitempty"`
	Status PowerWorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PowerWorkloadList contains a list of PowerWorkload
type PowerWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PowerWorkload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PowerWorkload{}, &PowerWorkloadList{})
}
