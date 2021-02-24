package newstate

type PowerNodeData struct {
	PowerNodeList      []string
	ProfileAssociation map[string][]string
}

func NewPowerNodeData() *PowerNodeData {
	profAssoc := make(map[string][]string, 0)

	return &PowerNodeData{
		PowerNodeList:      []string{},
		ProfileAssociation: profAssoc,
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

func (nd *PowerNodeData) AddProfile(profileName string) {
	if _, exists := nd.ProfileAssociation[profileName]; !exists {
		nd.ProfileAssociation[profileName] = make([]string, 0)
	}
}

func (nd *PowerNodeData) UpdateProfileAssociation(profileName string, workloadName string) {
	if workloads, exists := nd.ProfileAssociation[profileName]; !exists {
		workloads := []string{workloadName}
		nd.ProfileAssociation[profileName] = workloads
	} else {
		workloads = append(workloads, workloadName)
	}
}

func (nd *PowerNodeData) DeleteWorkloadFromProfile(profileName string, workloadName string) {
	if workloads, exists := nd.ProfileAssociation[profileName]; exists {
		for index, workload := range workloads {
			if workload == workloadName {
				workloads = append(workloads[:index], workloads[index+1:]...)
				nd.ProfileAssociation[profileName] = workloads
			}
		}
	}
}
