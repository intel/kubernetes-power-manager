package controllers

import (
	//"bytes"
	"context"
	"testing"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	

	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	ctrl "sigs.k8s.io/controller-runtime"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	//controllers "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
)

const (
	PowerProfileName = "TestPowerProfile"
	PowerProfileNamespace = "default"
	//AppQoSAddress = "127.0.0.1:5000"
)

func createPowerProfileReconcileObject(powerProfile *powerv1alpha1.PowerProfile) (*PowerProfileReconciler, error) {
//func createPowerProfileReconcileObject(powerProfile *powerv1alpha1.PowerProfile) (*controllers.PowerProfileReconciler, error) {
	s := scheme.Scheme
	
	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	objs := []runtime.Object{powerProfile}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)
	
	cl := fake.NewFakeClient(objs...)

	appqosCl := appqos.NewDefaultAppQoSClient()

	r := &PowerProfileReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerProfile"), Scheme: s, AppQoSClient: appqosCl}
	//r := &controllers.PowerProfileReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerProfile"), Scheme: s, AppQoSClient: appqosCl}

	return r, nil
}

func createPowerProfileListeners(appqosPowerProfiles []appqos.PowerProfile) (*httptest.Server, error) {
	var err error

	newListener, err := net.Listen("tcp", "127.0.0.1:5000")
	//newListener, err := net.Listen("tcp", "http://localhost:5000")
	if err != nil {
		fmt.Errorf("Failed to create Listerner")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", (func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
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
		}
	}))


	ts := httptest.NewUnstartedServer(mux)

	ts.Listener.Close()
	ts.Listener = newListener

	// Start the server.
	ts.Start()

	return ts, nil
}

func TestPowerProfileReconciler(t *testing.T) {
	tcases := []struct{
		powerProfile *powerv1alpha1.PowerProfile
		node *corev1.Node
		getPowerProfileResponse map[string]map[int]([]appqos.Pool)
		expectedNumberOfProfiles int
		testCase int
		extendedResources map[string]bool
	}{
		{
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance",
					Epp: "performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						"cpu": *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			expectedNumberOfProfiles: 2,
			extendedResources: map[string]bool{
				"performance": true,
				"performance-example-node1": true,
			},
		},
		{
			powerProfile: &powerv1alpha1.PowerProfile{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "performance-node-name",
                                        Namespace: PowerProfileNamespace,
                                },
                                Spec: powerv1alpha1.PowerProfileSpec{
                                        Name: "performance-node-name",
                                        Epp: "performance",
                                },
                        },
			node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                },
                                Status: corev1.NodeStatus{
                                        Capacity: map[corev1.ResourceName]resource.Quantity{
                                                "cpu": *resource.NewQuantity(42, resource.DecimalSI),
                                        },
                                },
                        },
			expectedNumberOfProfiles: 1,
			extendedResources: map[string]bool{
                                "performance-node-name": false,
				"performance": false,
                        },
		},
		{
			powerProfile: &powerv1alpha1.PowerProfile{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "performance",
                                        Namespace: PowerProfileNamespace,
                                },
                                Spec: powerv1alpha1.PowerProfileSpec{
                                        Name: "performance",
                                        Epp: "incorrect",
                                },
                        },
			node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                },
                                Status: corev1.NodeStatus{
                                        Capacity: map[corev1.ResourceName]resource.Quantity{
                                                "cpu": *resource.NewQuantity(42, resource.DecimalSI),
                                        },
                                },
                        },
			expectedNumberOfProfiles: 1,
                        extendedResources: map[string]bool{
                                "performance": false,
                        },
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		//controllers.AppQoSClientAddress = "http://localhost:5000"
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.powerProfile)
		if err != nil {
			t.Fatal("error creating reconciler object")
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Listener")
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error creating Node object")
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: tc.powerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling object")
		}

		powerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerProfiles")
		}

		if len(powerProfiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("Failed: %v Expected PowerProfiles, got %v", tc.expectedNumberOfProfiles, len(powerProfiles.Items))
		}

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{Name: tc.node.Name}, updatedNode)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving Node object")
		}

		for er, shouldExist := range tc.extendedResources {
			resourceName := corev1.ResourceName(fmt.Sprintf("power.intel.com/%s", er))
			if _, exists := updatedNode.Status.Capacity[resourceName]; exists != shouldExist {
				t.Errorf("Failed: Expected Extended Resource exists to be %v, got %v", shouldExist, exists)
			}
		}

		server.Close()
	}
}

