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
	"reflect"
	"sort"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

// PowerWorkloadReconciler reconciles a PowerWorkload object
type PowerWorkloadReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	AppQoSClient *appqos.AppQoSClient
	State        *state.PowerNodeData
}

const (
	SharedWorkloadName string = "shared-workload"
	WorkloadNameSuffix string = "-workload"
	DefaultPool        string = "Default"
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
			// Assume PowerWorkload has been deleted. Check each Power Node to delete from each AppQoS instance

			obseleteWorkloads, err := r.findObseleteWorkloads(req, r.State.PowerNodeList)
			if err != nil {
				return ctrl.Result{}, err
			}

			for address, pool := range obseleteWorkloads {
				err = r.AppQoSClient.DeletePool(address, *pool.ID)
				if err != nil {
					logger.Error(err, "Failed to delete PowerWorkload from AppQoS")
					continue
				}

				// We only need to call returnCoresToSharedPool as no cores will be taken away
				updatedSharedPool, id, err := r.returnCoresToSharedPool(*pool.Cores, address)
				if err != nil {
					logger.Error(err, "Error updating Shared Pool")
					continue
				}

				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, address, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					continue
				}
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve PowerWorkload object")
		return ctrl.Result{}, err
	}

	// If the NodeInfo list in the Workload is empty just outright delete the Workload and let the deletion
	// part of the controller take care of it
	if len(workload.Spec.Nodes) == 0 {
		err = r.Client.Delete(context.TODO(), workload)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Remove Pools from AppQoS instances that were potentially removed from this Workload
	nodesToCheck := r.State.Difference(workload.Spec.Nodes)
	obseleteWorkloads, err := r.findObseleteWorkloads(req, nodesToCheck)
	if err != nil {
		return ctrl.Result{}, err
	}

	for address, pool := range obseleteWorkloads {
		err = r.AppQoSClient.DeletePool(address, *pool.ID)
		if err != nil {
			logger.Error(err, "Failed to delete PowerWorkload from AppQoS")
			continue
		}

		// We only need to call returnCoresToSharedPool as no cores will be taken away
		updatedSharedPool, id, err := r.returnCoresToSharedPool(*pool.Cores, address)
		if err != nil {
			logger.Error(err, "Error updating Shared Pool")
			continue
		}

		appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, address, id)
		if err != nil {
			logger.Error(err, appqosPutResponse)
			continue
		}
	}

	// Loop through all Power Nodes requested with this PowerWorkload

	for _, node := range workload.Spec.Nodes {
		nodeAddress, err := r.getPodAddress(node.Name)
		if err != nil {
			// Continue with other nodes if there's a failure with this one

			logger.Error(err, fmt.Sprintf("Failed to get IP address for node: %s", node.Name))
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		powerProfileFromAppQoS, err := r.AppQoSClient.GetProfileByName(workload.Spec.PowerProfile, nodeAddress)
		if err != nil {
			// If we can't find the Power Profile, fail out now without requeueing request"

			logger.Error(err, fmt.Sprintf("Error retrieving Power Profile from AppQoS on node %s", node.Name))
			continue
		}

		if reflect.DeepEqual(powerProfileFromAppQoS, &appqos.PowerProfile{}) {
			profileNotFoundError := errors.NewServiceUnavailable(fmt.Sprintf("Power Profile '%s' not found on node '%s'", workload.Spec.PowerProfile, node.Name))
			logger.Error(profileNotFoundError, "Error retrieving Power Profile")
			continue
		}

		poolFromAppQoS, err := r.AppQoSClient.GetPoolByName(nodeAddress, req.NamespacedName.Name)
		if err != nil {
			logger.Error(err, "Error retrieving Pool from AppQoS")
			continue
		}

		if reflect.DeepEqual(poolFromAppQoS, &appqos.Pool{}) {
			// Pool does not exist so we need to create it. We need to check first if it is the Shared Workload

			if workload.Spec.AllCores {
				// If this is being designated as the Shared Workload, we need to make sure the Shared pool
				// doesn't already exist. If it does, just bail out.

				sharedPoolFromAppQoS, err := r.AppQoSClient.GetPoolByName(nodeAddress, SharedWorkloadName)
				if err != nil {
					logger.Error(err, fmt.Sprintf("Error retrieving Shared pool from AppQoS on node '%s'", node.Name))
					continue
				}

				if !reflect.DeepEqual(sharedPoolFromAppQoS, &appqos.Pool{}) {
					// Shared pool exists, so we skip this node

					logger.Info(fmt.Sprintf("Shared pool already exists on node '%s'", node.Name))
					continue
				}

				sharedCores, err := r.removeNonReservedCPUsFromDefaultPool(workload.Spec.ReservedCPUs, nodeAddress)
				if err != nil {
					logger.Error(err, fmt.Sprintf("Failed removing ReservedCPUs from Default pool on node '%s'", node.Name))
					continue
				}

				if len(sharedCores) == 0 {
					logger.Info(fmt.Sprintf("Length of shared cores cannot be zero (node '%s'", node.Name))
					continue
				}

				// getUpdatedSharedPool will default to the Default pool as the Shared pool doesn't exist yet
				updatedDefaultPool, id, err := r.removeCoresFromSharedPool(sharedCores, nodeAddress)
				if err != nil {
					logger.Error(err, fmt.Sprintf("Failed updating Default pool on node '%s'", node.Name))
					continue
				}

				if !reflect.DeepEqual(updatedDefaultPool, &appqos.Pool{}) {
					appqosPutResponse, err := r.AppQoSClient.PutPool(updatedDefaultPool, nodeAddress, id)
					if err != nil {
						logger.Error(err, appqosPutResponse)
						continue
					}
				}

				// This is strange because we have the Shared Workload name as a constant, but you can't get the memory
				// address of a constant so we have to create a variable
				sharedName := "shared-workload"
				cbmDefault := 1

				pool := &appqos.Pool{}
				pool.Name = &sharedName
				pool.Cores = &sharedCores
				pool.PowerProfile = powerProfileFromAppQoS.ID
				pool.Cbm = &cbmDefault

				appqosPostResponse, err := r.AppQoSClient.PostPool(pool, nodeAddress)
				if err != nil {
					logger.Error(err, appqosPostResponse)
					continue
				}

				// Finally, update the PowerWorkload on the node so it contains the new Shared CPU list
				for _, node := range workload.Spec.Nodes {
					if node.Name == node.Name {
						node.CpuIds = sharedCores
						break
					}
				}
			} else {
				// Have to update the Shared pool before creating the pool for the newly created
				// Power Workload

				updatedSharedPool, id, err := r.removeCoresFromSharedPool(node.CpuIds, nodeAddress)
				if err != nil {
					logger.Error(err, fmt.Sprintf("Error retrieving Shared pool on node '%s'", node.Name))
					continue
				}

				// Only update the AppQoS instance if there were any cores removed
				// In this instance where the Pool is being created, there will always be a removal of cores from the Shared pool
				if !reflect.DeepEqual(updatedSharedPool, &appqos.Pool{}) {
					appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, nodeAddress, id)
					if err != nil {
						logger.Error(err, appqosPutResponse)
						continue
					}
				}

				cbmDefault := 1

				pool := &appqos.Pool{}
				pool.Name = &req.NamespacedName.Name
				pool.Cores = &node.CpuIds
				pool.PowerProfile = powerProfileFromAppQoS.ID
				pool.Cbm = &cbmDefault

				appqosPostResponse, err := r.AppQoSClient.PostPool(pool, nodeAddress)
				if err != nil {
					logger.Error(err, appqosPostResponse)
					continue
				}
			}
		} else {
			// The pool already exists in AppQoS, so need to retrieve and update it

			if req.NamespacedName.Name == SharedWorkloadName {
				// Figure out this. May need to do some housekeeping to see if anything
				// has changed, view other commit for this. If not can just skip

				logger.Info("Shared Profile, skipping for now...")
				continue
			}

			// Must remove the CPUs assigned to the PowerWorkload from the Default pool,
			// then update the AppQoS instance with the newest config for the PowerWorkload,
			// and finally add any CPUs that were removed from the PowerWorkload's CPU list
			// back into the Default pool. It has to be done in this order as AppQoS will fail
			// if you try and assign CPUs to a new pool when they exist in another one

			updatedSharedPool, id, err := r.removeCoresFromSharedPool(node.CpuIds, nodeAddress)
			if err != nil {
				logger.Error(err, fmt.Sprintf("Failed updating Shared pool on node '%s'", node.Name))
				continue
			}

			// Only update the Shared Pool if there were cores removed
			if !reflect.DeepEqual(updatedSharedPool, &appqos.Pool{}) {
				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, nodeAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					continue
				}
			}

			returnedCPUs := util.CPUListDifference(node.CpuIds, *poolFromAppQoS.Cores)

			// Update the Workload (length of Core List in a Pool cannot be zero)
			updatedPool := &appqos.Pool{}
			updatedPool.Name = &req.NamespacedName.Name
			updatedPool.Cores = &node.CpuIds
			updatedPool.PowerProfile = powerProfileFromAppQoS.ID

			appqosPutResponse, err := r.AppQoSClient.PutPool(updatedPool, nodeAddress, *poolFromAppQoS.ID)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				continue
			}

			if len(returnedCPUs) > 0 {
				updatedSharedPool, id, err := r.returnCoresToSharedPool(returnedCPUs, nodeAddress)

				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, nodeAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					continue
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *PowerWorkloadReconciler) findObseleteWorkloads(req ctrl.Request, nodes []string) (map[string]*appqos.Pool, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	obseleteWorkloads := make(map[string]*appqos.Pool, 0)

	for _, nodeName := range nodes {
		address, err := r.getPodAddress(nodeName)
		if err != nil {
			return nil, err
		}

		pool, err := r.AppQoSClient.GetPoolByName(address, req.NamespacedName.Name)
		if err != nil {
			return nil, err
		}

		if reflect.DeepEqual(pool, &appqos.Pool{}) {
			logger.Info(fmt.Sprintf("PowerWorkload not found in AppQoS instance on node '%s'", nodeName))
			continue
		}

		obseleteWorkloads[address] = pool
	}

	return obseleteWorkloads, nil
}

