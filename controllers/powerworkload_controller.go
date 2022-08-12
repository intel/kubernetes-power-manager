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
	"os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	"github.com/intel/power-optimization-library/pkg/power"

	corev1 "k8s.io/api/core/v1"
)

// PowerWorkloadReconciler reconciles a PowerWorkload object
type PowerWorkloadReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PowerLibrary power.Node
}

const (
	SharedWorkloadName string = "shared-workload"
	WorkloadNameSuffix string = "-workload"
)

var sharedPowerWorkloadName = ""

// +kubebuilder:rbac:groups=power.intel.com,resources=powerworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerworkloads/status,verbs=get;update;patch

func (r *PowerWorkloadReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	nodeName := os.Getenv("NODE_NAME")

	workload := &powerv1.PowerWorkload{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, workload)
	if err != nil {
		if errors.IsNotFound(err) {
			// If the profile still exists in the Power Library, then only the Power Workloads was deleted
			// and we need to remove it from the Power Library here. If the profile doesn't exist, then
			// the Power Library will already have deleted it for us

			profileFromLibrary := r.PowerLibrary.GetProfile(req.NamespacedName.Name)

			if req.NamespacedName.Name == sharedPowerWorkloadName {
				sharedPowerWorkloadName = ""
				if profileFromLibrary != nil {
					err = r.PowerLibrary.RemoveSharedPool()
					if err != nil {
						return ctrl.Result{}, err
					}
				}
			}

			if profileFromLibrary != nil {
				err = r.PowerLibrary.RemoveExclusivePool(req.NamespacedName.Name)
				if err != nil {
					return ctrl.Result{}, err
				}
			}

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// If there are multiple nodes that the Shared PowerWorkload's Node Selector satisfies we need to fail here before anything is done
	if workload.Spec.AllCores {
		labelledNodeList := &corev1.NodeList{}
		listOption := workload.Spec.PowerNodeSelector

		err = r.Client.List(context.TODO(), labelledNodeList, client.MatchingLabels(listOption))
		if err != nil {
			logger.Error(err, "error retrieving Node with PowerNodeSelector", listOption)
			return ctrl.Result{}, err
		}

		// If there were no Nodes that matched the provided labels, check the NodeInfo of the Workload for a name
		if (len(labelledNodeList.Items) == 0 && workload.Spec.Node.Name != nodeName) || !util.NodeNameInNodeList(nodeName, labelledNodeList.Items) {
			return ctrl.Result{}, nil
		}

		if sharedPowerWorkloadName != "" && sharedPowerWorkloadName != req.NamespacedName.Name {
			// Delete this Shared PowerWorkload as another already exists
			err = r.Client.Delete(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "error deleting second Shared PowerWorkload")
				return ctrl.Result{}, err
			}

			sharedPowerWorkloadAlreadyExists := errors.NewServiceUnavailable("A Shared PowerWorkload already exists for this node")
			logger.Error(sharedPowerWorkloadAlreadyExists, "error creating Shared PowerWorkload")
			return ctrl.Result{}, nil
		}

		profileFromLibrary := r.PowerLibrary.GetProfile(workload.Spec.PowerProfile)
		if profileFromLibrary == nil {
			profileNotFound := errors.NewServiceUnavailable(fmt.Sprintf("Profile '%s' does not exist in Power Library", workload.Spec.PowerProfile))
			logger.Error(profileNotFound, "error retrieving Profile")
			return ctrl.Result{}, profileNotFound
		}

		err = r.PowerLibrary.AddSharedPool(workload.Spec.ReservedCPUs, profileFromLibrary)
		if err != nil {
			logger.Error(err, "error creating Shared Pool in Power Library")
			return ctrl.Result{}, err
		}

		sharedPowerWorkloadName = req.NamespacedName.Name

		return ctrl.Result{}, nil
	}

	if workload.Spec.Node.Name == nodeName {
		poolFromLibrary := r.PowerLibrary.GetExclusivePool(workload.Spec.PowerProfile)
		if poolFromLibrary == nil {
			poolDoesNotExistError := errors.NewServiceUnavailable(fmt.Sprintf("Pool '%s' does not exists in Power Library", workload.Spec.PowerProfile))
			logger.Error(poolDoesNotExistError, "error retrieving Pool from Library")
			return ctrl.Result{}, nil
		}

		cores := poolFromLibrary.GetCoreIds()
		coresToRemoveFromLibrary := detectCoresRemoved(cores, workload.Spec.Node.CpuIds)
		coresToBeAddedToLibrary := detectCoresAdded(cores, workload.Spec.Node.CpuIds)

		if len(coresToRemoveFromLibrary) > 0 {
			err = r.PowerLibrary.RemoveCoresFromExclusivePool(workload.Spec.PowerProfile, coresToRemoveFromLibrary)
			if err != nil {
				logger.Error(err, "error updating Power Library Core list")
				return ctrl.Result{}, err
			}
		}

		if len(coresToBeAddedToLibrary) > 0 {
			err = r.PowerLibrary.AddCoresToExclusivePool(workload.Spec.PowerProfile, coresToBeAddedToLibrary)
			if err != nil {
				logger.Error(err, "error updating Power Library Core list")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func detectCoresRemoved(originalCoreList []int, updatedCoreList []int) []int {
	coresRemoved := []int{}
	for _, core := range originalCoreList {
		if !coreInCoreList(core, updatedCoreList) {
			coresRemoved = append(coresRemoved, core)
		}
	}

	return coresRemoved
}

func detectCoresAdded(originalCoreList []int, updatedCoreList []int) []int {
	coresAdded := []int{}
	for _, core := range updatedCoreList {
		if !coreInCoreList(core, originalCoreList) {
			coresAdded = append(coresAdded, core)
		}
	}

	return coresAdded
}

func coreInCoreList(core int, coreList []int) bool {
	for _, c := range coreList {
		if c == core {
			return true
		}
	}

	return false
}

func (r *PowerWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.PowerWorkload{}).
		Complete(r)
}
