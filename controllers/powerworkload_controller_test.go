package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PowerWorkloadName      = "TestPowerWorkload"
	PowerWorkloadNamespace = "default"
)

type AppQoSPool struct {
	Node         string
	Name         string
	Id           int
	Cores        []int
	PowerProfile int
}

type AppQoSPowerProfile struct {
	Node string
	Name string
	Id   int
}

func createPowerWorkloadReconcilerObject(objs []runtime.Object) (*PowerWorkloadReconciler, error) {
	s := scheme.Scheme

	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)

	cl := fake.NewFakeClient(objs...)

	appqosCl := appqos.NewDefaultAppQoSClient()

	r := &PowerWorkloadReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerWorkload"), Scheme: s, AppQoSClient: appqosCl}

	return r, nil
}

func createPowerWorkloadListeners(appqosPools []appqos.Pool, appqosPowerProfiles []appqos.PowerProfile) (*httptest.Server, error) {
	var err error

	newListener, err := net.Listen("tcp", "127.0.0.1:5000")
	if err != nil {
		return nil, fmt.Errorf("Failed to create Listener: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", (func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			p := appqos.Pool{}
			_ = json.NewDecoder(r.Body).Decode(&p)
			for i := range appqosPools {
				if *appqosPools[i].Name == *p.Name {
					appqosPools[i].Cores = p.Cores
				}
			}
		} else if r.Method == "DELETE" {
			path := strings.Split(r.URL.Path, "/")
			id, _ := strconv.Atoi(path[len(path)-1])
			for i, pool := range appqosPools {
				if *pool.ID == id {
					appqosPools = append(appqosPools[:i], appqosPools[i+1:]...)
				}
			}
		}
	}))
	mux.HandleFunc("/power_profiles", (func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			b, err := json.Marshal(appqosPowerProfiles)
			if err == nil {
				fmt.Fprintln(w, string(b[:]))
			}
		} else if r.Method == "POST" {
			b, err := json.Marshal("okay")
			if err == nil {
				w.WriteHeader(http.StatusCreated)
				fmt.Fprintln(w, string(b[:]))
			}
		} else if r.Method == "PUT" {
			b, err := json.Marshal(appqosPowerProfiles)
			if err == nil {
				fmt.Fprintln(w, string(b[:]))
			}
		}
	}))
	mux.HandleFunc("/pools", (func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			b, err := json.Marshal(appqosPools)
			if err == nil {
				fmt.Fprintln(w, string(b[:]))
			}
		} else if r.Method == "POST" {
			p := appqos.Pool{}
			_ = json.NewDecoder(r.Body).Decode(&p)
			newId := len(appqosPools) + 1
			p.ID = &newId
			appqosPools = append(appqosPools, p)

			b, err := json.Marshal("okay")
			if err == nil {
				w.WriteHeader(http.StatusCreated)
				fmt.Fprintln(w, string(b[:]))
			}
		}
	}))

	ts := httptest.NewUnstartedServer(mux)

	ts.Listener.Close()
	ts.Listener = newListener

	// Start the server
	ts.Start()

	return ts, nil
}

