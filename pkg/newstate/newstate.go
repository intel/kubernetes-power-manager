package newstate

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
