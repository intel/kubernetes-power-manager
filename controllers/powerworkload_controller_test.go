package controllers

import (
	"context"
	"testing"
	"reflect"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	ctrl "sigs.k8s.io/controller-runtime"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	controllers "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
)

const (
	PowerWorkloadName = "TestPowerWorkload"
	PowerWorkloadNamespace = "default"
	AppQoSAddress = "127.0.0.1:5000"
)

func createPowerWorkloadReconcilerObject(powerWorkload *powerv1alpha1.PowerWorkload) (*controllers.PowerWorkloadReconciler, error) {
	s := scheme.Scheme

	if err  := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	objs := []runtime.Object{powerWorkload}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)

	cl := fake.NewFakeClient(objs...)

	appqosCl := appqos.NewDefaultAppQoSClient()

	r := &controllers.PowerWorkloadReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerWorkload"), Scheme: s, AppQoSClient: appqosCl}

	return r, nil
}

func createListeners(appqosPools []appqos.Pool, appqosPowerProfiles []appqos.PowerProfile) (*httptest.Server, error) {
	var err error

	newListener, err := net.Listen("tcp", AppQoSAddress)
	if err != nil {
		fmt.Errorf("Failed to create Listener")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", (func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" || r.Method == "PUT" {
			b, err := json.Marshal("okay")
			if err == nil {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, string(b[:]))
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

	ts := httptest.NewUnstartedServer(mux)

	ts.Listener.Close()
	ts.Listener = newListener

	// Start the server
	ts.Start()

	return ts, nil
}

func TestPowerWorkloadReconciler(t *testing.T) {
	tcases := []struct{
		powerWorkload *powerv1alpha1.PowerWorkload
		node *corev1.Node
		sharedWorkload *powerv1alpha1.PowerWorkload
		addToAppQoS bool
		sharedCores []int
		sharedPoolName string
	}{
		{
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance-example-node1-workload",
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
								Name: "example-container",
								Id: "abcdefg",
								Pod: "example-pod",
								ExclusiveCPUs: []int{
									2,
									3,
								},
								PowerProfile: "performance",
								Workload: "performance-example-node1-workload",
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
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
					Labels: map[string]string{
						"example-node": "true",
					},
				},
			},
			sharedWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "shared-example-node1-workload",
					AllCores: true,
					ReservedCPUs: []int{0},
					PowerProfile: "shared-example-node1",
				},
			},
			addToAppQoS: false,
			sharedCores: []int{0,1,2,3,4,5,6,7,8,9},
			sharedPoolName: "Shared",
		},
		{
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance-example-node1-workload",
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
								Name: "example-container",
								Id: "abcdefg",
								Pod: "example-pod",
								ExclusiveCPUs: []int{
									2,
									3,
								},
								PowerProfile: "performance",
								Workload: "performance-example-node1-workload",
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
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
					Labels: map[string]string{
						"example-node": "true",
					},
				},
			},
			sharedWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "shared-example-node1-workload",
					AllCores: true,
					ReservedCPUs: []int{0},
					PowerProfile: "shared-example-node1",
				},
			},
			addToAppQoS: true,
			sharedCores: []int{0,1,2,3,4,5,6,7,8,9},
			sharedPoolName: "Shared",
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		controllers.AppQoSClientAddress = "http://127.0.0.1:5000"

		id := 1
		appqosPools := []appqos.Pool{
			{
				Name: &tc.sharedPoolName,
				ID: &id,
				Cores: &tc.sharedCores,
			},
		}
		if tc.addToAppQoS {
			appqosPool := appqos.Pool{
				Name: &tc.powerWorkload.Name,
				ID: &id,
				Cores: &tc.powerWorkload.Spec.Node.CpuIds,
			}

			appqosPools = append(appqosPools, appqosPool)
		}

		appqosPowerProfiles := []appqos.PowerProfile{
			{
				Name: &tc.powerWorkload.Spec.PowerProfile,
				ID: &id,
			},
			{
				Name: &tc.sharedWorkload.Spec.PowerProfile,
				ID: &id,
			},
		}

		r, err := createPowerWorkloadReconcilerObject(tc.powerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating reconciler object")
		}

		server, err := createListeners(appqosPools, appqosPowerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Listeners")
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Node object")
		}

		err = r.Client.Create(context.TODO(), tc.sharedWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Shared PowerWorkload object")
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: tc.powerWorkload.Name,
				Namespace: PowerWorkloadNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling PowerWorkload object")
		}

		server.Close()
	}
}

