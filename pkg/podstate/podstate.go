package podstate

import (
	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	cgp "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cgroupsparser"
	"k8s.io/apimachinery/pkg/api/errors"
)

type State struct {
	PowerNodeStatus *powerv1alpha1.PowerNodeStatus
}

func NewState() (*State, error) {
	state := &State{}
	sharedPool, err := cgp.GetSharedPool()
	if err != nil {
		return state, err
	}

	if len(sharedPool) == 0 {
		return state, errors.NewServiceUnavailable("No shared pool discovered - kubepods cpuset cgroup not found")
	}

	state.PowerNodeStatus = &powerv1alpha1.PowerNodeStatus{}
	state.PowerNodeStatus.SharedPool = sharedPool

	return state, nil
}

func (s *State) UpdateStateGuaranteedPods(guaranteedPod powerv1alpha1.GuaranteedPod) error {
	if len(s.PowerNodeStatus.GuaranteedPods) == 0 {
		pods := make([]powerv1alpha1.GuaranteedPod, 0)
		s.PowerNodeStatus.GuaranteedPods = pods
	}

	// Check existing pods in state. If pod aleady exists, update and return
	for i, existingPod := range s.PowerNodeStatus.GuaranteedPods {
		if existingPod.Name == guaranteedPod.Name {
			s.PowerNodeStatus.GuaranteedPods[i] = guaranteedPod
			return nil
		}
	}

	// Otherwise append pod to GuaranteedPods in state
	s.PowerNodeStatus.GuaranteedPods = append(s.PowerNodeStatus.GuaranteedPods, guaranteedPod)
	return nil
}

func (s *State) GetPodFromState(podName string) powerv1alpha1.GuaranteedPod {
	for _, existingPod := range s.PowerNodeStatus.GuaranteedPods {
		if existingPod.Name == podName {
			return existingPod
		}
	}

	return powerv1alpha1.GuaranteedPod{}
}

func (s *State) GetCPUsFromPodState(podState powerv1alpha1.GuaranteedPod) []int {
	cpus := make([]int, 0)
	for _, container := range podState.Containers {
		for _, cpu := range container.ExclusiveCPUs {
			cpus = append(cpus, cpu)
		}
	}

	return cpus
}
