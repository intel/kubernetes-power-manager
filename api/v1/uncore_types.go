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

// UncoreSpec defines the desired state of Uncore
type UncoreSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SysMax       *uint          `json:"sysMax,omitempty"`
	SysMin       *uint          `json:"sysMin,omitempty"`
	DieSelectors *[]DieSelector `json:"dieSelector,omitempty"`
}

type DieSelector struct {
	Package *uint `json:"package"`
	Die     *uint `json:"die,omitempty"`
	Min     *uint `json:"min"`
	Max     *uint `json:"max"`
}

// UncoreStatus defines the observed state of Uncore
type UncoreStatus struct {
	StatusErrors `json:",inline,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Uncore is the Schema for the uncores API
type Uncore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UncoreSpec   `json:"spec,omitempty"`
	Status UncoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UncoreList contains a list of Uncore
type UncoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Uncore `json:"items"`
}

func (ucnre *Uncore) SetStatusErrors(err *[]string) {
	ucnre.Status.Errors = *err
}

func (ucnre *Uncore) GetStatusErrors() *[]string {
	return &ucnre.Status.Errors
}

func init() {
	SchemeBuilder.Register(&Uncore{}, &UncoreList{})
}
