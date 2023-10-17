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
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	State              *podstate.State
	PodResourcesClient podresourcesclient.PodResourcesClient
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerpods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerpods/status,verbs=get;update;patch

func (r *PowerPodReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerpod", req.NamespacedName)
	pod := &corev1.Pod{}
	logger.V(5).Info("retrieving pod instance")
	err := r.Get(context.TODO(), req.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Delete the Pod from the internal state in case it was never deleted
			// aAdded the check due to golangcilint errcheck
			_ = r.State.DeletePodFromState(req.NamespacedName.Name, req.NamespacedName.Namespace)
			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve the pod")
		return ctrl.Result{}, err
	}
	// First thing to check is if this pod is on the same node as the node agent that intercepted it
	// The NODE_NAME environment variable is passed in via the downwards API in the pod spec
	logger.V(5).Info("confirming the pod is on the same node as the power node-agent")
	nodeName := os.Getenv("NODE_NAME")
	if pod.Spec.NodeName != nodeName {
		return ctrl.Result{}, nil
	}

	if pod.ObjectMeta.Namespace == "kube-system" {
		return ctrl.Result{}, nil
	}

	if !pod.ObjectMeta.DeletionTimestamp.IsZero() || pod.Status.Phase == corev1.PodSucceeded {
		// If the pod's deletion timestamp is not zero, then the pod has been deleted

		powerPodState := r.State.GetPodFromState(pod.GetName(), pod.GetNamespace())

		logger.V(5).Info("removing the pod from the internal state")
		_ = r.State.DeletePodFromState(pod.GetName(), pod.GetNamespace())
		workloadToCPUsRemoved := make(map[string][]uint)

		logger.V(5).Info("removing pods CPUs from the internal state")
		for _, container := range powerPodState.Containers {
			workload := container.Workload
			cpus := container.ExclusiveCPUs
			logger.V(5).Info("Removing", "Workload", workload, "CPUs", cpus)
			if _, exists := workloadToCPUsRemoved[workload]; exists {
				workloadToCPUsRemoved[workload] = append(workloadToCPUsRemoved[workload], cpus...)
			} else {
				workloadToCPUsRemoved[workload] = cpus
			}
		}
		for workloadName, cpus := range workloadToCPUsRemoved {
			logger.V(5).Info("retrieving the workload instance", "Workload Name", workloadName)
			workload := &powerv1.PowerWorkload{}
			err = r.Get(context.TODO(), client.ObjectKey{
				Namespace: IntelPowerNamespace,
				Name:      workloadName,
			}, workload)
			if err != nil {
				logger.Error(err, "error while trying to retrieve the power workload")
				if errors.IsNotFound(err) {
					return ctrl.Result{Requeue: false}, err
				}
				return ctrl.Result{}, err
			}

			logger.V(5).Info("updating CPUs workload list with their CPU IDs and the container information")
			updatedWorkloadCPUList := getNewWorkloadCPUList(cpus, workload.Spec.Node.CpuIds, &logger)
			workload.Spec.Node.CpuIds = updatedWorkloadCPUList
			updatedWorkloadContainerList := getNewWorkloadContainerList(workload.Spec.Node.Containers, powerPodState.Containers, &logger)
			workload.Spec.Node.Containers = updatedWorkloadContainerList

			err = r.Client.Update(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "failed to update the power workload")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// If the pod's deletion timestamp is equal to zero, then the pod has been created or updated

	// Make sure the pod is running
	logger.V(5).Info("confirming the pod is in a running state")
	podNotRunningErr := errors.NewServiceUnavailable("the pod is not in the running phase")
	if pod.Status.Phase != corev1.PodRunning {
		return ctrl.Result{}, podNotRunningErr
	}

	// Get the containers of the pod that are requesting exclusive CPUs
	logger.V(5).Info("retrieving the containers requested for the exclusive CPUs")
	containersRequestingExclusiveCPUs := getContainersRequestingExclusiveCPUs(pod, &logger)
	if len(containersRequestingExclusiveCPUs) == 0 {
		logger.Info("no containers are requesting exclusive CPUs")
		return ctrl.Result{}, nil
	}
	podUID := pod.GetUID()
	logger.V(5).Info("retrieving the podUID", "UID", podUID)
	if podUID == "" {
		logger.Info("no pod UID found")
		return ctrl.Result{}, errors.NewServiceUnavailable("pod UID not found")
	}

	powerProfileCRs := &powerv1.PowerProfileList{}
	logger.V(5).Info("retrieving the power profiles from the cluster")
	if err = r.Client.List(context.TODO(), powerProfileCRs); err != nil {
		logger.Error(err, "error retrieving the power profiles from the cluster")
		return ctrl.Result{}, err
	}
	powerProfilesFromContainers, powerContainers, err := r.getPowerProfileRequestsFromContainers(containersRequestingExclusiveCPUs, powerProfileCRs.Items, pod, &logger)
	logger.V(5).Info("retrieving the power profiles and cores from the pod requests")
	if err != nil {
		logger.Error(err, "error retrieving the power profile from the pod requests")
		return ctrl.Result{}, err
	}
	for profile, cores := range powerProfilesFromContainers {
		logger.V(5).Info("retrieving the workload for the power profile")
		workloadName := fmt.Sprintf("%s-%s", profile, nodeName)
		workload := &powerv1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: PowerNamespace,
			Name:      workloadName,
		}, workload)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Error(err, "no power workload exists for this profile")
			} else {
				logger.Error(err, fmt.Sprintf("error retrieving the power workload '%s'", profile))
			}

			continue
		}

		// Power workload already exists so need to update it. If the node already
		// exists in the workload, we update the node's CPU list, if not we create
		// the entry for the node

		workload.Spec.Node.CpuIds = appendIfUnique(workload.Spec.Node.CpuIds, cores, &logger)
		sort.Slice(workload.Spec.Node.CpuIds, func(i, j int) bool { return workload.Spec.Node.CpuIds[i] < workload.Spec.Node.CpuIds[j] })

		containerList := make([]powerv1.Container, 0)
		for i, container := range powerContainers {
			logger.V(5).Info("updating the power container list")
			powerContainers[i].Workload = workloadName

			workloadContainer := container
			workloadContainer.Pod = pod.Name
			workloadContainer.Workload = workloadName
			containerList = append(containerList, workloadContainer)
		}
		for i, newContainer := range containerList {
			logger.V(5).Info("confirming the containers are not duplicated")
			for _, oldContainer := range workload.Spec.Node.Containers {
				if newContainer.Name == oldContainer.Name && reflect.DeepEqual(newContainer.ExclusiveCPUs, oldContainer.ExclusiveCPUs) {
					containerList[i] = containerList[len(containerList)-1]
					containerList = containerList[:len(containerList)-1]
				}
			}
		}
		workload.Spec.Node.Containers = append(workload.Spec.Node.Containers, containerList...)
		err = r.Client.Update(context.TODO(), workload)
		logger.V(5).Info("ammending the workload in the container list")
		if err != nil {
			logger.Error(err, "error while trying to update the power workload")
			return ctrl.Result{}, err
		}
	}

	// Finally, update the controller's state

	logger.V(5).Info("updating the controller's internal state")
	guaranteedPod := powerv1.GuaranteedPod{}
	guaranteedPod.Node = pod.Spec.NodeName
	guaranteedPod.Name = pod.GetName()
	guaranteedPod.Namespace = pod.Namespace
	guaranteedPod.UID = string(podUID)
	guaranteedPod.Containers = make([]powerv1.Container, 0)
	guaranteedPod.Containers = powerContainers
	err = r.State.UpdateStateGuaranteedPods(guaranteedPod)
	if err != nil {
		logger.Error(err, "error updating the internal state")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PowerPodReconciler) getPowerProfileRequestsFromContainers(containers []corev1.Container, profileCRs []powerv1.PowerProfile, pod *corev1.Pod, logger *logr.Logger) (map[string][]uint, []powerv1.Container, error) {

	logger.V(5).Info("get the power profiles from the containers")
	_ = context.Background()

	profiles := make(map[string][]uint)
	powerContainers := make([]powerv1.Container, 0)
	for _, container := range containers {
		logger.V(5).Info("retrieving the requested power profile from the container spec")
		profile, err := getContainerProfileFromRequests(container, logger)
		if err != nil {
			return map[string][]uint{}, []powerv1.Container{}, err
		}

		// If there was no profile requested in this container we can move onto the next one
		if profile == "" {
			logger.V(5).Info("no profile was requested by the container")
			continue
		}
		// checks if pod has been altered by time of day
		newProf, ok := pod.ObjectMeta.Annotations["PM-altered"]
		if ok {
			profile = newProf
		}
		if !verifyProfileExists(profile, profileCRs, logger) {
			powerProfileNotFoundError := errors.NewServiceUnavailable(fmt.Sprintf("power profile '%s' not found", profile))
			return map[string][]uint{}, []powerv1.Container{}, powerProfileNotFoundError
		}

		containerID := getContainerID(pod, container.Name)
		coreIDs, err := r.PodResourcesClient.GetContainerCPUs(pod.GetName(), container.Name)
		if err != nil {
			logger.V(5).Info("Error getting CoreIDs.", "ContainerID", containerID)
			return map[string][]uint{}, []powerv1.Container{}, err
		}
		cleanCoreList := getCleanCoreList(coreIDs)
		logger.V(5).Info("Reserving cores to container.", "ContainerID", containerID, "Cores", cleanCoreList)

		logger.V(5).Info("creating the power container")
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
		// for now we can only have one power profile per pod
		moreThanOneProfileError := errors.NewServiceUnavailable("cannot have more than one power profile per pod")
		return map[string][]uint{}, []powerv1.Container{}, moreThanOneProfileError
	}

	return profiles, powerContainers, nil
}

