package controllers

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	//"k8s.io/apimachinery/pkg/api/errors"
	grpc "google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/podresourcesclient"
	"github.com/intel/kubernetes-power-manager/pkg/podstate"

	//"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createFakePodResourcesListerClient(fakePodResources []*podresourcesapi.PodResources) *podresourcesclient.PodResourcesClient {
	fakeListResponse := &podresourcesapi.ListPodResourcesResponse{
		PodResources: fakePodResources,
	}

	podResourcesListerClient := &fakePodResourcesClient{}
	podResourcesListerClient.listResponse = fakeListResponse
	return &podresourcesclient.PodResourcesClient{Client: podResourcesListerClient}
}

func createPodReconcilerObject(objs []runtime.Object, podResourcesClient *podresourcesclient.PodResourcesClient) (*PowerPodReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()

	state, err := podstate.NewState()
	if err != nil {
		return nil, err
	}

	// Create a ReconcileNode object with the scheme and fake client.
	r := &PowerPodReconciler{cl, ctrl.Log.WithName("testing"), s, *state, *podResourcesClient}

	return r, nil
}

func TestPodCreation(t *testing.T) {
	tcases := []struct {
		testCase       string
		nodeName       string
		podName        string
		podResources   []*podresourcesapi.PodResources
		clientObjs     []runtime.Object
		workloadName   string
		expectedCpuIds []uint
	}{
		{
			testCase: "Test Case 1 - Single container",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadName:   "performance-TestNode",
			expectedCpuIds: []uint{1, 2, 3},
		},
		{
			testCase: "Test Case 2 - Two containers",
			nodeName: "TestNode",
			podName:  "test-pod-2",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-2",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
						{
							Name:   "test-container-2",
							CpuIds: []int64{4, 5, 6},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-2",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
							{
								Name: "test-container-2",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
							{
								Name:        "example-container-2",
								ContainerID: "docker://hijklmnop",
							},
						},
					},
				},
			},
			workloadName:   "performance-TestNode",
			expectedCpuIds: []uint{1, 2, 3, 4, 5, 6},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		podResourcesClient := createFakePodResourcesListerClient(tc.podResources)

		r, err := createPodReconcilerObject(tc.clientObjs, podResourcesClient)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.podName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		workload := &powerv1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.workloadName,
			Namespace: IntelPowerNamespace,
		}, workload)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Object", tc.testCase)
		}

		sortedCpuIds := workload.Spec.Node.CpuIds
		sort.Slice(workload.Spec.Node.CpuIds, func(i, j int) bool {
			return workload.Spec.Node.CpuIds[i] < workload.Spec.Node.CpuIds[j]
		})
		if !reflect.DeepEqual(tc.expectedCpuIds, sortedCpuIds) {
			t.Errorf("%s Failed - expected Cpu Ids to be %v, got %v", tc.testCase, tc.expectedCpuIds, sortedCpuIds)
		}
	}
}

func TestPodControllerErrors(t *testing.T) {
	tcases := []struct {
		testCase      string
		nodeName      string
		podName       string
		podResources  []*podresourcesapi.PodResources
		clientObjs    []runtime.Object
		workloadNames []string
	}{
		{
			testCase: "Test Case 1 - Pod Not Running error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodPending,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
			},
		},
		{
			testCase: "Test Case 2 - No Pod UID error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
			},
		},
		{
			testCase: "Test Case 3 - More Than One Profile error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "balance-performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "balance-performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
							{
								Name: "test-container-2",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                                 *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                              *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/balance-performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                                 *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                              *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/balance-performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
							{
								Name:        "example-container-2",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
				"balance-performance-TestNode",
			},
		},
		{
			testCase: "Test Case 4 - Resource Mismatch error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
			},
		},
		{
			testCase: "Test Case 5 - Profile CR Does Not Exist error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "balance-performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "balance-performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		podResourcesClient := createFakePodResourcesListerClient(tc.podResources)

		r, err := createPodReconcilerObject(tc.clientObjs, podResourcesClient)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.podName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err == nil {
			t.Errorf("%s Failed - expected Pod controller to have failed", tc.testCase)
		}

		for _, workloadName := range tc.workloadNames {
			workload := &powerv1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      workloadName,
				Namespace: IntelPowerNamespace,
			}, workload)
			if err != nil {
				t.Error(err)
				t.Fatalf("%s - error retrieving Power Workload Object", tc.testCase)
			}

			if len(workload.Spec.Node.CpuIds) > 0 {
				t.Errorf("%s Failed - expected Cpu Ids to be empty, got %v", tc.testCase, workload.Spec.Node.CpuIds)
			}
		}
	}
}

