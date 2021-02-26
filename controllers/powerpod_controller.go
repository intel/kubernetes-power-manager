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
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
)

const (
	PowerProfileAnnotation = "PowerProfile"
)

// PowerPodReconciler reconciles a PowerPod object
type PowerPodReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	State              state.State
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
			// Maybe do a check to see if the Pod is still recorded in state for some reason

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve Pod")
		return ctrl.Result{}, err
	}
/*
	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		// If the Pod's DeletionTimestamp is not zero then the Pod has been deleted

		powerPodState := r.State.GetPodFromState(pod.GetName())
		powerPodCPUs := r.State.GetCPUsFromPodState(powerPodState)
		workloadName := fmt.Sprintf("%s%s", powerPodState.PowerProfile.Name, WorkloadNameSuffix)

		workload := &powerv1alpha1.PowerWorkload{}
		err = r.Get(context.TODO(), client.ObjectKey{
			Namespace: req.NamespacedName.Namespace,
			Name:      workloadName,
		}, workload)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info(fmt.Sprintf("Could not find Workload '%s'", workloadName))
				return ctrl.Result{}, nil
			}

			logger.Error(err, "error while trying to retrieve PowerWorkload")
			return ctrl.Result{}, err
		}

		//updatedWorkloadCPUList := getNewWorkloadCPUList(powerPodCPUs, workload.Spec.CpuIds)
		_ = getNewWorkloadCPUList(powerPodCPUs, workload.Spec.CpuIds)
		updatedWorkloadCPUList := []int{1, 2}
		workload.Spec.CpuIds = updatedWorkloadCPUList

		err = r.Client.Update(context.TODO(), workload)
		if err != nil {
			logger.Error(err, "error while trying to update PowerWorkload")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
*/
	// If the Pod's DeletionTimestamp is equal to zero then the Pod has been created or updated

	// Make sure the Pod is running
	podNotRunningErr := errors.NewServiceUnavailable("pod not in running phase")
	if pod.Status.Phase != corev1.PodRunning {
		logger.Info("Pod not running", "pod status:", pod.Status.Phase)
		return ctrl.Result{}, podNotRunningErr
	}

	// Get the PowerProfile request by the pods
	// GOING TO MOVE OVER TO EXTENDED RESOURCES
	profileName := getProfileFromPodAnnotations(pod)
	if profileName == "" {
		logger.Info("No PowerProfile requested")
		return ctrl.Result{}, nil
	}

/*
	profile := &powerv1alpha1.PowerProfile{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: req.NamespacedName.Namespace,
		Name:      profileName,
	}, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "PowerProfile not found")
			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve PowerProfile")
		return ctrl.Result{}, err
	}
*/
	// Get the Containers of the Pod that are requesting exclusive CPUs
	containersRequestingExclusiveCPUs := getContainersRequestingExclusiveCPUs(pod)
	if len(containersRequestingExclusiveCPUs) == 0 {
		logger.Info("No containers are requesting exclusive CPUs")
		return ctrl.Result{}, nil
	}

	guaranteedPod := powerv1alpha1.GuaranteedPod{}
	guaranteedPod.Name = pod.GetName()
	podUID := pod.GetUID()
	if podUID == "" {
		logger.Info("No pod UID found")
		return ctrl.Result{}, errors.NewServiceUnavailable("pod UID not found")
	}
	guaranteedPod.UID = string(podUID)

	powerContainers := make([]powerv1alpha1.Container, 0)
	allCores := make([]int, 0)

	for _, container := range containersRequestingExclusiveCPUs {
		containerID := getContainerID(pod, container)
		coreIDs, err := r.PodResourcesClient.GetContainerCPUs(guaranteedPod.Name, container)
		
		if err != nil {
			logger.Error(err, "error call to PodResourcesClient")
			return ctrl.Result{}, nil
		}
		logger.Info("********************************")
		logger.Info(fmt.Sprintf("CoreIDs: %v", coreIDs))
		
		if err != nil {
			logger.Error(err, "failed to retrieve cpuset from groups")
			return ctrl.Result{}, err
		}

		powerContainer := &powerv1alpha1.Container{}
		powerContainer.Name = container
		powerContainer.ID = strings.TrimPrefix(containerID, "docker://")
		cleanCoreList := getCleanCoreList(coreIDs)
		powerContainer.ExclusiveCPUs = cleanCoreList

		powerContainers = append(powerContainers, *powerContainer)
		allCores = append(allCores, cleanCoreList...)
	}
	guaranteedPod.Containers = make([]powerv1alpha1.Container, 0)
	guaranteedPod.Containers = powerContainers
	//guaranteedPod.PowerProfile = *profile

	err = r.State.UpdateStateGuaranteedPods(guaranteedPod)
	if err != nil {
		logger.Error(err, "error updating internal state")
		return ctrl.Result{}, err
	}

	workloadName := fmt.Sprintf("%s%s", profileName, WorkloadNameSuffix)
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
					Name: 	pod.Spec.NodeName,
					CpuIds: allCores,
				},
			}
			workloadSpec := &powerv1alpha1.PowerWorkloadSpec{
				Nodes: 		nodeInfo,
				PowerProfile: 	0,
			}
/*
			// TODO: Replace this
			placeholderName := []string{"placeholders"}
			workloadSpec := &powerv1alpha1.PowerWorkloadSpec{
				Nodes:        placeholderName,
				CpuIds:       allCores,
				//PowerProfile: guaranteedPod.PowerProfile,
				// Change this to search for the PowerProfile's id in AppQoS instance
				PowerProfile: 0,
			}
*/
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

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve PowerWorkload")
		return ctrl.Result{}, err
	}
/*
	// PowerWorkload already exists so need to update it
	workload.Spec.CpuIds = append(workload.Spec.CpuIds, allCores...)
	err = r.Client.Update(context.TODO(), workload)
	if err != nil {
		logger.Error(err, "error while trying to update PowerWorkload")
		return ctrl.Result{}, err
	}
*/
	return ctrl.Result{}, nil
}

func getNewWorkloadCPUList(powerPodCPUs []string, workloadCPUs []string) []string {
	updatedWorkloadCPUList := make([]string, 0)

	for _, cpu := range workloadCPUs {
		if !IdInCPUList(cpu, powerPodCPUs) {
			updatedWorkloadCPUList = append(updatedWorkloadCPUList, cpu)
		}
	}

	return updatedWorkloadCPUList
}

func getProfileFromPodAnnotations(pod *corev1.Pod) string {
	annotations := pod.GetAnnotations()
	profile := annotations[PowerProfileAnnotation]

	return profile
}

func getContainersRequestingExclusiveCPUs(pod *corev1.Pod) []string {
	containersRequestingExclusiveCPUs := make([]string, 0)
	for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		if exclusiveCPUs(pod, &container) {
			containersRequestingExclusiveCPUs = append(containersRequestingExclusiveCPUs, container.Name)
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
