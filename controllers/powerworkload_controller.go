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
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/workloadstate"
)

// PowerWorkloadReconciler reconciles a PowerWorkload object
type PowerWorkloadReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	State  *workloadstate.Workloads
}

const (
	SharedWorkloadName string = "shared-workload"
	WorkloadNameSuffix string = "-workload"
)

// +kubebuilder:rbac:groups=power.intel.com,resources=powerworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerworkloads/status,verbs=get;update;patch

func (r *PowerWorkloadReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	logger.Info(fmt.Sprintf("%v", r.State))

	workload := &powerv1alpha1.PowerWorkload{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, workload)
	if err != nil {
		if errors.IsNotFound(err) {
			effectedCPUs := r.State.RemoveCPUFromState(req.NamespacedName.Name)
			logger.Info(fmt.Sprintf("CPUs tuned by deleted PowerWorkload: %v", effectedCPUs))

			sharedWorkload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name:      SharedWorkloadName,
			}, sharedWorkload)
			if err != nil {
				if errors.IsNotFound(err) {
					// TODO: Make call to AppQoS for the default values for Shared CPUs
					logger.Info(fmt.Sprintf("Shared PowerWorkload not found, cannot retune CPUs: %v", effectedCPUs))
					return ctrl.Result{}, nil
				}

				return ctrl.Result{}, err
			}

			maxCPUFreq := sharedWorkload.Spec.PowerProfile.Spec.Max
			minCPUFreq := sharedWorkload.Spec.PowerProfile.Spec.Min

			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, effectedCPUs))
			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, effectedCPUs))
		}

		logger.Error(err, "error while trying to retrieve PowerWorkload object")
		return ctrl.Result{}, err
	}

	cpusEffected := workload.Spec.CpuIds
	logger.Info(fmt.Sprintf("CPUs to be tuned: %v", cpusEffected))
	maxCPUFreq := workload.Spec.PowerProfile.Spec.Max
	minCPUFreq := workload.Spec.PowerProfile.Spec.Min

	if oldWorkload, exists := r.State.Workloads[req.NamespacedName.Name]; exists {
		oldWorkloadCPUsSorted := oldWorkload.CPUs
		updatedWorkloadCPUsSorted := cpusEffected
		sort.Strings(oldWorkloadCPUsSorted)
		sort.Strings(updatedWorkloadCPUsSorted)

		addedCPUs, removedCPUs := checkWorkloadCPUDifference(oldWorkloadCPUsSorted, updatedWorkloadCPUsSorted)
		if maxCPUFreq != oldWorkload.Max || minCPUFreq != oldWorkload.Min {
			// Frequency has changed; no need to check for CPU changes as all CPUs need to be tuned
			if maxCPUFreq != oldWorkload.Max {
				logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, workload.Spec.CpuIds))
				// TODO: work out what needs to be changed here i.e. what part of the State has to be updated
			}

			if minCPUFreq != oldWorkload.Min {
				logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, workload.Spec.CpuIds))
				// TODO: same as above
			}
		} else {
			// TODO: depending on AppQoS implementation, might need a for loop here
			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, addedCPUs))
			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, addedCPUs))
		}

		r.State.UpdateWorkloadState(req.NamespacedName.Name, cpusEffected, maxCPUFreq, minCPUFreq)

		if len(removedCPUs) > 0 {
			// Must return the removed CPUs to Shared Workload frequencies

			sharedWorkload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name:      SharedWorkloadName,
			}, sharedWorkload)
			if err != nil {
				if errors.IsNotFound(err) {
					// TODO: Make call to AppQoS for the default values for Shared CPUs
					logger.Info(fmt.Sprintf("Shared PowerWorkload not found, cannot retune CPUs: %v", removedCPUs))
					return ctrl.Result{}, nil
				}

				return ctrl.Result{}, err
			}

			maxCPUFreq := sharedWorkload.Spec.PowerProfile.Spec.Max
			minCPUFreq := sharedWorkload.Spec.PowerProfile.Spec.Min

			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, removedCPUs))
			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, removedCPUs))
		}

		return ctrl.Result{}, nil
	}

	// Workload does not exist in State

	logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, workload.Spec.CpuIds))
	logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, workload.Spec.CpuIds))

	r.State.UpdateWorkloadState(req.NamespacedName.Name, cpusEffected, maxCPUFreq, minCPUFreq)

	return ctrl.Result{}, nil
}

func checkWorkloadCPUDifference(oldCPUList []string, updatedCPUList []string) ([]string, []string) {
	addedCPUs := make([]string, 0)
	removedCPUs := make([]string, 0)

	for _, cpuID := range updatedCPUList {
		if IdInCPUList(cpuID, oldCPUList) {
			addedCPUs = append(addedCPUs, cpuID)
		}
	}

	for _, cpuID := range oldCPUList {
		if IdInCPUList(cpuID, updatedCPUList) {
			removedCPUs = append(removedCPUs, cpuID)
		}
	}

	return addedCPUs, removedCPUs
}

func IdInCPUList(id string, cpuList []string) bool {
	for _, cpuListID := range cpuList {
		if id == cpuListID {
			return true
		}
	}

	return false
}

func (r *PowerWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerWorkload{}).
		Complete(r)
}
