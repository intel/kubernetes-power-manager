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

				// =================================================
				// ====================IMPORTANT====================
				// =================================================
				// updateDefaultPool() has O(n) where
				// n = len(defaultPool.Cores) even though the
				// previous list it's comparing to is empty.
				// Consider (actaully do) change this implementation
				// =================================================
				// ====================IMPORTANT====================
				// =================================================

				// Pass in an emtpy slice so no CPUs are removed from the Shared pool
				updatedSharedPool, id, err := r.getUpdatedSharedPool([]int{}, address)
				updatedSharedCPUs := append(*updatedSharedPool.Cores, *pool.Cores...)
				sort.Ints(updatedSharedCPUs)
				updatedSharedPool.Cores = &updatedSharedCPUs
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
	logger.Info(fmt.Sprintf("Checking nodes: %v", nodesToCheck))
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

		updatedSharedPool, id, err := r.getUpdatedSharedPool([]int{}, address)
		updatedSharedCPUs := append(*updatedSharedPool.Cores, *pool.Cores...)
		sort.Ints(updatedSharedCPUs)
		updatedSharedPool.Cores = &updatedSharedCPUs
		appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, address, id)
		if err != nil {
			logger.Error(err, appqosPutResponse)
			continue
		}
	}

	// Loop through all Power Nodes requested with this PowerWorkload

	for _, targetNode := range workload.Spec.Nodes {
		logger.Info(fmt.Sprintf("NodeInfo: %v", workload.Spec.Nodes))
		nodeAddress, err := r.getPodAddress(targetNode.Name, req)
		if err != nil {
			// Continue with other nodes if there's a failure with this one
			logger.Error(err, fmt.Sprintf("Failed to get IP address for node: %s", targetNode.Name))
			continue
		}

		if poolFromAppQos, exists, err := r.AppQoSClient.GetPoolByName(nodeAddress, req.NamespacedName.Name); exists {
			if err != nil {
				logger.Error(err, "Failed while retreiving pool")
				continue
			}

			if req.NamespacedName.Name == SharedWorkloadName {
				defaultPool, _, err := r.AppQoSClient.GetPoolByName(nodeAddress, DefaultPool)
				if err != nil {
					logger.Error(err, "Failed retrieving Default pool from AppQoS")
					continue
				}

				updatedSharedPool, sharedPoolID := updatePool(*poolFromAppQos.Cores, *poolFromAppQos.PowerProfile, poolFromAppQos)

				sharedCoresWithoutReservedCPUs := util.CPUListDifference(workload.Spec.ReservedCPUs, *poolFromAppQos.Cores)
				if len(sharedCoresWithoutReservedCPUs) > 0 {
					updatedSharedPool, sharedPoolID = updatePool(sharedCoresWithoutReservedCPUs, *poolFromAppQos.PowerProfile, poolFromAppQos)
					appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, nodeAddress, sharedPoolID)
					if err != nil {
						logger.Error(err, appqosPutResponse)
						continue
					}
				}

				sharedCoresRetrievedFromDefaultPool, err := r.removeNonReservedCPUsFromDefaultPool(workload.Spec.ReservedCPUs, nodeAddress)
                                if err != nil {
                                        logger.Error(err, "Failed trying to remove ReservedCPUs from Default pool")
                                        continue
                                }

				if len(sharedCoresWithoutReservedCPUs) > 0 || len(sharedCoresRetrievedFromDefaultPool) > 0 {
					updatedDefaultPool, defaultPoolID := updatePoolWithoutPowerProfile(workload.Spec.ReservedCPUs, defaultPool)
					appqosPutResponse, err := r.AppQoSClient.PutPool(updatedDefaultPool, nodeAddress, defaultPoolID)
					if err != nil {
						logger.Error(err, appqosPutResponse)
                                                continue
					}
				}

				if len(sharedCoresRetrievedFromDefaultPool) > 0 {
					newSharedCores := append(*updatedSharedPool.Cores, sharedCoresRetrievedFromDefaultPool...)
					updatedSharedPool, sharedPoolID = updatePool(newSharedCores, *poolFromAppQos.PowerProfile, poolFromAppQos)
					appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, nodeAddress, sharedPoolID)
					if err != nil {
                                                logger.Error(err, appqosPutResponse)
                                                continue
                                        }
				}

				continue
			}

			// Must remove the CPUs assigned to the PowerWorkload from the Default pool,
			// then update the AppQoS instance with the newest config for the PowerWorkload,
			// and finally add any CPUs that were removed from the PowerWorkload's CPU list
			// back into the Default pool. It has to be done in this order as AppQoS will fail
			// if you try and assign CPUs to a new pool when they exist in another one

			// Check if CPUs were removed from PowerWorkload and need to be added back to Default
			// and update the Default pool in AppQoS instance

			// MAYBE DO A CHECK TO SEE IF LIST IS EMPTY, NO POINT PUTTING IF IT IS
			returnedCPUs := util.CPUListDifference(targetNode.CpuIds, *poolFromAppQos.Cores)
			updatedSharedPool, id, err := r.getUpdatedSharedPool(returnedCPUs, nodeAddress)
			if err != nil {
				logger.Error(err, "Failed updating Shared pool")
				continue
			}

			appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, nodeAddress, id)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				continue
			}

			// Update the Workload (length of Core List in a Pool cannot be zero)
			updatedPool := &appqos.Pool{}
			updatedPool.Name = poolFromAppQos.Name
			updatedPool.Cores = &targetNode.CpuIds
			updatedPool.PowerProfile = &workload.Spec.PowerProfile
			appqosPutResponse, err = r.AppQoSClient.PutPool(updatedPool, nodeAddress, *poolFromAppQos.ID)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				continue
			}
		} else {
			if workload.Spec.AllCores {
				if _, exists, err := r.AppQoSClient.GetPoolByName(nodeAddress, SharedWorkloadName); exists {
					if err != nil {
						logger.Error(err, "Failed retreiving Shared workload")
						continue
					}

					logger.Info("Shared workload already exist")

					// FOR NOW JUST RETURN, WILL COME BACK TO THIS
					return ctrl.Result{}, nil
				}

				sharedCores, err := r.removeNonReservedCPUsFromDefaultPool(workload.Spec.ReservedCPUs, nodeAddress)
				if err != nil {
					logger.Error(err, "Failed trying to remove ReservedCPUs from Default pool")
					continue
				}

				if len(sharedCores) == 0 {
					logger.Info("Length of shared cores cannot be zero")
					continue
				}

				// getUpdatedSharedPool will default to the Default pool as the Shared pool doesn't exist yet
				updatedDefaultPool, id, err := r.getUpdatedSharedPool(sharedCores, nodeAddress)
				if err != nil {
					logger.Error(err, "Failed updating Default pool")
					continue
				}

				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedDefaultPool, nodeAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					continue
				}

				sharedName := "shared-workload"

				pool := &appqos.Pool{}
				pool.Name = &sharedName
				//pool.Name = &SharedWorkloadName
				pool.Cores = &sharedCores
				// CHANGE BELOW TO USE THE SHARED PROFILE (MAY NEED ERROR CHECKING OR JUST MAKE THEM PROVIDE IT)
				pool.PowerProfile = &workload.Spec.PowerProfile
				cbmDefault := 1
				pool.Cbm = &cbmDefault

				appqosPostResponse, err := r.AppQoSClient.PostPool(pool, nodeAddress)
				if err != nil {
					logger.Error(err, appqosPostResponse)
					continue
				}

				// Finally, update the PwerWorkload on the node so it contains the new Shared CPU list
				for _, node := range workload.Spec.Nodes {
					if node.Name == targetNode.Name {
						node.CpuIds = sharedCores
						break
					}
				}
			} else {
				updatedSharedPool, id, err := r.getUpdatedSharedPool(targetNode.CpuIds, nodeAddress)
				if err != nil {
					logger.Error(err, "Failed updating Shared pool")
					continue
				}

				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedSharedPool, nodeAddress, id)
				if err != nil {
					logger.Error(err, appqosPutResponse)
					continue
				}

				pool := &appqos.Pool{}
				pool.Name = &req.NamespacedName.Name
				pool.Cores = &targetNode.CpuIds
				pool.PowerProfile = &workload.Spec.PowerProfile
				cbmDefault := 1
				pool.Cbm = &cbmDefault

				appqosPostResponse, err := r.AppQoSClient.PostPool(pool, nodeAddress)
				if err != nil {
					logger.Error(err, appqosPostResponse)
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
		address, err := r.getPodAddress(nodeName, req)
		if err != nil {
			return nil, err
		}

		if pool, exists, err := r.AppQoSClient.GetPoolByName(address, req.NamespacedName.Name); exists {
			if err != nil {
				return nil, err
			}

			obseleteWorkloads[address] = pool
		} else {
			logger.Info("PowerWorkload not found on AppQoS instance")
			continue
		}
	}

	return obseleteWorkloads, nil
}

