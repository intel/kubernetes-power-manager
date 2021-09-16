package controllers

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podresourcesclient"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podstate"
	grpc "google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	PowerPodName      = "TestPowerPod"
	PowerPodNamespace = "default"
)

type fakePodResourcesClient struct {
	listResponse *podresourcesapi.ListPodResourcesResponse
}

func (f *fakePodResourcesClient) List(ctx context.Context, in *podresourcesapi.ListPodResourcesRequest, opts ...grpc.CallOption) (*podresourcesapi.ListPodResourcesResponse, error) {
	return f.listResponse, nil
}

func createPowerPodReconcilerObject(objs []runtime.Object) (*PowerPodReconciler, error) {
	s := scheme.Scheme

	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)

	cl := fake.NewFakeClient(objs...)

	state, err := podstate.NewState()
	if err != nil {
		return nil, err
	}

	r := &PowerPodReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerWorkload"), Scheme: s, State: *state}
	return r, nil
}

func createFakePodResourcesListerClient(listResponse *podresourcesapi.ListPodResourcesResponse) *podresourcesclient.PodResourcesClient {
	podResourcesListerClient := &fakePodResourcesClient{}
	podResourcesListerClient.listResponse = listResponse
	return &podresourcesclient.PodResourcesClient{Client: podResourcesListerClient}
}

