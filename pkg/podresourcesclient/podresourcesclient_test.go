package podresourcesclient

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	"testing"
)

type fakePodResourcesClient struct {
	listResponse *podresourcesapi.ListPodResourcesResponse
}

func (f *fakePodResourcesClient) List(ctx context.Context, in *podresourcesapi.ListPodResourcesRequest, opts ...grpc.CallOption) (*podresourcesapi.ListPodResourcesResponse, error) {
	return f.listResponse, nil
}

func (f *fakePodResourcesClient) GetAllocatableResources(ctx context.Context, in *podresourcesapi.AllocatableResourcesRequest, opts ...grpc.CallOption) (*podresourcesapi.AllocatableResourcesResponse, error) {
	return &podresourcesapi.AllocatableResourcesResponse{}, nil
}

func (f *fakePodResourcesClient) Get(ctx context.Context, in *podresourcesapi.GetPodResourcesRequest, opts ...grpc.CallOption) (*podresourcesapi.GetPodResourcesResponse, error) {
	return &podresourcesapi.GetPodResourcesResponse{}, nil
}
func createFakePodResourcesListerClient(fakePodResources []*podresourcesapi.PodResources) *PodResourcesClient {
	fakeListResponse := &podresourcesapi.ListPodResourcesResponse{
		PodResources: fakePodResources,
	}

	podResourcesListerClient := &fakePodResourcesClient{}
	podResourcesListerClient.listResponse = fakeListResponse
	return &PodResourcesClient{Client: podResourcesListerClient, CpuControlPlaneClient: podResourcesListerClient}
}

func TestPodResourcesClient_listPodResources(t *testing.T) {
	podResources := []*podresourcesapi.PodResources{
		{
			Name:      "test-pod-1",
			Namespace: "default",
			Containers: []*podresourcesapi.ContainerResources{
				{
					Name:   "test-container-1",
					CpuIds: []int64{1, 5, 8},
				},
			},
		},
	}
	podClient := createFakePodResourcesListerClient(podResources)
	resp, err := podClient.listResources(false)
	assert.ElementsMatch(t, resp.PodResources, podResources)
	assert.NoError(t, err)
}

func TestPodResourcesClient_GetContainerCPUs(t *testing.T) {
	podResources := []*podresourcesapi.PodResources{
		{
			Name:      "test-pod-1",
			Namespace: "default",
			Containers: []*podresourcesapi.ContainerResources{
				{
					Name:   "test-container-1",
					CpuIds: []int64{1, 5, 8},
				},
			},
		},
	}
	podClient := createFakePodResourcesListerClient(podResources)
	cpus, err := podClient.GetContainerCPUs("test-pod-1", "test-container-1")
	assert.Equal(t, "1,5,8", cpus)
	assert.NoError(t, err)

	cpus, err = podClient.GetContainerCPUs("non-existent", "")
	assert.Empty(t, cpus)
	assert.ErrorContains(t, err, "not found")
}

func Test_cpuIDsToString(t *testing.T) {

	assert.Equal(t, "0-5", cpuIDsToString([]int64{0, 1, 2, 3, 4, 5}))
	assert.Equal(t, "0-2,4-6", cpuIDsToString([]int64{0, 1, 2, 4, 5, 6}))
	assert.Equal(t, "0-1,8", cpuIDsToString([]int64{0, 8, 1}))
	assert.Equal(t, "", cpuIDsToString([]int64{}))
	assert.Equal(t, "0,2", cpuIDsToString([]int64{0, 0, 0, 2, 2, 2, 0, 2, 0, 2, 0}))
	maxSize := 9999
	cpuIds := make([]int64, maxSize)
	for i := range cpuIds {
		cpuIds[i] = int64(i)
	}
	assert.Equal(t, fmt.Sprint("0-", maxSize-1), cpuIDsToString(cpuIds))

}
