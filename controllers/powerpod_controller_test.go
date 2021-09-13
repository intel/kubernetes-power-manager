package controllers

/*
import (
	"context"
	"testing"
	//"reflect"
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
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podstate"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podresourcesclient"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
)

const (
	PowerPodName = "TestPowerPod"
	PowerPodNamespace = "default"
	//AppQoSAddress = "127.0.0.1:5000"
)

//func createPowerPodReconcilerObject(pod *corev1.Pod) (*controllers.PowerPodReconciler, error) {
func createPowerPodReconcilerObject(pod *corev1.Pod) (*PowerPodReconciler, error) {
	s := scheme.Scheme

	if err  := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	objs := []runtime.Object{pod}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)

	cl := fake.NewFakeClient(objs...)

	state, err := podstate.NewState()
	if err != nil {
		return nil, err
	}

	podResourcesClient, err := podresourcesclient.NewPodResourcesClient()
	if err != nil {
		return nil, err
	}

	appqosCl := appqos.NewDefaultAppQoSClient()

	//r := &controllers.PowerPodReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerWorkload"), Scheme: s, State: *state, PodResourcesClient: *podResourcesClient, AppQoSClient: appqosCl}
	r := &PowerPodReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerWorkload"), Scheme: s, State: *state, PodResourcesClient: *podResourcesClient, AppQoSClient: appqosCl}

	return r, nil
}

func createPowerPodListeners(appqosPools []appqos.Pool, appqosPowerProfiles []appqos.PowerProfile) (*httptest.Server, error) {
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

func TestPowerPodReconcile(t *testing.T) {
	tcases := []struct{
		pod *corev1.Pod
		node *corev1.Node
		powerProfile *powerv1alpha1.PowerProfile
		resources map[string]string
	}{
		{
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-pod",
					Namespace: PowerPodNamespace,
					UID: "abcdefg",
				},
				Spec: corev1.PodSpec{
					NodeName: "example-node1",
					Containers: []corev1.Container{
						{
							Name: "example-container-1",
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceName("cpu"): *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("memory"): *resource.NewQuantity(200, resource.DecimalSI),
									corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
								},
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceName("cpu"): *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("memory"): *resource.NewQuantity(200, resource.DecimalSI),
									corev1.ResourceName("power.intel.com/performance-example-node1"): *resource.NewQuantity(2, resource.DecimalSI),
                                                                },
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					QOSClass: corev1.PodQOSGuaranteed,
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance-example-node1",
					Namespace: PowerPodNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance-example-node1",
					Max: 3200,
					Min: 2800,
					Epp: "performance",
				},
			},
			resources: map[string]string{
				"cpu": "2",
				"memory": "200Mi",
				"power.intel.com/performance-example-node1": "2",
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)

		r, err := createPowerPodReconcilerObject(tc.pod)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating reconciler object")
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating Node object")
		}

		err = r.Client.Create(context.TODO(), tc.powerProfile)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating PowerProfile object")
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: tc.pod.Name,
				Namespace: PowerPodNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling PowerWorkload object")
		}

		powerWorkloads := &powerv1alpha1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), powerWorkloads)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerWorkload list")
		}

		if len(powerWorkloads.Items) != 1 {
			t.Errorf("Failed: Expected 1 PowerWorkload to be created, got %v", len(powerWorkloads.Items))
		}
	}
}
*/
