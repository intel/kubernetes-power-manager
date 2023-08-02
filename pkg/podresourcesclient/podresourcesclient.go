package podresourcesclient

import (
	"context"
	"fmt"
	"time"

	"github.com/intel/kubernetes-power-manager/pkg/cpuset"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/api/errors"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
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
	client, _, err := getV1Client(socket, timeout, maxMessage)
	if err != nil {
		return podResourcesClient, errors.NewServiceUnavailable("failed to create podresouces client")
	}
	podResourcesClient.Client = client
	return podResourcesClient, nil
}

// GetV1Client returns a client for the PodResourcesLister grpc service
// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/podresources/client.go
func getV1Client(socket string, connectionTimeout time.Duration, maxMsgSize int) (podresourcesapi.PodResourcesListerClient, *grpc.ClientConn, error) {
	addr, dialer, err := util.GetAddressAndDialer(socket)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing socket %s: %v", socket, err)
	}
	return podresourcesapi.NewPodResourcesListerClient(conn), conn, nil
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
