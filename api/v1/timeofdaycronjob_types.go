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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//batchv1 "k8s.io/api/batch/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TimeOfDayCronJobSpec defines the desired state of TimeOfDayCronJob
type TimeOfDayCronJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Hour         int          `json:"hour"`
	Minute       int          `json:"minute"`
	Second       int          `json:"second,omitempty"`
	TimeZone     *string      `json:"timeZone"`
	Profile      *string      `json:"profile"`
	Pods         *[]PodInfo   `json:"pods,omitempty"`
	ReservedCPUs *[]uint      `json:"reservedCPUs,omitempty"`
	CState       *CStatesSpec `json:"cState,omitempty"`
}

type PodInfo struct {
	Labels metav1.LabelSelector `json:"labels"`
	Target string               `json:"target"`
}

// TimeOfDayCronJobStatus defines the observed state of TimeOfDayCronJob
type TimeOfDayCronJobStatus struct {
	Active             []v1.ObjectReference `json:"active,omitempty"`
	LastScheduleTime   *metav1.Time         `json:"lastScheduleTime,omitempty"`
	LastSuccessfulTime *metav1.Time         `json:"lastSuccessfulTime,omitempty"`

	StatusErrors `json:",inline,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TimeOfDayCronJob is the Schema for the timeofdaycronjobs API
type TimeOfDayCronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TimeOfDayCronJobSpec   `json:"spec,omitempty"`
	Status            TimeOfDayCronJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TimeOfDayCronJobList contains a list of TimeOfDayCronJob
type TimeOfDayCronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TimeOfDayCronJob `json:"items"`
}

func (todcr *TimeOfDayCronJob) SetStatusErrors(errs *[]string) {
	todcr.Status.Errors = *errs
}
func (todcr *TimeOfDayCronJob) GetStatusErrors() *[]string {
	return &todcr.Status.Errors
}

func init() {
	SchemeBuilder.Register(&TimeOfDayCronJob{}, &TimeOfDayCronJobList{})
}
