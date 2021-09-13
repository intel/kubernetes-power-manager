package controllers

import (
	"context"
	"fmt"
	"reflect"

	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PowerConfigName      = "power-config"
	PowerConfigNamespace = "default"
)

func createPowerConfigReconcilerObject(objs []runtime.Object) (*PowerConfigReconciler, error) {
	s := scheme.Scheme

	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)

	cl := fake.NewFakeClient(objs...)

	powerNodeData := &state.PowerNodeData{
		PowerNodeList: []string{},
	}

	r := &PowerConfigReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerConfig"), Scheme: s, State: powerNodeData}

	return r, nil
}

func TestNumberOfConfigsGreaterThanOne(t *testing.T) {
	tcases := []struct {
		testCase                      string
		secondConfigName              string
		powerConfigs                  *powerv1alpha1.PowerConfigList
		nodeList                      *corev1.NodeList
		powerProfiles                 *powerv1alpha1.PowerProfileList
		expectedPowerConfigName       string
		expectedNumberOfNodes         int
		expectedNumberOfPowerProfiles int
		expectedPowerProfiles         []string
	}{
		{
			testCase:         "Test Case 1",
			secondConfigName: "power-config2",
			powerConfigs: &powerv1alpha1.PowerConfigList{
				Items: []powerv1alpha1.PowerConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "power-config",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerConfigSpec{
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							PowerProfiles: []string{
								"performance",
							},
						},
						Status: powerv1alpha1.PowerConfigStatus{
							Nodes: []string{
								"example-node1",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "power-config2",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerConfigSpec{
							PowerNodeSelector: map[string]string{
								"incorrect-label": "true",
							},
							PowerProfiles: []string{
								"performance",
								"balance_performance",
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
								"example-node":    "true",
								"incorrect-label": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{
								"incorrect-label": "true",
							},
						},
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
				},
			},
			expectedPowerConfigName:       "power-config",
			expectedNumberOfNodes:         1,
			expectedNumberOfPowerProfiles: 1,
			expectedPowerProfiles: []string{
				"performance",
			},
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		objs := make([]runtime.Object, 0)
		for i := range tc.powerConfigs.Items {
			objs = append(objs, &tc.powerConfigs.Items[i])
		}
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}
		for i := range tc.powerProfiles.Items {
			objs = append(objs, &tc.powerProfiles.Items[i])
		}

		r, err := createPowerConfigReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconcile object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.secondConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling PowerConfig object", tc.testCase))
		}

		powerConfigs := &powerv1alpha1.PowerConfigList{}
		err = r.Client.List(context.TODO(), powerConfigs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerConfig list object", tc.testCase))
		}

		if len(powerConfigs.Items) != 1 {
			t.Errorf("%s - Failed: Expected number of PowerConfigs to be 1, got %v", tc.testCase, len(powerConfigs.Items))
		}

		powerConfig := &powerv1alpha1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      PowerConfigName,
			Namespace: PowerConfigNamespace,
		}, powerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerConfig object", tc.testCase))
		}

		if powerConfig.Name != tc.expectedPowerConfigName {
			t.Errorf("%s - Failed: Expected PowerConfig name to be %v, got %v", tc.testCase, tc.expectedPowerConfigName, powerConfig.Name)
		}

		if len(powerConfig.Status.Nodes) != tc.expectedNumberOfNodes {
			t.Errorf("%s - Failed: Expected number of node in PowerConfig status to be %v, got %v", tc.testCase, tc.expectedNumberOfNodes, len(powerConfig.Status.Nodes))
		}

		powerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list object", tc.testCase))
		}

		if len(powerProfiles.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles ot be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfiles.Items))
		}

		for _, profileName := range tc.expectedPowerProfiles {
			profile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      profileName,
				Namespace: PowerConfigNamespace,
			}, profile)
			if err != nil {
				if errors.IsNotFound(err) {
					t.Errorf("%s - Failed: Expected PowerProfile '%s' to exist", tc.testCase, profileName)
				} else {
					t.Error(err)
					t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile object", tc.testCase))
				}
			}
		}
	}
}

