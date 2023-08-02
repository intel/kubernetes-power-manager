package podstate

import (
	"reflect"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestNewState(t *testing.T) {
	state, err := NewState()

	//Test case -- verifying that no error is returned
	assert.Nil(t, err)

	//Test case -- verifying that a state object is returned
	if state == nil {
		t.Errorf("Expected a State object, but got nil")
	}

	//Test case -- verifying that the GuaranteedPods slice is empty
	if len(state.GuaranteedPods) != 0 {
		t.Errorf("Expected GuaranteedPods to be empty, but got %v", state.GuaranteedPods)
	}
}

func TestUpdateStateGuaranteedPods(t *testing.T) {
	state := &State{
		GuaranteedPods: []powerv1.GuaranteedPod{
			{Name: "pod1"},
			{Name: "pod2"},
			{Name: "pod3"},
		},
	}

	//Test case -- updating an existing pod
	err := state.UpdateStateGuaranteedPods(powerv1.GuaranteedPod{Name: "pod2"})
	assert.Nil(t, err)
	assert.Equal(t, state.GuaranteedPods[1].Name, "pod2")

	//Test case -- adding a new pod
	err = state.UpdateStateGuaranteedPods(powerv1.GuaranteedPod{Name: "pod4"})
	assert.Nil(t, err)
	assert.Equal(t, state.GuaranteedPods[len(state.GuaranteedPods)-1].Name, "pod4")
}

func TestGetPodFromState(t *testing.T) {
	//Test case -- a state instance with some sample data
	state := &State{
		GuaranteedPods: []powerv1.GuaranteedPod{
			{Name: "pod1"},
			{Name: "pod2"},
		},
	}

	//Test case -- Existing pod in the state
	existingPodName := "pod1"
	existingPod := state.GetPodFromState(existingPodName, "")
	assert.Equal(t, existingPodName, existingPod.Name)

	//Test case -- Non-Existing pod in the state
	nonExistingPodName := "pod4"
	nonExistingPod := state.GetPodFromState(nonExistingPodName, "")
	if !reflect.DeepEqual(nonExistingPod, powerv1.GuaranteedPod{}) {
		t.Errorf("Expected: empty pod. Actual: Got pod %+v", nonExistingPod)
	}
}

func TestGetCPUsFromPodState(t *testing.T) {
	//Test case -- sample pod state with two containers
	podState := powerv1.GuaranteedPod{
		Containers: []powerv1.Container{
			{ExclusiveCPUs: []uint{1, 2}},
			{ExclusiveCPUs: []uint{3}},
		},
	}

	//Creating a state instance
	state := &State{}
	cpus := state.GetCPUsFromPodState(podState)
	expectedCPUs := []uint{1, 2, 3}

	assert.Equal(t, expectedCPUs, cpus)

}

func TestDeletePodFromState(t *testing.T) {
	//Create a state instance with some sample data

	state := &State{
		GuaranteedPods: []powerv1.GuaranteedPod{
			{Name: "pod1"},
			{Name: "pod2"},
		},
	}

	//Test case: Delteing an existing pod
	podToDelete := "pod1"
	err := state.DeletePodFromState(podToDelete, "")
	assert.Nil(t, err)

	for _, pod := range state.GuaranteedPods {
		if pod.Name == podToDelete {
			t.Errorf("Expected: pod %q to be deleted. Actual: Still exists", podToDelete)
		}
	}

	//Test case -- Deleting a non-existing pod
	nonExistingPodState := "pod3"
	err = state.DeletePodFromState(nonExistingPodState, "")
	assert.Nil(t, err)
}
