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
var cPlaneSocket = "unix:///var/lib/kubelet/pod-resources/cci-dra-driver-podrsc.sock"
var cPlaneRetries = 3
var timeout = 2 * time.Minute

// PodResourcesClient stores a client to the Kubelet PodResources API server
type PodResourcesClient struct {
	Client podresourcesapi.PodResourcesListerClient
	CpuControlPlaneClient podresourcesapi.PodResourcesListerClient
}

// NewPodResourcesClient returns a new client to the Kubelet PodResources API server
func NewPodResourcesClient() (*PodResourcesClient, error) {
	return newClient(socket)
}

// NewControlPlaneClient returns a new client to the CPU control plane socket
func NewControlPlaneClient() (*PodResourcesClient, error) {
	return newClient(cPlaneSocket)
}

func NewDualSocketPodClient() (*PodResourcesClient, error) {
	client, err := NewPodResourcesClient()
	if err != nil {
		return client, err
	}
	cPlane, err := NewControlPlaneClient()
	client.CpuControlPlaneClient = cPlane.Client
	return client, err
}

func newClient(socket string) (*PodResourcesClient, error) {
	client, _, err := getV1Client(socket, timeout, maxMessage)
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("failed to create podresources client: %v", err))
	}
	return &PodResourcesClient{Client: client}, nil
}

// GetV1Client returns a client for the PodResourcesLister grpc service
// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/podresources/client.go
func getV1Client(socket string, connectionTimeout time.Duration, maxMsgSize int) (podresourcesapi.PodResourcesListerClient, *grpc.ClientConn, error) {
	addr, dialer, err := util.GetAddressAndDialer(socket)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting address and dialer for socket %s: %v", socket, err)
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

func (p *PodResourcesClient) listResources(controlPlaneClient bool) (*podresourcesapi.ListPodResourcesResponse, error) {
	var client podresourcesapi.PodResourcesListerClient
	clientType := "default"
	if controlPlaneClient && p.CpuControlPlaneClient != nil{
		client = p.CpuControlPlaneClient
		clientType = "cpuControlPlane"
	} else {
		client = p.Client
	}

	req := podresourcesapi.ListPodResourcesRequest{}
	resp, err := client.List(context.TODO(), &req)
	// only default client errs are logged as controlplane socket isn't guaranteed to be there
	// otherwise we'd be spamming the logs in static cpu policy deployments
	if err != nil && clientType == "default" {
		return nil, fmt.Errorf("can't receive response from %s client: %v", clientType, err)
	}
	return resp, nil
}


// GetContainerCPUs returns a string in cpuset format of CPUs allocated to the container
func (p *PodResourcesClient) GetContainerCPUs(podName, containerName string) (string, error) {
	podresourcesResponse, err := p.listResources(false)
	if err != nil {
		return "", err
	}
	cpuSetString, err := parseContainers(podresourcesResponse.PodResources, podName, containerName)
	if err != nil {
		return "", err
	}
	if cpuSetString == "" {
		// if cplane socket responds but has no resources then retry
		// to ensure we get up to date info
		for i := 0; i < cPlaneRetries; i++ {
			podresourcesResponse, err = p.listResources(true)
			if err != nil {
				return "", err
			}
			cpuSetString, err = parseContainers(podresourcesResponse.PodResources, podName, containerName)
			if err == nil && cpuSetString != "" {
				return cpuSetString, err
			}
		}
	}
	return cpuSetString, err
}

func parseContainers(resources []*podresourcesapi.PodResources, podName, containerName string) (string, error) {
	for _, podresource := range resources {
		if podresource.Name == podName {
			for _, container := range podresource.Containers {
				if container.Name == containerName {
					cpuSetString := cpuIDsToString(container.CpuIds)
					return cpuSetString, nil
				}
			}
		}
	}
	return "", errors.NewServiceUnavailable(fmt.Sprintf("resources for Pod:%v Container:%v not found", podName, containerName))
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