func TestPowerConfigCreation(t *testing.T) {
	tcases := []struct {
		testCase                      string
		powerConfig                   *powerv1alpha1.PowerConfig
		nodeList                      *corev1.NodeList
		expectedNumberOfNodes         int
		expectedNumberOfPowerNodes    int
		expectedPowerNodes            []string
		expectedNumberOfPowerProfiles int
		expectedPowerProfiles         []string
		expectedDaemonSetToBeCreated  bool
		expectedDaemonSetNodeSelector map[string]string
	}{
		{
			testCase: "Test Case 1",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedNumberOfNodes:      1,
			expectedNumberOfPowerNodes: 1,
			expectedPowerNodes: []string{
				"example-node1",
			},
			expectedNumberOfPowerProfiles: 1,
			expectedPowerProfiles: []string{
				"performance",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 2",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "example-node2",
							Labels: map[string]string{
								"example-node": "false",
							},
						},
					},
				},
			},
			expectedNumberOfNodes:      1,
			expectedNumberOfPowerNodes: 1,
			expectedPowerNodes: []string{
				"example-node1",
			},
			expectedNumberOfPowerProfiles: 1,
			expectedPowerProfiles: []string{
				"performance",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 3",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
			expectedNumberOfNodes:      2,
			expectedNumberOfPowerNodes: 2,
			expectedPowerNodes: []string{
				"example-node1",
				"example-node2",
			},
			expectedNumberOfPowerProfiles: 1,
			expectedPowerProfiles: []string{
				"performance",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 4",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			expectedNumberOfNodes:      1,
			expectedNumberOfPowerNodes: 1,
			expectedPowerNodes: []string{
				"example-node1",
			},
			expectedNumberOfPowerProfiles: 2,
			expectedPowerProfiles: []string{
				"performance",
				"balance-performance",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 5",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
						"balance-performance",
						"balance-power",
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
			expectedNumberOfNodes:      1,
			expectedNumberOfPowerNodes: 1,
			expectedPowerNodes: []string{
				"example-node1",
			},
			expectedNumberOfPowerProfiles: 3,
			expectedPowerProfiles: []string{
				"performance",
				"balance-performance",
				"balance-power",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 6",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
			expectedNumberOfNodes:      2,
			expectedNumberOfPowerNodes: 2,
			expectedPowerNodes: []string{
				"example-node1",
				"example-node2",
			},
			expectedNumberOfPowerProfiles: 2,
			expectedPowerProfiles: []string{
				"performance",
				"balance-performance",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 7",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
						"balance-performance",
						"balance-power",
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
			expectedNumberOfNodes:      2,
			expectedNumberOfPowerNodes: 2,
			expectedPowerNodes: []string{
				"example-node1",
				"example-node2",
			},
			expectedNumberOfPowerProfiles: 3,
			expectedPowerProfiles: []string{
				"performance",
				"balance-performance",
				"balance-power",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 8",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
			expectedNumberOfNodes:      3,
			expectedNumberOfPowerNodes: 3,
			expectedPowerNodes: []string{
				"example-node1",
				"example-node2",
				"example-node3",
			},
			expectedNumberOfPowerProfiles: 1,
			expectedPowerProfiles: []string{
				"performance",
			},
			expectedDaemonSetNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerConfig)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerConfigReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		powerConfig := &powerv1alpha1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      PowerConfigName,
			Namespace: PowerConfigNamespace,
		}, powerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerConfig object", tc.testCase))
		}

		if len(powerConfig.Status.Nodes) != tc.expectedNumberOfNodes {
			t.Errorf("%s - Failed: Expected number of nodes in PowerConfig status to be %v, got %v", tc.testCase, tc.expectedNumberOfNodes, len(powerConfig.Status.Nodes))
		}

		powerNodes := &powerv1alpha1.PowerNodeList{}
		err = r.Client.List(context.TODO(), powerNodes)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerNode list object", tc.testCase))
		}

		if len(powerNodes.Items) != tc.expectedNumberOfPowerNodes {
			t.Errorf("%s - Failed: Expected number of PowerNodes to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerNodes, len(powerNodes.Items))
		}

		for _, powerNodeName := range tc.expectedPowerNodes {
			powerNode := &powerv1alpha1.PowerNode{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerNodeName,
				Namespace: PowerConfigNamespace,
			}, powerNode)
			if err != nil {
				if errors.IsNotFound(err) {
					t.Errorf("%s - Failed: Expected PowerNode '%s' to exist", tc.testCase, powerNodeName)
				} else {
					t.Error(err)
					t.Fatal(fmt.Sprintf("%s - error retrieving PowerNode object", tc.testCase))
				}
			}
		}

		powerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list object", tc.testCase))
		}

		if len(powerProfiles.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfiles.Items))
		}

		for _, powerProfileName := range tc.expectedPowerProfiles {
			powerProfile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerProfileName,
				Namespace: PowerConfigNamespace,
			}, powerProfile)
			if err != nil {
				if errors.IsNotFound(err) {
					t.Errorf("%s - Failed: Expected PowerProfile '%s' to exist", tc.testCase, powerProfileName)
				} else {
					t.Error(err)
					t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile object", tc.testCase))
				}
			}
		}

		daemonSet := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: PowerConfigNamespace,
		}, daemonSet)
		if err != nil {
			if errors.IsNotFound(err) {
				t.Errorf("%s - Failed: Expected DaemonSet to be created", tc.testCase)
			} else {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving DaemonSet object", tc.testCase))
			}
		}

		if !reflect.DeepEqual(daemonSet.Spec.Template.Spec.NodeSelector, tc.expectedDaemonSetNodeSelector) {
			t.Errorf("%s - Failed. Expected DaemonSet NoeSelector to be %v, got %v", tc.testCase, tc.expectedDaemonSetNodeSelector, daemonSet.Spec.Template.Spec.NodeSelector)
		}
	}
}

