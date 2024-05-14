package controllers

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createConfigReconcilerObject(objs []client.Object) (*PowerConfigReconciler, error) {
	log.SetLogger(zap.New(
		zap.UseDevMode(true),
		func(opts *zap.Options) {
			opts.TimeEncoder = zapcore.ISO8601TimeEncoder
		},
	),
	)
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}
	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).WithStatusSubresource(objs...).Build()

	state := state.NewPowerNodeData()

	r := &PowerConfigReconciler{cl, ctrl.Log.WithName("testing"), s, state}

	return r, nil
}

func TestPowerConfig_Reconcile_Creation(t *testing.T) {
	tcases := []struct {
		testCase                 string
		nodeName                 string
		configName               string
		clientObjs               []client.Object
		profileNames             []string
		expectedNumberOfProfiles int
		expectedCustomDevices    []string
	}{
		{
			testCase:   "Test Case 1 - profiles: performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
						CustomDevices: []string{"device-plugin"},
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
			expectedCustomDevices:    []string{"device-plugin"},
		},
		{
			testCase:   "Test Case 2 - profiles: performance, balance-performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
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
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
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
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
				Namespace: IntelPowerNamespace,
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
				t.Errorf("%s failed: expected power profile '%s' to have been created", tc.testCase, profile)
			}
		}

		profiles := &powerv1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power profile objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("%s failed: expected number of power profile objects to be %v, got %v", tc.testCase, tc.expectedNumberOfProfiles, len(profiles.Items))
		}

		ds := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: IntelPowerNamespace,
		}, ds)
		if err != nil {
			t.Errorf("%s failed: expected daemonSet '%s' to have been created", tc.testCase, NodeAgentDSName)
		}
	}
}

func TestPowerConfig_Reconcile_Exists(t *testing.T) {
	tcases := []struct {
		testCase                 string
		nodeName                 string
		configName               string
		clientObjs               []client.Object
		expectedNumberOfProfiles int
		expectedNumberOfConfigs  int
	}{
		{
			testCase:   "Test Case 1 - profiles: performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: IntelPowerNamespace,
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
			Namespace: IntelPowerNamespace,
		}, config)
		if err == nil {
			t.Errorf("%s failed: expected power config object '%s' to have been deleted", tc.testCase, tc.configName)
		}

		configs := &powerv1.PowerConfigList{}
		err = r.Client.List(context.TODO(), configs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power config objects", tc.testCase)
		}

		if len(configs.Items) != tc.expectedNumberOfConfigs {
			t.Errorf("%s failed: expected number of power config objects to be %v, got %v", tc.testCase, tc.expectedNumberOfConfigs, len(configs.Items))
		}

		profiles := &powerv1.PowerProfileList{}
		err = r.Client.List(context.TODO(), profiles)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power profile objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("%s failed: expected number of power profile objects to be %v, got %v", tc.testCase, tc.expectedNumberOfProfiles, len(profiles.Items))
		}
	}
}

func TestPowerConfig_Reconcile_CustomDevices_Creation(t *testing.T) {
	tcases := []struct {
		testCase                string
		nodeName                string
		configName              string
		clientObjs              []client.Object
		expectedNumberOfConfigs int
		expectedCustomDevices   []string
	}{
		{
			testCase:   "custom devices creation - items in list",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
						CustomDevices: []string{"device-plugin"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
			expectedCustomDevices: []string{"device-plugin"},
		},
		{
			testCase:   "custom devices creation - empty array",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
						CustomDevices: []string{},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
			expectedCustomDevices: nil,
		},
		{
			testCase:   "custom devices creation - no devices",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
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
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
			expectedCustomDevices: nil,
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createConfigReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		config := &powerv1.PowerConfig{} // needs error handling
		_ = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.configName,
			Namespace: IntelPowerNamespace,
		}, config)
		if config.Name != tc.configName {
			t.Errorf("%s failed: expected power config object '%s' to have been created successfully", tc.testCase, tc.configName)
		}

		powerNode := &powerv1.PowerNode{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.nodeName,
			Namespace: IntelPowerNamespace,
		}, powerNode)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power node", tc.testCase)
		}

		if !reflect.DeepEqual(config.Spec.CustomDevices, tc.expectedCustomDevices) {
			t.Errorf("%s failed: expected customDevices for the power config object to be %v, got %v", tc.testCase, config.Spec.CustomDevices, tc.expectedCustomDevices)
		}
		if !reflect.DeepEqual(powerNode.Spec.CustomDevices, tc.expectedCustomDevices) {
			t.Errorf("%s failed: expected customDevices for the power node object to be %v, got %v", tc.testCase, powerNode.Spec.CustomDevices, tc.expectedCustomDevices)
		}
	}
}

