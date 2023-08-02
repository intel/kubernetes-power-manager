package state

import (
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
)

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestNewPowerNodeData(t *testing.T) {
	// calling the function to create a new PowerNodeData object
	powerNodeData := NewPowerNodeData()

	// asserting that the PowerNodeList field is empty
	expected := []string{}
	assert.Equal(t, powerNodeData.PowerNodeList, expected, "PowerNodeList field is not empty.")

	//making sure that list contains only newly added node
	for i := 0; i < len(expected); i++ {
		assert.Equal(t, powerNodeData.PowerNodeList, expected, "PowerNodeList field is not empty.")
		break
	}
}

func TestUpdatePowerNodeData(t *testing.T) {
	nd := NewPowerNodeData()

	nodeName := "GenericNode"
	nd.UpdatePowerNodeData(nodeName)

	found := false
	for _, node := range nd.PowerNodeList {
		if node == nodeName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Failed to add nodeName to PowerNodeList. Got: %v, Expected: %s", nd.PowerNodeList, nodeName)
	}

	// calling the UpdatePowerNodeData function again with the same generic node
	nd.UpdatePowerNodeData(nodeName)

	// making sure that the PowerNodeList remains unchanged
	assert.Equal(t, len(nd.PowerNodeList), 1, "PowerNodeList field is not empty.")
}

func TestDeletePowerNodeData(t *testing.T) {
	testData := &PowerNodeData{
		PowerNodeList: []string{"node1", "node2", "node3"}, //generic nodes
	}

	testData.DeletePowerNodeData("node2") //deleting node2

	expected := []string{"node1", "node3"}

	if !equalSlice(testData.PowerNodeList, expected) {
		t.Fatalf("Expected %v but got %v", expected, testData.PowerNodeList)
	}
}

func TestDifference(t *testing.T) {
	testData := &PowerNodeData{
		PowerNodeList: []string{"node1", "node2", "node3"},
	}

	nodeInfo := []powerv1.WorkloadNode{
		{Name: "node1"},
		{Name: "node3"},
	}

	expected := []string{"node2"}

	result := testData.Difference(nodeInfo)
	for i, node := range result {
		assert.Equal(t, node, expected[i], "PowerNodeList field is not empty.")
		break
	}
}

func TestNodeNotInNodeInfo(t *testing.T) {
	nodeInfo := []powerv1.WorkloadNode{
		{Name: "node1"},
		{Name: "node2"},
		{Name: "node3"},
	}

	testCases := []struct {
		nodeName string
		expected bool
	}{
		{"node1", false},
		{"node2", false},
		{"node3", false},
		{"node4", true},
		{"node5", true},
	}

	for _, testCase := range testCases {
		result := NodeNotInNodeInfo(testCase.nodeName, nodeInfo)
		if result != testCase.expected {
			t.Errorf("For nodeName: %v, expected %v, but got %v", testCase.nodeName, testCase.expected, result)
		}
	}
}
