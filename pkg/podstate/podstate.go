package podstate

import (
	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	//cgp "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cgroupsparser"
	//"k8s.io/apimachinery/pkg/api/errors"
	//"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
)

/*
type State struct {
	PowerNodeStatus *powerv1alpha1.PowerNodeStatus
}
*/

type State struct {
	GuaranteedPods []powerv1alpha1.GuaranteedPod
	//PowerNodeStatus *powerv1alpha1.PowerNodeCPUState
	//AppQoSClient	*appqos.AppQoSClient
}

//func NewState(appqosclient *appqos.AppQoSClient) (*State, error) {
func NewState() (*State, error) {
	state := &State{}
	//sharedPool := make([]int, 0)

	//state.PowerNodeStatus = &powerv1alpha1.PowerNodeStatus{}
	//state.PowerNodeStatus = &powerv1alpha1.PowerNodeCPUState{}
	//state.PowerNodeStatus.SharedPool = sharedPool

	guaranteedPods := make([]powerv1alpha1.GuaranteedPod, 0)
	state.GuaranteedPods = guaranteedPods

	return state, nil
}

func (s *State) UpdateStateGuaranteedPods(guaranteedPod powerv1alpha1.GuaranteedPod) error {
	/*
		if len(s.PowerNodeStatus.SharedPool) > 0 {
			// Remove CPUs from Shared Pool in Node's state
			allCoresInPod := make([]int, 0)
			for container := range guaranteedPod.Containers {
				allCoresInPod = append(allCoresInPod, container.ExclusiveCPUs...)
			}

			updatedSharedPool := util.CPUListDifference(allCoresInPod, s.PowerNodeStatus.SharedPool)
			s.PowerNodeStatus.SharedPool = updatedSharedPool
		}
	*/

	/*
		if len(s.PowerNodeStatus.GuaranteedPods) == 0 {
			pods := make([]powerv1alpha1.GuaranteedPod, 0)
			s.PowerNodeStatus.GuaranteedPods = pods
			return nil
		}
	*/

	/*
		if len(s.GuaranteedPods) == 0 {
			s.GuaranteedPods = append(s.GuaranteedPods, guaranteedPod)
			return nil
		}
	*/

	// Check existing pods in state. If pod aleady exists, update and return
	/*
		for i, existingPod := range s.PowerNodeStatus.GuaranteedPods {
			if existingPod.Name == guaranteedPod.Name {
				s.PowerNodeStatus.GuaranteedPods[i] = guaranteedPod
				return nil
			}
		}
	*/

	for i, existingPod := range s.GuaranteedPods {
		if existingPod.Name == guaranteedPod.Name {
			s.GuaranteedPods[i] = guaranteedPod
			return nil
		}
	}

	// Otherwise append pod to GuaranteedPods in state
	/*
		s.PowerNodeStatus.GuaranteedPods = append(s.PowerNodeStatus.GuaranteedPods, guaranteedPod)
		return nil
	*/

	s.GuaranteedPods = append(s.GuaranteedPods, guaranteedPod)
	return nil
}

func (s *State) GetPodFromState(podName string) powerv1alpha1.GuaranteedPod {
	/*
		for _, existingPod := range s.PowerNodeStatus.GuaranteedPods {
			if existingPod.Name == podName {
				return existingPod
			}
		}
	*/

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
		for _, cpu := range container.ExclusiveCPUs {
			cpus = append(cpus, cpu)
		}
	}

	return cpus
}

func (s *State) DeletePodFromState(podName string) error {
	/*
		if len(s.PowerNodeStatus.GuaranteedPods) == 0 {
			pods := make([]powerv1alpha1.GuaranteedPod, 0)
			s.PowerNodeStatus.GuaranteedPods = pods
			return nil
		}

		for i, pod := range s.PowerNodeStatus.GuaranteedPods {
			if pod.Name == podName {
				s.PowerNodeStatus.GuaranteedPods = append(s.PowerNodeStatus.GuaranteedPods[:i], s.PowerNodeStatus.GuaranteedPods[i+1:]...)
				return nil
			}
		}
	*/

	for i, pod := range s.GuaranteedPods {
		if pod.Name == podName {
			s.GuaranteedPods = append(s.GuaranteedPods[:i], s.GuaranteedPods[i+1:]...)
		}
	}

	return nil
}