func TestSharedWorkloadCreation(t *testing.T) {
	tcases := []struct{
		powerWorkload *powerv1alpha1.PowerWorkload
		node *corev1.Node
		defaultPoolName string
		defaultPoolCores []int
		expectedCoreList []int
	}{
		{
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "shared-example-node1-workload",
					AllCores: true,
					ReservedCPUs: []int{0},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                        Labels: map[string]string{
                                                "example-node": "true",
                                        },
                                },
			},
			defaultPoolName: "Default",
			defaultPoolCores: []int{0,1,2,3,4,5,6,7,8,9},
			expectedCoreList: []int{1,2,3,4,5,6,7,8,9},
		},
		{
			powerWorkload: &powerv1alpha1.PowerWorkload{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "shared-example-node2-workload",
                                        Namespace: PowerWorkloadNamespace,
                                },
                                Spec: powerv1alpha1.PowerWorkloadSpec{
                                        Name: "shared-example-node2-workload",
                                        AllCores: true,
                                        ReservedCPUs: []int{0,1,2,3},
                                        PowerNodeSelector: map[string]string{
                                                "example-node": "true",
                                        },
                                        PowerProfile: "shared-example-node2",
                                },
                        },
                        node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node2",
                                        Labels: map[string]string{
                                                "example-node": "true",
                                        },
                                },
                        },
                        defaultPoolName: "Default",
                        defaultPoolCores: []int{0,1,2,3,4,5,6,7,8,9},
                        expectedCoreList: []int{4,5,6,7,8,9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
                controllers.AppQoSClientAddress = "http://127.0.0.1:5000"

                id := 1
                appqosPools := []appqos.Pool{
			{
				Name: &tc.defaultPoolName,
				ID: &id,
				Cores: &tc.defaultPoolCores,
			},
                }

                appqosPowerProfiles := []appqos.PowerProfile{
                        {
                                Name: &tc.powerWorkload.Spec.PowerProfile,
                                ID: &id,
                        },
                }

                r, err := createPowerWorkloadReconcilerObject(tc.powerWorkload)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating reconciler object")
                }

                server, err := createListeners(appqosPools, appqosPowerProfiles)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating Listeners")
                }

                err = r.Client.Create(context.TODO(), tc.node)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating Node object")
                }

		req := reconcile.Request{
                        NamespacedName: client.ObjectKey{
                                Name: tc.powerWorkload.Name,
                                Namespace: PowerWorkloadNamespace,
                        },
                }

                _, err = r.Reconcile(req)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error reconciling PowerWorkload object")
                }

		server.Close()

		sharedWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.powerWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, sharedWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerWorkload object")
		}

		if !reflect.DeepEqual(sharedWorkload.Status.SharedCores, tc.expectedCoreList) {
			t.Errorf("Failed: Expected Shared Core List of %v, got %v", tc.expectedCoreList, sharedWorkload.Status.SharedCores)
		}
	}
}

func TestStatusSharedCoresUpdated(t *testing.T) {
	tcases := []struct{
		sharedWorkload *powerv1alpha1.PowerWorkload
		powerWorkload *powerv1alpha1.PowerWorkload
                node *corev1.Node
		sharedPoolName string
		sharedPoolCores []int
                defaultPoolName string
                defaultPoolCores []int
                expectedCoreList []int
	}{
		{
			sharedWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared-example-node1-workload",
					Namespace: PowerWorkloadNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "shared-example-node1-workload",
					AllCores: true,
					ReservedCPUs: []int{0,1},
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfile: "shared-example-node1",
				},
				Status: powerv1alpha1.PowerWorkloadStatus{
					SharedCores: []int{2,3,4,5,6,7,8,9},
				},
			},
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance-example-node1-workload",
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
								Name: "example-container",
								Id: "abcdefg",
								Pod: "example-pod",
								ExclusiveCPUs: []int{
									2,
									3,
								},
								PowerProfile: "performance",
								Workload: "performance-example-node1-workload",
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
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
					Labels: map[string]string{
						"example-node": "true",
					},
				},
			},
			sharedPoolName: "Shared",
			sharedPoolCores: []int{2,3,4,5,6,7,8,9},
			defaultPoolName: "Default",
			defaultPoolCores: []int{0,1},
			expectedCoreList: []int{4,5,6,7,8,9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
                controllers.AppQoSClientAddress = "http://127.0.0.1:5000"

                id := 1
                appqosPools := []appqos.Pool{
			{
				Name: &tc.defaultPoolName,
				ID: &id,
				Cores: &tc.defaultPoolCores,
			},
			{
				Name: &tc.sharedPoolName,
				ID: &id,
				Cores: &tc.sharedPoolCores,
			},
                }

                appqosPowerProfiles := []appqos.PowerProfile{
                        {
                                Name: &tc.powerWorkload.Spec.PowerProfile,
                                ID: &id,
                        },
			{
				Name: &tc.sharedWorkload.Spec.PowerProfile,
				ID: &id,
			},
                }

                r, err := createPowerWorkloadReconcilerObject(tc.powerWorkload)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating reconciler object")
                }

                server, err := createListeners(appqosPools, appqosPowerProfiles)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating Listeners")
                }

                err = r.Client.Create(context.TODO(), tc.node)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating Node object")
                }

		err = r.Client.Create(context.TODO(), tc.sharedWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Shared PowerWorkload object")
		}

		req := reconcile.Request{
                        NamespacedName: client.ObjectKey{
                                Name: tc.powerWorkload.Name,
                                Namespace: PowerWorkloadNamespace,
                        },
                }

                _, err = r.Reconcile(req)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error reconciling PowerWorkload object")
                }

		server.Close()

		sharedWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.sharedWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, sharedWorkload)

		if !reflect.DeepEqual(sharedWorkload.Status.SharedCores, tc.expectedCoreList) {
			t.Errorf("Failed: Expected Shared Core List of %v, got %v", tc.expectedCoreList, sharedWorkload.Status.SharedCores)
		}
	}
}