func TestPowerConfig_Reconcile_CustomDevices_Update(t *testing.T) {
	tcases := []struct {
		testCase              string
		nodeName              string
		configName            string
		clientObjs            []client.Object
		expectedCustomDevices []string
		updatedCustomDevices  []string
	}{
		{
			testCase:   "custom devices - update with []string",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
						CustomDevices: []string{"device-plugin"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
			expectedCustomDevices: []string{"device-plugin-1", "device-plugin-2", "device-plugin-3"},
			updatedCustomDevices:  []string{"device-plugin-1", "device-plugin-2", "device-plugin-3"},
		},
		{
			testCase:   "custom devices update - update with empty []string",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
						CustomDevices: []string{"device-plugin-1", "device-plugin-2"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
			expectedCustomDevices: nil,
			updatedCustomDevices:  []string{},
		},
		{
			testCase:   "custom devices update - empty customDevices, update with []string",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerConfigSpec{
						PowerNodeSelector: map[string]string{
							"feature.node.kubernetes.io/power-node": "true",
						},
						PowerProfiles: []string{
							"performance",
						},
						CustomDevices: []string{},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
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
			expectedCustomDevices: []string{"device-plugin-3", "device-plugin-4"},
			updatedCustomDevices:  []string{"device-plugin-3", "device-plugin-4"},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"

		r, err := createConfigReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		config := &powerv1.PowerConfig{} // needs error handling
		_ = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.configName,
			Namespace: IntelPowerNamespace,
		}, config)
		if config.Name != tc.configName {
			t.Errorf("%s failed: expected power config object '%s' to have been created successfully", tc.testCase, tc.configName)
		}

		// try to mimic a kubectl apply with updates to the current powerconfig
		config.Spec.CustomDevices = tc.updatedCustomDevices
		err = r.Client.Update(context.TODO(), config)
		_ = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.configName,
			Namespace: IntelPowerNamespace,
		}, config)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error updating config object", tc.testCase)
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		config = &powerv1.PowerConfig{} // needs error handling
		_ = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.configName,
			Namespace: IntelPowerNamespace,
		}, config)
		if config.Name != tc.configName {
			t.Errorf("%s failed: expected power config object '%s' to have been created successfully", tc.testCase, tc.configName)
		}

		powerNode := &powerv1.PowerNode{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.nodeName,
			Namespace: IntelPowerNamespace,
		}, powerNode)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power node", tc.testCase)
		}

		if !reflect.DeepEqual(config.Spec.CustomDevices, tc.expectedCustomDevices) {
			t.Errorf("%s failed: expected customDevices for the power config object to be %v, got %v", tc.testCase, config.Spec.CustomDevices, tc.expectedCustomDevices)
		}
		if !reflect.DeepEqual(powerNode.Spec.CustomDevices, tc.expectedCustomDevices) {
			t.Errorf("%s failed: expected customDevices for the power node object to be %v, got %v", tc.testCase, powerNode.Spec.CustomDevices, tc.expectedCustomDevices)
		}
	}
}

