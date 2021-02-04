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

	workload := &powerv1alpha1.PowerWorkload{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, workload)
	if err != nil {
		if errors.IsNotFound(err) {
			effectedCPUs := r.State.RemoveCPUFromState(req.NamespacedName.Name)
			logger.Info(fmt.Sprintf("CPUs tuned by deleted PowerWorkload: %v", effectedCPUs.CPUs))

			sharedWorkload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name:      SharedWorkloadName,
			}, sharedWorkload)
			if err != nil {
				if errors.IsNotFound(err) {
					// TODO: Make call to AppQoS for the default values for Shared CPUs
					logger.Info(fmt.Sprintf("Shared PowerWorkload not found, tuning CPUs to default values: %v", effectedCPUs))
					logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR DEFAULT MAX FREQUENCY ON CPUS %v", effectedCPUs.CPUs))
					logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR DEFAULT MIN FREQUENCY ON CPUS %v", effectedCPUs.CPUs))
					return ctrl.Result{}, nil
				}

				return ctrl.Result{}, err
			}

			// If the Shared PowerWorkload exists, we add the CPUs that were part of this non-shared workload back into it. The call to the
			// client for the Update will set the frequency of the CPUs so we don't have to here

			//maxCPUFreq := sharedWorkload.Spec.PowerProfile.Spec.Max
			//minCPUFreq := sharedWorkload.Spec.PowerProfile.Spec.Min
			//logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, effectedCPUs.CPUs))
			//logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, effectedCPUs.CPUs))

			sharedWorkload.Spec.CpuIds = append(sharedWorkload.Spec.CpuIds, effectedCPUs.CPUs...)
			err = r.Client.Update(context.TODO(), sharedWorkload)
			if err != nil {
				logger.Error(err, "error while trying to update Shared PowerWorkload")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve PowerWorkload object")
		return ctrl.Result{}, err
	}

	cpusEffected := workload.Spec.CpuIds
	// If the number of CPUs is zero, the PowerWorkload needs to be delete
	if len(cpusEffected) == 0 {
		err = r.Client.Delete(context.TODO(), workload)
		if err != nil {
			logger.Error(err, "errorwhile trying to delete PowerWorkload")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

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
		} else if len(addedCPUs) > 0 {
			// TODO: depending on AppQoS implementation, might need a for loop here
			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, addedCPUs))
			logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, addedCPUs))
		}

		r.State.UpdateWorkloadState(req.NamespacedName.Name, cpusEffected, maxCPUFreq, minCPUFreq)

		// Only update the removed CPUs if this is a non-shared PowerWorkload
		if len(removedCPUs) > 0 && req.NamespacedName.Name != SharedWorkloadName {
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

		if req.NamespacedName.Name != SharedWorkloadName {
			// Need to update the Shared PowerWorkload
			sharedWorkload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name: SharedWorkloadName,
			}, sharedWorkload)
			if err != nil {
				if errors.IsNotFound(err) {
					// No Shared PowerWorkload so no updating needed
					return ctrl.Result{}, nil
				}
	
				logger.Error(err, "error while trying to retrieve Shared PowerWorkload")
				return ctrl.Result{}, err
			}
	
			// Remove the CPUs that were added to this non-shared PowerWorkload
			updatedCPUList := removeSubsetFromWorkload(addedCPUs, sharedWorkload.Spec.CpuIds)
			// Add back the CPUs that were removed from this non-shared PowerWorkload
			updatedCPUList = append(updatedCPUList, removedCPUs...)
			sharedWorkload.Spec.CpuIds = updatedCPUList

			err = r.Client.Update(context.TODO(), sharedWorkload)
			if err != nil {
				logger.Error(err, "error while trying to update Shared PowerWrokload")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Workload does not exist in State

	logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MAX FREQUENCY %v ON CPUS %v", maxCPUFreq, workload.Spec.CpuIds))
	logger.Info(fmt.Sprintf("MAKING HTTP CALL TO AppQoS INSTANCE FOR MIN FREQUENCY %v ON CPUS %v", minCPUFreq, workload.Spec.CpuIds))

	r.State.UpdateWorkloadState(req.NamespacedName.Name, cpusEffected, maxCPUFreq, minCPUFreq)

	if req.NamespacedName.Name != SharedWorkloadName {
		sharedWorkload := &powerv1alpha1.PowerWorkload{}
	        err = r.Client.Get(context.TODO(), client.ObjectKey{
        		Namespace: req.NamespacedName.Namespace,
        		Name:      SharedWorkloadName,
        	}, sharedWorkload)
        	if err != nil {
        		if errors.IsNotFound(err) {
        			// No Shared PowerWorkload on the node, nothing to update
				return ctrl.Result{}, nil
	        	}

			return ctrl.Result{}, err
        	}

		updatedCPUList := removeSubsetFromWorkload(cpusEffected, sharedWorkload.Spec.CpuIds)
		sharedWorkload.Spec.CpuIds = updatedCPUList

		err = r.Client.Update(context.TODO(), sharedWorkload)
		if err != nil {
			logger.Error(err, "error while trying to update Shared PowerWorkload")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func checkWorkloadCPUDifference(oldCPUList []string, updatedCPUList []string) ([]string, []string) {
	addedCPUs := make([]string, 0)
	removedCPUs := make([]string, 0)

	for _, cpuID := range updatedCPUList {
		if !IdInCPUList(cpuID, oldCPUList) {
			addedCPUs = append(addedCPUs, cpuID)
		}
	}

	for _, cpuID := range oldCPUList {
		if !IdInCPUList(cpuID, updatedCPUList) {
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

func removeSubsetFromWorkload(toRemove []string, cpuList []string) []string {
	updatedCPUList := make([]string, 0)

	for _, cpuID := range cpuList {
		if !IdInCPUList(cpuID, toRemove) {
			updatedCPUList = append(updatedCPUList, cpuID)
		}
	}

	return updatedCPUList
}

func (r *PowerWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerWorkload{}).
		Complete(r)
}
