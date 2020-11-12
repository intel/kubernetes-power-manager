package cgroupsparser

import (
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/errors"
	//"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
	"strings"
)

const (
	cpusetFileConst = "/cpuset.cpus"
	kubepodsConst   = "kubepods"
	dockerPrefix    = "docker://"
)

var (
	unifiedCgroupPath = "/sys/fs/cgroup/"
	legacyCgroupPath  = "/sys/fs/cgroup/cpuset/"
	hybridCgroupPath  = "/sys/fs/cgroup/unified/"
)

func GetSharedPool() (string, error) {
	kubepodsCgroup, err := findKubepodsCgroup()
	if err != nil {
		return "", err
	}

	cpuStr, err := cpusetFileToString(kubepodsCgroup)
	if err != nil {
		return "", err
	}

	return cpuStr, nil
}

func findKubepodsCgroup() (string, error) {
	treeVersions := []string{unifiedCgroupPath, legacyCgroupPath, hybridCgroupPath}
	for _, treeVersion := range treeVersions {
		kubepodsCgroupPath, err := findCgroupPath(treeVersion, kubepodsConst)
		if err != nil {
			return "", err
		}

		if kubepodsCgroupPath != "" {
			return kubepodsCgroupPath, nil
		}
	}

	return "", nil
}

func findCgroupPath(base, substring string) (string, error) {
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

func ReadCgroupCpuset(podUID, containerID string) (string, error) {
	containerID = strings.TrimPrefix(containerID, dockerPrefix)
	kubepodsCgroupPath, err := findKubepodsCgroup()
	if err != nil {
		return "", err
	}

	if kubepodsCgroupPath == "" {
		return "", errors.NewServiceUnavailable("kubepods cgroup file not found")
	}

	podCgroupPath, err := findPodCgroup(kubepodsCgroupPath, podUID)
	if err != nil {
		return "", err
	}

	if podCgroupPath == "" {
		return "", errors.NewServiceUnavailable(fmt.Sprintf("podUID %s not found in kubepods cgroup %s", podUID, kubepodsCgroupPath))
	}

	containerCgroupPath, err := findCgroupPath(podCgroupPath, containerID)
	if err != nil {
		return "", err
	}

	if containerCgroupPath == "" {
		return "", errors.NewServiceUnavailable(fmt.Sprintf("containerID %s not found in pod cgroup %s", containerID, podCgroupPath))
	}

	cpuStr, err := cpusetFileToString(containerCgroupPath)
	if err != nil {
		return "", err
	}

	return cpuStr, nil
}

func findPodCgroup(kubepodsCgroupPath, podUID string) (string, error) {
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

func cpusetFileToString(path string) (string, error) {
	cpusetFile := fmt.Sprintf("%s%s", path, cpusetFileConst)
	cpusetBytes, err := ioutil.ReadFile(cpusetFile)
	if err != nil {
		return "", err
	}

	cpusetStr := strings.TrimSpace(string(cpusetBytes))
	//cpuSet := cpuset.MustParse(cpusetStr)

	//return cpuSet.String(), nil
	return cpusetStr, nil
}
