//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CStates) DeepCopyInto(out *CStates) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CStates.
func (in *CStates) DeepCopy() *CStates {
	if in == nil {
		return nil
	}
	out := new(CStates)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CStates) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CStatesList) DeepCopyInto(out *CStatesList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CStates, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CStatesList.
func (in *CStatesList) DeepCopy() *CStatesList {
	if in == nil {
		return nil
	}
	out := new(CStatesList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CStatesList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CStatesSpec) DeepCopyInto(out *CStatesSpec) {
	*out = *in
	if in.SharedPoolCStates != nil {
		in, out := &in.SharedPoolCStates, &out.SharedPoolCStates
		*out = make(map[string]bool, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ExclusivePoolCStates != nil {
		in, out := &in.ExclusivePoolCStates, &out.ExclusivePoolCStates
		*out = make(map[string]map[string]bool, len(*in))
		for key, val := range *in {
			var outVal map[string]bool
			if val == nil {
				(*out)[key] = nil
			} else {
				inVal := (*in)[key]
				in, out := &inVal, &outVal
				*out = make(map[string]bool, len(*in))
				for key, val := range *in {
					(*out)[key] = val
				}
			}
			(*out)[key] = outVal
		}
	}
	if in.IndividualCoreCStates != nil {
		in, out := &in.IndividualCoreCStates, &out.IndividualCoreCStates
		*out = make(map[string]map[string]bool, len(*in))
		for key, val := range *in {
			var outVal map[string]bool
			if val == nil {
				(*out)[key] = nil
			} else {
				inVal := (*in)[key]
				in, out := &inVal, &outVal
				*out = make(map[string]bool, len(*in))
				for key, val := range *in {
					(*out)[key] = val
				}
			}
			(*out)[key] = outVal
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CStatesSpec.
func (in *CStatesSpec) DeepCopy() *CStatesSpec {
	if in == nil {
		return nil
	}
	out := new(CStatesSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CStatesStatus) DeepCopyInto(out *CStatesStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CStatesStatus.
func (in *CStatesStatus) DeepCopy() *CStatesStatus {
	if in == nil {
		return nil
	}
	out := new(CStatesStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Container) DeepCopyInto(out *Container) {
	*out = *in
	if in.ExclusiveCPUs != nil {
		in, out := &in.ExclusiveCPUs, &out.ExclusiveCPUs
		*out = make([]uint, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Container.
func (in *Container) DeepCopy() *Container {
	if in == nil {
		return nil
	}
	out := new(Container)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DieSelector) DeepCopyInto(out *DieSelector) {
	*out = *in
	if in.Package != nil {
		in, out := &in.Package, &out.Package
		*out = new(uint)
		**out = **in
	}
	if in.Die != nil {
		in, out := &in.Die, &out.Die
		*out = new(uint)
		**out = **in
	}
	if in.Min != nil {
		in, out := &in.Min, &out.Min
		*out = new(uint)
		**out = **in
	}
	if in.Max != nil {
		in, out := &in.Max, &out.Max
		*out = new(uint)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DieSelector.
func (in *DieSelector) DeepCopy() *DieSelector {
	if in == nil {
		return nil
	}
	out := new(DieSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GuaranteedPod) DeepCopyInto(out *GuaranteedPod) {
	*out = *in
	if in.Containers != nil {
		in, out := &in.Containers, &out.Containers
		*out = make([]Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GuaranteedPod.
func (in *GuaranteedPod) DeepCopy() *GuaranteedPod {
	if in == nil {
		return nil
	}
	out := new(GuaranteedPod)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodInfo) DeepCopyInto(out *PodInfo) {
	*out = *in
	in.Labels.DeepCopyInto(&out.Labels)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodInfo.
func (in *PodInfo) DeepCopy() *PodInfo {
	if in == nil {
		return nil
	}
	out := new(PodInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerConfig) DeepCopyInto(out *PowerConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerConfig.
func (in *PowerConfig) DeepCopy() *PowerConfig {
	if in == nil {
		return nil
	}
	out := new(PowerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerConfigList) DeepCopyInto(out *PowerConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PowerConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerConfigList.
func (in *PowerConfigList) DeepCopy() *PowerConfigList {
	if in == nil {
		return nil
	}
	out := new(PowerConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerConfigSpec) DeepCopyInto(out *PowerConfigSpec) {
	*out = *in
	if in.PowerNodeSelector != nil {
		in, out := &in.PowerNodeSelector, &out.PowerNodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.PowerProfiles != nil {
		in, out := &in.PowerProfiles, &out.PowerProfiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.CustomDevices != nil {
		in, out := &in.CustomDevices, &out.CustomDevices
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerConfigSpec.
func (in *PowerConfigSpec) DeepCopy() *PowerConfigSpec {
	if in == nil {
		return nil
	}
	out := new(PowerConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerConfigStatus) DeepCopyInto(out *PowerConfigStatus) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerConfigStatus.
func (in *PowerConfigStatus) DeepCopy() *PowerConfigStatus {
	if in == nil {
		return nil
	}
	out := new(PowerConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerNode) DeepCopyInto(out *PowerNode) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerNode.
func (in *PowerNode) DeepCopy() *PowerNode {
	if in == nil {
		return nil
	}
	out := new(PowerNode)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerNode) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerNodeCPUState) DeepCopyInto(out *PowerNodeCPUState) {
	*out = *in
	if in.SharedPool != nil {
		in, out := &in.SharedPool, &out.SharedPool
		*out = make([]uint, len(*in))
		copy(*out, *in)
	}
	if in.GuaranteedPods != nil {
		in, out := &in.GuaranteedPods, &out.GuaranteedPods
		*out = make([]GuaranteedPod, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerNodeCPUState.
func (in *PowerNodeCPUState) DeepCopy() *PowerNodeCPUState {
	if in == nil {
		return nil
	}
	out := new(PowerNodeCPUState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerNodeList) DeepCopyInto(out *PowerNodeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PowerNode, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerNodeList.
func (in *PowerNodeList) DeepCopy() *PowerNodeList {
	if in == nil {
		return nil
	}
	out := new(PowerNodeList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerNodeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerNodeSpec) DeepCopyInto(out *PowerNodeSpec) {
	*out = *in
	if in.PowerProfiles != nil {
		in, out := &in.PowerProfiles, &out.PowerProfiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PowerWorkloads != nil {
		in, out := &in.PowerWorkloads, &out.PowerWorkloads
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ReservedPools != nil {
		in, out := &in.ReservedPools, &out.ReservedPools
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PowerContainers != nil {
		in, out := &in.PowerContainers, &out.PowerContainers
		*out = make([]Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CustomDevices != nil {
		in, out := &in.CustomDevices, &out.CustomDevices
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerNodeSpec.
func (in *PowerNodeSpec) DeepCopy() *PowerNodeSpec {
	if in == nil {
		return nil
	}
	out := new(PowerNodeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerNodeStatus) DeepCopyInto(out *PowerNodeStatus) {
	*out = *in
	in.PowerNodeCPUState.DeepCopyInto(&out.PowerNodeCPUState)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerNodeStatus.
func (in *PowerNodeStatus) DeepCopy() *PowerNodeStatus {
	if in == nil {
		return nil
	}
	out := new(PowerNodeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerPod) DeepCopyInto(out *PowerPod) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerPod.
func (in *PowerPod) DeepCopy() *PowerPod {
	if in == nil {
		return nil
	}
	out := new(PowerPod)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerPod) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerPodList) DeepCopyInto(out *PowerPodList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PowerPod, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerPodList.
func (in *PowerPodList) DeepCopy() *PowerPodList {
	if in == nil {
		return nil
	}
	out := new(PowerPodList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerPodList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerPodSpec) DeepCopyInto(out *PowerPodSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerPodSpec.
func (in *PowerPodSpec) DeepCopy() *PowerPodSpec {
	if in == nil {
		return nil
	}
	out := new(PowerPodSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerPodStatus) DeepCopyInto(out *PowerPodStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerPodStatus.
func (in *PowerPodStatus) DeepCopy() *PowerPodStatus {
	if in == nil {
		return nil
	}
	out := new(PowerPodStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerProfile) DeepCopyInto(out *PowerProfile) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerProfile.
func (in *PowerProfile) DeepCopy() *PowerProfile {
	if in == nil {
		return nil
	}
	out := new(PowerProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerProfile) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerProfileList) DeepCopyInto(out *PowerProfileList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PowerProfile, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerProfileList.
func (in *PowerProfileList) DeepCopy() *PowerProfileList {
	if in == nil {
		return nil
	}
	out := new(PowerProfileList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerProfileList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerProfileSpec) DeepCopyInto(out *PowerProfileSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerProfileSpec.
func (in *PowerProfileSpec) DeepCopy() *PowerProfileSpec {
	if in == nil {
		return nil
	}
	out := new(PowerProfileSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerProfileStatus) DeepCopyInto(out *PowerProfileStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerProfileStatus.
func (in *PowerProfileStatus) DeepCopy() *PowerProfileStatus {
	if in == nil {
		return nil
	}
	out := new(PowerProfileStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerWorkload) DeepCopyInto(out *PowerWorkload) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerWorkload.
func (in *PowerWorkload) DeepCopy() *PowerWorkload {
	if in == nil {
		return nil
	}
	out := new(PowerWorkload)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerWorkload) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerWorkloadList) DeepCopyInto(out *PowerWorkloadList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PowerWorkload, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerWorkloadList.
func (in *PowerWorkloadList) DeepCopy() *PowerWorkloadList {
	if in == nil {
		return nil
	}
	out := new(PowerWorkloadList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerWorkloadList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerWorkloadSpec) DeepCopyInto(out *PowerWorkloadSpec) {
	*out = *in
	if in.ReservedCPUs != nil {
		in, out := &in.ReservedCPUs, &out.ReservedCPUs
		*out = make([]ReservedSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.PowerNodeSelector != nil {
		in, out := &in.PowerNodeSelector, &out.PowerNodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.Node.DeepCopyInto(&out.Node)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerWorkloadSpec.
func (in *PowerWorkloadSpec) DeepCopy() *PowerWorkloadSpec {
	if in == nil {
		return nil
	}
	out := new(PowerWorkloadSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerWorkloadStatus) DeepCopyInto(out *PowerWorkloadStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerWorkloadStatus.
func (in *PowerWorkloadStatus) DeepCopy() *PowerWorkloadStatus {
	if in == nil {
		return nil
	}
	out := new(PowerWorkloadStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReservedSpec) DeepCopyInto(out *ReservedSpec) {
	*out = *in
	if in.Cores != nil {
		in, out := &in.Cores, &out.Cores
		*out = make([]uint, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReservedSpec.
func (in *ReservedSpec) DeepCopy() *ReservedSpec {
	if in == nil {
		return nil
	}
	out := new(ReservedSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScheduleInfo) DeepCopyInto(out *ScheduleInfo) {
	*out = *in
	if in.PowerProfile != nil {
		in, out := &in.PowerProfile, &out.PowerProfile
		*out = new(string)
		**out = **in
	}
	if in.Pods != nil {
		in, out := &in.Pods, &out.Pods
		*out = new([]PodInfo)
		if **in != nil {
			in, out := *in, *out
			*out = make([]PodInfo, len(*in))
			for i := range *in {
				(*in)[i].DeepCopyInto(&(*out)[i])
			}
		}
	}
	if in.CState != nil {
		in, out := &in.CState, &out.CState
		*out = new(CStatesSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScheduleInfo.
func (in *ScheduleInfo) DeepCopy() *ScheduleInfo {
	if in == nil {
		return nil
	}
	out := new(ScheduleInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedPoolInfo) DeepCopyInto(out *SharedPoolInfo) {
	*out = *in
	if in.CpuIds != nil {
		in, out := &in.CpuIds, &out.CpuIds
		*out = make([]uint, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedPoolInfo.
func (in *SharedPoolInfo) DeepCopy() *SharedPoolInfo {
	if in == nil {
		return nil
	}
	out := new(SharedPoolInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDay) DeepCopyInto(out *TimeOfDay) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDay.
func (in *TimeOfDay) DeepCopy() *TimeOfDay {
	if in == nil {
		return nil
	}
	out := new(TimeOfDay)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TimeOfDay) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDayCronJob) DeepCopyInto(out *TimeOfDayCronJob) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDayCronJob.
func (in *TimeOfDayCronJob) DeepCopy() *TimeOfDayCronJob {
	if in == nil {
		return nil
	}
	out := new(TimeOfDayCronJob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TimeOfDayCronJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDayCronJobList) DeepCopyInto(out *TimeOfDayCronJobList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TimeOfDayCronJob, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDayCronJobList.
func (in *TimeOfDayCronJobList) DeepCopy() *TimeOfDayCronJobList {
	if in == nil {
		return nil
	}
	out := new(TimeOfDayCronJobList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TimeOfDayCronJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDayCronJobSpec) DeepCopyInto(out *TimeOfDayCronJobSpec) {
	*out = *in
	if in.TimeZone != nil {
		in, out := &in.TimeZone, &out.TimeZone
		*out = new(string)
		**out = **in
	}
	if in.Profile != nil {
		in, out := &in.Profile, &out.Profile
		*out = new(string)
		**out = **in
	}
	if in.Pods != nil {
		in, out := &in.Pods, &out.Pods
		*out = new([]PodInfo)
		if **in != nil {
			in, out := *in, *out
			*out = make([]PodInfo, len(*in))
			for i := range *in {
				(*in)[i].DeepCopyInto(&(*out)[i])
			}
		}
	}
	if in.ReservedCPUs != nil {
		in, out := &in.ReservedCPUs, &out.ReservedCPUs
		*out = new([]uint)
		if **in != nil {
			in, out := *in, *out
			*out = make([]uint, len(*in))
			copy(*out, *in)
		}
	}
	if in.CState != nil {
		in, out := &in.CState, &out.CState
		*out = new(CStatesSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDayCronJobSpec.
func (in *TimeOfDayCronJobSpec) DeepCopy() *TimeOfDayCronJobSpec {
	if in == nil {
		return nil
	}
	out := new(TimeOfDayCronJobSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDayCronJobStatus) DeepCopyInto(out *TimeOfDayCronJobStatus) {
	*out = *in
	if in.Active != nil {
		in, out := &in.Active, &out.Active
		*out = make([]corev1.ObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.LastScheduleTime != nil {
		in, out := &in.LastScheduleTime, &out.LastScheduleTime
		*out = (*in).DeepCopy()
	}
	if in.LastSuccessfulTime != nil {
		in, out := &in.LastSuccessfulTime, &out.LastSuccessfulTime
		*out = (*in).DeepCopy()
	}
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDayCronJobStatus.
func (in *TimeOfDayCronJobStatus) DeepCopy() *TimeOfDayCronJobStatus {
	if in == nil {
		return nil
	}
	out := new(TimeOfDayCronJobStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDayList) DeepCopyInto(out *TimeOfDayList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TimeOfDay, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDayList.
func (in *TimeOfDayList) DeepCopy() *TimeOfDayList {
	if in == nil {
		return nil
	}
	out := new(TimeOfDayList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TimeOfDayList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDaySpec) DeepCopyInto(out *TimeOfDaySpec) {
	*out = *in
	if in.Schedule != nil {
		in, out := &in.Schedule, &out.Schedule
		*out = make([]ScheduleInfo, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ReservedCPUs != nil {
		in, out := &in.ReservedCPUs, &out.ReservedCPUs
		*out = new([]uint)
		if **in != nil {
			in, out := *in, *out
			*out = make([]uint, len(*in))
			copy(*out, *in)
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDaySpec.
func (in *TimeOfDaySpec) DeepCopy() *TimeOfDaySpec {
	if in == nil {
		return nil
	}
	out := new(TimeOfDaySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOfDayStatus) DeepCopyInto(out *TimeOfDayStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOfDayStatus.
func (in *TimeOfDayStatus) DeepCopy() *TimeOfDayStatus {
	if in == nil {
		return nil
	}
	out := new(TimeOfDayStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Uncore) DeepCopyInto(out *Uncore) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Uncore.
func (in *Uncore) DeepCopy() *Uncore {
	if in == nil {
		return nil
	}
	out := new(Uncore)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Uncore) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UncoreList) DeepCopyInto(out *UncoreList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Uncore, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UncoreList.
func (in *UncoreList) DeepCopy() *UncoreList {
	if in == nil {
		return nil
	}
	out := new(UncoreList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UncoreList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UncoreSpec) DeepCopyInto(out *UncoreSpec) {
	*out = *in
	if in.SysMax != nil {
		in, out := &in.SysMax, &out.SysMax
		*out = new(uint)
		**out = **in
	}
	if in.SysMin != nil {
		in, out := &in.SysMin, &out.SysMin
		*out = new(uint)
		**out = **in
	}
	if in.DieSelectors != nil {
		in, out := &in.DieSelectors, &out.DieSelectors
		*out = new([]DieSelector)
		if **in != nil {
			in, out := *in, *out
			*out = make([]DieSelector, len(*in))
			for i := range *in {
				(*in)[i].DeepCopyInto(&(*out)[i])
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UncoreSpec.
func (in *UncoreSpec) DeepCopy() *UncoreSpec {
	if in == nil {
		return nil
	}
	out := new(UncoreSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UncoreStatus) DeepCopyInto(out *UncoreStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UncoreStatus.
func (in *UncoreStatus) DeepCopy() *UncoreStatus {
	if in == nil {
		return nil
	}
	out := new(UncoreStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadInfo) DeepCopyInto(out *WorkloadInfo) {
	*out = *in
	if in.CpuIds != nil {
		in, out := &in.CpuIds, &out.CpuIds
		*out = make([]uint, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadInfo.
func (in *WorkloadInfo) DeepCopy() *WorkloadInfo {
	if in == nil {
		return nil
	}
	out := new(WorkloadInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadNode) DeepCopyInto(out *WorkloadNode) {
	*out = *in
	if in.Containers != nil {
		in, out := &in.Containers, &out.Containers
		*out = make([]Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CpuIds != nil {
		in, out := &in.CpuIds, &out.CpuIds
		*out = make([]uint, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadNode.
func (in *WorkloadNode) DeepCopy() *WorkloadNode {
	if in == nil {
		return nil
	}
	out := new(WorkloadNode)
	in.DeepCopyInto(out)
	return out
}