func TestNonSharedWorkloadCreation(t *testing.T) {
	tcases := []struct {
		testCase                      string
		powerWorkloadNames            []string
		nodeName                      string
		powerWorkloads                *powerv1alpha1.PowerWorkloadList
		sharedWorkloadName            string
		appqosPools                   []AppQoSPool
		appqosPowerProfiles           []AppQoSPowerProfile
		nodeList                      *corev1.NodeList
		expectedSharedWorkloadCPUList []int
		expectedDefaultPoolCPUList    []int
		expectedSharedPoolCPUList     []int
	}{
		{
			testCase:           "Test Case 1",
			powerWorkloadNames: []string{"performance-example-node1-workload"},
			nodeName:           "example-node1",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:     "shared-example-node1-workload",
							AllCores: true,
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							ReservedCPUs: []int{0},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
						},
					},
				},
			},
			sharedWorkloadName: "shared-example-node1-workload",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance",
					Id:   2,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedSharedWorkloadCPUList: []int{1, 4, 5, 6, 7, 8, 9},
			expectedDefaultPoolCPUList:    []int{0},
			expectedSharedPoolCPUList:     []int{1, 4, 5, 6, 7, 8, 9},
		},
		{
			testCase:           "Test Case 2",
			powerWorkloadNames: []string{"performance-example-node1-workload"},
			nodeName:           "example-node1",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
									{
										Name: "example-container2",
										Id:   "xyz",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											4,
											5,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
									4,
									5,
								},
							},
							PowerProfile: "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:     "shared-example-node1-workload",
							AllCores: true,
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							ReservedCPUs: []int{0},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
						},
					},
				},
			},
			sharedWorkloadName: "shared-example-node1-workload",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance",
					Id:   2,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedSharedWorkloadCPUList: []int{1, 6, 7, 8, 9},
			expectedDefaultPoolCPUList:    []int{0},
			expectedSharedPoolCPUList:     []int{1, 6, 7, 8, 9},
		},
		{
			testCase: "Test Case 3",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
				"balance-performance-example-node1-workload",
			},
			nodeName: "example-node1",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
									{
										Name: "example-container2",
										Id:   "xyz",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											4,
											5,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
									4,
									5,
								},
							},
							PowerProfile: "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "balance-performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container3",
										Id:   "abcdefg",
										Pod:  "example-pod2",
										ExclusiveCPUs: []int{
											6,
											7,
										},
										PowerProfile: "balance-performance",
										Workload:     "balance-performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									6,
									7,
								},
							},
							PowerProfile: "balance-performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:     "shared-example-node1-workload",
							AllCores: true,
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							ReservedCPUs: []int{0},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
						},
					},
				},
			},
			sharedWorkloadName: "shared-example-node1-workload",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance",
					Id:   2,
				},
				{
					Node: "example-node1",
					Name: "balance-performance",
					Id:   3,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedSharedWorkloadCPUList: []int{1, 8, 9},
			expectedDefaultPoolCPUList:    []int{0},
			expectedSharedPoolCPUList:     []int{1, 8, 9},
		},
		{
			testCase: "Test Case 4",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
				"balance-performance-example-node1-workload",
			},
			nodeName: "example-node1",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
									{
										Name: "example-container2",
										Id:   "xyz",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											4,
											5,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
									4,
									5,
								},
							},
							PowerProfile: "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "balance-performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container3",
										Id:   "abcdefg",
										Pod:  "example-pod2",
										ExclusiveCPUs: []int{
											6,
											7,
										},
										PowerProfile: "balance-performance",
										Workload:     "balance-performance-example-node1-workload",
									},
									{
										Name: "example-container4",
										Id:   "xyza",
										Pod:  "example-pod2",
										ExclusiveCPUs: []int{
											8,
											9,
										},
										PowerProfile: "balance-performance",
										Workload:     "balance-performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									6,
									7,
									8,
									9,
								},
							},
							PowerProfile: "balance-performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:     "shared-example-node1-workload",
							AllCores: true,
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							ReservedCPUs: []int{0},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
						},
					},
				},
			},
			sharedWorkloadName: "shared-example-node1-workload",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance",
					Id:   2,
				},
				{
					Node: "example-node1",
					Name: "balance-performance",
					Id:   3,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedSharedWorkloadCPUList: []int{1},
			expectedDefaultPoolCPUList:    []int{0},
			expectedSharedPoolCPUList:     []int{1},
		},
		{
			testCase: "Test Case 5",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
			},
			nodeName: "example-node1",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											3,
											4,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
									{
										Name: "example-container2",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											5,
											6,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									3,
									4,
									5,
									6,
								},
							},
							PowerProfile: "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:     "shared-example-node1-workload",
							AllCores: true,
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							ReservedCPUs: []int{0},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{1, 2, 5, 6, 7, 8, 9},
						},
					},
				},
			},
			sharedWorkloadName: "shared-example-node1-workload",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{1, 2, 5, 6, 7, 8, 9},
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    3,
					Cores: []int{3, 4},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance",
					Id:   2,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedSharedWorkloadCPUList: []int{1, 2, 7, 8, 9},
			expectedDefaultPoolCPUList:    []int{0},
			expectedSharedPoolCPUList:     []int{1, 2, 7, 8, 9},
		},
		{
			testCase: "Test Case 6",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
				"balance-performance-example-node1-workload",
			},
			nodeName: "example-node1",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											3,
											4,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
									{
										Name: "example-container2",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											5,
											6,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									3,
									4,
									5,
									6,
								},
							},
							PowerProfile: "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "balance-performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container3",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											7,
											8,
										},
										PowerProfile: "balance-performance",
										Workload:     "balance-performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									7,
									8,
								},
							},
							PowerProfile: "balance-performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:     "shared-example-node1-workload",
							AllCores: true,
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							ReservedCPUs: []int{0},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{1, 2, 5, 6, 7, 8, 9},
						},
					},
				},
			},
			sharedWorkloadName: "shared-example-node1-workload",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0},
				},
				{
					Node:         "example-node1",
					Name:         "Shared",
					Id:           2,
					Cores:        []int{1, 2, 5, 6, 7, 8, 9},
					PowerProfile: 1,
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    3,
					Cores: []int{3, 4},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance",
					Id:   2,
				},
				{
					Node: "example-node1",
					Name: "balance-performance",
					Id:   3,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedSharedWorkloadCPUList: []int{1, 2, 9},
			expectedDefaultPoolCPUList:    []int{0},
			expectedSharedPoolCPUList:     []int{1, 2, 9},
		},
		{
			testCase:           "Test Case 7",
			powerWorkloadNames: []string{"performance-example-node1-workload"},
			nodeName:           "example-node1",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance",
						},
					},
				},
			},
			sharedWorkloadName: "shared-example-node1-workload",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "performance",
					Id:   2,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedDefaultPoolCPUList: []int{0, 1, 4, 5, 6, 7, 8, 9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			var pool appqos.Pool
			if tc.appqosPools[i].PowerProfile != 0 {
				pool = appqos.Pool{
					Name:         &tc.appqosPools[i].Name,
					ID:           &tc.appqosPools[i].Id,
					Cores:        &tc.appqosPools[i].Cores,
					PowerProfile: &tc.appqosPools[i].PowerProfile,
				}
			} else {
				pool = appqos.Pool{
					Name:  &tc.appqosPools[i].Name,
					ID:    &tc.appqosPools[i].Id,
					Cores: &tc.appqosPools[i].Cores,
				}
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		for i := range tc.powerWorkloads.Items {
			objs = append(objs, &tc.powerWorkloads.Items[i])
		}
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		for _, powerWorkloadName := range tc.powerWorkloadNames {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      powerWorkloadName,
					Namespace: PowerWorkloadNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object '%s'", tc.testCase, powerWorkloadName))
			}
		}

		defaultPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", "Default")
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Default Pool from AppQoS", tc.testCase))
		}

		if !reflect.DeepEqual(*defaultPool.Cores, tc.expectedDefaultPoolCPUList) {
			t.Errorf("%s - Failed: Expected Default Pool CPU list to be %v, got %v", tc.testCase, tc.expectedDefaultPoolCPUList, *defaultPool.Cores)
		}

		if len(tc.expectedSharedPoolCPUList) > 0 {
			sharedPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", "Shared")
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving Shared Pool from AppQoS", tc.testCase))
			}

			if !reflect.DeepEqual(*sharedPool.Cores, tc.expectedSharedPoolCPUList) {
				t.Errorf("%s - Failed: Expected Shared Pool CPU list to be %v, got %v", tc.testCase, tc.expectedSharedPoolCPUList, *sharedPool.Cores)
			}
		}

		server.Close()

		if len(tc.expectedSharedWorkloadCPUList) > 0 {
			sharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      tc.sharedWorkloadName,
				Namespace: PowerWorkloadNamespace,
			}, sharedPowerWorkload)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving Shared PowerWorkload object", tc.testCase))
			}

			if !reflect.DeepEqual(sharedPowerWorkload.Status.SharedCores, tc.expectedSharedWorkloadCPUList) {
				t.Errorf("%s - Failed: Expected Shared PowerWorkload core list to be %v, got %v", tc.testCase, tc.expectedSharedWorkloadCPUList, sharedPowerWorkload.Status.SharedCores)
			}
		}
	}
}

