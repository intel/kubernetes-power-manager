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

// PowerNodeSpec defines the desired state of PowerNode
type PowerNodeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the node
	NodeName string `json:"nodeName,omitempty"`

	PowerProfiles []string `json:"powerProfiles,omitempty"`

	PowerWorkloads []string `json:"powerWorkloads,omitempty"`

	SharedPool string `json:"sharedPool,omitempty"`

	UnaffectedCores string `json:"unaffectedCores,omitempty"`

	ReservedPools []string `json:"reservedPools,omitempty"`

	// Information about the containers in the cluster utilizing some PowerWorkload
	PowerContainers []Container `json:"powerContainers,omitempty"`

	// The CustomDevices include alternative devices that represent other resources
	CustomDevices []string `json:"customDevices,omitempty"`

	// The PowerProfiles in the cluster that are currently being used by Pods
	//ActiveProfiles map[string]bool `json:"activeProfiles,omitempty"`

	// Information about the active PowerWorkloads in the cluster
	//ActiveWorkloads []WorkloadInfo `json:"activeWorkloads,omitempty"`

	// Shows what cores are in the Default and Shared Pools
	//SharedPool SharedPoolInfo `json:"sharedPools,omitempty"`

	// The CPUs that are not effected by any Power Profiles
	//UneffectedCpus []int `json:"uneffectedCpus,omitempty"`
}

// PowerNodeStatus defines the observed state of PowerNode
type PowerNodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The state of the Guaranteed Pods and Shared Pool in a cluster
	PowerNodeCPUState `json:"powerNodeCPUState,omitempty"`
}

type PowerNodeCPUState struct {
	// The CPUs that are currently part of the Shared pool on a Node
	SharedPool []uint `json:"sharedPool,omitempty"`

	// Pods that are requesting CPUs in the Guaranteed QoS class
	GuaranteedPods []GuaranteedPod `json:"guaranteedPods,omitempty"`
}

type GuaranteedPod struct {
	// The name of the Node the Pod is running on
	Node string `json:"node,omitempty"`

	// The name of the Pod
	Name string `json:"name,omitempty"`

	Namespace string `json:"namespace,omitempty"`

	// The UID of the Pod
	UID string `json:"uid,omitempty"`

	// The Containers that are running in the Pod
	Containers []Container `json:"containers,omitempty"`
}

type Container struct {
	// The name of the Container
	Name string `json:"name,omitempty"`

	// The ID of the Container
	Id string `json:"id,omitempty"`

	// The name of the Pod the Container is running on
	Pod string `json:"pod,omitempty"`

	// The exclusive CPUs given to this Container
	ExclusiveCPUs []uint `json:"exclusiveCpus,omitempty"`

	// The PowerProfile that the Container is utilizing
	PowerProfile string `json:"powerProfile,omitempty"`

	// The PowerWorkload that the Container is utilizing
	Workload string `json:"workload,omitempty"`
}

type WorkloadInfo struct {
	// The name of the PowerWorkload
	Name string `json:"name,omitempty"`

	// The CPUs that are utilizing the PowerWorkload
	CpuIds []uint `json:"cores,omitempty"`
}

type SharedPoolInfo struct {
	// The name or either Default or Shared pool
	Profile string `json:"name,omitempty"`

	// The cores that are a part of this Shared Pool
	CpuIds []uint `json:"sharedPoolCpuIds,omitempty"`
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
