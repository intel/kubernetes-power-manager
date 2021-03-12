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
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podresourcesclient"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podstate"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
)

const (
	PowerProfileAnnotation = "PowerProfile"
	ResourcePrefix = "power.intel.com/"
)

// PowerPodReconciler reconciles a PowerPod object
type PowerPodReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	State              podstate.State
	PodResourcesClient podresourcesclient.PodResourcesClient
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerpods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerpods/status,verbs=get;update;patch

func (r *PowerPodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerpod", req.NamespacedName)

	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), req.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Maybe do a check to see if the Pod is still recorded in State for some reason

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve Pod")
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Pod deletion stamp: %v", pod.ObjectMeta.DeletionTimestamp))

	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		// If the Pod's DeletionTimestamp is not zero then the Pod has been deleted

		powerPodState := r.State.GetPodFromState(pod.GetName())
		workloadToCPUsRemoved := make(map[string][]int, 0)
		for _, container := range powerPodState.Containers {
			profile := container.PowerProfile
			cpus := container.ExclusiveCPUs
			if _, exists := workloadToCPUsRemoved[profile]; exists {
				workloadToCPUsRemoved[profile] = append(workloadToCPUsRemoved[profile], cpus...)
			} else {
				workloadToCPUsRemoved[profile] = cpus
			}
		}

		logger.Info(fmt.Sprintf("Workloads and CPUs: %v", workloadToCPUsRemoved))

		for workloadName, cpus := range workloadToCPUsRemoved {
			workload := &powerv1alpha1.PowerWorkload{}
			err = r.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name: workloadName,
			}, workload)

			if err != nil {
				if errors.IsNotFound(err) {
					return ctrl.Result{}, nil
				}
				logger.Error(err, "error while trying to retrieve PowerWorkload")
				return ctrl.Result{}, err
			}

			updatedWorkloadCPUList := getNewWorkloadCPUList(powerPodState.Node, cpus, workload.Spec.Nodes)
			for i, node := range workload.Spec.Nodes {
				if node.Name == powerPodState.Node {
					if len(updatedWorkloadCPUList) == 0 {
						workload.Spec.Nodes = append(workload.Spec.Nodes[:i], workload.Spec.Nodes[i+1:]...)
					} else {
						workload.Spec.Nodes[i].CpuIds = updatedWorkloadCPUList
					}
					break
				}
			}

			logger.Info(fmt.Sprintf("Updated workload cpus: %v", updatedWorkloadCPUList))
			logger.Info(fmt.Sprintf("Updated workload spec: %v", workload))

			err = r.Update(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "Failed updating PowerWorkload")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// If the Pod's DeletionTimestamp is equal to zero then the Pod has been created or updated

	// Make sure the Pod is running
	podNotRunningErr := errors.NewServiceUnavailable("pod not in running phase")
	if pod.Status.Phase != corev1.PodRunning {
		logger.Info("Pod not running", "pod status:", pod.Status.Phase)
		return ctrl.Result{}, podNotRunningErr
	}

	// Get the Containers of the Pod that are requesting exclusive CPUs
	profiles := make(map[string][]int, 0)
	containersRequestingExclusiveCPUs := getContainersRequestingExclusiveCPUs(pod)
	if len(containersRequestingExclusiveCPUs) == 0 {
		logger.Info("No containers are requesting exclusive CPUs")
		return ctrl.Result{}, nil
	}

	guaranteedPod := powerv1alpha1.GuaranteedPod{}
	guaranteedPod.Node = pod.Spec.NodeName
	guaranteedPod.Name = pod.GetName()
	podUID := pod.GetUID()
	if podUID == "" {
		logger.Info("No pod UID found")
		return ctrl.Result{}, errors.NewServiceUnavailable("pod UID not found")
	}
	guaranteedPod.UID = string(podUID)

	powerContainers := make([]powerv1alpha1.Container, 0)

	for _, container := range containersRequestingExclusiveCPUs {
		containerProfile := getContainerProfileFromRequests(container)
		// COME BACK AND THINK ABOUT THIS
		if containerProfile == "" {
			continue
		} else if containerProfile == "Cannot have more than one profile per container" {
			logger.Info("Cannot have more than one PowerProfile per container"
			continue
		}

		containerID := getContainerID(pod, container.Name)
		coreIDs, err := r.PodResourcesClient.GetContainerCPUs(guaranteedPod.Name, container.Name)

		if err != nil {
			logger.Error(err, "error call to PodResourcesClient")
			return ctrl.Result{}, nil
		}

		// ============================================
		// ===================DELETE===================
		// ============================================
		logger.Info("********************************")
		logger.Info(fmt.Sprintf("CoreIDs: %v", coreIDs))
		// ============================================
		// ===================DELETE===================
		// ============================================

		powerContainer := &powerv1alpha1.Container{}
		powerContainer.Name = container.Name
		powerContainer.ID = strings.TrimPrefix(containerID, "docker://")
		cleanCoreList := getCleanCoreList(coreIDs)
		powerContainer.ExclusiveCPUs = cleanCoreList
		powerContainer.PowerProfile = containerProfile

		if _, exists := profiles[containerProfile]; exists {
			profiles[containerProfile] = append(profiles[containerProfile], cleanCoreList...)
		} else {
			profiles[containerProfile] = cleanCoreList
		}
		powerContainers = append(powerContainers, *powerContainer)
	}
	guaranteedPod.Containers = make([]powerv1alpha1.Container, 0)
	guaranteedPod.Containers = powerContainers

	err = r.State.UpdateStateGuaranteedPods(guaranteedPod)
	if err != nil {
		logger.Error(err, "error updating internal state")
		return ctrl.Result{}, err
	}

	for workloadName, cpuList := range profiles {
		workload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: req.NamespacedName.Namespace,
			Name:      workloadName,
		}, workload)
		if err != nil {
			if errors.IsNotFound(err) {
				// This is the first Pod to request this PowerProfile, need to create corresponding
				// PowerWorkload

				nodeInfo := []powerv1alpha1.NodeInfo{
					powerv1alpha1.NodeInfo{
						Name:   pod.Spec.NodeName,
						CpuIds: cpuList,
					},
				}

				powerProfileID := getPowerProfileIDFromName("replace when the time is right")

				workloadSpec := &powerv1alpha1.PowerWorkloadSpec{
					Nodes:        nodeInfo,
					PowerProfile: powerProfileID,
				}
				workload = &powerv1alpha1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: req.NamespacedName.Namespace,
						Name:      workloadName,
					},
				}
				workload.Spec = *workloadSpec
				err = r.Client.Create(context.TODO(), workload)
				if err != nil {
					logger.Error(err, "error while trying to create PowerWorkload")
					return ctrl.Result{}, err
				}

				continue
			}

			logger.Error(err, "error while trying to retrieve PowerWorkload")
			continue
		}

		// PowerWorkload already exists so need to update it. If the Node already
		// exists in the Workload, we update the Node's CPU list, if not we create
		// the entry for the node

		nodeExisted := false
		for i, node := range workload.Spec.Nodes {
			if node.Name == pod.Spec.NodeName {
				nodeExisted = true
				workload.Spec.Nodes[i].CpuIds = appendIfUnique(node.CpuIds, cpuList)
				sort.Ints(workload.Spec.Nodes[i].CpuIds)
			}
		}

		if !nodeExisted {
			nodeInfo := powerv1alpha1.NodeInfo{
				Name:   pod.Spec.NodeName,
				CpuIds: cpuList,
			}

			workload.Spec.Nodes = append(workload.Spec.Nodes, nodeInfo)
		}

		err = r.Client.Update(context.TODO(), workload)
		if err != nil {
			logger.Error(err, "error while trying to update PowerWorkload")
			//return ctrl.Result{}, err
			continue
		}
	}

	return ctrl.Result{}, nil
}