func TestSharedWorkloadCreation(t *testing.T) {
	tcases := []struct {
		testCase                           string
		nodeName                           string
		sharedPowerWorkloadName            string
		powerWorkloadNames                 []string
		sharedPowerWorkload                *powerv1alpha1.PowerWorkload
		powerWorkloads                     *powerv1alpha1.PowerWorkloadList
		nodeList                           *corev1.NodeList
		appqosPools                        []AppQoSPool
		appqosPowerProfiles                []AppQoSPowerProfile
		expectedSharedPowerWorkloadCPUList []int
	}{
		{
			testCase:                "Test Case 1",
			nodeName:                "example-node1",
			sharedPowerWorkloadName: "shared-example-node1-workload",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			testCase:                "Test Case 2",
			nodeName:                "example-node1",
			sharedPowerWorkloadName: "shared-example-node1-workload",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1, 2, 3, 7, 8, 9},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{4, 5, 6},
		},
		{
			testCase:                "Test Case 3",
			nodeName:                "example-node1",
			sharedPowerWorkloadName: "shared-example-node1-workload",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
			},
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance-example-node1",
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   2,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{4, 5, 6, 7, 8, 9},
		},
		{
			testCase:                "Test Case 4",
			sharedPowerWorkloadName: "shared-example-node1-workload",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:  &tc.appqosPools[i].Name,
				ID:    &tc.appqosPools[i].Id,
				Cores: &tc.appqosPools[i].Cores,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		for i := range tc.powerWorkloads.Items {
			objs = append(objs, &tc.powerWorkloads.Items[i])
		}
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		for _, powerWorkloadName := range tc.powerWorkloadNames {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      powerWorkloadName,
					Namespace: PowerWorkloadNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object '%s'", tc.testCase, powerWorkloadName))
			}
		}

		err = r.Client.Create(context.TODO(), tc.sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Shared PowerWorkload", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.sharedPowerWorkloadName,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object '%s'", tc.testCase, tc.sharedPowerWorkloadName))
		}

		server.Close()

		sharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.sharedPowerWorkloadName,
			Namespace: PowerWorkloadNamespace,
		}, sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Shared PowerWorkload object", tc.testCase))
		}

		if !reflect.DeepEqual(sharedPowerWorkload.Status.SharedCores, tc.expectedSharedPowerWorkloadCPUList) {
			t.Errorf("%s - Failed: Expected Shared PowerWorkload CPU List to be %v, got %v", tc.testCase, tc.expectedSharedPowerWorkloadCPUList, sharedPowerWorkload.Status.SharedCores)
		}

		if sharedPowerWorkload.Status.Node != tc.nodeName {
			t.Errorf("%s - Failed: Expected Shared PowerWorkload Status Node to be %v, got %v", tc.testCase, tc.nodeName, sharedPowerWorkload.Status.Node)
		}
	}
}

func TestNonSharedWorkloadDeletion(t *testing.T) {
	tcases := []struct {
		testCase                           string
		nodeName                           string
		sharedPowerWorkloadName            string
		powerWorkloadNames                 []string
		powerWorkloads                     *powerv1alpha1.PowerWorkloadList
		nodeList                           *corev1.NodeList
		appqosPools                        []AppQoSPool
		appqosPowerProfiles                []AppQoSPowerProfile
		expectedSharedPowerWorkloadCPUList []int
	}{
		{
			testCase:                "Test Case 1",
			sharedPowerWorkloadName: "shared-example-node1-workload",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance-example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:         "shared-example-node1-workload",
							AllCores:     true,
							ReservedCPUs: []int{0, 1},
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{4, 5, 6, 7, 8, 9},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:         "example-node1",
					Name:         "Shared",
					Id:           2,
					Cores:        []int{4, 5, 6, 7, 8, 9},
					PowerProfile: 1,
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    3,
					Cores: []int{2, 3},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   2,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			testCase:                "Test Case 2",
			sharedPowerWorkloadName: "shared-example-node1-workload",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance-example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "balance-performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container2",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											4,
											5,
										},
										PowerProfile: "balance-performance",
										Workload:     "balance-performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									4,
									5,
								},
							},
							PowerProfile: "balance-performance-example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:         "shared-example-node1-workload",
							AllCores:     true,
							ReservedCPUs: []int{0, 1},
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{6, 7, 8, 9},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:         "example-node1",
					Name:         "Shared",
					Id:           2,
					Cores:        []int{6, 7, 8, 9},
					PowerProfile: 1,
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    3,
					Cores: []int{2, 3},
				},
				{
					Node:  "example-node1",
					Name:  "balance-performance-example-node1-workload",
					Id:    4,
					Cores: []int{4, 5},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   2,
				},
				{
					Node: "example-node1",
					Name: "balance-performance-example-node1",
					Id:   3,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{2, 3, 6, 7, 8, 9},
		},
		{
			testCase:                "Test Case 3",
			sharedPowerWorkloadName: "shared-example-node1-workload",
			powerWorkloadNames: []string{
				"performance-example-node1-workload",
				"balance-performance-example-node1-workload",
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance-example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "balance-performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container2",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											4,
											5,
										},
										PowerProfile: "balance-performance",
										Workload:     "balance-performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									4,
									5,
								},
							},
							PowerProfile: "balance-performance-example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:         "shared-example-node1-workload",
							AllCores:     true,
							ReservedCPUs: []int{0, 1},
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{6, 7, 8, 9},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:         "example-node1",
					Name:         "Shared",
					Id:           2,
					Cores:        []int{6, 7, 8, 9},
					PowerProfile: 1,
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    3,
					Cores: []int{2, 3},
				},
				{
					Node:  "example-node1",
					Name:  "balance-performance-example-node1-workload",
					Id:    4,
					Cores: []int{4, 5},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   2,
				},
				{
					Node: "example-node1",
					Name: "balance-performance-example-node1",
					Id:   3,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{2, 3, 4, 5, 6, 7, 8, 9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			var pool appqos.Pool
			if tc.appqosPools[i].PowerProfile != 0 {
				pool = appqos.Pool{
					Name:         &tc.appqosPools[i].Name,
					ID:           &tc.appqosPools[i].Id,
					Cores:        &tc.appqosPools[i].Cores,
					PowerProfile: &tc.appqosPools[i].PowerProfile,
				}
			} else {
				pool = appqos.Pool{
					Name:  &tc.appqosPools[i].Name,
					ID:    &tc.appqosPools[i].Id,
					Cores: &tc.appqosPools[i].Cores,
				}
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		for i := range tc.powerWorkloads.Items {
			objs = append(objs, &tc.powerWorkloads.Items[i])
		}
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		for _, workloadName := range tc.powerWorkloadNames {
			workload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      workloadName,
				Namespace: PowerWorkloadNamespace,
			}, workload)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving PowerWorkload object", tc.testCase))
			}

			err = r.Client.Delete(context.TODO(), workload)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error deleting PowerWorkload object", tc.testCase))
			}

			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      workloadName,
					Namespace: PowerWorkloadNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object '%s'", tc.testCase, tc.sharedPowerWorkloadName))
			}
		}

		server.Close()

		sharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.sharedPowerWorkloadName,
			Namespace: PowerWorkloadNamespace,
		}, sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Shared PowerWorkload object", tc.testCase))
		}

		if !reflect.DeepEqual(sharedPowerWorkload.Status.SharedCores, tc.expectedSharedPowerWorkloadCPUList) {
			t.Errorf("%s - Failed: Expected Shared PowerWorkload CPU list to be %v, got %v", tc.testCase, tc.expectedSharedPowerWorkloadCPUList, sharedPowerWorkload.Status.SharedCores)
		}
	}
}

