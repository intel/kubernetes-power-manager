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
	"reflect"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

// PowerWorkloadReconciler reconciles a PowerWorkload object
type PowerWorkloadReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	AppQoSClient *appqos.AppQoSClient
}

const (
	SharedWorkloadName string = "shared-workload"
	WorkloadNameSuffix string = "-workload"
	DefaultPool        string = "Default"
)

var sharedPowerWorkloadName = ""

// +kubebuilder:rbac:groups=power.intel.com,resources=powerworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerworkloads/status,verbs=get;update;patch

func (r *PowerWorkloadReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	nodeName := os.Getenv("NODE_NAME")
	workload := &powerv1alpha1.PowerWorkload{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, workload)
	if err != nil {
		if errors.IsNotFound(err) {
			// Assume PowerWorkload has been deleted. Check each Power Node to delete from each AppQoS instance

			var pool *appqos.Pool

			if strings.HasPrefix(req.NamespacedName.Name, "shared-") {
				if req.NamespacedName.Name != sharedPowerWorkloadName {
					logger.Info("The deleted Shared PowerWorkload was not assigned to this Node")
					return ctrl.Result{}, nil
				} else {
					pool, err = r.AppQoSClient.GetSharedPool(AppQoSClientAddress)
					sharedPowerWorkloadName = ""
				}
			} else {
				workloadSuffix := fmt.Sprintf("%s-workload", nodeName)
				if strings.HasSuffix(req.NamespacedName.Name, workloadSuffix) {
					pool, err = r.AppQoSClient.GetPoolByName(AppQoSClientAddress, req.NamespacedName.Name)
				}
			}
			if err != nil {
				logger.Error(err, "error retrieving Pool from AppQoS instance")
				return ctrl.Result{}, err
			}

			if reflect.DeepEqual(pool, &appqos.Pool{}) {
				return ctrl.Result{}, nil
			}

			if *pool.Name == "Shared" {
				powerWorkloads := &powerv1alpha1.PowerWorkloadList{}
				err = r.Client.List(context.TODO(), powerWorkloads)
				if err != nil {
					logger.Error(err, "error retrieving PowerWorkload list")
					return ctrl.Result{}, err
				}

				numberOfSharedPowerWorkloads := 0
				for _, powerWorkload := range powerWorkloads.Items {
					if strings.HasPrefix(powerWorkload.Name, "shared-") {
						numberOfSharedPowerWorkloads++
					}
				}

				if numberOfSharedPowerWorkloads > 0 {
					// This is a Shared PowerWorkload that was deleted because it was not the original one
					return ctrl.Result{}, nil
				}
			}

			err = r.AppQoSClient.DeletePool(AppQoSClientAddress, *pool.ID)
			if err != nil {
				logger.Error(err, "error deleting Pool from AppQoS instance")
				return ctrl.Result{}, err
			}

			updatedSharedPool, id, err := r.returnCoresToSharedPool(*pool.Cores, AppQoSClientAddress)
			if err != nil {
				logger.Error(err, "error updating Shared Pool")
				return ctrl.Result{}, err
			}

			appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, AppQoSClientAddress, id)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				return ctrl.Result{}, err
			}

			sharedWorkload := powerv1alpha1.PowerWorkload{}
			powerWorkloads := &powerv1alpha1.PowerWorkloadList{}
			err = r.Client.List(context.TODO(), powerWorkloads)
			if err != nil {
				logger.Error(err, "error retrieving PowerWorkload list")
				return ctrl.Result{}, err
			} else if len(powerWorkloads.Items) > 0 {
				for _, powerWorkload := range powerWorkloads.Items {
					if powerWorkload.Status.Node == nodeName {
						sharedWorkload = powerWorkload
					}
				}

				if !reflect.DeepEqual(sharedWorkload, powerv1alpha1.PowerWorkload{}) {
					sharedWorkload.Status.SharedCores = *updatedSharedPool.Cores
					err = r.Client.Status().Update(context.TODO(), &sharedWorkload)
					if err != nil {
						logger.Error(err, "error updating SharedWorkload")
						return ctrl.Result{}, err
					}
				}
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error retrieving PowerWorkload object")
		return ctrl.Result{}, err
	}

	// If there are multiple nodes that the Shared PowerWorkload's Node Selector satisfies we need to fail here before anything is done
	if workload.Spec.AllCores {
		if !strings.HasPrefix(workload.Name, "shared-") {
			// Shared PowerWorkload does not start with 'shared-' so it cannot be eligible as the Shared PowerWorkload

			err = r.Client.Delete(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "error deleting incorrect Shared PowerWorkload")
				return ctrl.Result{}, err
			}

			powerWorkloadIncorrectBeginning := errors.NewServiceUnavailable("Shared PowerWorkload must begin with 'shared-'")
			logger.Error(powerWorkloadIncorrectBeginning, "error creating Shared PowerWorkload")
			return ctrl.Result{}, nil
		}

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

		if len(labelledNodeList.Items) > 1 {
			tooManyNodesError := errors.NewServiceUnavailable("Shared PowerWorkload cannot be assigned to multiple Nodes")
			logger.Error(tooManyNodesError, "error creating Shared PowerWorkload")
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
	} else {
		// Check to make sure this PowerWorkload is meant for this Node
		if workload.Spec.Node.Name != nodeName {
			// Not meant for this Node, continue

			return ctrl.Result{}, nil
		}

		if strings.HasPrefix(workload.Name, "shared-") {
			// Non Shared PowerWorkload cannot start with 'shared-'

			err = r.Client.Delete(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "error deleting incorrect PowerWorkload")
				return ctrl.Result{}, err
			}

			powerWorkloadIncorrectBeginning := errors.NewServiceUnavailable("PowerWorkload must not begin with 'shared-'")
			logger.Error(powerWorkloadIncorrectBeginning, "error creating Shared PowerWorkload")
			return ctrl.Result{}, nil
		}
	}

	// Get the PowerProfile from the AppQoS instance
	powerProfileFromAppQoS, err := r.AppQoSClient.GetProfileByName(workload.Spec.PowerProfile, AppQoSClientAddress)
	if err != nil {
		logger.Error(err, fmt.Sprintf("error retrieving PowerProfile '%s' from AppQoS instance", workload.Spec.PowerProfile))
		return ctrl.Result{}, err
	}

	// PowerProfile does not exist in AppQoS instance
	if reflect.DeepEqual(powerProfileFromAppQoS, &appqos.PowerProfile{}) {
		profileNotFoundError := errors.NewServiceUnavailable(fmt.Sprintf("PowerProfile '%s' not found in AppQoS instance", workload.Spec.PowerProfile))
		logger.Error(profileNotFoundError, "error retrieving Power Profile")
		return ctrl.Result{}, err
	}

	// Get the Pool associated with this PowerWorkload
	poolFromAppQoS, err := r.AppQoSClient.GetPoolByName(AppQoSClientAddress, req.NamespacedName.Name)
	if err != nil {
		logger.Error(err, "error retrieving Pool from AppQoS")
		return ctrl.Result{}, err
	}

	// Check if the Pool associated with this PowerWorkload exists
	if reflect.DeepEqual(poolFromAppQoS, &appqos.Pool{}) {
		// Pool does not exist so we need to create it. We need to check first if it is the Shared Workload

		if workload.Spec.AllCores {
			// This is being designated as the Shared Workload for this Node
			// Need to make sure there is no other shared pool

			sharedPowerWorkloadName = req.NamespacedName.Name

			sharedPool, err := r.AppQoSClient.GetSharedPool(AppQoSClientAddress)
			if err != nil {
				logger.Error(err, "error retrieving Shared Pool from AppQoS")
				return ctrl.Result{}, err
			}

			commonCPUs := util.CommonCPUs(workload.Spec.ReservedCPUs, workload.Status.SharedCores)
			updatedSharedCPUList := util.CPUListDifference(commonCPUs, workload.Status.SharedCores)

			if *sharedPool.Name == "Shared" && len(updatedSharedCPUList) > 0 {
				updatedSharedPool, id := updatePool(updatedSharedCPUList, *sharedPool.PowerProfile, sharedPool)
				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, AppQoSClientAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					return ctrl.Result{}, err
				}
			}

			var coresRemovedFromDefaultPool []int
			if *sharedPool.Name == "Shared" {
				defaultPool, err := r.AppQoSClient.GetPoolByName(AppQoSClientAddress, DefaultPool)
				if err != nil {
					logger.Error(err, "error retrieving Default Pool from AppQoS")
					return ctrl.Result{}, err
				}

				coresRemovedFromDefaultPool = util.CPUListDifference(workload.Spec.ReservedCPUs, *defaultPool.Cores)

				updatedDefaultPool, id := updatePoolWithoutPowerProfile(workload.Spec.ReservedCPUs, defaultPool)
				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedDefaultPool, AppQoSClientAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					return ctrl.Result{}, err
				}
			} else {
				coresRemovedFromDefaultPool = util.CPUListDifference(workload.Spec.ReservedCPUs, *sharedPool.Cores)

				updatedSharedPool, id := updatePoolWithoutPowerProfile(workload.Spec.ReservedCPUs, sharedPool)
				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, AppQoSClientAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					return ctrl.Result{}, err
				}
			}

			workload.Status.SharedCores = append(updatedSharedCPUList, coresRemovedFromDefaultPool...)
			sort.Ints(workload.Status.SharedCores)
			if *sharedPool.Name == "Shared" {
				updatedSharedPool, id := updatePool(workload.Status.SharedCores, *sharedPool.PowerProfile, sharedPool)
				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, AppQoSClientAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					return ctrl.Result{}, err
				}
			} else {
				cbmDefault := 1
				sharedPoolName := "Shared"

				pool := &appqos.Pool{}
				pool.Name = &sharedPoolName
				pool.Cores = &workload.Status.SharedCores
				pool.PowerProfile = powerProfileFromAppQoS.ID
				pool.Cbm = &cbmDefault

				appqosPostResponse, err := r.AppQoSClient.PostPool(pool, AppQoSClientAddress)
				if err != nil {
					logger.Error(err, appqosPostResponse)
					return ctrl.Result{}, err
				}
			}

			// Update the Status of the Shared PowerWorkload with the current Shared pool cores
			workload.Status.Node = nodeName
			err = r.Client.Status().Update(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "error updating Shared PowerWorkload status")
				return ctrl.Result{}, err
			}
		} else {
			// Have to update the Shared pool before creating the pool for the newly created
			// Power Workload

			updatedSharedPool, id, err := r.removeCoresFromSharedPool(workload.Spec.Node.CpuIds, AppQoSClientAddress)
			if err != nil {
				logger.Error(err, "error retrieving Shared pool")
				return ctrl.Result{}, err
			}

			// Only update the AppQoS instance if there were any cores removed
			// In this instance where the Pool is being created, there will always be a removal of cores from the Shared pool
			if !reflect.DeepEqual(updatedSharedPool, &appqos.Pool{}) {
				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, AppQoSClientAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					return ctrl.Result{}, err
				}

				sharedWorkload := &powerv1alpha1.PowerWorkload{}
				err = r.Client.Get(context.TODO(), client.ObjectKey{
					Name:      fmt.Sprintf("shared-%s-workload", workload.Spec.Node.Name),
					Namespace: req.NamespacedName.Namespace,
				}, sharedWorkload)
				if err != nil {
					// If the Shared Workload is not found, we don't need to update the Status
					if !errors.IsNotFound(err) {
						logger.Error(err, "error retrieving Shared PowerWorkload")
						return ctrl.Result{}, err
					}
				} else {
					sharedWorkload.Status.SharedCores = *updatedSharedPool.Cores
					err = r.Client.Status().Update(context.TODO(), sharedWorkload)
					if err != nil {
						logger.Error(err, "error updating status of Shared PowerWorkload")
						return ctrl.Result{}, err
					}
				}
			}

			cbmDefault := 1

			pool := &appqos.Pool{}
			pool.Name = &req.NamespacedName.Name
			pool.Cores = &workload.Spec.Node.CpuIds
			pool.PowerProfile = powerProfileFromAppQoS.ID
			pool.Cbm = &cbmDefault

			appqosPostResponse, err := r.AppQoSClient.PostPool(pool, AppQoSClientAddress)
			if err != nil {
				logger.Error(err, appqosPostResponse)
				return ctrl.Result{}, err
			}
		}
	} else {
		// The pool already exists in AppQoS, so need to retrieve and update it

		// Must remove the CPUs assigned to the PowerWorkload from the Default pool,
		// then update the AppQoS instance with the newest config for the PowerWorkload,
		// and finally add any CPUs that were removed from the PowerWorkload's CPU list
		// back into the Default pool. It has to be done in this order as AppQoS will fail
		// if you try and assign CPUs to a new pool when they exist in another one

		updatedSharedPool, id, err := r.removeCoresFromSharedPool(workload.Spec.Node.CpuIds, AppQoSClientAddress)
		if err != nil {
			logger.Error(err, "error updating Shared pool")
			return ctrl.Result{}, err
		}

		// Only update the Shared Pool if there were cores removed
		if !reflect.DeepEqual(updatedSharedPool, &appqos.Pool{}) {
			appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, AppQoSClientAddress, id)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				return ctrl.Result{}, err
			}

			sharedWorkload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      fmt.Sprintf("shared-%s-workload", workload.Spec.Node.Name),
				Namespace: req.NamespacedName.Namespace,
			}, sharedWorkload)
			if err != nil {
				// The Shared Workload is not found, we don't need to update the Status
				if !errors.IsNotFound(err) {
					logger.Error(err, "error retrieving Shared PowerWorkload")
					return ctrl.Result{}, err
				}
			} else {
				sharedWorkload.Status.SharedCores = *updatedSharedPool.Cores
				err = r.Client.Status().Update(context.TODO(), sharedWorkload)
				if err != nil {
					logger.Error(err, "error updating Shared PowerWorkload Status")
					return ctrl.Result{}, err
				}
			}
		}

		returnedCPUs := util.CPUListDifference(workload.Spec.Node.CpuIds, *poolFromAppQoS.Cores)

		// Update the Workload's Pool (length of Core List in a Pool cannot be zero)
		updatedPool := &appqos.Pool{}
		updatedPool.Name = &req.NamespacedName.Name
		updatedPool.Cores = &workload.Spec.Node.CpuIds
		updatedPool.PowerProfile = powerProfileFromAppQoS.ID

		appqosPutResponse, err := r.AppQoSClient.PutPool(updatedPool, AppQoSClientAddress, *poolFromAppQoS.ID)
		if err != nil {
			logger.Error(err, appqosPutResponse)
			return ctrl.Result{}, err
		}

		if len(returnedCPUs) > 0 {
			updatedSharedPool, id, err := r.returnCoresToSharedPool(returnedCPUs, AppQoSClientAddress)
			if err != nil {
				logger.Error(err, "error updating Shared Pool")
				return ctrl.Result{}, err
			}

			appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, AppQoSClientAddress, id)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *PowerWorkloadReconciler) removeCoresFromSharedPool(workloadCPUList []int, nodeAddress string) (*appqos.Pool, int, error) {
	// Removes the CPUs in workloadCPUList from the Shared Pool if they exist. Returns an empty Pool if
	// no cores have been removed

	sharedPool, err := r.AppQoSClient.GetSharedPool(nodeAddress)
	if err != nil {
		return &appqos.Pool{}, 0, err
	}

	updatedSharedCoreList := util.CPUListDifference(workloadCPUList, *sharedPool.Cores)
	if len(updatedSharedCoreList) == 0 {
		// Return empty Pool so the calling function knows there's no need to update the AppQoS instance

		return &appqos.Pool{}, 0, nil
	}

	var updatedSharedPool *appqos.Pool
	var sharedPoolID int

	// The Default pool won't have an associated Power Profile
	if sharedPool.PowerProfile == nil {
		updatedSharedPool, sharedPoolID = updatePoolWithoutPowerProfile(updatedSharedCoreList, sharedPool)
	} else {
		updatedSharedPool, sharedPoolID = updatePool(updatedSharedCoreList, *sharedPool.PowerProfile, sharedPool)
	}

	return updatedSharedPool, sharedPoolID, nil
}

