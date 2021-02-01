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
	//"io/ioutil"
	"strings"

	"github.com/go-logr/logr"
	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	cgp "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cgroupsparser"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/configstate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	powerProfileAnnotation = "PowerProfile"
	configNameSuffix       = "-config"
)

type State struct {
	PowerNodeStatus *powerv1alpha1.PowerNodeStatus
}

var podConfigState map[string]configstate.ConfigState = make(map[string]configstate.ConfigState, 0)
//var state = newState()

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	State state.State
	ConfigState configstate.Configs
}

// +kubebuilder:rbac:groups=power.intel.com,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=pods/status,verbs=get;update;patch

func (r *PodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerpod", req.NamespacedName)

	logger.Info("*******************************")
	logger.Info(fmt.Sprintf("%v", r.State.PowerNodeStatus))//.SharedPool))
	logger.Info(fmt.Sprintf("%v", r.State.PowerNodeStatus))//.GuaranteedPods))
	logger.Info("*******************************")

	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), req.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod has not been found, can't do anything
			// TODO: Determine if you need to do a sanity check to make sure the Pod isn't for some
			// reason recorded in any State records

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Pod has been deleted, cleaning up...")
		podState := r.State.GetPodFromState(pod.GetName())
		podStateCPUs := r.State.GetCPUsFromPodState(podState)

		configName := fmt.Sprintf("%s%s", podState.Profile.Name, configNameSuffix)
		if oldConfig, exists := podConfigState[configName]; exists {
			config := &powerv1alpha1.Config{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: pod.GetNamespace(),
				Name:      configName,
			}, config)
			if err != nil {
				logger.Error(err, "Error while retrieving Config")
				return ctrl.Result{}, nil
			}

			// Remove the CPUs associated with this Pod to determine if this should be a
			// Config deletion or update
			updatedConfigCPUs := make([]string, 0)
			for _, oldConfigCPU := range oldConfig.CPUs {
				if cpuIsStillBeingUsed(oldConfigCPU, podStateCPUs) {
					updatedConfigCPUs = append(updatedConfigCPUs, oldConfigCPU)
				}
			}

			// TODO: update this so the Pod controller isn't updating the ConfigState at all, only reading from it.
			if len(updatedConfigCPUs) > 0 {
				// Update Config as it's still in use
				logger.Info("Updating Config...")
				oldConfig.CPUs = updatedConfigCPUs
				podConfigState[configName] = oldConfig
				config.Spec.CpuIds = oldConfig.CPUs

				err = r.Client.Update(context.TODO(), config)
				if err != nil {
					logger.Error(err, "Error while updating config")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, nil
			} else {
				// Delete Config as no other Pod is using it
				logger.Info("Deleting Config...")
				delete(podConfigState, configName)

				err = r.Client.Delete(context.TODO(), config)
				if err != nil {
					logger.Error(err, "Error while deleting Config")
					return ctrl.Result{}, nil
				}

				return ctrl.Result{}, nil
			}
		}

		return ctrl.Result{}, nil
	}

	// If the DeletionTimestamp is equal to zero than this Pod has just been created

	// Make sure the Pod is running
	podNotRunningErr := errors.NewServiceUnavailable("pod not in running phase")
	if pod.Status.Phase != corev1.PodRunning {
		logger.Info("Pod not running", "pod status:", pod.Status.Phase)
		return ctrl.Result{}, podNotRunningErr
	}

	// Get the PowerProfile requested by the Pod
	profileName := getProfileFromPodAnnotations(pod)
	if profileName == "" {
		logger.Info("No PowerProfile detected, skipping...")
		return ctrl.Result{}, nil
	}

	profile := &powerv1alpha1.Profile{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: pod.GetNamespace(),
		Name:      profileName,
	}, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, fmt.Sprintf("Could not find Profile %s", profileName))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

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
	guaranteedPod.UID = string(pod.UID)

	powerContainers := make([]powerv1alpha1.Container, 0)
	allCores := make([]string, 0)

	for _, container := range containersRequestingExclusiveCPUs {
		containerID := getContainerID(pod, container)
		coreIDs, err := cgp.ReadCgroupCpuset(string(podUID), containerID)
		if err != nil {
			logger.Error(err, "failed to retrieve cpuset from groups")
			return ctrl.Result{}, err
		}

		powerContainer := &powerv1alpha1.Container{}
		powerContainer.Name = container
		powerContainer.ID = strings.TrimPrefix(containerID, "docker://")
		// TODO: Look into possibility of a cpu list such as "0-12"
		splitCores := strings.Split(coreIDs, ",")
		powerContainer.ExclusiveCPUs = splitCores

		powerContainers = append(powerContainers, *powerContainer)
		for _, core := range splitCores {
			allCores = append(allCores, core)
		}
	}
	guaranteedPod.Containers = make([]powerv1alpha1.Container, 0)
	guaranteedPod.Containers = powerContainers

	guaranteedPod.Profile = *profile
	err = r.State.UpdateStateGuaranteedPods(guaranteedPod)
	if err != nil {
		logger.Error(err, "error updating internal state")
		return ctrl.Result{}, err
	}

	// Update podConfigState
	configName := fmt.Sprintf("%s%s", profileName, configNameSuffix)
	cs := configstate.ConfigState{}
	cs.CPUs = allCores
	cs.Max = guaranteedPod.Profile.Spec.Max
	cs.Min = guaranteedPod.Profile.Spec.Min
	if oldConfig, exists := podConfigState[configName]; exists {
		for _, cpu := range cs.CPUs {
			oldConfig.CPUs = append(oldConfig.CPUs, cpu)
		}
		cs.CPUs = oldConfig.CPUs
		podConfigState[configName] = cs

		logger.Info(fmt.Sprintf("Config %s already exists, updating allocated CPUs...", configName))
		config := &powerv1alpha1.Config{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: pod.GetNamespace(),
			Name:      configName,
		}, config)

		config.Spec.CpuIds = cs.CPUs
		config.Spec.Profile = guaranteedPod.Profile

		err = r.Client.Update(context.TODO(), config)
		if err != nil {
			logger.Error(err, "Error while updating config")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	podConfigState[configName] = cs

	// Create the Config associated with the Profile
	logger.Info("Creating Config based on provided Profile")
	placeholderNodes := []string{"Placeholder"}
	configSpec := &powerv1alpha1.ConfigSpec{
		Nodes:   placeholderNodes,
		CpuIds:  cs.CPUs,
		Profile: guaranteedPod.Profile,
	}
	config := &powerv1alpha1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.GetNamespace(),
			Name:      configName,
		},
	}
	config.Spec = *configSpec
	err = r.Client.Create(context.TODO(), config)
	if err != nil {
		logger.Error(err, "Error while creating Config")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func getProfileFromProfileList(name string, profiles *powerv1alpha1.ProfileList) (powerv1alpha1.Profile, error) {
	for _, profile := range profiles.Items {
		if profile.Name == name {
			return profile, nil
		}
	}

	errorMsg := fmt.Sprintf("Profile %s not found", name)
	return powerv1alpha1.Profile{}, errors.NewServiceUnavailable(errorMsg)
}

func cpuIsStillBeingUsed(cpu string, cpuList []string) bool {
	for _, c := range cpuList {
		if cpu == c {
			return false
		}
	}

	return true
}

/*
// DELETE (Only here while import is broken)
func newState() *State {
	state := &State{}
	state.PowerNodeStatus = &powerv1alpha1.PowerNodeStatus{}
	state.PowerNodeStatus.SharedPool = "0-63"

	return state
}

func updateStateWithGuaranteedPod(pod powerv1alpha1.GuaranteedPod) error {
	if len(state.PowerNodeStatus.GuaranteedPods) == 0 {
		pods := make([]powerv1alpha1.GuaranteedPod, 0)
		state.PowerNodeStatus.GuaranteedPods = pods
	}

	for i, existingPod := range state.PowerNodeStatus.GuaranteedPods {
		if existingPod.Name == pod.Name {
			state.PowerNodeStatus.GuaranteedPods[i] = pod
			return nil
		}
	}

	state.PowerNodeStatus.GuaranteedPods = append(state.PowerNodeStatus.GuaranteedPods, pod)
	return nil
}

func getPodFromState(podName string) powerv1alpha1.GuaranteedPod {
	for _, existingPod := range state.PowerNodeStatus.GuaranteedPods {
		if existingPod.Name == podName {
			return existingPod
		}
	}

	return powerv1alpha1.GuaranteedPod{}
}

func getCpusFromPodState(podState powerv1alpha1.GuaranteedPod) []string {
	cpus := make([]string, 0)
	for _, container := range podState.Containers {
		for _, cpu := range container.ExclusiveCPUs {
			cpus = append(cpus, cpu)
		}
	}

	return cpus
}

// END DELETE
*/

/*
// DELETE (Only here while import is broken)
func ReadCgroupCpuset(podUID, containerID string) (string, error) {
	containerID = strings.TrimPrefix(containerID, "docker://")
	//kubepodsCgroupPath, err := findKubepodsCgroup()
	kubepodsCgroupPath := "/sys/fs/cgroup/cpuset/kubepods.slice/"
	//if err != nil {
	//	return "", err
	//}
	//if kubepodsCgroupPath == "" {
	//	return "", errors.NewServiceUnavailable("kubepods cgroup file not found")
	//}

	//podCgroupPath, err := findPodCgroup(kubepodsCgroupPath, podUID)
	podUIDUnderscores := strings.ReplaceAll(podUID, "-", "_")
	podCgroupPath := fmt.Sprintf("%skubepods-pod%s.slice/", kubepodsCgroupPath, podUIDUnderscores)
	//if err != nil {
	//	return "", err
	//}
	//if podCgroupPath == "" {
	//	return "", errors.NewServiceUnavailable(fmt.Sprintf("%s%s%s%s", "podUID ", podUID, " not found in kubepods cgroup ", kubepodsCgroupPath))
	//}

	//containerCgroupPath, err := findCgroupPath(podCgroupPath, containerID)
	containerCgroupPath := fmt.Sprintf("%sdocker-%s.scope", podCgroupPath, containerID)
	//if err != nil {
	//	return "", err
	//}
	//if containerCgroupPath == "" {
	//	return "", errors.NewServiceUnavailable(fmt.Sprintf("%s%s%s%s", "containerID ", containerID, " not found in pod cgroup ", podCgroupPath))
	//}

	cpuStr, err := cpusetFileToString(containerCgroupPath)
	if err != nil {
		return "", err
	}

	return cpuStr, nil
}

func findKubepodsCgroup() (string, error) {
	treeVersions := []string{"/sys/fs/cgroup/", "/sys/fs/cgroup/cpuset/"}
	for _, treeVersion := range treeVersions {
		kubepodsCgroupPath, err := findCgroupPath(treeVersion, "kubepods")
		if err != nil {
			return "", err
		}
		if kubepodsCgroupPath != "" {
			return kubepodsCgroupPath, nil
		}
	}
	return "", nil
}

func findPodCgroup(kubepodsCgroupPath string, podUID string) (string, error) {
	podUIDUnderscores := strings.ReplaceAll(podUID, "-", "_")
	fileVersions := []string{podUID, podUIDUnderscores}
	for _, fileVersion := range fileVersions {
		podCgroupPath, err := findCgroupPath(kubepodsCgroupPath, fileVersion)
		if err != nil {
			return "", err
		}
		if podCgroupPath != "" {
			return podCgroupPath, nil
		}
	}
	return "", nil
}

func findCgroupPath(base string, substring string) (string, error) {
	var fullPath string
	items, err := ioutil.ReadDir(base)
	if err != nil {
		return fullPath, err
	}
	for _, item := range items {
		if strings.Contains(item.Name(), substring) {
			fullPath = fmt.Sprintf("%s%s%s", base, item.Name(), "/")
			return fullPath, nil
		}
	}
	return fullPath, nil
}

func cpusetFileToString(path string) (string, error) {
	cpusetFile := fmt.Sprintf("%s%s", path, "/cpuset.cpus")
	cpusetBytes, err := ioutil.ReadFile(cpusetFile)
	if err != nil {
		return "", err
	}

	cpusetStr := strings.TrimSpace(string(cpusetBytes))
	return cpusetStr, nil
}

//END DELETE
*/

func getProfileFromPodAnnotations(pod *corev1.Pod) string {
	annotations := pod.GetAnnotations()
	profile := annotations[powerProfileAnnotation]

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

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
