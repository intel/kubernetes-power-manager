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
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strconv"
	"strings"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/intel/kubernetes-power-manager/pkg/podresourcesclient"
	"github.com/intel/kubernetes-power-manager/pkg/podstate"
	"github.com/intel/kubernetes-power-manager/pkg/util"
)

const (
	PowerProfileAnnotation = "PowerProfile"
	ResourcePrefix         = "power.intel.com/"
	CPUResource            = "cpu"
	PowerNamespace         = "intel-power"
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

func (r *PowerPodReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerpod", req.NamespacedName)
	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), req.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Delete the Pod from the internal state in case it was never deleted
			err = r.State.DeletePodFromState(req.NamespacedName.Name)
			if err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve Pod")
		return ctrl.Result{}, err
	}
	// First thing to check is if this pod is on the same node as the node agent that intercepted it
	// The NODE_NAME environment variable is passed in via the downwards API in the PodSpec
	nodeName := os.Getenv("NODE_NAME")
	if pod.Spec.NodeName != nodeName {
		return ctrl.Result{}, nil
	}

	if pod.ObjectMeta.Namespace == "kube-system" {
		return ctrl.Result{}, nil
	}

	if !pod.ObjectMeta.DeletionTimestamp.IsZero() || pod.Status.Phase == corev1.PodSucceeded {
		// If the Pod's DeletionTimestamp is not zero then the Pod has been deleted

		powerPodState := r.State.GetPodFromState(pod.GetName())

		err = r.State.DeletePodFromState(pod.GetName())
		if err != nil {
			logger.Error(err, "error removing Pod from internal state")
			return ctrl.Result{}, err
		}

		workloadToCPUsRemoved := make(map[string][]uint)

		for _, container := range powerPodState.Containers {
			workload := container.Workload
			cpus := container.ExclusiveCPUs
			if _, exists := workloadToCPUsRemoved[workload]; exists {
				workloadToCPUsRemoved[workload] = append(workloadToCPUsRemoved[workload], cpus...)
			} else {
				workloadToCPUsRemoved[workload] = cpus
			}
		}

		for workloadName, cpus := range workloadToCPUsRemoved {
			workload := &powerv1.PowerWorkload{}
			err = r.Get(context.TODO(), client.ObjectKey{
				Namespace: IntelPowerNamespace,
				Name:      workloadName,
			}, workload)
			if err != nil {
				if errors.IsNotFound(err) {
					return ctrl.Result{}, nil
				}
				logger.Error(err, "error while trying to retrieve PowerWorkload")
				return ctrl.Result{}, err
			}

			updatedWorkloadCPUList := getNewWorkloadCPUList(cpus, workload.Spec.Node.CpuIds)
			workload.Spec.Node.CpuIds = updatedWorkloadCPUList
			updatedWorkloadContainerList := getNewWorkloadContainerList(workload.Spec.Node.Containers, powerPodState.Containers)
			workload.Spec.Node.Containers = updatedWorkloadContainerList

			err = r.Client.Update(context.TODO(), workload)
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
	containersRequestingExclusiveCPUs := getContainersRequestingExclusiveCPUs(pod)
	if len(containersRequestingExclusiveCPUs) == 0 {
		logger.Info("No containers are requesting exclusive CPUs")
		return ctrl.Result{}, nil
	}
	podUID := pod.GetUID()
	if podUID == "" {
		logger.Info("No pod UID found")
		return ctrl.Result{}, errors.NewServiceUnavailable("pod UID not found")
	}

	powerProfileCRs := &powerv1.PowerProfileList{}
	err = r.Client.List(context.TODO(), powerProfileCRs)
	if err != nil {
		logger.Error(err, "Error retrieving Power Profiles from cluster")
		return ctrl.Result{}, nil
	}
	powerProfilesFromContainers, powerContainers, err := r.getPowerProfileRequestsFromContainers(containersRequestingExclusiveCPUs, powerProfileCRs.Items, pod)
	if err != nil {
		logger.Error(err, "Error retrieving Power Profile from Pod requests")
		return ctrl.Result{}, err
	}
	for profile, cores := range powerProfilesFromContainers {
		workloadName := fmt.Sprintf("%s-%s", profile, nodeName)
		workload := &powerv1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: PowerNamespace,
			Name:      workloadName,
		}, workload)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Error(err, "no Power Workload exists for this Profile")
			} else {
				logger.Error(err, fmt.Sprintf("Error retrieving PowerWorkload '%s'", profile))
			}

			continue
		}

		// PowerWorkload already exists so need to update it. If the Node already
		// exists in the Workload, we update the Node's CPU list, if not we create
		// the entry for the node

		workload.Spec.Node.CpuIds = appendIfUnique(workload.Spec.Node.CpuIds, cores)
		sort.Slice(workload.Spec.Node.CpuIds, func(i, j int) bool { return workload.Spec.Node.CpuIds[i] < workload.Spec.Node.CpuIds[j] })

		containerList := make([]powerv1.Container, 0)
		for i, container := range powerContainers {
			powerContainers[i].Workload = workloadName

			workloadContainer := container
			workloadContainer.Pod = pod.Name
			containerList = append(containerList, workloadContainer)
		}
		for i, newContainer := range containerList {
			for _, oldContainer := range workload.Spec.Node.Containers {
				if newContainer.Name == oldContainer.Name && reflect.DeepEqual(newContainer.ExclusiveCPUs, oldContainer.ExclusiveCPUs) {
					containerList[i] = containerList[len(containerList)-1]
					containerList = containerList[:len(containerList)-1]
				}
			}
		}
		workload.Spec.Node.Containers = append(workload.Spec.Node.Containers, containerList...)
		err = r.Client.Update(context.TODO(), workload)
		if err != nil {
			logger.Error(err, "error while trying to update PowerWorkload")
			return ctrl.Result{}, err
		}
	}

	// Finally, update the controller's State

	guaranteedPod := powerv1.GuaranteedPod{}
	guaranteedPod.Node = pod.Spec.NodeName
	guaranteedPod.Name = pod.GetName()
	guaranteedPod.UID = string(podUID)
	guaranteedPod.Containers = make([]powerv1.Container, 0)
	guaranteedPod.Containers = powerContainers
	err = r.State.UpdateStateGuaranteedPods(guaranteedPod)
	if err != nil {
		logger.Error(err, "error updating internal state")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PowerPodReconciler) getPowerProfileRequestsFromContainers(containers []corev1.Container, profileCRs []powerv1.PowerProfile, pod *corev1.Pod) (map[string][]uint, []powerv1.Container, error) {

	_ = context.Background()

	profiles := make(map[string][]uint)
	powerContainers := make([]powerv1.Container, 0)

	for _, container := range containers {
		profile, err := getContainerProfileFromRequests(container)
		if err != nil {
			return map[string][]uint{}, []powerv1.Container{}, err
		}

		// If there was no Profile requested in this container we can move onto the next one
		if profile == "" {
			continue
		}

		if !profileExists(profile, profileCRs) {
			powerProfileNotFoundError := errors.NewServiceUnavailable(fmt.Sprintf("Power Profile '%s' not found", profile))
			return map[string][]uint{}, []powerv1.Container{}, powerProfileNotFoundError
		}

		containerID := getContainerID(pod, container.Name)
		coreIDs, err := r.PodResourcesClient.GetContainerCPUs(pod.GetName(), container.Name)
		if err != nil {
			return map[string][]uint{}, []powerv1.Container{}, err
		}
		cleanCoreList := getCleanCoreList(coreIDs)

		powerContainer := &powerv1.Container{}
		powerContainer.Name = container.Name
		powerContainer.Id = strings.TrimPrefix(containerID, "docker://")
		powerContainer.ExclusiveCPUs = cleanCoreList
		powerContainer.PowerProfile = profile
		powerContainers = append(powerContainers, *powerContainer)

		if _, exists := profiles[profile]; exists {
			profiles[profile] = append(profiles[profile], cleanCoreList...)
		} else {
			profiles[profile] = cleanCoreList
		}
	}

	if len(reflect.ValueOf(profiles).MapKeys()) > 1 {
		// For now we can only have one Power Profile per Pod

		moreThanOneProfileError := errors.NewServiceUnavailable("Cannot have more than one Power Profile per Pod")
		return map[string][]uint{}, []powerv1.Container{}, moreThanOneProfileError
	}

	return profiles, powerContainers, nil
}