func (r *PowerWorkloadReconciler) getPodAddress(nodeName string) (string, error) {
	_ = context.Background()
	logger := r.Log.WithName("getPodAddress")

	pods := &corev1.PodList{}
	err := r.Client.List(context.TODO(), pods, client.MatchingLabels(client.MatchingLabels{"name": AppQoSPodLabel}))
	if err != nil {
		logger.Error(err, "Failed to list AppQoS pods")
		return "", nil
	}

	appqosPod, err := util.GetPodFromNodeName(pods, nodeName)
	if err != nil {
		appqosNode := &corev1.Node{}
		err := r.Client.Get(context.TODO(), client.ObjectKey{
			Name: nodeName,
		}, appqosNode)
		if err != nil {
			logger.Error(err, "Error getting AppQoS node")
			return "", err
		}

		appqosPod, err = util.GetPodFromNodeAddresses(pods, appqosNode)
		if err != nil {
			return "", err
		}
	}

	addressPrefix := r.AppQoSClient.GetAddressPrefix()
	podHostname := appqosPod.Spec.Hostname
	podSubdomain := appqosPod.Spec.Subdomain
	fullHostname := fmt.Sprintf("%s%s.%s:%d", addressPrefix, podHostname, podSubdomain, appqosPod.Spec.Containers[0].Ports[0].ContainerPort)

	return fullHostname, nil

}