func (r *PowerWorkloadReconciler) getPodAddress(nodeName string, req ctrl.Request) (string, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	// TODO: DELETE WHEN APPQOS CONTAINERIZED
	if 1 == 1 {
		node := &corev1.Node{}
		err := r.Client.Get(context.TODO(), client.ObjectKey{
			Name: nodeName,
		}, node)
		if err != nil {
			return "", err
		}
		address := fmt.Sprintf("%s%s%s", "https://", node.Status.Addresses[0].Address, ":5000")
		return address, nil
	}

	pods := &corev1.PodList{}
	err := r.Client.List(context.TODO(), pods, client.MatchingLabels(client.MatchingLabels{"name": PowerPodNameConst}))
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

	var podIP string
	notFoundError := errors.NewServiceUnavailable("pod address not available")
	if appqosPod.Status.PodIP != "" {
		podIP = appqosPod.Status.PodIP
	} else if len(appqosPod.Status.PodIPs) != 0 {
		podIP = appqosPod.Status.PodIPs[0].IP
	} else {
		return "", notFoundError
	}

	if len(appqosPod.Spec.Containers) == 0 {
		return "", notFoundError
	}

	if len(appqosPod.Spec.Containers[0].Ports) == 0 {
		return "", notFoundError
	}

	addressPrefix := r.AppQoSClient.GetAddressPrefix()
	address := fmt.Sprintf("%s%s%s%d", addressPrefix, podIP, ":", appqosPod.Spec.Containers[0].Ports[0].ContainerPort)
	return address, nil
}