func profileExists(profile string, powerProfiles []powerv1.PowerProfile) bool {
	for _, powerProfile := range powerProfiles {
		if powerProfile.Name == profile {
			return true
		}
	}

	return false
}

func getNewWorkloadCPUList(cpuList []uint, nodeCpuIds []uint) []uint {
	updatedWorkloadCPUList := make([]uint, 0)

	for _, cpu := range nodeCpuIds {
		if !util.CPUInCPUList(cpu, cpuList) {
			updatedWorkloadCPUList = append(updatedWorkloadCPUList, cpu)
		}
	}

	return updatedWorkloadCPUList
}

func appendIfUnique(cpuList []uint, cpus []uint) []uint {
	for _, cpu := range cpus {
		if !util.CPUInCPUList(cpu, cpuList) {
			cpuList = append(cpuList, cpu)
		}
	}

	return cpuList
}

func getNewWorkloadContainerList(nodeContainers []powerv1.Container, podStateContainers []powerv1.Container) []powerv1.Container {
	newNodeContainers := make([]powerv1.Container, 0)

	for _, container := range nodeContainers {
		if !isContainerInList(container.Name, podStateContainers) {
			newNodeContainers = append(newNodeContainers, container)
		}
	}

	return newNodeContainers
}

func isContainerInList(name string, containers []powerv1.Container) bool {
	for _, container := range containers {
		if container.Name == name {
			return true
		}
	}

	return false
}