func TestSharedWorkloadDeletion(t *testing.T) {
	tcases := []struct {
		testCase                           string
		nodeName                           string
		originalSharedPowerWorkloadName    string
		secondSharedPowerWorkloadName      string
		originalSharedPowerWorkload        *powerv1alpha1.PowerWorkload
		secondSharedPowerWorkload          *powerv1alpha1.PowerWorkload
		nodeList                           *corev1.NodeList
		appqosPools                        []AppQoSPool
		appqosPowerProfiles                []AppQoSPowerProfile
		expectedSharedPowerWorkloadCPUList []int
	}{
		{
			testCase:                        "Test Case 1",
			nodeName:                        "example-node1",
			originalSharedPowerWorkloadName: "shared-example-node1-workload",
			secondSharedPowerWorkloadName:   "shared-second-example-node1-workload",
			originalSharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
				Status: powerv1alpha1.PowerWorkloadStatus{
					SharedCores: []int{2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			secondSharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-second-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-second-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1, 2},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{3, 4, 5, 6, 7, 8, 9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"
		sharedPowerWorkloadName = tc.originalSharedPowerWorkloadName

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:  &tc.appqosPools[i].Name,
				ID:    &tc.appqosPools[i].Id,
				Cores: &tc.appqosPools[i].Cores,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.originalSharedPowerWorkload)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		originalSharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.originalSharedPowerWorkloadName,
			Namespace: PowerWorkloadNamespace,
		}, originalSharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving original PowerWorkload object", tc.testCase))
		}

		err = r.Delete(context.TODO(), originalSharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error deleting original PowerWorkload object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.originalSharedPowerWorkloadName,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling second PowerWorkload object", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.secondSharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating second Shared PowerWorkload object", tc.testCase))
		}

		req = reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.secondSharedPowerWorkloadName,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling second PowerWorkload object", tc.testCase))
		}

		secondSharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.secondSharedPowerWorkloadName,
			Namespace: PowerWorkloadNamespace,
		}, secondSharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving second PowerWorkload object", tc.testCase))
		}

		if !reflect.DeepEqual(secondSharedPowerWorkload.Status.SharedCores, tc.expectedSharedPowerWorkloadCPUList) {
			t.Errorf("%s - Failed: Expected second Shared PowerWorkload CPU list to be %v, got %v", tc.testCase, tc.expectedSharedPowerWorkloadCPUList, secondSharedPowerWorkload.Status.SharedCores)
		}

		sharedPowerWorkloadName = ""
		server.Close()
	}
}

