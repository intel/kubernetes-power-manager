package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PowerNodeNamespace = "default"
	AppQoSAddress      = "127.0.0.1:5000"
)

func createPowerNodeReconcilerObject(objs []runtime.Object) (*PowerNodeReconciler, error) {
	s := scheme.Scheme

	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)

	cl := fake.NewFakeClient(objs...)

	appqosCl := appqos.NewDefaultAppQoSClient()

	r := &PowerNodeReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerProfile"), Scheme: s, AppQoSClient: appqosCl}

	return r, nil
}

func createListeners(appqosPools []appqos.Pool) (*httptest.Server, error) {
	var err error

	newListener, err := net.Listen("tcp", "127.0.0.1:5000")
	if err != nil {
		return nil, fmt.Errorf("Failed to create Listerner: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/pools", (func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			b, err := json.Marshal(appqosPools)
			if err == nil {
				fmt.Fprintln(w, string(b[:]))
			}
		}
	}))

	ts := httptest.NewUnstartedServer(mux)

	ts.Listener.Close()
	ts.Listener = newListener

	// Start the server.
	ts.Start()

	return ts, nil
}

func TestPowerNodeReconciler(t *testing.T) {
	tcases := []struct {
		testCase                string
		powerNode               *powerv1alpha1.PowerNode
		pools                   map[string][]int
		powerProfileList        *powerv1alpha1.PowerProfileList
		powerWorkloadList       *powerv1alpha1.PowerWorkloadList
		expectedActiveProfiles  map[string]bool
		expectedActiveWorkloads []powerv1alpha1.WorkloadInfo
		expectedPowerContainers []powerv1alpha1.Container
		expectedSharedPools     []powerv1alpha1.SharedPoolInfo
	}{
		{
			testCase: "Test Case 1",
			powerNode: &powerv1alpha1.PowerNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-node1",
					Namespace: PowerNodeNamespace,
				},
				Spec: powerv1alpha1.PowerNodeSpec{
					NodeName: "example-node1",
				},
			},
			pools: map[string][]int{
				"Default": []int{4, 5, 6, 7, 8, 9},
			},
			powerProfileList: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Epp:  "performance",
						},
					},
				},
			},
			powerWorkloadList: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container",
										Id:            "abcdefg",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{0, 1, 2, 3},
										PowerProfile:  "performance-example-node1",
										Workload:      "performance-example-node1-workload",
									},
								},
								CpuIds: []int{0, 1, 2, 3},
							},
							PowerProfile: "performance-example-node1",
						},
					},
				},
			},
			expectedActiveProfiles: map[string]bool{
				"performance-example-node1": true,
			},
			expectedActiveWorkloads: []powerv1alpha1.WorkloadInfo{
				{
					Name:   "performance-example-node1-workload",
					CpuIds: []int{0, 1, 2, 3},
				},
			},
			expectedPowerContainers: []powerv1alpha1.Container{
				{
					Name:          "example-container",
					Id:            "abcdefg",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{0, 1, 2, 3},
					PowerProfile:  "performance-example-node1",
					Workload:      "performance-example-node1-workload",
				},
			},
			expectedSharedPools: []powerv1alpha1.SharedPoolInfo{
				{
					Name:             "Default",
					SharedPoolCpuIds: []int{4, 5, 6, 7, 8, 9},
				},
			},
		},
		{
			testCase: "Test Case 2",
			powerNode: &powerv1alpha1.PowerNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-node2",
					Namespace: PowerNodeNamespace,
				},
				Spec: powerv1alpha1.PowerNodeSpec{
					NodeName: "example-node2",
				},
			},
			pools: map[string][]int{
				"Default": []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
			powerProfileList: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node2",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node2",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
				},
			},
			powerWorkloadList: &powerv1alpha1.PowerWorkloadList{},
			expectedActiveProfiles: map[string]bool{
				"performance-example-node2": false,
			},
			expectedSharedPools: []powerv1alpha1.SharedPoolInfo{
				{
					Name:             "Default",
					SharedPoolCpuIds: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
		},
		{
			testCase: "Test Case 3",
			powerNode: &powerv1alpha1.PowerNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-node3",
					Namespace: PowerNodeNamespace,
				},
				Spec: powerv1alpha1.PowerNodeSpec{
					NodeName: "example-node3",
				},
			},
			pools: map[string][]int{
				"Default": []int{0, 1},
				"Shared":  []int{6, 7, 8, 9},
			},
			powerProfileList: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node3",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node3",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
				},
			},
			powerWorkloadList: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node3-workload",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node3",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container1",
										Id:            "abcdefg",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{2, 3},
										PowerProfile:  "performance-example-node3",
										Workload:      "performance-example-node3-workload",
									},
									{
										Name:          "example-container2",
										Id:            "hijklmop",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{4, 5},
										PowerProfile:  "performance-example-node3",
										Workload:      "performance-example-node3-workload",
									},
								},
								CpuIds: []int{2, 3, 4, 5},
							},
							PowerProfile: "performance-example-node3",
						},
					},
				},
			},
			expectedActiveProfiles: map[string]bool{
				"performance-example-node3": true,
			},
			expectedActiveWorkloads: []powerv1alpha1.WorkloadInfo{
				{
					Name:   "performance-example-node3-workload",
					CpuIds: []int{2, 3, 4, 5},
				},
			},
			expectedPowerContainers: []powerv1alpha1.Container{
				{
					Name:          "example-container1",
					Id:            "abcdefg",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{2, 3},
					PowerProfile:  "performance-example-node3",
					Workload:      "performance-example-node3-workload",
				},
				{
					Name:          "example-container2",
					Id:            "hijklmop",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{4, 5},
					PowerProfile:  "performance-example-node3",
					Workload:      "performance-example-node3-workload",
				},
			},
			expectedSharedPools: []powerv1alpha1.SharedPoolInfo{
				{
					Name:             "Default",
					SharedPoolCpuIds: []int{0, 1},
				},
				{
					Name:             "Shared",
					SharedPoolCpuIds: []int{6, 7, 8, 9},
				},
			},
		},
		{
			testCase: "Test Case 4",
			powerNode: &powerv1alpha1.PowerNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-node4",
					Namespace: PowerNodeNamespace,
				},
				Spec: powerv1alpha1.PowerNodeSpec{
					NodeName: "example-node4",
				},
			},
			pools: map[string][]int{
				"Default": []int{0, 1},
				"Shared":  []int{6, 7, 8, 9},
			},
			powerProfileList: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node4",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node4",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node4",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance-example-node4",
							Max:  2400,
							Min:  2000,
							Epp:  "balance_performance",
						},
					},
				},
			},
			powerWorkloadList: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node4-workload",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node4",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container1",
										Id:            "abcdefg",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{2, 3},
										PowerProfile:  "performance-example-node4",
										Workload:      "performance-example-node4-workload",
									},
									{
										Name:          "example-container2",
										Id:            "hijklmop",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{4, 5},
										PowerProfile:  "performance-example-node4",
										Workload:      "performance-example-node4-workload",
									},
								},
								CpuIds: []int{2, 3, 4, 5},
							},
							PowerProfile: "performance-example-node4",
						},
					},
				},
			},
			expectedActiveProfiles: map[string]bool{
				"performance-example-node4":         true,
				"balance-performance-example-node4": false,
			},
			expectedActiveWorkloads: []powerv1alpha1.WorkloadInfo{
				{
					Name:   "performance-example-node4-workload",
					CpuIds: []int{2, 3, 4, 5},
				},
			},
			expectedPowerContainers: []powerv1alpha1.Container{
				{
					Name:          "example-container1",
					Id:            "abcdefg",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{2, 3},
					PowerProfile:  "performance-example-node4",
					Workload:      "performance-example-node4-workload",
				},
				{
					Name:          "example-container2",
					Id:            "hijklmop",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{4, 5},
					PowerProfile:  "performance-example-node4",
					Workload:      "performance-example-node4-workload",
				},
			},
			expectedSharedPools: []powerv1alpha1.SharedPoolInfo{
				{
					Name:             "Default",
					SharedPoolCpuIds: []int{0, 1},
				},
				{
					Name:             "Shared",
					SharedPoolCpuIds: []int{6, 7, 8, 9},
				},
			},
		},
		{
			testCase: "Test Case 5",
			powerNode: &powerv1alpha1.PowerNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-node5",
					Namespace: PowerNodeNamespace,
				},
				Spec: powerv1alpha1.PowerNodeSpec{
					NodeName: "example-node5",
				},
			},
			pools: map[string][]int{
				"Default": []int{0, 1},
				"Shared":  []int{8, 9},
			},
			powerProfileList: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance_performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node5",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node5",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node5",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance-example-node5",
							Max:  2400,
							Min:  2000,
							Epp:  "balance_performance",
						},
					},
				},
			},
			powerWorkloadList: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node5-workload",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node5",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container1",
										Id:            "abcdefg",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{2, 3},
										PowerProfile:  "performance-example-node5",
										Workload:      "performance-example-node5-workload",
									},
									{
										Name:          "example-container2",
										Id:            "hijklmop",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{4, 5},
										PowerProfile:  "performance-example-node5",
										Workload:      "performance-example-node5-workload",
									},
								},
								CpuIds: []int{2, 3, 4, 5},
							},
							PowerProfile: "performance-example-node5",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node5-workload",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node5",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container3",
										Id:            "xyz",
										Pod:           "example-pod2",
										ExclusiveCPUs: []int{6, 7},
										PowerProfile:  "balance-performance-example-node5",
										Workload:      "balance-performance-example-node5-workload",
									},
								},
								CpuIds: []int{6, 7},
							},
							PowerProfile: "balance-performance-example-node5",
						},
					},
				},
			},
			expectedActiveProfiles: map[string]bool{
				"balance-performance-example-node5": true,
				"performance-example-node5":         true,
			},
			expectedActiveWorkloads: []powerv1alpha1.WorkloadInfo{
				{
					Name:   "balance-performance-example-node5-workload",
					CpuIds: []int{6, 7},
				},
				{
					Name:   "performance-example-node5-workload",
					CpuIds: []int{2, 3, 4, 5},
				},
			},
			expectedPowerContainers: []powerv1alpha1.Container{
				{
					Name:          "example-container3",
					Id:            "xyz",
					Pod:           "example-pod2",
					ExclusiveCPUs: []int{6, 7},
					PowerProfile:  "balance-performance-example-node5",
					Workload:      "balance-performance-example-node5-workload",
				},
				{
					Name:          "example-container1",
					Id:            "abcdefg",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{2, 3},
					PowerProfile:  "performance-example-node5",
					Workload:      "performance-example-node5-workload",
				},
				{
					Name:          "example-container2",
					Id:            "hijklmop",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{4, 5},
					PowerProfile:  "performance-example-node5",
					Workload:      "performance-example-node5-workload",
				},
			},
			expectedSharedPools: []powerv1alpha1.SharedPoolInfo{
				{
					Name:             "Default",
					SharedPoolCpuIds: []int{0, 1},
				},
				{
					Name:             "Shared",
					SharedPoolCpuIds: []int{8, 9},
				},
			},
		},
		{
			testCase: "Test Case 6",
			powerNode: &powerv1alpha1.PowerNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-node6",
					Namespace: PowerNodeNamespace,
				},
				Spec: powerv1alpha1.PowerNodeSpec{
					NodeName: "example-node6",
				},
			},
			pools: map[string][]int{
				"Default": []int{0, 1},
				"Shared":  []int{6, 7, 8, 9},
			},
			powerProfileList: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-incorrect-node",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-inccorect-node",
							Max:  3700,
							Min:  3400,
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node6",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node6",
							Max:  3200,
							Min:  2800,
							Epp:  "performance",
						},
					},
				},
			},
			powerWorkloadList: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node6-workload",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node6",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container1",
										Id:            "abcdefg",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{2, 3},
										PowerProfile:  "performance-example-node6",
										Workload:      "performance-example-node6-workload",
									},
									{
										Name:          "example-container2",
										Id:            "hijklmop",
										Pod:           "example-pod",
										ExclusiveCPUs: []int{4, 5},
										PowerProfile:  "performance-example-node6",
										Workload:      "performance-example-node6-workload",
									},
								},
								CpuIds: []int{2, 3, 4, 5},
							},
							PowerProfile: "performance-example-node5",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-incorrect-node-workload",
							Namespace: PowerNodeNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-incorrect-node-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "incorrect-node",
								Containers: []powerv1alpha1.Container{
									{
										Name:          "example-container3",
										Id:            "xyz",
										Pod:           "example-pod2",
										ExclusiveCPUs: []int{2, 3},
										PowerProfile:  "performance-incorrect-node",
										Workload:      "performance-incorrect-node-workload",
									},
								},
								CpuIds: []int{2, 3},
							},
							PowerProfile: "performance-incorrect-node",
						},
					},
				},
			},
			expectedActiveProfiles: map[string]bool{
				"performance-example-node6":  true,
				"performance-incorrect-node": false,
			},
			expectedActiveWorkloads: []powerv1alpha1.WorkloadInfo{
				{
					Name:   "performance-example-node6-workload",
					CpuIds: []int{2, 3, 4, 5},
				},
			},
			expectedPowerContainers: []powerv1alpha1.Container{
				{
					Name:          "example-container1",
					Id:            "abcdefg",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{2, 3},
					PowerProfile:  "performance-example-node6",
					Workload:      "performance-example-node6-workload",
				},
				{
					Name:          "example-container2",
					Id:            "hijklmop",
					Pod:           "example-pod",
					ExclusiveCPUs: []int{4, 5},
					PowerProfile:  "performance-example-node6",
					Workload:      "performance-example-node6-workload",
				},
			},
			expectedSharedPools: []powerv1alpha1.SharedPoolInfo{
				{
					Name:             "Default",
					SharedPoolCpuIds: []int{0, 1},
				},
				{
					Name:             "Shared",
					SharedPoolCpuIds: []int{6, 7, 8, 9},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.powerNode.Name)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for name, cores := range tc.pools {
			id := 1
			// Necessary because of pointer...
			newName := name
			newCores := cores
			pool := appqos.Pool{
				Name:  &newName,
				ID:    &id,
				Cores: &newCores,
			}
			appqosPools = append(appqosPools, pool)
		}

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerNode)
		for i := range tc.powerProfileList.Items {
			objs = append(objs, &tc.powerProfileList.Items[i])
		}
		for i := range tc.powerWorkloadList.Items {
			objs = append(objs, &tc.powerWorkloadList.Items[i])
		}

		r, err := createPowerNodeReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating reconcile object")
		}

		server, err := createListeners(appqosPools)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.powerNode.Name,
				Namespace: PowerNodeNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object", tc.testCase))
		}

		server.Close()

		powerNode := &powerv1alpha1.PowerNode{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.powerNode.Name,
			Namespace: PowerNodeNamespace,
		}, powerNode)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerNode object", tc.testCase))
		}

		if !reflect.DeepEqual(powerNode.Spec.ActiveProfiles, tc.expectedActiveProfiles) {
			t.Errorf("%s - Failed: Expected Active Profiles to be %v, got %v", tc.testCase, tc.expectedActiveProfiles, powerNode.Spec.ActiveProfiles)
		}

		if !reflect.DeepEqual(powerNode.Spec.ActiveWorkloads, tc.expectedActiveWorkloads) {
			t.Errorf("%s - Failed: Expected Active Workloads to be %v, got %v", tc.testCase, tc.expectedActiveWorkloads, powerNode.Spec.ActiveWorkloads)
		}

		if !reflect.DeepEqual(powerNode.Spec.PowerContainers, tc.expectedPowerContainers) {
			t.Errorf("%s - Failed: Expected Power Containers to be %v, got %v", tc.testCase, tc.expectedPowerContainers, powerNode.Spec.PowerContainers)
		}

		if !reflect.DeepEqual(powerNode.Spec.SharedPools, tc.expectedSharedPools) {
			t.Errorf("%s - Failed: Expected Shared Pools to be %v, got %v", tc.testCase, tc.expectedSharedPools, powerNode.Spec.SharedPools)
		}
	}
}