func TestUnusedProfileRemoval(t *testing.T) {
	tcases := []struct {
		testCase                      string
		powerConfig                   *powerv1alpha1.PowerConfig
		nodeList                      *corev1.NodeList
		powerProfiles                 *powerv1alpha1.PowerProfileList
		expectedNumberOfPowerProfiles int
		expectedPowerProfileNotExists map[string]bool
	}{
		{
			testCase: "Test Case 1",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance_performance",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 1,
			expectedPowerProfileNotExists: map[string]bool{
				"performance":         false,
				"balance-performance": true,
			},
		},
		{
			testCase: "Test Case 2",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power",
							Epp:  "balance_power",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 2,
			expectedPowerProfileNotExists: map[string]bool{
				"performance":         false,
				"balance-performance": false,
				"balance-power":       true,
			},
		},
		{
			testCase: "Test Case 3",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"balance-performance",
						"balance-power",
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
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 2,
			expectedPowerProfileNotExists: map[string]bool{
				"performance":         true,
				"balance-performance": false,
				"balance-power":       false,
			},
		},
		{
			testCase: "Test Case 4",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
						"balance-performance",
						"balance-power",
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
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "power",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "power",
							Epp:  "power",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 3,
			expectedPowerProfileNotExists: map[string]bool{
				"performance":         false,
				"balance-performance": false,
				"balance-power":       false,
				"power":               true,
			},
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerConfig)
		for i := range tc.powerProfiles.Items {
			objs = append(objs, &tc.powerProfiles.Items[i])
		}

		r, err := createPowerConfigReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		powerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list object", tc.testCase))
		}

		if len(powerProfiles.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfiles.Items))
		}

		for powerProfileName, exists := range tc.expectedPowerProfileNotExists {
			profile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerProfileName,
				Namespace: PowerConfigNamespace,
			}, profile)
			if errors.IsNotFound(err) != exists {
				t.Errorf("%s - Failed: Expected PowerProfile '%s' to exist to be %v, got %v", tc.testCase, powerProfileName, exists, errors.IsNotFound(err))
			}

			if err != nil && !errors.IsNotFound(err) {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile object", tc.testCase))
			}
		}
	}
}

