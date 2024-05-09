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

type ScheduleInfo struct {
	Time string `json:"time"`

	PowerProfile *string      `json:"powerProfile,omitempty"`
	Pods         *[]PodInfo   `json:"pods,omitempty"`
	CState       *CStatesSpec `json:"cState,omitempty"`
}

// TimeOfDaySpec defines the desired state of TimeOfDay
type TimeOfDaySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Time Zone to use for scheduling
	TimeZone string `json:"timeZone,omitempty"`

	// Schedule for adjusting performance mode
	Schedule     []ScheduleInfo `json:"schedule"`
	ReservedCPUs *[]uint        `json:"reservedCPUs,omitempty"`
}

// TimeOfDayStatus defines the observed state of TimeOfDay
type TimeOfDayStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The time of the last update
	LastSchedule string `json:"lastSchedule,omitempty"`

	// The time of the next update
	NextSchedule string `json:"nextSchedule,omitempty"`

	// PowerProfile associated with Time of Day
	PowerProfile string `json:"powerProfile,omitempty"`

	StatusErrors `json:",inline,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TimeOfDay is the Schema for the timeofdays API
type TimeOfDay struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TimeOfDaySpec   `json:"spec,omitempty"`
	Status TimeOfDayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TimeOfDayList contains a list of TimeOfDay
type TimeOfDayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TimeOfDay `json:"items"`
}

func (tod *TimeOfDay) SetStatusErrors(errs *[]string) {
	tod.Status.Errors = *errs
}

func (tod *TimeOfDay) GetStatusErrors() *[]string {
	return &tod.Status.Errors
}

func init() {
	SchemeBuilder.Register(&TimeOfDay{}, &TimeOfDayList{})
}