func (r *PowerWorkloadReconciler) getCorrectSharedPool(nodeAddress string) (*appqos.Pool, error) {
	// Returns the correct Shared Pool (either Default or Shared). First it looks in AppQoS for the
	// Shared pool and then if that isn't found gets the Default pool

	sharedPool, err := r.AppQoSClient.GetPoolByName(nodeAddress, SharedWorkloadName)
	if err != nil {
		return &appqos.Pool{}, err
	}

	if reflect.DeepEqual(sharedPool, &appqos.Pool{}) {
		// If the Shared Pool does not exist, we have to use the Default Pool

		sharedPool, err = r.AppQoSClient.GetPoolByName(nodeAddress, DefaultPool)
		if err != nil {
			return &appqos.Pool{}, err
		}

		if reflect.DeepEqual(sharedPool, &appqos.Pool{}) {
			notFoundError := errors.NewServiceUnavailable("Default Pool not found")
			return &appqos.Pool{}, notFoundError
		}
	}

	return sharedPool, nil
}

func (r *PowerWorkloadReconciler) removeCoresFromSharedPool(workloadCPUList []int, nodeAddress string) (*appqos.Pool, int, error) {
	// Removes the CPUs in workloadCPUList from the Shared Pool if they exist. Returns an empty Pool if
	// no cores have been removed

	sharedPool, err := r.getCorrectSharedPool(nodeAddress)
	if err != nil {
		return &appqos.Pool{}, 0, err
	}

	updatedSharedCoreList := util.CPUListDifference(workloadCPUList, *sharedPool.Cores)
	if len(updatedSharedCoreList) == 0 {
		// Return empty Pool so the calling function knows there's no need to update the AppQoS instance

		return &appqos.Pool{}, 0, nil
	}

	updatedSharedPool := &appqos.Pool{}
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

	sharedPool, err := r.getCorrectSharedPool(nodeAddress)
	if err != nil {
		return &appqos.Pool{}, 0, err
	}

	updatedSharedPool := &appqos.Pool{}
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

func (r *PowerWorkloadReconciler) removeNonReservedCPUsFromDefaultPool(reservedCPUs []int, nodeAddress string) ([]int, error) {
	sharedCores := make([]int, 0)

	defaultPool, err := r.AppQoSClient.GetPoolByName(nodeAddress, DefaultPool)
	if err != nil {
		return sharedCores, err
	}

	if reflect.DeepEqual(defaultPool, &appqos.Pool{}) {
		notFoundError := errors.NewServiceUnavailable("Default Pool not found")
		return sharedCores, notFoundError
	}

	return util.CPUListDifference(reservedCPUs, *defaultPool.Cores), nil
}

func (r *PowerWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerWorkload{}).
		Complete(r)
}
