package podstate

import (
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
)

type State struct {
	GuaranteedPods []powerv1.GuaranteedPod
}

func NewState() (*State, error) {
	state := &State{}
	guaranteedPods := make([]powerv1.GuaranteedPod, 0)
	state.GuaranteedPods = guaranteedPods

	return state, nil
}

func (s *State) UpdateStateGuaranteedPods(guaranteedPod powerv1.GuaranteedPod) error {
	for i, existingPod := range s.GuaranteedPods {
		if existingPod.Name == guaranteedPod.Name {
			s.GuaranteedPods[i] = guaranteedPod
			return nil
		}
	}

	s.GuaranteedPods = append(s.GuaranteedPods, guaranteedPod)
	return nil
}

func (s *State) GetPodFromState(podName string) powerv1.GuaranteedPod {
	for _, existingPod := range s.GuaranteedPods {
		if existingPod.Name == podName {
			return existingPod
		}
	}

	return powerv1.GuaranteedPod{}
}

func (s *State) GetCPUsFromPodState(podState powerv1.GuaranteedPod) []uint {
	cpus := make([]uint, 0)
	for _, container := range podState.Containers {
		cpus = append(cpus, container.ExclusiveCPUs...)
	}

	return cpus
}

func (s *State) DeletePodFromState(podName string) error {
	for i, pod := range s.GuaranteedPods {
		if pod.Name == podName {
			s.GuaranteedPods = append(s.GuaranteedPods[:i], s.GuaranteedPods[i+1:]...)
		}
	}

	return nil
}