func TestPowerConfig_Reconcile_ProfilesNoLongerRequested(t *testing.T) {
	tcases := []struct {
		testCase                 string
		nodeName                 string
		configName               string
		clientObjs               []client.Object
		profilesDeleted          []string
		profileNames             []string
		expectedNumberOfProfiles int
	}{
		{
			testCase:   "Test Case 1 - profiles to remove: performance",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
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
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
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
			clientObjs: []client.Object{
				&powerv1.PowerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error retrieving the power profile objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("%s failed: expected number of power profile objects is %v, got %v", tc.testCase, tc.expectedNumberOfProfiles, len(profiles.Items))
		}

		for _, profile := range tc.profilesDeleted {
			deletedProfile := &powerv1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      profile,
				Namespace: IntelPowerNamespace,
			}, deletedProfile)
			if err == nil {
				t.Errorf("%s failed: expected power profile object '%s' to have been deleted", tc.testCase, profile)
			}
		}

		for _, profile := range tc.profileNames {
			createdProfile := &powerv1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      profile,
				Namespace: IntelPowerNamespace,
			}, createdProfile)
			if err != nil {
				t.Errorf("%s failed: expected power profile object '%s' to have been created", tc.testCase, profile)
			}
		}
	}
}

func TestPowerConfig_Reconcile_Deletion(t *testing.T) {
	tcases := []struct {
		testCase                string
		nodeName                string
		configName              string
		clientObjs              []client.Object
		expectedNumberOfObjects int
	}{
		{
			testCase:   "Test Case 1",
			nodeName:   "TestNode",
			configName: "test-config",
			clientObjs: []client.Object{
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
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.configName,
				Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error retrieving the power profile objects", tc.testCase)
		}

		if len(profiles.Items) != tc.expectedNumberOfObjects {
			t.Errorf("%s failed: expected number of power profile objects is %v, got %v", tc.testCase, tc.expectedNumberOfObjects, len(profiles.Items))
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power workload objects", tc.testCase)
		}

		if len(workloads.Items) != tc.expectedNumberOfObjects {
			t.Errorf("%s failed: expected number of power workload objects is %v, got %v", tc.testCase, tc.expectedNumberOfObjects, len(workloads.Items))
		}

		powerNodes := &powerv1.PowerNodeList{}
		err = r.Client.List(context.TODO(), powerNodes)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving power node objects", tc.testCase)
		}

		if len(powerNodes.Items) != tc.expectedNumberOfObjects {
			t.Errorf("%s failed: expected number of power node objects is %v, got %v", tc.testCase, tc.expectedNumberOfObjects, len(powerNodes.Items))
		}

		ds := &appsv1.DaemonSet{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      NodeAgentDSName,
			Namespace: IntelPowerNamespace,
		}, ds)
		if err == nil {
			t.Errorf("%s failed: expected daemonSet '%s' to have been deleted", tc.testCase, NodeAgentDSName)
		}
	}
}

// go test -fuzz FuzzPowerConfigController -run=FuzzPowerConfigController
func FuzzPowerConfigController(f *testing.F) {
	f.Add("sample-config", "performance", "feature.node.kubernetes.io/power-node", "device-plugin")
	f.Fuzz(func(t *testing.T, name string, prof string, label string, devicePlugin string) {
		clientObjs := []client.Object{
			&powerv1.PowerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerConfigSpec{
					PowerNodeSelector: map[string]string{
						label: "true",
					},
					PowerProfiles: []string{
						prof,
					},
					CustomDevices: []string{devicePlugin},
				},
			},
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestNode",
					Labels: map[string]string{
						label: "true",
					},
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
		}
		NodeAgentDaemonSetPath = "../build/manifests/power-node-agent-ds.yaml"
		r, err := createConfigReconcilerObject(clientObjs)
		assert.Nil(t, err)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      "test-config",
				Namespace: IntelPowerNamespace,
			},
		}

		r.Reconcile(context.TODO(), req)

	})
}

// tests positive and negative cases for SetupWithManager function
func TestPowerConfig_Reconcile_SetupPass(t *testing.T) {
	r, err := createConfigReconcilerObject([]client.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(cacheMk))
	err = (&PowerConfigReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}
func TestPowerConfig_Reconcile_SetupFail(t *testing.T) {
	r, err := createConfigReconcilerObject([]client.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("setup fail"))

	err = (&PowerConfigReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