func getContainerProfileFromRequests(container corev1.Container) (string, error) {
	profileName := ""
	moreThanOneProfileError := errors.NewServiceUnavailable("Cannot have more than one Power Profile per Container")
	resourceRequestsMismatchError := errors.NewServiceUnavailable("Mismatch between CPU requests and PowerProfile Requests")

	for resource := range container.Resources.Requests {
		if strings.HasPrefix(string(resource), ResourcePrefix) {
			if profileName == "" {
				profileName = string(resource[len(ResourcePrefix):])
			} else {
				// Cannot have more than one profile for a singular container
				return "", moreThanOneProfileError
			}
		}
	}

	if profileName != "" {
		// Check if there is a mismatch in CPU requests and PowerProfile requests
		powerProfileResourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ResourcePrefix, profileName))
		numRequestsPowerProfile := container.Resources.Requests[powerProfileResourceName]
		numLimitsPowerProfile := container.Resources.Limits[powerProfileResourceName]
		numRequestsCPU := container.Resources.Requests[CPUResource]
		numLimistCPU := container.Resources.Limits[CPUResource]
		if numRequestsCPU != numRequestsPowerProfile || numLimistCPU != numLimitsPowerProfile {
			return "", resourceRequestsMismatchError
		}
	}

	return profileName, nil
}

func getContainersRequestingExclusiveCPUs(pod *corev1.Pod) []corev1.Container {

	containersRequestingExclusiveCPUs := make([]corev1.Container, 0)
	containerList := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, container := range containerList {
		if exclusiveCPUs(pod, &container) {
			containersRequestingExclusiveCPUs = append(containersRequestingExclusiveCPUs, container)
		}
	}

	return containersRequestingExclusiveCPUs
}

func exclusiveCPUs(pod *corev1.Pod, container *corev1.Container) bool {
	if pod.Status.QOSClass != corev1.PodQOSGuaranteed {
		return false
	}

	cpuQuantity := container.Resources.Requests[corev1.ResourceCPU]
	return cpuQuantity.Value()*1000 == cpuQuantity.MilliValue()
}

func getContainerID(pod *corev1.Pod, containerName string) string {
	for _, containerStatus := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if containerStatus.Name == containerName {
			return containerStatus.ContainerID
		}
	}

	return ""
}

func getCleanCoreList(coreIDs string) []uint {
	cleanCores := make([]uint, 0)
	commaSeparated := strings.Split(coreIDs, ",")
	for _, splitCore := range commaSeparated {
		hyphenSeparated := strings.Split(splitCore, "-")
		if len(hyphenSeparated) == 1 {
			intCore, err := strconv.ParseUint(hyphenSeparated[0], 10, 32)
			if err != nil {
				fmt.Printf("error getting core list: %v", err)
				return []uint{}
			}
			cleanCores = append(cleanCores, uint(intCore))
		} else {
			startCore, err := strconv.Atoi(hyphenSeparated[0])
			if err != nil {
				fmt.Printf("error getting core list: %v", err)
				return []uint{}
			}
			endCore, err := strconv.Atoi(hyphenSeparated[len(hyphenSeparated)-1])
			if err != nil {
				fmt.Printf("error getting core list: %v", err)
				return []uint{}
			}
			for i := startCore; i <= endCore; i++ {
				cleanCores = append(cleanCores, uint(i))
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
