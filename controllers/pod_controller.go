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
	"io/ioutil"
	"strings"

	"github.com/go-logr/logr"
	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	//cgroupsparser "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cgroupsparser"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	powerProfileAnnotation = "PowerProfile"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=power.intel.com,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=pods/status,verbs=get;update;patch

func (r *PodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerpod", req.NamespacedName)

	logger.Info("Pod creation has been detected...")
	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), req.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod has been deleted
			// For now, simply ignore
			// TODO: delete Pod Config CR
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

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

	// DELETE
	logger.Info("Containers requesting exclusive CPUs:")
	for _, c := range containersRequestingExclusiveCPUs {
		logger.Info(fmt.Sprintf("- %s", c))
	}
	// END DELETE

	guaranteedPod := &powerv1alpha1.GuaranteedPod{}
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
		//coreIDs, err := cgroupsparser.ReadCgroupCpuset(string(podUID), containerID)
		coreIDs, err := ReadCgroupCpuset(string(podUID), containerID)
		if err != nil {
			logger.Error(err, "failed to retrieve cpuset form groups")
			return ctrl.Result{}, err
		}

		powerContainer := &powerv1alpha1.Container{}
		powerContainer.Name = container
		powerContainer.ID = strings.TrimPrefix(containerID, "docker://")
		powerContainer.ExclusiveCPUs = coreIDs

		powerContainers = append(powerContainers, *powerContainer)
		allCores = append(allCores, coreIDs)
	}
	guaranteedPod.Containers = make([]powerv1alpha1.Container, 0)
	guaranteedPod.Containers = powerContainers

	// Get the PowerProfile requested by the Pod
	profileName := getProfileFromPodAnnotations(pod)
	if profileName == "" {
		logger.Info("No PowerProfile detected, skipping...")
	} else {
		logger.Info(fmt.Sprintf("PowerProfile: %s", profileName))
	}

	profileList := &powerv1alpha1.ProfileList{}
	err = r.Client.List(context.TODO(), profileList)
	if err != nil {
		logger.Info("Something went wrong")
		return ctrl.Result{}, err
	}

	profile, err := getProfileFromProfileList(profileName, profileList)
	if err != nil {
		logger.Error(err, "failed to retrieve profile")
		// Don't keep looping
		return ctrl.Result{}, nil
	}
	guaranteedPod.Profile = profile

	logger.Info("Guaranteed Pod Configuration:")
	logger.Info(fmt.Sprintf("Pod Name: %s", guaranteedPod.Name))
	logger.Info(fmt.Sprintf("Pod UID: %s", guaranteedPod.UID))
	for i, container := range guaranteedPod.Containers {
		logger.Info(fmt.Sprintf("Container %d", i))
		logger.Info(fmt.Sprintf("- Name: %s", container.Name))
		logger.Info(fmt.Sprintf("- ID: %s", container.ID))
		logger.Info(fmt.Sprintf("- Cores: %s", container.ExclusiveCPUs))
	}
	logger.Info(fmt.Sprintf("Profile Name: %s", guaranteedPod.Profile.Spec.Name))
	logger.Info(fmt.Sprintf("Profile Max: %d", guaranteedPod.Profile.Spec.Max))
	logger.Info(fmt.Sprintf("Profile Min: %d", guaranteedPod.Profile.Spec.Min))
	logger.Info(fmt.Sprintf("Profile Cstate: %t", guaranteedPod.Profile.Spec.Cstate))

	// Create the Config associated with the Profile
	logger.Info("Creating Config based on provided Profile")
	placeholderNodes := []string{"Placeholder"}
	configSpec := &powerv1alpha1.ConfigSpec{
		Nodes:   placeholderNodes,
		CpuIds:  allCores,
		Profile: guaranteedPod.Profile,
	}
	config := &powerv1alpha1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      fmt.Sprintf("%s-config", profileName),
		},
	}
	config.Spec = *configSpec
	err = r.Client.Create(context.TODO(), config)

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