func TestPodReconcileNewWorkloadCreated(t *testing.T) {
	tcases := []struct {
		testCase                                string
		pods                                    *corev1.PodList
		node                                    *corev1.Node
		powerProfiles                           *powerv1alpha1.PowerProfileList
		resources                               map[string]string
		podResources                            []podresourcesapi.PodResources
		containerResources                      map[string][]podresourcesapi.ContainerResources
		expectedNumberOfPowerWorkloads          int
		expectedPowerWorkloadName               string
		expectedNumberOfPowerWorkloadContainers int
		expectedPowerWorkloadContainerCpuIds    map[string][]int
		expectedPowerWorkloadCpuIds             []int
		expectedPowerWorkloadPowerProfile       string
	}{
		{
			testCase: "Test Case 1",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-pod",
							Namespace: PowerPodNamespace,
							UID:       "abcdefg",
						},
						Spec: corev1.PodSpec{
							NodeName: "example-node1",
							Containers: []corev1.Container{
								{
									Name: "example-container-1",
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
									},
								},
							},
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
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerPodNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
				},
			},
			resources: map[string]string{
				"cpu":    "2",
				"memory": "200Mi",
				"power.intel.com/performance-example-node1": "2",
			},
			podResources: []podresourcesapi.PodResources{
				{
					Name:       "example-pod",
					Containers: []*podresourcesapi.ContainerResources{},
				},
			},
			containerResources: map[string][]podresourcesapi.ContainerResources{
				"example-pod": []podresourcesapi.ContainerResources{
					{
						Name:   "example-container-1",
						CpuIds: []int64{1, 2},
					},
				},
			},
			expectedNumberOfPowerWorkloads:          1,
			expectedPowerWorkloadName:               "performance-example-node1-workload",
			expectedNumberOfPowerWorkloadContainers: 1,
			expectedPowerWorkloadContainerCpuIds: map[string][]int{
				"example-container-1": []int{1, 2},
			},
			expectedPowerWorkloadCpuIds:       []int{1, 2},
			expectedPowerWorkloadPowerProfile: "performance-example-node1",
		},
		{
			testCase: "Test Case 2",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-pod",
							Namespace: PowerPodNamespace,
							UID:       "abcdefg",
						},
						Spec: corev1.PodSpec{
							NodeName: "example-node1",
							Containers: []corev1.Container{
								{
									Name: "example-container-1",
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
									},
								},
								{
									Name: "example-container-2",
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
									},
								},
							},
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
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerPodNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
				},
			},
			podResources: []podresourcesapi.PodResources{
				{
					Name:       "example-pod",
					Containers: []*podresourcesapi.ContainerResources{},
				},
			},
			containerResources: map[string][]podresourcesapi.ContainerResources{
				"example-pod": []podresourcesapi.ContainerResources{
					{
						Name:   "example-container-1",
						CpuIds: []int64{1, 2},
					},
					{
						Name:   "example-container-2",
						CpuIds: []int64{3, 4},
					},
				},
			},
			expectedNumberOfPowerWorkloads:          1,
			expectedPowerWorkloadName:               "performance-example-node1-workload",
			expectedNumberOfPowerWorkloadContainers: 2,
			expectedPowerWorkloadContainerCpuIds: map[string][]int{
				"example-container-1": []int{1, 2},
				"example-container-2": []int{3, 4},
			},
			expectedPowerWorkloadCpuIds:       []int{1, 2, 3, 4},
			expectedPowerWorkloadPowerProfile: "performance-example-node1",
		},
		{
			testCase: "Test Case 3",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-pod",
							Namespace: PowerPodNamespace,
							UID:       "abcdefg",
						},
						Spec: corev1.PodSpec{
							NodeName: "example-node1",
							Containers: []corev1.Container{
								{
									Name: "example-container-1",
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
									},
								},
								{
									Name: "example-container-2",
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
									},
								},
							},
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-pod2",
							Namespace: PowerPodNamespace,
							UID:       "efghijk",
						},
						Spec: corev1.PodSpec{
							NodeName: "example-node1",
							Containers: []corev1.Container{
								{
									Name: "example-container-3",
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
									},
								},
							},
						},
						Status: corev1.PodStatus{
							Phase:    corev1.PodRunning,
							QOSClass: corev1.PodQOSGuaranteed,
							ContainerStatuses: []corev1.ContainerStatus{
								{
									Name:        "example-container-3",
									ContainerID: "docker://defg",
								},
							},
						},
					},
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerPodNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
				},
			},
			podResources: []podresourcesapi.PodResources{
				{
					Name:       "example-pod",
					Containers: []*podresourcesapi.ContainerResources{},
				},
				{
					Name:       "example-pod2",
					Containers: []*podresourcesapi.ContainerResources{},
				},
			},
			containerResources: map[string][]podresourcesapi.ContainerResources{
				"example-pod": []podresourcesapi.ContainerResources{
					{
						Name:   "example-container-1",
						CpuIds: []int64{1, 2},
					},
					{
						Name:   "example-container-2",
						CpuIds: []int64{3, 4},
					},
				},
				"example-pod2": []podresourcesapi.ContainerResources{
					{
						Name:   "example-container-3",
						CpuIds: []int64{5, 6},
					},
				},
			},
			expectedNumberOfPowerWorkloads:          1,
			expectedPowerWorkloadName:               "performance-example-node1-workload",
			expectedNumberOfPowerWorkloadContainers: 3,
			expectedPowerWorkloadContainerCpuIds: map[string][]int{
				"example-container-1": []int{1, 2},
				"example-container-2": []int{3, 4},
				"example-container-3": []int{5, 6},
			},
			expectedPowerWorkloadCpuIds:       []int{1, 2, 3, 4, 5, 6},
			expectedPowerWorkloadPowerProfile: "performance-example-node1",
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)

		objs := make([]runtime.Object, 0)
		for i := range tc.pods.Items {
			objs = append(objs, &tc.pods.Items[i])
		}
		objs = append(objs, tc.node)
		for i := range tc.powerProfiles.Items {
			objs = append(objs, &tc.powerProfiles.Items[i])
		}

		r, err := createPowerPodReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		fakePodResources := []*podresourcesapi.PodResources{}
		for i := range tc.podResources {
			fakeContainers := []*podresourcesapi.ContainerResources{}
			for j := range tc.containerResources[tc.podResources[i].Name] {
				fakeContainers = append(fakeContainers, &tc.containerResources[tc.podResources[i].Name][j])
			}
			tc.podResources[i].Containers = fakeContainers
			fakePodResources = append(fakePodResources, &tc.podResources[i])
		}
		fakeListResponse := &podresourcesapi.ListPodResourcesResponse{
			PodResources: fakePodResources,
		}

		podResourcesClient := createFakePodResourcesListerClient(fakeListResponse)
		r.PodResourcesClient = *podResourcesClient

		for i := range tc.pods.Items {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      tc.pods.Items[i].Name,
					Namespace: PowerPodNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object", tc.testCase))
			}
		}

		powerWorkloads := &powerv1alpha1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), powerWorkloads)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerWorkload list object", tc.testCase))
		}

		if len(powerWorkloads.Items) != tc.expectedNumberOfPowerWorkloads {
			t.Errorf("%s - Failed: Expected number of PowerWorkloads to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerWorkloads, len(powerWorkloads.Items))
		}

		powerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.expectedPowerWorkloadName,
			Namespace: PowerPodNamespace,
		}, powerWorkload)
		if err != nil {
			if errors.IsNotFound(err) {
				t.Errorf("%s - Failed: Expected PowerWorkload %s to exist", tc.testCase, tc.expectedPowerWorkloadName)
				t.Fatal("Unable to retrieve PowerWorkload object, cannot continue")
			} else {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving PowerWorkload object", tc.testCase))
			}
		}

		if len(powerWorkload.Spec.Node.Containers) != tc.expectedNumberOfPowerWorkloadContainers {
			t.Errorf("%s - Failed: Expected number of PowerWorkload Containers to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerWorkloadContainers, len(powerWorkload.Spec.Node.Containers))
		}

		for containerName, cpuIds := range tc.expectedPowerWorkloadContainerCpuIds {
			containerFromNodeInfo := powerv1alpha1.Container{}
			for _, container := range powerWorkload.Spec.Node.Containers {
				if container.Name == containerName {
					containerFromNodeInfo = container
				}
			}

			if reflect.DeepEqual(containerFromNodeInfo, powerv1alpha1.Container{}) {
				t.Errorf("%s - Failed: Expected Container '%s' to exist", tc.testCase, containerName)
			} else {
				if !reflect.DeepEqual(containerFromNodeInfo.ExclusiveCPUs, cpuIds) {
					t.Errorf("%s - Failed: Expected PowerWorkload Container '%s' CpuIds to be %v, got %v", tc.testCase, containerName, cpuIds, containerFromNodeInfo.ExclusiveCPUs)
				}
			}
		}

		if !reflect.DeepEqual(powerWorkload.Spec.Node.CpuIds, tc.expectedPowerWorkloadCpuIds) {
			t.Errorf("%s - Failed: Expected PowerWorkload CpuIds to be %v, got %v", tc.testCase, tc.expectedPowerWorkloadCpuIds, powerWorkload.Spec.Node.CpuIds)
		}

		if powerWorkload.Spec.PowerProfile != tc.expectedPowerWorkloadPowerProfile {
			t.Errorf("%s - Failed: Expected PowerWorkload PowerProfile to be %v, got %v", tc.testCase, tc.expectedPowerWorkloadPowerProfile, powerWorkload.Spec.PowerProfile)
		}
	}
}

