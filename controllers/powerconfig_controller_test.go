package controllers

import (
	"context"

	//. "github.com/onsi/ginkgo"
	//. "github.com/onsi/gomega"

	"testing"

	//"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	ctrl "sigs.k8s.io/controller-runtime"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	//controllers "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
)

const (
	PowerConfigName = "TestPowerConfig"
	PowerConfigNamespace = "default"
)

func createReconcileObject(powerConfig *powerv1alpha1.PowerConfig) (*PowerConfigReconciler, error) {
//func createReconcileObject(powerConfig *powerv1alpha1.PowerConfig) (*controllers.PowerConfigReconciler, error) {
	s := scheme.Scheme
	
	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	objs := []runtime.Object{powerConfig}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)
	
	cl := fake.NewFakeClient(objs...)

	powerNodeData := &state.PowerNodeData{
		PowerNodeList: []string{},
	}

	r := &PowerConfigReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerConfig"), Scheme: s, State: powerNodeData}
	//r := &controllers.PowerConfigReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerConfig"), Scheme: s, State: powerNodeData}

	return r, nil
}

func TestPowerConfigReconciler(t *testing.T) {
	tcases := []struct{
		powerConfig *powerv1alpha1.PowerConfig
		nodeList *corev1.NodeList
		expectedNumberOfNodes int
		expectedNumberOfProfiles int
	}{
		{
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"test-node": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"test-node": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{
								"test-node": "true",
							},
						},
					},
				},
			},
			expectedNumberOfNodes: 2,
			expectedNumberOfProfiles: 1,
		},
		{
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"test-node": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"test-node": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{
								"test-node": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node3",
							Labels: map[string]string{},
						},
					},
				},
			},
			expectedNumberOfNodes: 2,
			expectedNumberOfProfiles: 1,
		},
		{
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"test-node": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"test-node": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node3",
							Labels: map[string]string{},
						},
					},
				},
			},
			expectedNumberOfNodes: 1,
			expectedNumberOfProfiles: 1,
		},
		{
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"test-node": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{},
						},
					},
				},
			},
			expectedNumberOfNodes: 0,
			expectedNumberOfProfiles: 1,
		},
		{
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"test-node": "true",
					},
					PowerProfiles: []string{
						"performance",
						"incorrect-profile",
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"test-node": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{
								"test-node": "true",
							},
						},
					},
				},
				
			},
			expectedNumberOfNodes: 2,
			expectedNumberOfProfiles: 1,
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"
		//controllers.NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createReconcileObject(tc.powerConfig)
		if err != nil {
			t.Fatal("error creating Reconcile object")
		}

		for i := range tc.nodeList.Items {
			node := &corev1.Node{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind: "Node",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.nodeList.Items[i].Name,
					Labels: tc.nodeList.Items[i].Labels,
				},
			}

			err = r.Client.Create(context.TODO(), node)
			if err != nil {
				t.Errorf("Error: %v", err)
				t.Fatal("error creating Node")
			}
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		createdPowerConfig := &powerv1alpha1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: PowerConfigName,
			Namespace: PowerConfigNamespace,
		}, createdPowerConfig)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error retrieving power config")
		}

		if len(createdPowerConfig.Status.Nodes) != tc.expectedNumberOfNodes {
			t.Errorf("Failed: %v Expected Nodes, got %v", tc.expectedNumberOfNodes, len(createdPowerConfig.Status.Nodes))
		}

		profiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error retrieving PowerProfiles")
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("Failed: %v Expected profiles, got %v", tc.expectedNumberOfProfiles, len(profiles.Items))
		}

		nodeAgentDS := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			//Name: controllers.NodeAgentDSName,
			Name: NodeAgentDSName,
			Namespace: PowerConfigNamespace,
		}, nodeAgentDS)
		if err != nil {
			t.Errorf("Failed: Expected Power Node Agent to be created - %v", err)
		}

		powerNodeList := &powerv1alpha1.PowerNodeList{}
		err = r.Client.List(context.TODO(), powerNodeList)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerNode List")
		}

		if len(powerNodeList.Items) != tc.expectedNumberOfNodes {
			t.Errorf("Failed: Expected %v PowerNodes, got %v", tc.expectedNumberOfNodes, len(powerNodeList.Items))
		}
	}
}