func TestSharedWorkloadUpdated(t *testing.T) {
	tcases := []struct {
		testCase                           string
		nodeName                           string
		sharedPowerWorkload                *powerv1alpha1.PowerWorkload
		nodeList                           *corev1.NodeList
		appqosPools                        []AppQoSPool
		appqosPowerProfiles                []AppQoSPowerProfile
		newReservedCPUList                 []int
		expectedSharedPowerWorkloadCPUList []int
		expectedDefaultPoolCPUList         []int
	}{
		{
			testCase: "Test Case 1",
			nodeName: "example-node1",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
				Status: powerv1alpha1.PowerWorkloadStatus{
					SharedCores: []int{2, 3, 4, 5, 6, 7, 8, 9},
					Node:        "example-node1",
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:         "example-node1",
					Name:         "Shared",
					Id:           2,
					Cores:        []int{2, 3, 4, 5, 6, 7, 8, 9},
					PowerProfile: 1,
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
			newReservedCPUList:                 []int{0, 1, 2, 3},
			expectedSharedPowerWorkloadCPUList: []int{4, 5, 6, 7, 8, 9},
			expectedDefaultPoolCPUList:         []int{0, 1, 2, 3},
		},
		{
			testCase: "Test Case 2",
			nodeName: "example-node1",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
				Status: powerv1alpha1.PowerWorkloadStatus{
					SharedCores: []int{2, 3, 4, 5, 6, 7, 8, 9},
					Node:        "example-node1",
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:         "example-node1",
					Name:         "Shared",
					Id:           2,
					Cores:        []int{2, 3, 4, 5, 6, 7, 8, 9},
					PowerProfile: 1,
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
			newReservedCPUList:                 []int{0, 3},
			expectedSharedPowerWorkloadCPUList: []int{1, 2, 4, 5, 6, 7, 8, 9},
			expectedDefaultPoolCPUList:         []int{0, 3},
		},
		{
			testCase: "Test Case 3",
			nodeName: "example-node1",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
				Status: powerv1alpha1.PowerWorkloadStatus{
					SharedCores: []int{2, 3, 4, 5, 6, 7, 8, 9},
					Node:        "example-node1",
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:         "example-node1",
					Name:         "Shared",
					Id:           2,
					Cores:        []int{2, 3, 4, 5, 6, 7, 8, 9},
					PowerProfile: 1,
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
			newReservedCPUList:                 []int{4, 5},
			expectedSharedPowerWorkloadCPUList: []int{0, 1, 2, 3, 6, 7, 8, 9},
			expectedDefaultPoolCPUList:         []int{4, 5},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:         &tc.appqosPools[i].Name,
				ID:           &tc.appqosPools[i].Id,
				Cores:        &tc.appqosPools[i].Cores,
				PowerProfile: &tc.appqosPools[i].PowerProfile,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.sharedPowerWorkload)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.sharedPowerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		sharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.sharedPowerWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Shared PowerWorkload", tc.testCase))
		}

		sharedPowerWorkload.Spec.ReservedCPUs = tc.newReservedCPUList
		err = r.Client.Update(context.TODO(), sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error updating Shared PowerWorkload", tc.testCase))
		}

		req = reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.sharedPowerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.sharedPowerWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Shared PowerWorkload", tc.testCase))
		}

		if !reflect.DeepEqual(sharedPowerWorkload.Status.SharedCores, tc.expectedSharedPowerWorkloadCPUList) {
			t.Errorf("%s - Failed: Expected Shared PowerWorkload CPU list to be %v, got %v", tc.testCase, tc.expectedSharedPowerWorkloadCPUList, sharedPowerWorkload.Status.SharedCores)
		}

		defaultPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", "Default")
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Default Pool from AppQoS", tc.testCase))
		}

		if !reflect.DeepEqual(*defaultPool.Cores, tc.expectedDefaultPoolCPUList) {
			t.Errorf("%s - Failed: Expected Default Pool CPU list to be %v, got %v", tc.testCase, tc.expectedDefaultPoolCPUList, *defaultPool.Cores)
		}

		sharedPowerWorkloadName = ""
		server.Close()
	}
}

func TestSharedWorkloadIncorrectNameBeginning(t *testing.T) {
	tcases := []struct {
		testCase                   string
		nodeName                   string
		powerWorkloadNames         []string
		sharedPowerWorkloadName    string
		sharedPowerWorkload        *powerv1alpha1.PowerWorkload
		powerWorkloads             *powerv1alpha1.PowerWorkloadList
		nodeList                   *corev1.NodeList
		appqosPools                []AppQoSPool
		appqosPowerProfiles        []AppQoSPowerProfile
		expectedDefaultPoolCPUList []int
	}{
		{
			testCase:                "Test Case 1",
			nodeName:                "example-node1",
			sharedPowerWorkloadName: "incorrect-name-example-node1-workload",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "incorrect-name-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "incorrect-name-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node1": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node1": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
				},
			},
			expectedDefaultPoolCPUList: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			testCase: "Test Case 2",
			nodeName: "example-node1",
			powerWorkloadNames: []string{
				"shared-example-node1-workload",
			},
			sharedPowerWorkloadName: "incorrect-name-example-node1-workload",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "incorrect-name-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "incorrect-name-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1, 2, 3},
					PowerNodeSelector: map[string]string{
						"example-node1": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:         "shared-example-node1-workload",
							AllCores:     true,
							ReservedCPUs: []int{0, 1},
							PowerNodeSelector: map[string]string{
								"example-node1": "true",
							},
							PowerProfile: "shared-example-node1",
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node1": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
				},
			},
			expectedDefaultPoolCPUList: []int{0, 1},
		},
		{
			testCase: "Test Case 3",
			nodeName: "example-node1",
			powerWorkloadNames: []string{
				"shared-example-node1-workload",
				"performance-example-node1-workload",
			},
			sharedPowerWorkloadName: "shared1-example-node1-workload",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared1-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared1-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1, 2, 3},
					PowerNodeSelector: map[string]string{
						"example-node1": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:         "shared-example-node1-workload",
							AllCores:     true,
							ReservedCPUs: []int{0, 1},
							PowerNodeSelector: map[string]string{
								"example-node1": "true",
							},
							PowerProfile: "shared-example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container2",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											4,
											5,
										},
										PowerProfile: "performance",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									4,
									5,
								},
							},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node1": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
				},
				{
					Node: "example-node1",
					Name: "performance",
				},
			},
			expectedDefaultPoolCPUList: []int{0, 1},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:         &tc.appqosPools[i].Name,
				ID:           &tc.appqosPools[i].Id,
				Cores:        &tc.appqosPools[i].Cores,
				PowerProfile: &tc.appqosPools[i].PowerProfile,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		for i := range tc.powerWorkloads.Items {
			objs = append(objs, &tc.powerWorkloads.Items[i])
		}
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		for _, powerWorkloadName := range tc.powerWorkloadNames {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      powerWorkloadName,
					Namespace: PowerWorkloadNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
			}
		}

		err = r.Client.Create(context.TODO(), tc.sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Shared PowerWorkload", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.sharedPowerWorkloadName,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		sharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.sharedPowerWorkloadName,
			Namespace: PowerWorkloadNamespace,
		}, sharedPowerWorkload)
		if err != nil {
			if !errors.IsNotFound(err) {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retreiving Shared PowerWorkload", tc.testCase))
			}
		} else {
			t.Errorf("%s - Failed: Expected Shared PowerWorkload to have been delete", tc.testCase)
		}

		defaultPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", "Default")
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Default Pool from AppQoS", tc.testCase))
		}

		if !reflect.DeepEqual(*defaultPool.Cores, tc.expectedDefaultPoolCPUList) {
			t.Errorf("%s - Failed: Expected Default Pool CPU list to be %v, got %v", tc.testCase, tc.expectedDefaultPoolCPUList, *defaultPool.Cores)
		}

		server.Close()
	}
}

