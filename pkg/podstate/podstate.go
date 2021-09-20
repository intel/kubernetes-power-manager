package podstate

import (
	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
)

type State struct {
	GuaranteedPods []powerv1alpha1.GuaranteedPod
}

//func NewState(appqosclient *appqos.AppQoSClient) (*State, error) {
func NewState() (*State, error) {
	state := &State{}
	guaranteedPods := make([]powerv1alpha1.GuaranteedPod, 0)
	state.GuaranteedPods = guaranteedPods

	return state, nil
}

func (s *State) UpdateStateGuaranteedPods(guaranteedPod powerv1alpha1.GuaranteedPod) error {
	for i, existingPod := range s.GuaranteedPods {
		if existingPod.Name == guaranteedPod.Name {
			s.GuaranteedPods[i] = guaranteedPod
			return nil
		}
	}

	s.GuaranteedPods = append(s.GuaranteedPods, guaranteedPod)
	return nil
}

func (s *State) GetPodFromState(podName string) powerv1alpha1.GuaranteedPod {
	for _, existingPod := range s.GuaranteedPods {
		if existingPod.Name == podName {
			return existingPod
		}
	}

	return powerv1alpha1.GuaranteedPod{}
}

func (s *State) GetCPUsFromPodState(podState powerv1alpha1.GuaranteedPod) []int {
	cpus := make([]int, 0)
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
