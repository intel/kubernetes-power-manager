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
	"strings"

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
	PowerLibrary power.Host
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
	if req.Namespace != IntelPowerNamespace {
		logger.Error(fmt.Errorf("incorrect namespace"), "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{}, nil
	}
	nodeName := os.Getenv("NODE_NAME")

	workload := &powerv1.PowerWorkload{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, workload)
	logger.V(5).Info("Retriving Power workload instance")
	if err != nil {
		if errors.IsNotFound(err) {
			// If the profile still exists in the Power Library, then only the Power Workloads was deleted
			// and we need to remove it from the Power Library here. If the profile doesn't exist, then
			// the Power Library will already have deleted it for us
			if req.NamespacedName.Name == sharedPowerWorkloadName {
				err = r.PowerLibrary.GetSharedPool().SetPowerProfile(nil)
				if err != nil {
					logger.Error(err, "failed to remove exclusive pool")
					return ctrl.Result{}, err
				}
				sharedPowerWorkloadName = ""
			} else {
				pool := r.PowerLibrary.GetExclusivePool(strings.ReplaceAll(req.NamespacedName.Name, ("-" + nodeName), ""))
				if pool != nil {
					err = pool.Remove()
					if err != nil {
						logger.Error(err, "failed to remove exclusive pool")
						return ctrl.Result{}, err
					}
				}
			}

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// If there are multiple nodes that the Shared PowerWorkload's Node Selector satisfies we need to fail here before anything is done
	logger.V(5).Info("Checking that the Node Selector is satisfied with the Shared PowerWorkload")
	if workload.Spec.AllCores {
		labelledNodeList := &corev1.NodeList{}
		listOption := workload.Spec.PowerNodeSelector

		err = r.Client.List(context.TODO(), labelledNodeList, client.MatchingLabels(listOption))
		if err != nil {
			logger.Error(err, "error retrieving Node with PowerNodeSelector", listOption)
			return ctrl.Result{}, err
		}

		// If there were no Nodes that matched the provided labels, check the NodeInfo of the Workload for a name
		logger.V(5).Info("Checking NodeInfo to see if Node name is provided")
		if (len(labelledNodeList.Items) == 0 && workload.Spec.Node.Name != nodeName) || !util.NodeNameInNodeList(nodeName, labelledNodeList.Items) {
			return ctrl.Result{}, nil
		}

		logger.V(5).Info("Verifying that there is only one Shared PowerWorkload and if there is more than one delete this instance")
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

		// add cores to shared pool by selecting which cores should be reserved
		// remaining cores will be moved to the shared pool
		logger.V(5).Info("Creating Shared Pool in the Power Library")
		err = r.PowerLibrary.GetReservedPool().SetCpuIDs(workload.Spec.ReservedCPUs)
		if err != nil {
			logger.Error(err, "error configuring Shared Pool in Power Library")
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

		logger.V(5).Info("Updating Cpu list in Power Library")
		cores := poolFromLibrary.Cpus().IDs()
		coresToRemoveFromLibrary := detectCoresRemoved(cores, workload.Spec.Node.CpuIds, &logger)
		coresToBeAddedToLibrary := detectCoresAdded(cores, workload.Spec.Node.CpuIds, &logger)

		if len(coresToRemoveFromLibrary) > 0 {
			err = r.PowerLibrary.GetSharedPool().MoveCpuIDs(coresToRemoveFromLibrary)
			if err != nil {
				logger.Error(err, "error updating Power Library Cpu list")
				return ctrl.Result{}, err
			}
		}

		if len(coresToBeAddedToLibrary) > 0 {
			err = r.PowerLibrary.GetExclusivePool(workload.Spec.PowerProfile).MoveCpuIDs(coresToBeAddedToLibrary)
			if err != nil {
				logger.Error(err, "error updating Power Library Cpu list")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func detectCoresRemoved(originalCoreList []uint, updatedCoreList []uint, logger *logr.Logger) []uint {
	var coresRemoved []uint
	logger.V(5).Info("Detecting if Cores are Removed from the CoreList")
	for _, core := range originalCoreList {
		if !coreInCoreList(core, updatedCoreList) {
			coresRemoved = append(coresRemoved, core)
		}
	}

	return coresRemoved
}

func detectCoresAdded(originalCoreList []uint, updatedCoreList []uint, logger *logr.Logger) []uint {
	var coresAdded []uint
	logger.V(5).Info("Creating Shared Pool in the Power Library")
	for _, core := range updatedCoreList {
		if !coreInCoreList(core, originalCoreList) {
			coresAdded = append(coresAdded, core)
		}
	}

	return coresAdded
}

func coreInCoreList(core uint, coreList []uint) bool {
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