func TestNonSharedPowerWorkloadBeginningWithShared(t *testing.T) {
	tcases := []struct {
		testCase                   string
		nodeName                   string
		powerWorkload              *powerv1alpha1.PowerWorkload
		nodeList                   *corev1.NodeList
		appqosPools                []AppQoSPool
		appqosPowerProfiles        []AppQoSPowerProfile
		expectedDefaultPoolCPUList []int
	}{
		{
			testCase: "Test Case 1",
			nodeName: "example-node1",
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-performance-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "shared-performance-example-node1-workload",
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					Node: powerv1alpha1.NodeInfo{
						Name: "example-node1",
						Containers: []powerv1alpha1.Container{
							{
								Name: "example-container2",
								Id:   "abcdefg",
								Pod:  "example-pod",
								ExclusiveCPUs: []int{
									4,
									5,
								},
								PowerProfile: "performance",
								Workload:     "shared-performance-example-node1-workload",
							},
						},
						CpuIds: []int{
							4,
							5,
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "performance",
				},
			},
			expectedDefaultPoolCPUList: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			testCase: "Test Case 2",
			nodeName: "example-node1",
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-balance-performance-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "shared-balance-performance-example-node1-workload",
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					Node: powerv1alpha1.NodeInfo{
						Name: "example-node1",
						Containers: []powerv1alpha1.Container{
							{
								Name: "example-container2",
								Id:   "abcdefg",
								Pod:  "example-pod",
								ExclusiveCPUs: []int{
									4,
									5,
									6,
									7,
								},
								PowerProfile: "balance-performance",
								Workload:     "shared-balance-performance-example-node1-workload",
							},
						},
						CpuIds: []int{
							4,
							5,
							6,
							7,
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "balance-performance",
				},
			},
			expectedDefaultPoolCPUList: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:         &tc.appqosPools[i].Name,
				ID:           &tc.appqosPools[i].Id,
				Cores:        &tc.appqosPools[i].Cores,
				PowerProfile: &tc.appqosPools[i].PowerProfile,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerWorkload)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.powerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		powerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.powerWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, powerWorkload)
		if err != nil {
			if !errors.IsNotFound(err) {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retreiving PowerWorkload object", tc.testCase))
			}
		} else {
			t.Errorf("%s - Failed: Expected PowerWorkload object to be deleted", tc.testCase)
		}

		defaultPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", "Default")
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Default Pool from AppQoS", tc.testCase))
		}

		if !reflect.DeepEqual(*defaultPool.Cores, tc.expectedDefaultPoolCPUList) {
			t.Errorf("%s - Failed: Expected Default Pool CPU list to be %v, got %v", tc.testCase, tc.expectedDefaultPoolCPUList, *defaultPool.Cores)
		}

		server.Close()
	}
}

func TestSecondSharedWorkloadCreatedWhileOriginalExists(t *testing.T) {
	tcases := []struct {
		testCase                           string
		nodeName                           string
		originalSharedPowerWorkload        *powerv1alpha1.PowerWorkload
		secondSharedPowerWorkload          *powerv1alpha1.PowerWorkload
		nodeList                           *corev1.NodeList
		appqosPools                        []AppQoSPool
		appqosPowerProfiles                []AppQoSPowerProfile
		expectedSharedPowerWorkloadCPUList []int
		expectedDefaultPoolCPUList         []int
		expectedSharedPoolCPUList          []int
	}{
		{
			testCase: "Test Case 1",
			nodeName: "example-node1",
			originalSharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			secondSharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-second-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-second-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1, 2, 3},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node",
					Name: "shared-example-node1",
					Id:   1,
				},
			},
			expectedSharedPowerWorkloadCPUList: []int{2, 3, 4, 5, 6, 7, 8, 9},
			expectedDefaultPoolCPUList:         []int{0, 1},
			expectedSharedPoolCPUList:          []int{2, 3, 4, 5, 6, 7, 8, 9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:         &tc.appqosPools[i].Name,
				ID:           &tc.appqosPools[i].Id,
				Cores:        &tc.appqosPools[i].Cores,
				PowerProfile: &tc.appqosPools[i].PowerProfile,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.originalSharedPowerWorkload)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.originalSharedPowerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.secondSharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating second Shared PowerWorkload", tc.testCase))
		}

		req = reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.secondSharedPowerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		// Need to reconcile a second time after the second Shared PowerWorkload is deleted
		req = reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.secondSharedPowerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		originalSharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.originalSharedPowerWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, originalSharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving original Shared PowerWorkload object", tc.testCase))
		}

		if !reflect.DeepEqual(originalSharedPowerWorkload.Status.SharedCores, tc.expectedSharedPowerWorkloadCPUList) {
			t.Errorf("%s - Failed: Expected Shared PowerWorkload CPU list to be %v, got %v", tc.testCase, tc.expectedSharedPowerWorkloadCPUList, originalSharedPowerWorkload.Status.SharedCores)
		}

		defaultPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", "Default")
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Default Pool from AppQoS", tc.testCase))
		}

		if !reflect.DeepEqual(*defaultPool.Cores, tc.expectedDefaultPoolCPUList) {
			t.Errorf("%s - Failed: Expected Default Pool CPU list to be %v, got %v", tc.testCase, tc.expectedDefaultPoolCPUList, *defaultPool.Cores)
		}

		sharedPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", "Shared")
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Shared Pool from AppQoS", tc.testCase))
		}

		if !reflect.DeepEqual(*sharedPool.Cores, tc.expectedSharedPoolCPUList) {
			t.Errorf("%s - Failed: Expected Shared Pool CPU list to be %v, got %v", tc.testCase, tc.expectedSharedPoolCPUList, *sharedPool.Cores)
		}

		server.Close()

		secondSharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.secondSharedPowerWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, secondSharedPowerWorkload)
		if err != nil {
			if !errors.IsNotFound(err) {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving second Shared PowerWorkload object", tc.testCase))
			}
		} else {
			t.Errorf("%s - Failed: Expected second Shared PowerWorkload to not exist", tc.testCase)
		}
	}
}

