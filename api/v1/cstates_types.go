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

// CStatesSpec defines the desired state of CStates
type CStatesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SharedPoolCStates     map[string]bool            `json:"sharedPoolCStates,omitempty"`
	ExclusivePoolCStates  map[string]map[string]bool `json:"exclusivePoolCStates,omitempty"`
	IndividualCoreCStates map[string]map[string]bool `json:"individualCoreCStates,omitempty"`
}

// CStatesStatus defines the observed state of CStates
type CStatesStatus struct {
	StatusErrors `json:",inline,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CStates is the Schema for the cstates API
type CStates struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CStatesSpec   `json:"spec,omitempty"`
	Status CStatesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CStatesList contains a list of CStates
type CStatesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CStates `json:"items"`
}

func (csts *CStates) SetStatusErrors(errs *[]string) {
	csts.Status.Errors = *errs
}

func (csts *CStates) GetStatusErrors() *[]string {
	return &csts.Status.Errors
}

func init() {
	SchemeBuilder.Register(&CStates{}, &CStatesList{})
}