func TestPodDeletion(t *testing.T) {
	tcases := []struct {
		testCase                          string
		pods                              *corev1.PodList
		powerWorkloads                    *powerv1alpha1.PowerWorkloadList
		node                              *corev1.Node
		powerProfiles                     *powerv1alpha1.PowerProfileList
		podResources                      []podresourcesapi.PodResources
		containerResources                map[string][]podresourcesapi.ContainerResources
		powerWorkloadNames                []string
		expectedNumberOfPowerWorkloads    int
		expectedPowerWorkloadToNotBeFound map[string]bool
		expectedPowerWorkloadCpuIds       map[string][]int
	}{
		{
			testCase: "Test Case 1",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-pod",
							Namespace: PowerPodNamespace,
							UID:       "abcdefg",
						},
						Spec: corev1.PodSpec{
							NodeName: "example-node1",
							Containers: []corev1.Container{
								{
									Name: "example-container-1",
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceName("cpu"):                                       *resource.NewQuantity(2, resource.DecimalSI),
											corev1.ResourceName("memory"):                                    *resource.NewQuantity(200, resource.DecimalSI),
											corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
										},
									},
								},
							},
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
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerPodNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container-1",
										Id:            "abcdefg",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{1, 2},
										PowerProfile:  "performance-example-node1",
										Workload:      "performance-example-node1-workload",
									},
								},
								CpuIds: []int{1, 2},
							},
						},
					},
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerPodNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerPodNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Max:  3700,
							Min:  3400,
							Epp:  "performance",
						},
					},
				},
			},
			podResources: []podresourcesapi.PodResources{
				{
					Name:       "example-pod",
					Containers: []*podresourcesapi.ContainerResources{},
				},
			},
			containerResources: map[string][]podresourcesapi.ContainerResources{
				"example-pod": []podresourcesapi.ContainerResources{
					{
						Name:   "example-container-1",
						CpuIds: []int64{1, 2},
					},
				},
			},
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
			},
			expectedNumberOfPowerWorkloads: 0,
			expectedPowerWorkloadToNotBeFound: map[string]bool{
				"performance-example-node1-workload": true,
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)

		objs := make([]runtime.Object, 0)
		for i := range tc.pods.Items {
			objs = append(objs, &tc.pods.Items[i])
		}
		for i := range tc.powerWorkloads.Items {
			objs = append(objs, &tc.powerWorkloads.Items[i])
		}
		objs = append(objs, tc.node)
		for i := range tc.powerProfiles.Items {
			objs = append(objs, &tc.powerProfiles.Items[i])
		}

		r, err := createPowerPodReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		fakePodResources := []*podresourcesapi.PodResources{}
		for i := range tc.podResources {
			fakeContainers := []*podresourcesapi.ContainerResources{}
			for j := range tc.containerResources[tc.podResources[i].Name] {
				fakeContainers = append(fakeContainers, &tc.containerResources[tc.podResources[i].Name][j])
			}
			tc.podResources[i].Containers = fakeContainers
			fakePodResources = append(fakePodResources, &tc.podResources[i])
		}
		fakeListResponse := &podresourcesapi.ListPodResourcesResponse{
			PodResources: fakePodResources,
		}

		podResourcesClient := createFakePodResourcesListerClient(fakeListResponse)
		r.PodResourcesClient = *podResourcesClient

		for i := range tc.pods.Items {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      tc.pods.Items[i].Name,
					Namespace: PowerPodNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object", tc.testCase))
			}
		}

		for i := range tc.pods.Items {
			now := metav1.Now()
			tc.pods.Items[i].DeletionTimestamp = &now
			err = r.Client.Update(context.TODO(), &tc.pods.Items[i])
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error updating Pod '%s' DeletionTimestamp", tc.testCase, tc.pods.Items[i].Name))
			}
		}

		for i := range tc.pods.Items {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      tc.pods.Items[i].Name,
					Namespace: PowerPodNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object", tc.testCase))
			}
		}

		powerWorkloads := &powerv1alpha1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), powerWorkloads)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerWorkload list object", tc.testCase))
		}

		if len(powerWorkloads.Items) != tc.expectedNumberOfPowerWorkloads {
			t.Errorf("%s - Failed: Expected number of PowerWorkloads to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerWorkloads, len(powerWorkloads.Items))
		}

		for _, powerWorkloadName := range tc.powerWorkloadNames {
			powerWorkload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerWorkloadName,
				Namespace: PowerPodNamespace,
			}, powerWorkload)
			if errors.IsNotFound(err) != tc.expectedPowerWorkloadToNotBeFound[powerWorkloadName] {
				t.Errorf("%s - Failed: Expected PowerWorkload '%s' to exist to be %v, got %v", tc.testCase, powerWorkloadName, tc.expectedPowerWorkloadToNotBeFound[powerWorkloadName], errors.IsNotFound(err))
			}

			if !errors.IsNotFound(err) {
				if !reflect.DeepEqual(powerWorkload.Spec.Node.CpuIds, tc.expectedPowerWorkloadCpuIds[powerWorkloadName]) {
					t.Errorf("%s - Failed: Expected PowerWorkload '%s' CpuIds to be %v, got %v", tc.testCase, powerWorkloadName, tc.expectedPowerWorkloadCpuIds[powerWorkloadName], powerWorkload.Spec.Node.CpuIds)
				}
			}
		}
	}
}