func getNewWorkloadCPUList(nodeName string, cpuList []int, nodeInfoList []powerv1alpha1.NodeInfo) []int {
	updatedWorkloadCPUList := make([]int, 0)

	for _, node := range nodeInfoList {
		if node.Name != nodeName {
			continue
		}

		for _, cpu := range node.CpuIds {
			if !util.CPUInCPUList(cpu, cpuList) {
				updatedWorkloadCPUList = append(updatedWorkloadCPUList, cpu)
			}
		}
	}

	return updatedWorkloadCPUList
}

func appendIfUnique(cpuList []int, cpus []int) []int {
	for _, cpu := range cpus {
		if !util.CPUInCPUList(cpu, cpuList) {
			cpuList = append(cpuList, cpu)
		}
	}

	return cpuList
}

func getPowerProfileIDFromName(profileName string) int {
	// ==========================
	// ==========DELETE==========
	// ==========================
	if profileName == profileName {
		return 0
	}

	return 0
}

func getContainerProfileFromRequests(container corev1.Container) string {
	profileName := ""

	for resource, _ := range container.Resources.Requests {
		if strings.HasPrefix(string(resource), ResourcePrefix) {
			if profileName == "" {
				profileName = string(resource[len(ResourcePrefix):])
			} else {
				// Cannot have more than one profile for a singular container
				return "Cannot have more than one profile per container"
			}
		}
	}

	return profileName
}

func getContainersRequestingExclusiveCPUs(pod *corev1.Pod) []corev1.Container {
	containersRequestingExclusiveCPUs := make([]corev1.Container, 0)
	for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		if exclusiveCPUs(pod, &container) {
			containersRequestingExclusiveCPUs = append(containersRequestingExclusiveCPUs, container)
			//containers = append(containers, container)
		}
	}

	return containersRequestingExclusiveCPUs
}

func exclusiveCPUs(pod *corev1.Pod, container *corev1.Container) bool {
	if pod.Status.QOSClass != corev1.PodQOSGuaranteed {
		return false
	}

	cpuQuantity := container.Resources.Requests[corev1.ResourceCPU]
	if cpuQuantity.Value()*1000 != cpuQuantity.MilliValue() {
		return false
	}

	return true
}

func getContainerID(pod *corev1.Pod, containerName string) string {
	for _, containerStatus := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if containerStatus.Name == containerName {
			return containerStatus.ContainerID
		}
	}

	return ""
}

func getCleanCoreList(coreIDs string) []int {
	cleanCores := make([]int, 0)
	commaSeparated := strings.Split(coreIDs, ",")
	for _, splitCore := range commaSeparated {
		hyphenSeparated := strings.Split(splitCore, "-")
		if len(hyphenSeparated) == 1 {
			intCore, _ := strconv.Atoi(hyphenSeparated[0])
			cleanCores = append(cleanCores, intCore)
		} else {
			startCore, _ := strconv.Atoi(hyphenSeparated[0])
			endCore, _ := strconv.Atoi(hyphenSeparated[len(hyphenSeparated)-1])
			for i := startCore; i <= endCore; i++ {
				cleanCores = append(cleanCores, i)
			}
		}
	}

	return cleanCores
}

func (r *PowerPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
