package controllers

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/state"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createConfigReconcilerObject(objs []runtime.Object) (*PowerConfigReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	state := state.NewPowerNodeData()

	r := &PowerConfigReconciler{cl, ctrl.Log.WithName("testing"), s, state}

	return r, nil
}

func TestPowerConfigCreation(t *testing.T) {
	tcases := []struct {
		testCase                 string
		nodeName                 string
		configName               string
		clientObjs               []runtime.Object
		profileNames             []string
		expectedNumberOfProfiles int
	}{
		{
			testCase:   "Test Case 1 - profiles: performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
						Labels: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
			profileNames: []string{
				"performance",
			},
			expectedNumberOfProfiles: 1,
		},
		{
			testCase:   "Test Case 2 - profiles: performance, balance-performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
							"balance-performance",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
						Labels: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
			profileNames: []string{
				"performance",
				"balance-performance",
			},
			expectedNumberOfProfiles: 2,
		},
		{
			testCase:   "Test Case 3 - profiles: performance, balance-performance, balance-power",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
							"balance-performance",
							"balance-power",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
						Labels: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
			profileNames: []string{
				"performance",
				"balance-performance",
				"balance-power",
			},
			expectedNumberOfProfiles: 3,
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createConfigReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		for _, profile := range tc.profileNames {
			p := &powerv1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      profile,
				Namespace: IntelPowerNamespace,
			}, p)
			if err != nil {
				t.Errorf("%s Failed - Expected Power Profile '%s' to have been created", tc.testCase, profile)
			}
		}

		profiles := &powerv1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Profile Objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("%s Failed - Expected number of Power Profile Objects to be %v, got %v", tc.testCase, tc.expectedNumberOfProfiles, len(profiles.Items))
		}

		ds := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: IntelPowerNamespace,
		}, ds)
		if err != nil {
			t.Errorf("%s Failed - Expected DaemonSet '%s' to have been created", tc.testCase, NodeAgentDSName)
		}
	}
}

func TestPowerConfigExists(t *testing.T) {
	tcases := []struct {
		testCase                 string
		nodeName                 string
		configName               string
		clientObjs               []runtime.Object
		expectedNumberOfProfiles int
		expectedNumberOfConfigs  int
	}{
		{
			testCase:   "Test Case 1 - profiles: performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
					},
				},
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config2",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
					},
				},
			},
			expectedNumberOfProfiles: 0,
			expectedNumberOfConfigs:  1,
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createConfigReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		config := &powerv1.PowerConfig{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.configName,
			Namespace: "default",
		}, config)
		if err == nil {
			t.Errorf("%s Failed - Expected Power Config Object '%s' to have been deleted", tc.testCase, tc.configName)
		}

		configs := &powerv1.PowerConfigList{}
		err = r.Client.List(context.TODO(), configs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Config Objects", tc.testCase)
		}

		if len(configs.Items) != tc.expectedNumberOfConfigs {
			t.Errorf("%s Failed - Expected number of Power Config Objects to be %v, got %v", tc.testCase, tc.expectedNumberOfConfigs, len(configs.Items))
		}

		profiles := &powerv1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Profile Objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("%s Failed - Expected number of Power Profile Objects to be %v, got %v", tc.testCase, tc.expectedNumberOfProfiles, len(profiles.Items))
		}
	}
}

func TestProfilesNoLongerRequested(t *testing.T) {
	tcases := []struct {
		testCase                 string
		nodeName                 string
		configName               string
		clientObjs               []runtime.Object
		profilesDeleted          []string
		profileNames             []string
		expectedNumberOfProfiles int
	}{
		{
			testCase:   "Test Case 1 - profiles to remove: performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"balance-performance",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
						Labels: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
						Max:  3600,
						Min:  3200,
						Epp:  "performance",
					},
				},
			},
			profilesDeleted: []string{
				"performance",
			},
			profileNames: []string{
				"balance-performance",
			},
			expectedNumberOfProfiles: 1,
		},
		{
			testCase:   "Test Case 2 - profiles to remove: balance-performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
						Labels: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "balance-performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "balance-performance",
						Max:  3600,
						Min:  3200,
						Epp:  "balance-performance",
					},
				},
			},
			profilesDeleted: []string{
				"balance-performance",
			},
			profileNames: []string{
				"performance",
			},
			expectedNumberOfProfiles: 1,
		},
		{
			testCase:   "Test Case 2 - profiles to remove: balance-performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "default",
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
						Labels: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "balance-performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "balance-performance",
						Max:  3600,
						Min:  3200,
						Epp:  "balance-performance",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "balance-power",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "balance-power",
						Max:  3600,
						Min:  3200,
						Epp:  "balance-power",
					},
				},
			},
			profilesDeleted: []string{
				"balance-performance",
				"balance-power",
			},
			profileNames: []string{
				"performance",
			},
			expectedNumberOfProfiles: 1,
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createConfigReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		profiles := &powerv1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Profile Objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("%s Failed - Expected number of Power Profile Objects to be %v, got %v", tc.testCase, tc.expectedNumberOfProfiles, len(profiles.Items))
		}

		for _, profile := range tc.profilesDeleted {
			deletedProfile := &powerv1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      profile,
				Namespace: IntelPowerNamespace,
			}, deletedProfile)
			if err == nil {
				t.Errorf("%s Failed - Expected Power Profile Object '%s' to have been deleted", tc.testCase, profile)
			}
		}

		for _, profile := range tc.profileNames {
			createdProfile := &powerv1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      profile,
				Namespace: IntelPowerNamespace,
			}, createdProfile)
			if err != nil {
				t.Errorf("%s Failed - Expected Power Profile Object '%s' to have been created", tc.testCase, profile)
			}
		}
	}
}

func TestPowerConfigDeletion(t *testing.T) {
	tcases := []struct {
		testCase                string
		nodeName                string
		configName              string
		clientObjs              []runtime.Object
		expectedNumberOfObjects int
	}{
		{
			testCase:   "Test Case 1",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
						Labels: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
						Max:  3600,
						Min:  3200,
						Epp:  "performance",
					},
				},
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{},
				},
				&powerv1.PowerNode{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerNodeSpec{},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      NodeAgentDSName,
						Namespace: IntelPowerNamespace,
					},
				},
			},
			expectedNumberOfObjects: 0,
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createConfigReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		profiles := &powerv1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Profile Objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfObjects {
			t.Errorf("%s Failed - Expected number of Power Profile Objects to be %v, got %v", tc.testCase, tc.expectedNumberOfObjects, len(profiles.Items))
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Objects", tc.testCase)
		}

		if len(workloads.Items) != tc.expectedNumberOfObjects {
			t.Errorf("%s Failed - Expected number of Power Workload Objects to be %v, got %v", tc.testCase, tc.expectedNumberOfObjects, len(workloads.Items))
		}

		powerNodes := &powerv1.PowerNodeList{}
		err = r.Client.List(context.TODO(), powerNodes)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Node Objects", tc.testCase)
		}

		if len(powerNodes.Items) != tc.expectedNumberOfObjects {
			t.Errorf("%s Failed - Expected number of Power Node Objects to be %v, got %v", tc.testCase, tc.expectedNumberOfObjects, len(powerNodes.Items))
		}

		ds := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: IntelPowerNamespace,
		}, ds)
		if err == nil {
			t.Errorf("%s Failed - Expected DaemonSet '%s' to have been deleted", tc.testCase, NodeAgentDSName)
		}
	}
}
