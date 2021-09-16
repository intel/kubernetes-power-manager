package podresourcesclient

import (
	"context"
	"fmt"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cpuset"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/api/errors"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	"time"
)

var maxMessage = 1024 * 1024 * 4 // size in bytes => 4MB
var socket = "unix:///var/lib/kubelet/pod-resources/kubelet.sock"
var timeout = 2 * time.Minute

// PodResourcesClient stores a client to the Kubelet PodResources API server
type PodResourcesClient struct {
	Client podresourcesapi.PodResourcesListerClient
}

// NewPodResourcesClient returns a new client to the Kubelet PodResources API server
func NewPodResourcesClient() (*PodResourcesClient, error) {
	podResourcesClient := &PodResourcesClient{}
	client, err := getV1Client(socket, timeout, maxMessage)
	if err != nil {
		return podResourcesClient, errors.NewServiceUnavailable("failed to create podresouces client")
	}
	podResourcesClient.Client = client
	return podResourcesClient, nil
}

// getV1Client returns a client for the PodResourcesLister grpc service
func getV1Client(socket string, connectionTimeout time.Duration, maxMsgSize int) (podresourcesapi.PodResourcesListerClient, error) {
	addr, dialer, err := util.GetAddressAndDialer(socket)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure(), grpc.WithContextDialer(dialer), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("error dialing socket %s: %v", socket, err))
	}
	return podresourcesapi.NewPodResourcesListerClient(conn), nil
}

func (p *PodResourcesClient) listPodResources() (*podresourcesapi.ListPodResourcesResponse, error) {
	req := podresourcesapi.ListPodResourcesRequest{}
	resp, err := p.Client.List(context.TODO(), &req)
	if err != nil {
		fmt.Println("Can't receive response:", err)
		return &podresourcesapi.ListPodResourcesResponse{}, err
	}
	return resp, nil
}

// GetContainerCPUs returns a string in cpuset format of CPUs allocated to the container
func (p *PodResourcesClient) GetContainerCPUs(podName, containerName string) (string, error) {
	podresourcesResponse, err := p.listPodResources()
	if err != nil {
		return "", err
	}
	for _, podresource := range podresourcesResponse.PodResources {
		if podresource.Name == podName {
			for _, container := range podresource.Containers {
				if container.Name == containerName {
					cpuSetString := cpuIDsToString(container.CpuIds)
					return cpuSetString, nil
				}
			}
		}
	}
	return "", errors.NewServiceUnavailable(fmt.Sprintf("cpus for Pod:%v Container:%v not found", podName, containerName))
}

// cpuIDsToString returns a string in cpuset format
func cpuIDsToString(cpuIds []int64) string {
	intSlice := make([]int, 0)
	for _, num := range cpuIds {
		intSlice = append(intSlice, int(num))
	}

	cpuSet := cpuset.NewCPUSet(intSlice...)
	cpuSetString := cpuSet.String()

	return cpuSetString
}
