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
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=use

func (r *PowerWorkloadReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		err := fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{Requeue: false}, err
	}
	nodeName := os.Getenv("NODE_NAME")

	workload := &powerv1.PowerWorkload{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, workload)
	logger.V(5).Info("retrieving the power workload instance")
	if err != nil {
		if errors.IsNotFound(err) {
			// If the profile still exists in the power library, then only the power workload was deleted
			// and we need to remove it from the power library here. If the profile doesn't exist, then
			// the power library will have deleted it for us
			if req.NamespacedName.Name == sharedPowerWorkloadName {
				err = r.PowerLibrary.GetReservedPool().MoveCpus(*r.PowerLibrary.GetSharedPool().Cpus())
				if err != nil {
					logger.Error(err, "failed to remove shared pool")
					return ctrl.Result{}, err
				}
				sharedPowerWorkloadName = ""
			} else {
				pool := r.PowerLibrary.GetExclusivePool(strings.ReplaceAll(req.NamespacedName.Name, ("-" + nodeName), ""))
				if pool != nil {
					err = pool.Remove()
					if err != nil {
						logger.Error(err, "failed to remove the exclusive pool")
						return ctrl.Result{}, err
					}
				}
			}

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// If there are multiple nodes the shared power workload's node selector satisfies we need to fail here before anything is done
	logger.V(5).Info("checking the node elector is satisfied with the shared power workload")
	if workload.Spec.AllCores {
		labelledNodeList := &corev1.NodeList{}
		listOption := workload.Spec.PowerNodeSelector

		err = r.Client.List(context.TODO(), labelledNodeList, client.MatchingLabels(listOption))
		if err != nil {
			logger.Error(err, "error retrieving the node with the power node selector", "selector", listOption)
			return ctrl.Result{}, err
		}

		// If there were no nodes that matched the provided labels, check the node info of the workload for a name
		logger.V(5).Info("checking the node info to see if the node name has been provided")
		if (len(labelledNodeList.Items) == 0 && workload.Spec.Node.Name != nodeName) || !util.NodeNameInNodeList(nodeName, labelledNodeList.Items) {
			return ctrl.Result{}, nil
		}

		logger.V(5).Info("verifying there is only one shared power workload and if there is more than one delete this instance")
		if sharedPowerWorkloadName != "" && sharedPowerWorkloadName != req.NamespacedName.Name {
			// Delete this shared power workload as another already exists
			err = r.Client.Delete(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "error deleting the second shared power workload")
				return ctrl.Result{}, err
			}

			sharedPowerWorkloadAlreadyExists := errors.NewServiceUnavailable("a shared power workload already exists for this node")
			logger.Error(sharedPowerWorkloadAlreadyExists, "error creating the shared power workload")
			return ctrl.Result{Requeue: false}, sharedPowerWorkloadAlreadyExists
		}

		// add cores to shared pool by selecting which cores should be reserved,
		// remaining cores will be moved to the shared pool
		logger.V(5).Info("creating the shared pool in the power library")
		err = r.PowerLibrary.GetReservedPool().SetCpuIDs(workload.Spec.ReservedCPUs)
		if err != nil {
			logger.Error(err, "error configuring the shared pool in the power library")
			return ctrl.Result{}, err
		}

		sharedPowerWorkloadName = req.NamespacedName.Name

		return ctrl.Result{}, nil
	}

	if workload.Spec.Node.Name == nodeName {
		poolFromLibrary := r.PowerLibrary.GetExclusivePool(workload.Spec.PowerProfile)
		if poolFromLibrary == nil {
			poolDoesNotExistError := errors.NewServiceUnavailable(fmt.Sprintf("pool '%s' does not exist in the power library", workload.Spec.PowerProfile))
			logger.Error(poolDoesNotExistError, "error retrieving the pool from the power library")
			return ctrl.Result{Requeue: false}, poolDoesNotExistError
		}

		logger.V(5).Info("updating the CPU list in the power library")
		cores := poolFromLibrary.Cpus().IDs()
		coresToRemoveFromLibrary := detectCoresRemoved(cores, workload.Spec.Node.CpuIds, &logger)
		coresToBeAddedToLibrary := detectCoresAdded(cores, workload.Spec.Node.CpuIds, &logger)

		if len(coresToRemoveFromLibrary) > 0 {
			err = r.PowerLibrary.GetSharedPool().MoveCpuIDs(coresToRemoveFromLibrary)
			if err != nil {
				logger.Error(err, "error updating the power library CPU list")
				return ctrl.Result{}, err
			}
		}

		if len(coresToBeAddedToLibrary) > 0 {
			err = r.PowerLibrary.GetExclusivePool(workload.Spec.PowerProfile).MoveCpuIDs(coresToBeAddedToLibrary)
			if err != nil {
				logger.Error(err, "error updating the power library CPU list")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func detectCoresRemoved(originalCoreList []uint, updatedCoreList []uint, logger *logr.Logger) []uint {
	var coresRemoved []uint
	logger.V(5).Info("detecting if cores are removed from the cores list")
	for _, core := range originalCoreList {
		if !validateCoreIsInCoreList(core, updatedCoreList) {
			coresRemoved = append(coresRemoved, core)
		}
	}

	return coresRemoved
}

func detectCoresAdded(originalCoreList []uint, updatedCoreList []uint, logger *logr.Logger) []uint {
	var coresAdded []uint
	logger.V(5).Info("detecting if cores are added to the cores list")
	for _, core := range updatedCoreList {
		if !validateCoreIsInCoreList(core, originalCoreList) {
			coresAdded = append(coresAdded, core)
		}
	}

	return coresAdded
}

func validateCoreIsInCoreList(core uint, coreList []uint) bool {
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