func TestMultiplePowerConfigs(t *testing.T) {
	tcases := []struct{
		secondConfig *powerv1alpha1.PowerConfig
		nodeList *corev1.NodeList
		expectedNumberOfNodes int
		expectedNumberOfProfiles int
	}{
		{
			secondConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"unselected-label": "true",
					},
					PowerProfiles: []string{
						"performance",
						"balance-performance",
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node1",
							Labels: map[string]string{
								"correct-label": "true",
								"unselected-label": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{
								"correct-label": "true",
                                                                "unselected-label": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node3",
							Labels: map[string]string{
                                                                "unselected-label": "true",
							},
						},
					},
				},
			},
			expectedNumberOfNodes: 0,
			expectedNumberOfProfiles: 1,
		},
	}

	for _, tc := range tcases {
		//controllers.NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"
		configName := "InitialPowerConfig"

		initialPowerConfig := &powerv1alpha1.PowerConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: configName,
				Namespace: PowerConfigNamespace,
			},
			Spec: powerv1alpha1.PowerConfigSpec{
				PowerNodeSelector: map[string]string{
					"correct-label": "true",
				},
				PowerProfiles: []string{
					"performance",
				},
			},
		}

		r, err := createReconcileObject(initialPowerConfig)
		if err != nil {
			t.Fatal("error creating Reconcile object")
		}

		for i := range tc.nodeList.Items {
			node := &corev1.Node{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind: "Node",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.nodeList.Items[i].Name,
					Labels: tc.nodeList.Items[i].Labels,
				},
			}

			err = r.Client.Create(context.TODO(), node)
			if err != nil {
				t.Errorf("Error: %v", err)
				t.Fatal("error creating Node")
			}
		}

		initialReq := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: configName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(initialReq)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling initail object")
		}

		err = r.Client.Create(context.TODO(), tc.secondConfig)
		if err != nil {
			t.Error(err)
			t.Fatal("error creating second object")
		}

		secondReq := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(secondReq)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling second object")
		}

		initialCreatedPowerConfig := &powerv1alpha1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: configName,
			Namespace: PowerConfigNamespace,
		}, initialCreatedPowerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving intitial object")
		}

		if len(initialCreatedPowerConfig.Status.Nodes) != 2 {
			t.Errorf("Failed: Expected 2 Nodes, got %v", len(initialCreatedPowerConfig.Status.Nodes))
		}

		secondCreatedPowerConfig := &powerv1alpha1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: PowerConfigName,
			Namespace: PowerConfigNamespace,
		}, secondCreatedPowerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving second object")
		}

		if len(secondCreatedPowerConfig.Status.Nodes) != tc.expectedNumberOfNodes {
			t.Errorf("Failed: Expected %v Nodes, got %v", tc.expectedNumberOfNodes, len(secondCreatedPowerConfig.Status.Nodes))
		}

		profiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerProfiles")
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("Failed: Expected %v PowerProfiles, got %v", tc.expectedNumberOfProfiles, len(profiles.Items))
		}
	}
}

func TestUnusedProfileRemoval(t *testing.T) {
	tcases := []struct{
		powerConfig *powerv1alpha1.PowerConfig
		oldProfileList []string
		expectedNumberOfProfiles int
	}{
		{
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			oldProfileList: []string{
				"performance",
				"balance_performance",
			},
			expectedNumberOfProfiles: 1,
		},
	}

	for _, tc := range tcases {
		//controllers.NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createReconcileObject(tc.powerConfig)
		if err != nil {
			t.Fatal("error creating Reconcile object")
		}

		for _, profile := range tc.oldProfileList {
			powerProfile := &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: profile,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: profile,
				},
			}

			err := r.Client.Create(context.TODO(), powerProfile)
			if err != nil {
				t.Error(err)
				t.Fatal("error creating PowerProfile object")
			}
		}

		req := reconcile.Request{
			client.ObjectKey{
				Name: PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal("error reconciling object")
		}

		profiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerProfile objects")
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("Failed: Expected %v PowerProfiles, got %v", tc.expectedNumberOfProfiles, len(profiles.Items))
		}
	}
}

