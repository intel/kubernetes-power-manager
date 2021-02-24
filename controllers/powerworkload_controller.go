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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/newstate"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

// PowerWorkloadReconciler reconciles a PowerWorkload object
type PowerWorkloadReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	AppQoSClient *appqos.AppQoSClient
	State        *newstate.PowerNodeData
	//State  *workloadstate.Workloads
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

			obseleteWorkloads, err := r.findObseleteWorkloads(req)
			if err != nil {
				return ctrl.Result{}, err
			}

			for address, pool := range obseleteWorkloads {
				err = r.AppQoSClient.DeletePool(address, *pool.ID)
				if err != nil {
					logger.Error(err, "Failed to delete PowerWorkload from AppQoS")
					continue
				}

				defaultPool, err := r.AppQoSClient.GetPoolByName(address, DefaultPool)
				if err != nil {
					logger.Error(err, "Failed to retrieve Default pool")
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
				updatedDefaultPool := updateDefaultPool([]int{}, defaultPool)
				updatedDefaultCPUs := append(*defaultPool.Cores, *pool.Cores...)
				sort.Ints(updatedDefaultCPUs)
				updatedDefaultPool.Cores = &updatedDefaultCPUs
				appqosPutResponse, err := r.AppQoSClient.PutPool(updatedDefaultPool, address, *defaultPool.ID)
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

	// Loop through all Power Nodes requested with this PowerWorkload

	targetNodes := workload.Spec.Nodes

	for _, targetNode := range targetNodes {
		nodeAddress, err := r.getPodAddress(targetNode, req)
		if err != nil {
			// Continue with other nodes if there's a failure with this one
			logger.Error(err, "Failed to get IP address for node: ", targetNode)
			continue
		}

		defaultPool, err := r.AppQoSClient.GetPoolByName(nodeAddress, DefaultPool)
		if err != nil {
			logger.Error(err, "Failed to retrieve Default pool")
			return ctrl.Result{}, nil
		}

		poolFromAppQos, err := r.AppQoSClient.GetPoolByName(nodeAddress, req.NamespacedName.Name)
		if err != nil {
			logger.Error(err, "Failed while retreiving pool")
			continue
		}

		if !reflect.DeepEqual(poolFromAppQos, &appqos.Pool{}) {
			// Must remove the CPUs assigned to the PowerWorkload from the Default pool,
			// then update the AppQoS instance with the newest config for the PowerWorkload,
			// and finally add any CPUs that were removed from the PowerWorkload's CPU list
			// back into the Default pool. It has to be done in this order as AppQoS will fail
			// if you try and assign CPUs to a new pool when they exist in another one

			// Check if CPUs were removed from PowerWorkload and need to be added back to Default
			// and update the Default pool in AppQoS instance
			returnedCPUs := cpusRemovedFromWorkload(*poolFromAppQos.Cores, workload.Spec.CpuIds)
			updatedDefaultPool := updateDefaultPool(workload.Spec.CpuIds, defaultPool)
			appqosPutResponse, err := r.AppQoSClient.PutPool(updatedDefaultPool, nodeAddress, *defaultPool.ID)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				return ctrl.Result{}, nil
			}

			// Update the Workload
			updatedPool := &appqos.Pool{}
			updatedPool.Name = poolFromAppQos.Name
			updatedPool.Cores = &workload.Spec.CpuIds
			updatedPool.PowerProfile = &workload.Spec.PowerProfile
			appqosPutResponse, err = r.AppQoSClient.PutPool(updatedPool, nodeAddress, *poolFromAppQos.ID)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				continue
			}

			// Return the CPUs that were removed from the Workload to the Default pool
			updatedDefaultCPUList := append(*updatedDefaultPool.Cores, returnedCPUs...)
			sort.Ints(updatedDefaultCPUList)
			updatedDefaultPool.Cores = &updatedDefaultCPUList
			appqosPutResponse, err = r.AppQoSClient.PutPool(updatedDefaultPool, nodeAddress, *defaultPool.ID)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				return ctrl.Result{}, nil
			}
		} else {
			updatedDefaultPool := updateDefaultPool(workload.Spec.CpuIds, defaultPool)
			appqosPutResponse, err := r.AppQoSClient.PutPool(updatedDefaultPool, nodeAddress, *defaultPool.ID)
			if err != nil {
				logger.Error(err, appqosPutResponse)
				return ctrl.Result{}, nil
			}

			pool := &appqos.Pool{}
			pool.Name = &req.NamespacedName.Name
			pool.Cores = &workload.Spec.CpuIds
			pool.PowerProfile = &workload.Spec.PowerProfile
			cbmDefault := 1
			pool.Cbm = &cbmDefault

			appqosPostResponse, err := r.AppQoSClient.PostPool(pool, "https://localhost:5000")
			if err != nil {
				logger.Error(err, appqosPostResponse)
				continue
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *PowerWorkloadReconciler) findObseleteWorkloads(req ctrl.Request) (map[string]*appqos.Pool, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	obseleteWorkloads := make(map[string]*appqos.Pool, 0)

	for _, nodeName := range r.State.PowerNodeList {
		address, err := r.getPodAddress(nodeName, req)
		if err != nil {
			return nil, err
		}

		pool, err := r.AppQoSClient.GetPoolByName(address, req.NamespacedName.Name)
		if err != nil {
			return nil, err
		}
		if *pool.Name == "" {
			logger.Info("PowerWorkload not found on AppQoS instance")
			continue
		}

		obseleteWorkloads[address] = pool
	}

	return obseleteWorkloads, nil
}

func (r *PowerWorkloadReconciler) getPodAddress(nodeName string, req ctrl.Request) (string, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	// TODO: DELETE WHEN APPQOS CONTAINERIZED
	if 1 == 1 {
		return "https://localhost:5000", nil
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

func updateDefaultPool(workloadCPUList []int, defaultPool *appqos.Pool) *appqos.Pool {
	updatedDefaultPool := &appqos.Pool{}

	updatedDefaultCoreList := removeCPUsFromCPUList(workloadCPUList, *defaultPool.Cores)
	updatedDefaultPool.Name = defaultPool.Name
	updatedDefaultPool.PowerProfile = defaultPool.PowerProfile
	updatedDefaultPool.Cores = &updatedDefaultCoreList

	return updatedDefaultPool
}

func cpusRemovedFromWorkload(previousCPUList []int, updatedCPUList []int) []int {
	returnedCPUs := make([]int, 0)

	for _, poolCPU := range previousCPUList {
		if !cpuInCPUList(poolCPU, updatedCPUList) {
			returnedCPUs = append(returnedCPUs, poolCPU)
		}
	}

	return returnedCPUs
}

func removeCPUsFromCPUList(cpusToRemove []int, cpuList []int) []int {
	updatedCPUList := make([]int, 0)

	for _, cpu := range cpuList {
		if !cpuInCPUList(cpu, cpusToRemove) {
			updatedCPUList = append(updatedCPUList, cpu)
		}
	}

	return updatedCPUList
}

/*
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
*/

func cpuInCPUList(cpu int, cpuList []int) bool {
	for _, cpuListID := range cpuList {
		if cpu == cpuListID {
			return true
		}
	}

	return false
}

func IdInCPUList(id string, cpuList []string) bool {
	for _, cpuListID := range cpuList {
		if id == cpuListID {
			return true
		}
	}

	return false
}

/*
func removeSubsetFromWorkload(toRemove []string, cpuList []string) []string {
	updatedCPUList := make([]string, 0)

	for _, cpuID := range cpuList {
		if !IdInCPUList(cpuID, toRemove) {
			updatedCPUList = append(updatedCPUList, cpuID)
		}
	}

	return updatedCPUList
}
*/

func (r *PowerWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerWorkload{}).
		Complete(r)
}