func verifyProfileExists(profile string, powerProfiles []powerv1.PowerProfile, logger *logr.Logger) bool {
	logger.V(5).Info("confirming the power profile exists in the cluster")
	for _, powerProfile := range powerProfiles {
		if powerProfile.Name == profile {
			return true
		}
	}

	return false
}

func getNewWorkloadCPUList(cpuList []uint, nodeCpuIds []uint, logger *logr.Logger) []uint {
	updatedWorkloadCPUList := make([]uint, 0)

	logger.V(5).Info("getting the updated workload's CPU list")
	for _, cpu := range nodeCpuIds {
		if !util.CPUInCPUList(cpu, cpuList) {
			updatedWorkloadCPUList = append(updatedWorkloadCPUList, cpu)
		}
	}

	return updatedWorkloadCPUList
}

func appendIfUnique(cpuList []uint, cpus []uint, logger *logr.Logger) []uint {
	for _, cpu := range cpus {
		if !util.CPUInCPUList(cpu, cpuList) {
			cpuList = append(cpuList, cpu)
		}
	}

	return cpuList
}

func getNewWorkloadContainerList(nodeContainers []powerv1.Container, podStateContainers []powerv1.Container, logger *logr.Logger) []powerv1.Container {
	newNodeContainers := make([]powerv1.Container, 0)

	logger.V(5).Info("checking if there are new containers for the workload")
	for _, container := range nodeContainers {
		if !isContainerInList(container.Name, container.Id, podStateContainers, logger) {
			newNodeContainers = append(newNodeContainers, container)
		}
	}

	return newNodeContainers
}