func (r *PowerWorkloadReconciler) getUpdatedSharedPool(workloadCPUList []int, nodeAddress string) (*appqos.Pool, int, error) {
	// Returns the updated Shared pool along with the corresponding id for it
	// Uses Shared pool if it exists, Default pool otherwise

	sharedPool := &appqos.Pool{}

	if sharedPoolFromAppQoS, exists, err := r.AppQoSClient.GetPoolByName(nodeAddress, SharedWorkloadName); exists {
		if err != nil {
			return &appqos.Pool{}, 0, err
		}

		sharedPool = sharedPoolFromAppQoS
	} else {
		defaultPool, _, err := r.AppQoSClient.GetPoolByName(nodeAddress, DefaultPool)
		if err != nil {
			return &appqos.Pool{}, 0, err
		}

		sharedPool = defaultPool
	}

	updatedSharedCoreList := util.CPUListDifference(workloadCPUList, *sharedPool.Cores)
	updatedSharedPool := &appqos.Pool{}
	var sharedPoolID int
	if sharedPool.PowerProfile == nil {
		updatedSharedPool, sharedPoolID = updatePoolWithoutPowerProfile(updatedSharedCoreList, sharedPool)
	} else {
		updatedSharedPool, sharedPoolID = updatePool(updatedSharedCoreList, *sharedPool.PowerProfile, sharedPool)
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

	defaultPool, _, err := r.AppQoSClient.GetPoolByName(nodeAddress, DefaultPool)
	if err != nil {
		return sharedCores, err
	}

	return util.CPUListDifference(reservedCPUs, *defaultPool.Cores), nil
}

func (r *PowerWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerWorkload{}).
		Complete(r)
}