func TestPowerProfileDeletion(t *testing.T) {
	tcases := []struct{
                powerProfile *powerv1alpha1.PowerProfile
                node *corev1.Node
		profileToDelete string
		expectedNumberOfProfilesBeforeDeletion int
		expectedNumberOfProfilesAfterDeletion int
		expectedExtendedResourcesToExist map[string]bool
		additionalProfileReq []string
        }{
		{
			powerProfile: &powerv1alpha1.PowerProfile{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "performance",
                                        Namespace: PowerProfileNamespace,
                                },
                                Spec: powerv1alpha1.PowerProfileSpec{
                                        Name: "performance",
                                        Epp: "performance",
                                },
                        },
                        node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                },
                                Status: corev1.NodeStatus{
                                        Capacity: map[corev1.ResourceName]resource.Quantity{
                                                "cpu": *resource.NewQuantity(42, resource.DecimalSI),
                                        },
                                },
                        },
			profileToDelete: "performance-example-node1",
			expectedNumberOfProfilesBeforeDeletion: 2,
			expectedNumberOfProfilesAfterDeletion: 1,
			expectedExtendedResourcesToExist: map[string]bool{
				"performance": true,
				"performance-example-node1": false,
			},
		},
		{
			powerProfile: &powerv1alpha1.PowerProfile{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "performance",
                                        Namespace: PowerProfileNamespace,
                                },
                                Spec: powerv1alpha1.PowerProfileSpec{
                                        Name: "performance",
                                        Epp: "performance",
                                },
                        },
                        node: &corev1.Node{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: "example-node1",
                                },
                                Status: corev1.NodeStatus{
                                        Capacity: map[corev1.ResourceName]resource.Quantity{
                                                "cpu": *resource.NewQuantity(42, resource.DecimalSI),
                                        },
                                },
                        },
			profileToDelete: "performance",
			expectedNumberOfProfilesBeforeDeletion: 2,
			expectedNumberOfProfilesAfterDeletion: 0,
			expectedExtendedResourcesToExist: map[string]bool{
                                "performance": false,
                                "performance-example-node1": false,
                        },
			additionalProfileReq: []string{
				"performance-example-node1",
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		//controllers.AppQoSClientAddress = "http://localhost:5000"
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.powerProfile)
		if err != nil {
			t.Fatal("error creating reconciler object")
		}

		id := 1
		extendedProfile := fmt.Sprintf("%s-%s", tc.powerProfile.Name, tc.node.Name)
		profilesFromAppQoS := []appqos.PowerProfile{
			{
				Name: &tc.profileToDelete,
				ID: &id,
			},
			{
				Name: &extendedProfile,
				ID: &id,
			},
		}

		server, err := createPowerProfileListeners(profilesFromAppQoS)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Listener")
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error creating Node object")
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: tc.powerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling object")
		}

		powerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerProfiles")
		}

		if len(powerProfiles.Items) != tc.expectedNumberOfProfilesBeforeDeletion {
			t.Errorf("Failed: %v Expected PowerProfiles, got %v", tc.expectedNumberOfProfilesBeforeDeletion, len(powerProfiles.Items))
		}

		profileToDelete := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
                        Name: tc.profileToDelete,
                        Namespace: PowerProfileNamespace,
                }, profileToDelete)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerProfile object")
		}

		err = r.Client.Delete(context.TODO(), profileToDelete)
		if err != nil {
			t.Error(err)
			t.Fatal("error deleting PowerProfile object")
		}

		req = reconcile.Request{
                        NamespacedName: client.ObjectKey{
                                Name: tc.profileToDelete,
                                Namespace: PowerProfileNamespace,
                        },
                }

		_, err = r.Reconcile(req)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error reconciling object")
                }

		for _, additionalReq := range tc.additionalProfileReq {
			req = reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: additionalReq,
					Namespace: PowerProfileNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal("error reconciling object")
			}
		}

		server.Close()

		updatedPowerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), updatedPowerProfiles)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error retrieving PowerProfiles")
                }

		if len(updatedPowerProfiles.Items) != tc.expectedNumberOfProfilesAfterDeletion {
                        t.Errorf("Failed: %v Expected PowerProfiles after deletion, got %v", tc.expectedNumberOfProfilesAfterDeletion, len(updatedPowerProfiles.Items))
                }

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{Name: tc.node.Name}, updatedNode)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving Node object")
		}

		for profileExtendedResource, shouldExist := range tc.expectedExtendedResourcesToExist {
			resourceName := corev1.ResourceName(fmt.Sprintf("power.intel.com/%s", profileExtendedResource))
			if _, exists := updatedNode.Status.Capacity[resourceName]; exists != shouldExist {
				t.Errorf("Failed: %v Expected Extended Resource existing to be %v, got %v", profileExtendedResource, shouldExist, exists)
				t.Errorf("Node: %v", updatedNode)
			}
		}
	}
}