func TestIncorrectNumberOfNodesSelected(t *testing.T) {
	tcases := []struct {
		testCase                    string
		sharedPowerWorkload         *powerv1alpha1.PowerWorkload
		nodeName                    string
		nodes                       *corev1.NodeList
		expectedNumberOfSharedCores int
	}{
		{
			testCase: "Test Case 1",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node2-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node2-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "false",
					},
					PowerProfile: "shared-example-node2",
				},
			},
			nodeName: "example-node2",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
						},
					},
				},
			},
			expectedNumberOfSharedCores: 0,
		},
		{
			testCase: "Test Case 2",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			nodeName: "example-node1",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node3",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedNumberOfSharedCores: 0,
		},
		{
			testCase: "Test Case 3",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "false",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			nodeName: "example-node1",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
						},
					},
				},
			},
			expectedNumberOfSharedCores: 0,
		},
		{
			testCase: "Test Case 4",
			sharedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name:         "shared-example-node1-workload",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			nodeName: "example-node1",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedNumberOfSharedCores: 0,
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.sharedPowerWorkload)
		for i := range tc.nodes.Items {
			objs = append(objs, &tc.nodes.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.sharedPowerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling shared PowerWorkload object", tc.testCase))
		}

		sharedPowerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.sharedPowerWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, sharedPowerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Shared PowerWorkload", tc.testCase))
		}

		if len(sharedPowerWorkload.Status.SharedCores) != tc.expectedNumberOfSharedCores {
			t.Errorf("%s - Failed: Expected number of Shared Cores to be %v, got %v", tc.testCase, tc.expectedNumberOfSharedCores, len(sharedPowerWorkload.Status.SharedCores))
		}
	}
}

func TestIncorrectNodeForWorkload(t *testing.T) {
	tcases := []struct {
		testCase                   string
		powerWorkload              *powerv1alpha1.PowerWorkload
		nodeName                   string
		appqosPools                []AppQoSPool
		appqosPowerProfiles        []AppQoSPowerProfile
		nodeList                   *corev1.NodeList
		expectedPoolNameToNotExist string
	}{
		{
			testCase: "Test Case 1",
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "performance-example-node1-workload",
					Node: powerv1alpha1.NodeInfo{
						Name: "example-node1",
						Containers: []powerv1alpha1.Container{
							{
								Name:          "example-container1",
								Id:            "abcdefg",
								Pod:           "example-pod",
								ExclusiveCPUs: []int{2, 3},
								PowerProfile:  "performance-example-node1",
								Workload:      "performance-example-node1-workload",
							},
						},
						CpuIds: []int{2, 3},
					},
					PowerProfile: "performance-example-node1",
				},
			},
			nodeName: "example-node2",
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   1,
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
						},
					},
				},
			},
			expectedPoolNameToNotExist: "performance-example-node1-workload",
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:  &tc.appqosPools[i].Name,
				ID:    &tc.appqosPools[i].Id,
				Cores: &tc.appqosPools[i].Cores,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerWorkload)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.powerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object '%s'", tc.testCase, tc.powerWorkload.Name))
		}

		workloadPool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", tc.expectedPoolNameToNotExist)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Pool from AppQoS", tc.testCase))
		}

		if !reflect.DeepEqual(*workloadPool, appqos.Pool{}) {
			t.Errorf("%s - Failed: Expected Pool from AppQoS '%s' to not exist", tc.testCase, tc.expectedPoolNameToNotExist)
		}

		server.Close()
	}
}

