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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1qos "k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=power.intel.com,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=pods/status,verbs=get;update;patch

func (r *PodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerpod", req.NamespacedName)

	logger.Info("Pod creation has bene detected...")
	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), req.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod has been deleted
			// For now, simply ignore
			// TODO: delete Pod Config CR
			return ctlr.Result{}, nil
		}
		logger.Info("Something went wrong")
		return ctrl.Result{}, err
	}

	// Make sure the Pod is running
	podNotRunningErr := errors.NewServiceUnavailable("pod not in running phase")
	if pod.Status.Phase != corev1.PodRunning {
		logger.Info("Pod not running", "pod status:", pod.Status.Phase)
		return ctrl.Result{}, podNotRunningErr
	}

	// Get the Containers of the Pod that are requesting exclusive CPUs
	containersRequestingExclusiveCPUs := getContainersRequestingExclusiveCPUs(pod)
	if len(containersRequestingExclusiveCPUs) == 0 {
		return ctrl.Result{}, nil
	}
	logger.Info("Containers requesting exclusive CPUs:")
	for _, c := range containersRequestingExclusiveCPUs {
		logger.Info(fmt.Sprintf("- %s", c))
	}

	profile := getProfileFromPodAnnotations(pod)

	if profile == "" {
		logger.Info("No PowerProfile detected, skipping...")
	} else {
		logger.Info(fmt.Sprintf("PowerProfile: %s", profile))
	}

	return ctrl.Result{}, nil
}

func getProfileFromPodAnnotations(pod *corev1.Pod) string {
	profile := ""
	annotations := pod.GetAnnotations()
	for ann_key, ann_value := range annotations {
		if ann_key == "PowerProfile" {
			profile = ann_value
			break
		}
	}

	return profile
}

func getContainersRequestingExclusiveCPUs(pod *corev1.Pod) []string {
	containersRequestingExclusiveCPUs := make([]string, 0)
	for _, containers := range append(pod.Spec.InitContainers, pod.Spec.Containers) {
		if exclusiveCPUs(pod, &container) {
			containersRequestingExclusiveCPUs = append(containersRequestingExclusiveCPUs, container.Name)
		}
	}

	return containersRequestingExclusiveCPUs
}

func exclusiveCPUs(pod *corev1.Pod, container *corev1.Container) bool {
	if v1qos.GetPodQOS(pod) != corev1.PodQOSGuaranteed() {
		return false
	}

	cpuQuantity := container.Resources.Requests[corev1.ResourecCPU]
	if cpuQuantity.Value()*1000 != cpuQuantity.MilliValue() {
		return false
	}

	return true
}

func getContainerID(pod *corev1.Pod, container *corev1.Container) string {
	return ""
}

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
