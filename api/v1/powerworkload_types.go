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

type WorkloadNode struct {
	Name string `json:"name,omitempty"`

	Containers []Container `json:"containers,omitempty"`

	CpuIds []uint `json:"cpuIds,omitempty"`
}

// PowerWorkloadSpec defines the desired state of PowerWorkload
type PowerWorkloadSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the workload
	Name string `json:"name"`

	// AllCores determines if the Workload is to be applied to all cores (i.e. use the Default Workload)
	AllCores bool `json:"allCores,omitempty"`

	// Reserved CPUs are the CPUs that have been reserved by Kubelet for use by the Kubernetes admin process
	// This list must match the list in the user's Kubelet configuration
	ReservedCPUs []ReservedSpec `json:"reservedCPUs,omitempty"`

	// The labels signifying the nodes the user wants to use
	PowerNodeSelector map[string]string `json:"powerNodeSelector,omitempty"`

	// Holds the info on the node name and cpu ids for each node
	//Node NodeInfo `json:"nodeInfo,omitempty"`

	Node WorkloadNode `json:"workloadNodes,omitempty"`

	// PowerProfile is the Profile that this PowerWorkload is based on
	PowerProfile string `json:"powerProfile,omitempty"`
}

type ReservedSpec struct {
	Cores        []uint `json:"cores"`
	PowerProfile string `json:"powerProfile,omitempty"`
}

// PowerWorkloadStatus defines the observed state of PowerWorkload
type PowerWorkloadStatus struct {
	// The Node that this Shared PowerWorkload is associated with
	Node         string `json:"node:,omitempty"`
	StatusErrors `json:",inline,omitempty"`
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

func (wrkld *PowerWorkload) SetStatusErrors(errs *[]string) {
	wrkld.Status.Errors = *errs
}

func (wrkld *PowerWorkload) GetStatusErrors() *[]string {
	return &wrkld.Status.Errors
}

func init() {
	SchemeBuilder.Register(&PowerWorkload{}, &PowerWorkloadList{})
}