func TestNonSharedWorkloadUpdate(t *testing.T) {
	tcases := []struct {
		testCase                   string
		nodeName                   string
		powerWorkloadName          string
		powerWorkloads             *powerv1alpha1.PowerWorkloadList
		nodeList                   *corev1.NodeList
		updatedNodeInfo            powerv1alpha1.NodeInfo
		appqosPools                []AppQoSPool
		appqosPowerProfiles        []AppQoSPowerProfile
		expectedAppQoSPoolCPULists map[string][]int
	}{
		{
			testCase:          "Test Case 1",
			nodeName:          "example-node1",
			powerWorkloadName: "performance-example-node1-workload",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:         "shared-example-node1-workload",
							AllCores:     true,
							ReservedCPUs: []int{0, 1},
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{4, 5, 6, 7, 8, 9},
							Node:        "example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance-example-node1",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance-example-node1",
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			updatedNodeInfo: powerv1alpha1.NodeInfo{
				Name: "example-node1",
				Containers: []powerv1alpha1.Container{
					{
						Name: "example-container1",
						Id:   "abcdefg",
						Pod:  "example-pod",
						ExclusiveCPUs: []int{
							2,
							3,
						},
						PowerProfile: "performance-example-node1",
						Workload:     "performance-example-node1-workload",
					},
					{
						Name: "example-container2",
						Id:   "abcdefgasdf",
						Pod:  "example-pod2",
						ExclusiveCPUs: []int{
							4,
							5,
						},
						PowerProfile: "performance-example-node1",
						Workload:     "performance-example-node1-workload",
					},
				},
				CpuIds: []int{
					2,
					3,
					4,
					5,
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{4, 5, 6, 7, 8, 9},
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    3,
					Cores: []int{2, 3},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   2,
				},
			},
			expectedAppQoSPoolCPULists: map[string][]int{
				"Default":                            []int{0, 1},
				"Shared":                             []int{6, 7, 8, 9},
				"performance-example-node1-workload": []int{2, 3, 4, 5},
			},
		},
		{
			testCase:          "Test Case 2",
			nodeName:          "example-node1",
			powerWorkloadName: "performance-example-node1-workload",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance-example-node1",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance-example-node1",
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			updatedNodeInfo: powerv1alpha1.NodeInfo{
				Name: "example-node1",
				Containers: []powerv1alpha1.Container{
					{
						Name: "example-container1",
						Id:   "abcdefg",
						Pod:  "example-pod",
						ExclusiveCPUs: []int{
							2,
							3,
						},
						PowerProfile: "performance-example-node1",
						Workload:     "performance-example-node1-workload",
					},
					{
						Name: "example-container2",
						Id:   "abcdefgasdf",
						Pod:  "example-pod2",
						ExclusiveCPUs: []int{
							4,
							5,
						},
						PowerProfile: "performance-example-node1",
						Workload:     "performance-example-node1-workload",
					},
				},
				CpuIds: []int{
					2,
					3,
					4,
					5,
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1, 4, 5, 6, 7, 8, 9},
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    2,
					Cores: []int{2, 3},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   1,
				},
			},
			expectedAppQoSPoolCPULists: map[string][]int{
				"Default":                            []int{0, 1, 6, 7, 8, 9},
				"performance-example-node1-workload": []int{2, 3, 4, 5},
			},
		},
		{
			testCase:          "Test Case 3",
			nodeName:          "example-node1",
			powerWorkloadName: "performance-example-node1-workload",
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name:         "shared-example-node1-workload",
							AllCores:     true,
							ReservedCPUs: []int{0, 1},
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							PowerProfile: "shared-example-node1",
						},
						Status: powerv1alpha1.PowerWorkloadStatus{
							SharedCores: []int{4, 5, 6, 7, 8, 9},
							Node:        "example-node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerWorkloadNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
								Containers: []powerv1alpha1.Container{
									{
										Name: "example-container1",
										Id:   "abcdefg",
										Pod:  "example-pod",
										ExclusiveCPUs: []int{
											2,
											3,
										},
										PowerProfile: "performance-example-node1",
										Workload:     "performance-example-node1-workload",
									},
								},
								CpuIds: []int{
									2,
									3,
								},
							},
							PowerProfile: "performance-example-node1",
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"example-node": "true",
							},
						},
					},
				},
			},
			updatedNodeInfo: powerv1alpha1.NodeInfo{
				Name: "example-node1",
				Containers: []powerv1alpha1.Container{
					{
						Name: "example-container2",
						Id:   "abcdefgasdf",
						Pod:  "example-pod2",
						ExclusiveCPUs: []int{
							4,
							5,
						},
						PowerProfile: "performance-example-node1",
						Workload:     "performance-example-node1-workload",
					},
				},
				CpuIds: []int{
					4,
					5,
				},
			},
			appqosPools: []AppQoSPool{
				{
					Node:  "example-node1",
					Name:  "Default",
					Id:    1,
					Cores: []int{0, 1},
				},
				{
					Node:  "example-node1",
					Name:  "Shared",
					Id:    2,
					Cores: []int{4, 5, 6, 7, 8, 9},
				},
				{
					Node:  "example-node1",
					Name:  "performance-example-node1-workload",
					Id:    3,
					Cores: []int{2, 3},
				},
			},
			appqosPowerProfiles: []AppQoSPowerProfile{
				{
					Node: "example-node1",
					Name: "shared-example-node1",
					Id:   1,
				},
				{
					Node: "example-node1",
					Name: "performance-example-node1",
					Id:   2,
				},
			},
			expectedAppQoSPoolCPULists: map[string][]int{
				"Default":                            []int{0, 1},
				"Shared":                             []int{2, 3, 6, 7, 8, 9},
				"performance-example-node1-workload": []int{4, 5},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		AppQoSClientAddress = "http://127.0.0.1:5000"

		appqosPools := make([]appqos.Pool, 0)
		for i := range tc.appqosPools {
			pool := appqos.Pool{
				Name:  &tc.appqosPools[i].Name,
				ID:    &tc.appqosPools[i].Id,
				Cores: &tc.appqosPools[i].Cores,
			}
			appqosPools = append(appqosPools, pool)
		}

		appqosPowerProfiles := make([]appqos.PowerProfile, 0)
		for i := range tc.appqosPowerProfiles {
			profile := appqos.PowerProfile{
				Name: &tc.appqosPowerProfiles[i].Name,
				ID:   &tc.appqosPowerProfiles[i].Id,
			}
			appqosPowerProfiles = append(appqosPowerProfiles, profile)
		}

		objs := make([]runtime.Object, 0)
		for i := range tc.powerWorkloads.Items {
			objs = append(objs, &tc.powerWorkloads.Items[i])
		}
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerWorkloadReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerWorkloadListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listeners", tc.testCase))
		}

		powerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.powerWorkloadName,
			Namespace: PowerWorkloadNamespace,
		}, powerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerWorkload object", tc.testCase))
		}

		powerWorkload.Spec.Node = tc.updatedNodeInfo
		err = r.Client.Update(context.TODO(), powerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error updated PowerWorkload", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.powerWorkloadName,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling PowerWorkload object", tc.testCase))
		}

		for poolName, cpuIds := range tc.expectedAppQoSPoolCPULists {
			pool, err := r.AppQoSClient.GetPoolByName("http://127.0.0.1:5000", poolName)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving Pool '%s' from AppQos", tc.testCase, poolName))
			}

			if !reflect.DeepEqual(*pool.Cores, cpuIds) {
				t.Errorf("%s - Failed: Expected Pool %v to have CPU List of %v, got %v", tc.testCase, poolName, cpuIds, *pool.Cores)
			}
		}

		server.Close()
	}
}