func (r *PowerWorkloadReconciler) returnCoresToSharedPool(returnedCPUs []int, nodeAddress string) (*appqos.Pool, int, error) {
	// Returns the cores that are no longer used by a Power Workload to the Shared Pool

	sharedPool, err := r.AppQoSClient.GetSharedPool(nodeAddress)
	if err != nil {
		return &appqos.Pool{}, 0, err
	}

	var updatedSharedPool *appqos.Pool
	var sharedPoolID int
	newSharedCPUList := append(*sharedPool.Cores, returnedCPUs...)
	sort.Ints(newSharedCPUList)

	// The Default pool won't have an associated Power Profile
	if sharedPool.PowerProfile == nil {
		updatedSharedPool, sharedPoolID = updatePoolWithoutPowerProfile(newSharedCPUList, sharedPool)
	} else {
		updatedSharedPool, sharedPoolID = updatePool(newSharedCPUList, *sharedPool.PowerProfile, sharedPool)
	}

	return updatedSharedPool, sharedPoolID, nil
}

func updatePool(newCPUList []int, newPowerProfile int, pool *appqos.Pool) (*appqos.Pool, int) {
	updatedPool := &appqos.Pool{}
	updatedPool.Name = pool.Name
	updatedPool.Cores = &newCPUList
	updatedPool.PowerProfile = &newPowerProfile

	return updatedPool, *pool.ID
}

func updatePoolWithoutPowerProfile(newCPUList []int, pool *appqos.Pool) (*appqos.Pool, int) {
	updatedPool := &appqos.Pool{}
	updatedPool.Name = pool.Name
	updatedPool.Cores = &newCPUList

	return updatedPool, *pool.ID
}

func (r *PowerWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerWorkload{}).
		Complete(r)
}