func TestSharedWorkloadIncorrectNode(t *testing.T) {
	tcases := []struct{
		sharedWorkload *powerv1alpha1.PowerWorkload
		node *corev1.Node
	}{
		{
			sharedWorkload: &powerv1alpha1.PowerWorkload{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "shared-example-node1-workload",
                                        Namespace: PowerWorkloadNamespace,
                                },
                                Spec: powerv1alpha1.PowerWorkloadSpec{
                                        Name: "shared-example-node1-workload",
                                        AllCores: true,
                                        ReservedCPUs: []int{0,1},
					PowerNodeSelector: map[string]string{
                                                "example-node": "true",
                                        },
					Node: powerv1alpha1.NodeInfo{
						Name: "example-node1",
					},
                                        PowerProfile: "shared-example-node1",
                                },
                        },
			node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                        Labels: map[string]string{
                                                "example-node": "true",
                                        },
                                },
                        },
		},
		{
			sharedWorkload: &powerv1alpha1.PowerWorkload{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "shared-example-node1-workload",
                                        Namespace: PowerWorkloadNamespace,
                                },
                                Spec: powerv1alpha1.PowerWorkloadSpec{
                                        Name: "shared-example-node1-workload",
                                        AllCores: true,
                                        ReservedCPUs: []int{0,1},
                                        PowerNodeSelector: map[string]string{
                                                "example-node": "true",
                                        },
                                        PowerProfile: "shared-example-node1",
                                },
                        },
			node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                        Labels: map[string]string{
                                                "example-node": "false",
                                        },
                                },
                        },
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", "incorrect-node")

		r, err := createPowerWorkloadReconcilerObject(tc.sharedWorkload)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating reconciler object")
                }

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Node object")
		}

		req := reconcile.Request{
                        NamespacedName: client.ObjectKey{
                                Name: tc.sharedWorkload.Name,
                                Namespace: PowerWorkloadNamespace,
                        },
                }

                _, err = r.Reconcile(req)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error reconciling PowerWorkload object")
                }

		sharedWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.sharedWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, sharedWorkload)

		if len(sharedWorkload.Status.SharedCores) != 0 {
			t.Errorf("Failed: Expected Shared Core List to be empty, got %v", sharedWorkload.Status.SharedCores)
		}
	}
}

func TestSharedPoolAlreadyExistsInAppQoS(t *testing.T) {
	tcases := []struct{
		sharedWorkload *powerv1alpha1.PowerWorkload
		node *corev1.Node
		sharedPoolName string
		sharedPoolCores []int
	}{
		{
			sharedWorkload: &powerv1alpha1.PowerWorkload{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "shared-example-node1-workload",
                                        Namespace: PowerWorkloadNamespace,
                                },
                                Spec: powerv1alpha1.PowerWorkloadSpec{
                                        Name: "shared-example-node1-workload",
                                        AllCores: true,
                                        ReservedCPUs: []int{0,1},
                                        PowerNodeSelector: map[string]string{
                                                "example-node": "true",
                                        },
                                        PowerProfile: "shared-example-node1",
                                },
                        },
			node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                        Labels: map[string]string{
                                                "example-node": "true",
                                        },
                                },
                        },
			sharedPoolName: "Shared",
			sharedPoolCores: []int{0,1,2,3,4,5,6,7,8,9},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
                controllers.AppQoSClientAddress = "http://127.0.0.1:5000"

                id := 1
                appqosPools := []appqos.Pool{
                        {
                                Name: &tc.sharedPoolName,
                                ID: &id,
                                Cores: &tc.sharedPoolCores,
                        },
                }

                appqosPowerProfiles := []appqos.PowerProfile{
                        {
                                Name: &tc.sharedWorkload.Spec.PowerProfile,
                                ID: &id,
                        },
                }

                r, err := createPowerWorkloadReconcilerObject(tc.sharedWorkload)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating reconciler object")
                }

                server, err := createListeners(appqosPools, appqosPowerProfiles)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error creating Listeners")
                }

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Node object")
		}

		req := reconcile.Request{
                        NamespacedName: client.ObjectKey{
                                Name: tc.sharedWorkload.Name,
                                Namespace: PowerWorkloadNamespace,
                        },
                }

                _, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling object")
		}

		server.Close()

		sharedWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.sharedWorkload.Name,
			Namespace: PowerWorkloadNamespace,
		}, sharedWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving Shared PowerWorkload object")
		}

		if len(sharedWorkload.Status.SharedCores) != 0 {
			t.Errorf("Failed: Expected Shared Core List to be empty, got %v", sharedWorkload.Status.SharedCores)
		}
	}
}