func TestPowerConfigCreatedProfilesAlreadyExist(t *testing.T) {
	tcases := []struct {
		testCase                      string
		powerConfig                   *powerv1alpha1.PowerConfig
		nodeList                      *corev1.NodeList
		powerProfiles                 *powerv1alpha1.PowerProfileList
		expectedNumberOfPowerProfiles int
		expectedProfiles              []string
	}{
		{
			testCase: "Test Case 1",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 1,
			expectedProfiles: []string{
				"performance",
			},
		},
		{
			testCase: "Test Case 2",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 2,
			expectedProfiles: []string{
				"performance",
				"balance-performance",
			},
		},
		{
			testCase: "Test Case 3",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance-performance",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 2,
			expectedProfiles: []string{
				"performance",
				"balance-performance",
			},
		},
		{
			testCase: "Test Case 4",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
						"balance-performance",
						"balance-power",
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
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance-performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power",
							Epp:  "balance-power",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 3,
			expectedProfiles: []string{
				"performance",
				"balance-performance",
				"balance-power",
			},
		},
		{
			testCase: "Test Case 5",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
						"balance-performance",
						"balance-power",
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
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance-performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power",
							Epp:  "balance-power",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "power",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "power",
							Epp:  "power",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles: 3,
			expectedProfiles: []string{
				"performance",
				"balance-performance",
				"balance-power",
			},
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerConfig)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}
		for i := range tc.powerProfiles.Items {
			objs = append(objs, &tc.powerProfiles.Items[i])
		}

		r, err := createPowerConfigReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		powerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list object", tc.testCase))
		}

		if len(powerProfiles.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfiles.Items))
		}

		for _, powerProfileName := range tc.expectedProfiles {
			powerProfile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerProfileName,
				Namespace: PowerConfigNamespace,
			}, powerProfile)
			if err != nil {
				if errors.IsNotFound(err) {
					t.Errorf("%s - Failed: Expected PowerProfile '%s' to exist", tc.testCase, powerProfileName)
				} else {
					t.Error(err)
					t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list object", tc.testCase))
				}
			}
		}
	}
}

