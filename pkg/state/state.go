package state

import (
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
)

type PowerNodeData struct {
	PowerNodeList []string
}

func NewPowerNodeData() *PowerNodeData {
	return &PowerNodeData{
		PowerNodeList: []string{},
	}
}

func (nd *PowerNodeData) UpdatePowerNodeData(nodeName string) {
	for _, node := range nd.PowerNodeList {
		if nodeName == node {
			return
		}
	}

	nd.PowerNodeList = append(nd.PowerNodeList, nodeName)
}

func (nd *PowerNodeData) DeletePowerNodeData(nodeName string) {
	for index, node := range nd.PowerNodeList {
		if node == nodeName {
			nd.PowerNodeList = append(nd.PowerNodeList[:index], nd.PowerNodeList[index+1:]...)
		}
	}
}

func (nd *PowerNodeData) Difference(nodeInfo []powerv1.WorkloadNode) []string {
	difference := make([]string, 0)

	for _, node := range nd.PowerNodeList {
		if NodeNotInNodeInfo(node, nodeInfo) {
			difference = append(difference, node)
		}
	}

	return difference
}

func NodeNotInNodeInfo(nodeName string, nodeInfo []powerv1.WorkloadNode) bool {
	for _, node := range nodeInfo {
		if nodeName == node.Name {
			return false
		}
	}

	return true
}