// Helper function - if container is in a list of containers
func isContainerInList(name string, uid string, containers []powerv1.Container, logger *logr.Logger) bool {
	for _, container := range containers {
		if container.Name == name && container.Id == uid {
			return true
		}
	}

	return false
}

func getContainerProfileFromRequests(container corev1.Container, logger *logr.Logger) (string, error) {
	profileName := ""
	moreThanOneProfileError := errors.NewServiceUnavailable("cannot have more than one power profile per container")
	resourceRequestsMismatchError := errors.NewServiceUnavailable("mismatch between CPU requests and the power profile requests")

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
		// Check if there is a mismatch in CPU requests and the power profile requests
		logger.V(5).Info("confirming that CPU requests and the power profiles request match")
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

func getContainersRequestingExclusiveCPUs(pod *corev1.Pod, logger *logr.Logger) []corev1.Container {

	logger.V(5).Info("receiving containers requesting exclusive CPUs")
	containersRequestingExclusiveCPUs := make([]corev1.Container, 0)
	containerList := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, container := range containerList {
		if doesContainerRequireExclusiveCPUs(pod, &container) {
			containersRequestingExclusiveCPUs = append(containersRequestingExclusiveCPUs, container)
		}
	}
	logger.V(5).Info("the containers requesting exclusive CPUs are: ", containersRequestingExclusiveCPUs)
	return containersRequestingExclusiveCPUs

}

func doesContainerRequireExclusiveCPUs(pod *corev1.Pod, container *corev1.Container) bool {
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
				fmt.Printf("error getting the core list: %v", err)
				return []uint{}
			}
			cleanCores = append(cleanCores, uint(intCore))
		} else {
			startCore, err := strconv.Atoi(hyphenSeparated[0])
			if err != nil {
				fmt.Printf("error getting the core list: %v", err)
				return []uint{}
			}
			endCore, err := strconv.Atoi(hyphenSeparated[len(hyphenSeparated)-1])
			if err != nil {
				fmt.Printf("error getting the core list: %v", err)
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