func TestPowerConfigDeletion(t *testing.T) {
	tcases := []struct {
		testCase                       string
		powerConfig                    *powerv1alpha1.PowerConfig
		powerProfiles                  *powerv1alpha1.PowerProfileList
		powerWorkloads                 *powerv1alpha1.PowerWorkloadList
		powerNodes                     *powerv1alpha1.PowerNodeList
		nodeList                       *corev1.NodeList
		daemonSet                      *appsv1.DaemonSet
		expectedNumberOfPowerProfiles  int
		expectedNumberOfPowerNodes     int
		expectedNumberOfPowerWorkloads int
	}{
		{
			testCase: "Test Case 1",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
				Status: powerv1alpha1.PowerConfigStatus{
					Nodes: []string{
						"example-node1",
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "shared-example-node1",
							Max:  1500,
							Min:  1100,
							Epp:  "power",
						},
					},
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerConfigNamespace,
						},
					},
				},
			},
			powerNodes: &powerv1alpha1.PowerNodeList{
				Items: []powerv1alpha1.PowerNode{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-node1",
							Namespace: PowerConfigNamespace,
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
			daemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodeAgentDSName,
					Namespace: PowerConfigNamespace,
				},
			},
			expectedNumberOfPowerProfiles:  0,
			expectedNumberOfPowerNodes:     0,
			expectedNumberOfPowerWorkloads: 0,
		},
		{
			testCase: "Test Case 2",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
				Status: powerv1alpha1.PowerConfigStatus{
					Nodes: []string{
						"example-node1",
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "shared-example-node1",
							Max:  1500,
							Min:  1100,
							Epp:  "power",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerConfigNamespace,
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
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerConfigNamespace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerConfigNamespace,
						},
					},
				},
			},
			powerNodes: &powerv1alpha1.PowerNodeList{
				Items: []powerv1alpha1.PowerNode{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-node1",
							Namespace: PowerConfigNamespace,
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
			daemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodeAgentDSName,
					Namespace: PowerConfigNamespace,
				},
			},
			expectedNumberOfPowerProfiles:  0,
			expectedNumberOfPowerNodes:     0,
			expectedNumberOfPowerWorkloads: 0,
		},
		{
			testCase: "Test Case 3",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
						"balance-performance",
					},
				},
				Status: powerv1alpha1.PowerConfigStatus{
					Nodes: []string{
						"example-node1",
					},
				},
			},
			powerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance-performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "shared-example-node1",
							Max:  1500,
							Min:  1100,
							Epp:  "power",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Max:  3700,
							Min:  3400,
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1",
							Namespace: PowerConfigNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance-example-node1",
							Max:  3200,
							Min:  2800,
							Epp:  "balance-performance",
						},
					},
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-example-node1-workload",
							Namespace: PowerConfigNamespace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerConfigNamespace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1-workload",
							Namespace: PowerConfigNamespace,
						},
					},
				},
			},
			powerNodes: &powerv1alpha1.PowerNodeList{
				Items: []powerv1alpha1.PowerNode{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example-node1",
							Namespace: PowerConfigNamespace,
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
			daemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodeAgentDSName,
					Namespace: PowerConfigNamespace,
				},
			},
			expectedNumberOfPowerProfiles:  0,
			expectedNumberOfPowerNodes:     0,
			expectedNumberOfPowerWorkloads: 0,
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerConfig)
		for i := range tc.powerProfiles.Items {
			objs = append(objs, &tc.powerProfiles.Items[i])
		}
		for i := range tc.powerWorkloads.Items {
			objs = append(objs, &tc.powerWorkloads.Items[i])
		}
		for i := range tc.powerNodes.Items {
			objs = append(objs, &tc.powerNodes.Items[i])
		}
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}
		objs = append(objs, tc.daemonSet)

		r, err := createPowerConfigReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		err = r.Client.Delete(context.TODO(), tc.powerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error deleting PowerConfig object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		powerProfiles := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list object", tc.testCase))
		}

		if len(powerProfiles.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfiles.Items))
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

		powerNodes := &powerv1alpha1.PowerNodeList{}
		err = r.Client.List(context.TODO(), powerNodes)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerNode list objects", tc.testCase))
		}

		if len(powerNodes.Items) != tc.expectedNumberOfPowerNodes {
			t.Errorf("%s - Failed: Expected number of PowerNodes to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerNodes, len(powerNodes.Items))
		}

		daemonSet := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: PowerConfigNamespace,
		}, daemonSet)
		if err != nil {
			if !errors.IsNotFound(err) {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving DaemonSet object", tc.testCase))
			}
		} else {
			t.Errorf("%s - Failed: Expected DaemonSet object to not exist", tc.testCase)
		}
	}
}