func TestPodControllerReturningNil(t *testing.T) {
	tcases := []struct {
		testCase      string
		nodeName      string
		podName       string
		podResources  []*podresourcesapi.PodResources
		clientObjs    []runtime.Object
		workloadNames []string
	}{
		{
			testCase: "Test Case 1 - Incorrect Node error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "balance-performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "balance-performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "IncorrectNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
			},
		},
		{
			testCase: "Test Case 2 - Kube-System Namespace error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "balance-performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "balance-performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "kube-system",
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSGuaranteed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
			},
		},
		{
			testCase: "Test Case 3 - Not Exclusive Pod error",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: IntelPowerNamespace,
						UID:       "abcdefg",
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
						Containers: []corev1.Container{
							{
								Name: "test-container-1",
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
										corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(3, resource.DecimalSI),
									},
								},
							},
						},
						EphemeralContainers: []corev1.EphemeralContainer{},
					},
					Status: corev1.PodStatus{
						Phase:    corev1.PodRunning,
						QOSClass: corev1.PodQOSBurstable,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:        "example-container-1",
								ContainerID: "docker://abcdefg",
							},
						},
					},
				},
			},
			workloadNames: []string{
				"performance-TestNode",
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		podResourcesClient := createFakePodResourcesListerClient(tc.podResources)

		r, err := createPodReconcilerObject(tc.clientObjs, podResourcesClient)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.podName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Errorf("%s Failed - expected Pod controller to not have failed", tc.testCase)
		}

		for _, workloadName := range tc.workloadNames {
			workload := &powerv1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      workloadName,
				Namespace: IntelPowerNamespace,
			}, workload)
			if err != nil {
				t.Error(err)
				t.Fatalf("%s - error retrieving Power Workload Object", tc.testCase)
			}

			if len(workload.Spec.Node.CpuIds) > 0 {
				t.Errorf("%s Failed - expected Cpu Ids to be empty, got %v", tc.testCase, workload.Spec.Node.CpuIds)
			}
		}
	}
}

func TestPodDeletion(t *testing.T) {
	tcases := []struct {
		testCase      string
		nodeName      string
		podName       string
		podResources  []*podresourcesapi.PodResources
		clientObjs    []runtime.Object
		guaranteedPod powerv1.GuaranteedPod
		workloadName  string
	}{
		{
			testCase: "Test Case 1: Single Container",
			nodeName: "TestNode",
			podName:  "test-pod-1",
			podResources: []*podresourcesapi.PodResources{
				{
					Name:      "test-pod-1",
					Namespace: IntelPowerNamespace,
					Containers: []*podresourcesapi.ContainerResources{
						{
							Name:   "test-container-1",
							CpuIds: []int64{1, 2, 3},
						},
					},
				},
			},
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name: "performance-TestNode",
						Node: powerv1.WorkloadNode{
							Name:       "TestNode",
							Containers: []powerv1.Container{},
							CpuIds:     []uint{1, 2, 3},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-pod-1",
						Namespace:         IntelPowerNamespace,
						UID:               "abcdefg",
						DeletionTimestamp: &metav1.Time{Time: time.Date(9999, time.Month(1), 21, 1, 10, 30, 0, time.UTC)},
					},
					Spec: corev1.PodSpec{
						NodeName: "TestNode",
					},
				},
			},
			guaranteedPod: powerv1.GuaranteedPod{
				Node: "TestNode",
				Name: "test-pod-1",
				UID:  "abcdefg",
				Containers: []powerv1.Container{
					{
						Name:          "test-container-1",
						Id:            "abcdefg",
						Pod:           "test-pod-1",
						ExclusiveCPUs: []uint{1, 2, 3},
						PowerProfile:  "performance",
						Workload:      "performance-TestNode",
					},
				},
			},
			workloadName: "performance-TestNode",
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		podResourcesClient := createFakePodResourcesListerClient(tc.podResources)

		r, err := createPodReconcilerObject(tc.clientObjs, podResourcesClient)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		err = r.State.UpdateStateGuaranteedPods(tc.guaranteedPod)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s Failed - error adding Pod's State", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.podName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Errorf("%s Failed - expected Pod controller to not have failed", tc.testCase)
		}

		workload := &powerv1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.workloadName,
			Namespace: IntelPowerNamespace,
		}, workload)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Object", tc.testCase)
		}

		if len(workload.Spec.Node.CpuIds) > 0 {
			t.Errorf("%s Failed - expected Cpu Ids to be empty, got %v", tc.testCase, workload.Spec.Node.CpuIds)
		}
	}
}