/*
func TestPowerConfigStatusUpdate(t *testing.T) {
	tcases := []struct{
		powerConfig *powerv1alpha1.PowerConfig
		nodeList *corev1.NodeList
	}{
		{
			powerConfig: &powerv1alpha1.PowerConfig{
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: PowerConfigName,
                                        Namespace: PowerConfigNamespace,
                                },
                                Spec: powerv1alpha1.PowerConfigSpec{
                                        PowerNodeSelector: map[string]string{
                                                "test-node": "true",
                                        },
                                },
                        },
                        nodeList: &corev1.NodeList{
                                Items: []corev1.Node{
                                        {
                                                ObjectMeta: metav1.ObjectMeta{
                                                        Name: "example-node1",
                                                        Labels: map[string]string{
                                                                "test-node": "true",
								"secondary-label": "true",
                                                        },
                                                },
                                        },
                                        {
                                                ObjectMeta: metav1.ObjectMeta{
                                                        Name: "example-node2",
                                                        Labels: map[string]string{
								"secondary-label": "true",
                                                        },
                                                },
                                        },
                                },

                        },
		},
	}

	for _, tc := range tcases {
		//controllers.NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

                r, err := createReconcileObject(tc.powerConfig)
                if err != nil {
                        t.Fatal("error creating Reconcile object")
                }

                for i := range tc.nodeList.Items {
                        node := &corev1.Node{
                                TypeMeta: metav1.TypeMeta{
                                        APIVersion: "v1",
                                        Kind: "Node",
                                },
                                ObjectMeta: metav1.ObjectMeta{
                                        Name: tc.nodeList.Items[i].Name,
                                        Labels: tc.nodeList.Items[i].Labels,
                                },
                        }

                        err = r.Client.Create(context.TODO(), node)
                        if err != nil {
                                t.Errorf("Error: %v", err)
                                t.Fatal("error creating Node")
                        }
                }

                req := reconcile.Request{
                        NamespacedName: client.ObjectKey{
                                Name: PowerConfigName,
                                Namespace: PowerConfigNamespace,
                        },
                }

                _, err = r.Reconcile(req)
                if err != nil {
                        t.Errorf("Error: %v", err)
                        t.Fatal("error reconciling PowerConfig object")
                }

		createdPowerConfig := &powerv1alpha1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: PowerConfigName,
			Namespace: PowerConfigNamespace,
		}, createdPowerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal("error retrieving PowerConfig object")
		}

		if len(createdPowerConfig.Status.Nodes) != 1 {
			t.Errorf("Failed: %v Expected Nodes, got %v", 1, len(createdPowerConfig.Status.Nodes))
		}

		createdPowerConfig.Spec.PowerNodeSelector = map[string]string{}
		createdPowerConfig.Spec.PowerNodeSelector = map[string]string{
			"secondary-label": "true",
		}

		err = r.Client.Update(context.TODO(), createdPowerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal("error updating PowerConfig object")
		}

		 _, err = r.Reconcile(req)
                if err != nil {
                        t.Errorf("Error: %v", err)
                        t.Fatal("error reconciling PowerConfig object")
                }

		err = r.Client.Get(context.TODO(), client.ObjectKey{
                        Name: PowerConfigName,
                        Namespace: PowerConfigNamespace,
                }, createdPowerConfig)
                if err != nil {
                        t.Error(err)
                        t.Fatal("error retrieving PowerConfig object")
                }

		 if len(createdPowerConfig.Status.Nodes) != 6 {
                        t.Errorf("Failed: %v Expected Nodes, got %v", 2, len(createdPowerConfig.Status.Nodes))
			t.Errorf("Nodes: %v", createdPowerConfig)
                }
	}
}
*/