func TestPowerConfigNodeSelectorChange(t *testing.T) {
	tcases := []struct {
		testCase        string
		powerConfig     *powerv1alpha1.PowerConfig
		nodeList        *corev1.NodeList
		newNodeSelector map[string]string
	}{
		{
			testCase: "Test Case 1",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			newNodeSelector: map[string]string{
				"new-node-selector": "true",
			},
		},
		{
			testCase: "Test Case 2",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
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
								"example-node": "true",
							},
						},
					},
				},
			},
			newNodeSelector: map[string]string{
				"new-node-selector": "true",
				"other-label":       "true",
			},
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerConfig)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerConfigReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		powerConfig := &powerv1alpha1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      PowerConfigName,
			Namespace: PowerConfigNamespace,
		}, powerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerConfig object", tc.testCase))
		}

		powerConfig.Spec.PowerNodeSelector = tc.newNodeSelector
		err = r.Client.Status().Update(context.TODO(), powerConfig)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error updating PowerConfig PowerNodeSelector", tc.testCase))
		}

		req = reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		daemonSet := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: PowerConfigNamespace,
		}, daemonSet)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving DaemonSet object", tc.testCase))
		}

		if !reflect.DeepEqual(daemonSet.Spec.Template.Spec.NodeSelector, tc.newNodeSelector) {
			t.Errorf("%s - Failed: Expected DaemonSet NodeSelector to be %v, got %v", tc.testCase, tc.newNodeSelector, daemonSet.Spec.Template.Spec.NodeSelector)
		}
	}
}

func TestPowerConfigCreationDaemonSetAlreadyExists(t *testing.T) {
	tcases := []struct {
		testCase               string
		powerConfig            *powerv1alpha1.PowerConfig
		daemonSet              *appsv1.DaemonSet
		nodeList               *corev1.NodeList
		expectedDSNodeSelector map[string]string
	}{
		{
			testCase: "Test Case 1",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			daemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodeAgentDSName,
					Namespace: PowerConfigNamespace,
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							NodeSelector: map[string]string{
								"example-node": "true",
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
						},
					},
				},
			},
			expectedDSNodeSelector: map[string]string{
				"example-node": "true",
			},
		},
		{
			testCase: "Test Case 2",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"new-node-selector": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			daemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodeAgentDSName,
					Namespace: PowerConfigNamespace,
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							NodeSelector: map[string]string{
								"example-node": "true",
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
						},
					},
				},
			},
			expectedDSNodeSelector: map[string]string{
				"new-node-selector": "true",
			},
		},
		{
			testCase: "Test Case 3",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"example-node":      "true",
						"new-node-selector": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			daemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodeAgentDSName,
					Namespace: PowerConfigNamespace,
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							NodeSelector: map[string]string{
								"example-node": "true",
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
						},
					},
				},
			},
			expectedDSNodeSelector: map[string]string{
				"example-node":      "true",
				"new-node-selector": "true",
			},
		},
		{
			testCase: "Test Case 4",
			powerConfig: &powerv1alpha1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PowerConfigName,
					Namespace: PowerConfigNamespace,
				},
				Spec: powerv1alpha1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						"sst_bf_capability": "true",
						"new-node-selector": "true",
					},
					PowerProfiles: []string{
						"performance",
					},
				},
			},
			daemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodeAgentDSName,
					Namespace: PowerConfigNamespace,
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							NodeSelector: map[string]string{
								"example-node": "true",
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
						},
					},
				},
			},
			expectedDSNodeSelector: map[string]string{
				"sst_bf_capability": "true",
				"new-node-selector": "true",
			},
		},
	}

	for _, tc := range tcases {
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		objs := make([]runtime.Object, 0)
		objs = append(objs, tc.powerConfig)
		objs = append(objs, tc.daemonSet)
		for i := range tc.nodeList.Items {
			objs = append(objs, &tc.nodeList.Items[i])
		}

		r, err := createPowerConfigReconcilerObject(objs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      PowerConfigName,
				Namespace: PowerConfigNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Errorf("Error: %v", err)
			t.Fatal("error reconciling PowerConfig object")
		}

		daemonSet := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: PowerConfigNamespace,
		}, daemonSet)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving DaemonSet object", tc.testCase))
		}

		if !reflect.DeepEqual(daemonSet.Spec.Template.Spec.NodeSelector, tc.expectedDSNodeSelector) {
			t.Errorf("%s - Failed: Expected DaemonSet NodeSelector to be %v, got %v", tc.testCase, tc.expectedDSNodeSelector, daemonSet.Spec.Template.Spec.NodeSelector)
		}
	}
}
