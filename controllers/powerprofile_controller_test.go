package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PowerProfileName      = "TestPowerProfile"
	PowerProfileNamespace = "default"
)

func createPowerProfileReconcileObject(powerProfile *powerv1alpha1.PowerProfile) (*PowerProfileReconciler, error) {
	s := scheme.Scheme

	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	objs := []runtime.Object{powerProfile}

	s.AddKnownTypes(powerv1alpha1.GroupVersion)

	cl := fake.NewFakeClient(objs...)

	appqosCl := appqos.NewDefaultAppQoSClient()

	r := &PowerProfileReconciler{Client: cl, Log: ctrl.Log.WithName("controllers").WithName("PowerProfile"), Scheme: s, AppQoSClient: appqosCl}

	return r, nil
}

func createPowerProfileListeners(appqosPowerProfiles []appqos.PowerProfile) (*httptest.Server, error) {
	var err error

	newListener, err := net.Listen("tcp", "127.0.0.1:5000")
	if err != nil {
		return nil, fmt.Errorf("Failed to create Listerner: %v", err)
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

func TestBaseProfileCreation(t *testing.T) {
	tcases := []struct {
		testCase                     string
		basePowerProfile             *powerv1alpha1.PowerProfile
		node                         *corev1.Node
		expectedNumberOfProfiles     int
		expectedExtendedProfile      string
		expectedPowerProfileTests    map[string]map[string]bool
		expectedPowerProfileEppValue string
	}{
		{
			testCase: "Test Case 1",
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance",
					Epp:  "performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			expectedNumberOfProfiles: 2,
			expectedExtendedProfile:  "performance-example-node1",
			expectedPowerProfileTests: map[string]map[string]bool{
				"performance": map[string]bool{
					"extendedResourcesCreated": true,
					"maxMinValuesToEqualZero":  true,
				},
				"performance-example-node1": map[string]bool{
					"extendedResourcesCreated": true,
					"maxMinValuesToEqualZero":  false,
				},
			},
			expectedPowerProfileEppValue: "performance",
		},
		{
			testCase: "Test Case 2",
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-performance",
					Epp:  "balance_performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			expectedNumberOfProfiles: 2,
			expectedExtendedProfile:  "balance-performance-example-node1",
			expectedPowerProfileTests: map[string]map[string]bool{
				"balance-performance": map[string]bool{
					"extendedResourcesCreated": true,
					"maxMinValuesToEqualZero":  true,
				},
				"balance-performance-example-node1": map[string]bool{
					"extendedResourcesCreated": true,
					"maxMinValuesToEqualZero":  false,
				},
			},
			expectedPowerProfileEppValue: "balance_performance",
		},
		{
			testCase: "Test Case 3",
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-power",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-power",
					Epp:  "balance_power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			expectedNumberOfProfiles: 2,
			expectedExtendedProfile:  "balance-power-example-node1",
			expectedPowerProfileTests: map[string]map[string]bool{
				"balance-power": map[string]bool{
					"extendedResourcesCreated": true,
					"maxMinValuesToEqualZero":  true,
				},
				"balance-power-example-node1": map[string]bool{
					"extendedResourcesCreated": true,
					"maxMinValuesToEqualZero":  false,
				},
			},
			expectedPowerProfileEppValue: "balance_power",
		},
		{
			testCase: "Test Case 4",
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "power",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "power",
					Max:  1500,
					Min:  1100,
					Epp:  "power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			expectedNumberOfProfiles: 1,
			expectedPowerProfileTests: map[string]map[string]bool{
				"power": map[string]bool{
					"extendedResourcesCreated": true,
					"maxMinValuesToEqualZero":  false,
				},
			},
			expectedPowerProfileEppValue: "power",
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.basePowerProfile)
		if err != nil {
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listener", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Node object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.basePowerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		server.Close()

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.node.Name,
		}, updatedNode)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Node object", tc.testCase))
		}

		powerProfileList := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfileList)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list", tc.testCase))
		}

		if len(powerProfileList.Items) != tc.expectedNumberOfProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfProfiles, len(powerProfileList.Items))
		}

		for powerProfileName, expectedTests := range tc.expectedPowerProfileTests {
			powerProfile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerProfileName,
				Namespace: PowerProfileNamespace,
			}, powerProfile)
			if err != nil {
				if errors.IsNotFound(err) {
					t.Errorf("%s - Failed: Expected PowerProfile '%s' to exist", tc.testCase, powerProfileName)
				} else {
					t.Error(err)
					t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile object", tc.testCase))
				}
			}

			resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, powerProfileName))
			if _, exists := updatedNode.Status.Capacity[resourceName]; exists != expectedTests["extendedResourcesCreated"] {
				t.Errorf("%s - Failed: Expected '%s' extended resources to be created to be %v, got %v", tc.testCase, powerProfileName, expectedTests["extendedResourcesCreated"], exists)
			}

			if (powerProfile.Spec.Max == 0) != expectedTests["maxMinValuesToEqualZero"] {
				t.Errorf("%s - Failed: Expected PowerProfile '%s' Max value to be 0 to be %v, got %v", tc.testCase, powerProfileName, expectedTests["maxMinValuesToEqualZero"], (powerProfile.Spec.Max == 0))
			}

			if powerProfile.Spec.Epp != tc.expectedPowerProfileEppValue {
				t.Errorf("%s - Failed: Expected PowerProfile '%s' Epp value to be %v, got %v", tc.testCase, powerProfileName, tc.expectedPowerProfileEppValue, powerProfile.Spec.Epp)
			}
		}
	}
}

func TestExtendedNonPowerEppProfileCreation(t *testing.T) {
	tcases := []struct {
		testCase                             string
		extendedPowerProfile                 *powerv1alpha1.PowerProfile
		node                                 *corev1.Node
		otherPowerProfiles                   *powerv1alpha1.PowerProfileList
		expectedExtendedResourcesToBeCreated bool
	}{
		{
			testCase: "Test Case 1",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 2",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node2",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node2",
							Epp:  "performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 3",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node2",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node2",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance_performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance-example-node1",
							Epp:  "balance_performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 3",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-performance-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "balance_performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance_performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 4",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-performance-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "balance_performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance_performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node2",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance-example-node2",
							Epp:  "balance_performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 5",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-performance-example-node1",
					Max:  3200,
					Min:  2800,
					Epp:  "balance_performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance_performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node2",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance-example-node2",
							Epp:  "balance_performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Epp:  "performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 6",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-power-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-power-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "balance_power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power",
							Epp:  "balance_power",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 7",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-power-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-power-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "balance_performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power",
							Epp:  "balance_power",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power-example-node2",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power-example-node2",
							Epp:  "balance_power",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 8",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-power-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-power",
					Max:  3200,
					Min:  2800,
					Epp:  "balance_power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power",
							Epp:  "balance_power",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-power-example-node2",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power-example-node2",
							Epp:  "balance_power",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Epp:  "performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: false,
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.extendedPowerProfile)
		if err != nil {
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listener", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Node object", tc.testCase))
		}

		for _, profile := range tc.otherPowerProfiles.Items {
			err = r.Client.Create(context.TODO(), &profile)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error creating PowerProfile object '%s'", tc.testCase, profile.Name))
			}
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.extendedPowerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		server.Close()

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.node.Name,
		}, updatedNode)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Node object", tc.testCase))
		}

		resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, tc.extendedPowerProfile.Name))
		if _, exists := updatedNode.Status.Capacity[resourceName]; exists != tc.expectedExtendedResourcesToBeCreated {
			t.Errorf("%s - Failed: Expected extended resources to be created to be %v, got %v", tc.testCase, tc.expectedExtendedResourcesToBeCreated, exists)
		}

		powerProfileList := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfileList)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfileList", tc.testCase))
		}

		if len(powerProfileList.Items) != len(tc.otherPowerProfiles.Items)+1 {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, len(tc.otherPowerProfiles.Items)+1, len(powerProfileList.Items))
		}
	}
}

func TestExtendedPowerEppProfileCreation(t *testing.T) {
	tcases := []struct {
		testCase                             string
		extendedPowerProfile                 *powerv1alpha1.PowerProfile
		node                                 *corev1.Node
		otherPowerProfiles                   *powerv1alpha1.PowerProfileList
		expectedExtendedResourcesToBeCreated bool
		expectedMaxFrequencyValue            int
		expectedMinFrequencyValue            int
		expectedNumberOfPowerProfiles        int
	}{
		{
			testCase: "Test Case 1",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "shared",
					Max:  1500,
					Min:  1100,
					Epp:  "power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{},
			},
			expectedExtendedResourcesToBeCreated: true,
			expectedMaxFrequencyValue:            1500,
			expectedMinFrequencyValue:            1100,
			expectedNumberOfPowerProfiles:        1,
		},
		{
			testCase: "Test Case 2",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "shared",
					Max:  1500,
					Min:  1100,
					Epp:  "power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: true,
			expectedMaxFrequencyValue:            1500,
			expectedMinFrequencyValue:            1100,
			expectedNumberOfPowerProfiles:        2,
		},
		{
			testCase: "Test Case 3",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "shared",
					Max:  1500,
					Min:  1100,
					Epp:  "power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerProfileNamespace,
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
			expectedExtendedResourcesToBeCreated: true,
			expectedMaxFrequencyValue:            1500,
			expectedMinFrequencyValue:            1100,
			expectedNumberOfPowerProfiles:        3,
		},
		{
			testCase: "Test Case 4",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "shared",
					Max:  1500,
					Min:  1100,
					Epp:  "power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerProfileNamespace,
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
							Name:      "balance-performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp:  "balance_performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "balance-performance-example-node1",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance-example-node1",
							Epp:  "balance_performance",
						},
					},
				},
			},
			expectedExtendedResourcesToBeCreated: true,
			expectedMaxFrequencyValue:            1500,
			expectedMinFrequencyValue:            1100,
			expectedNumberOfPowerProfiles:        5,
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.extendedPowerProfile)
		if err != nil {
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listener", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Node object", tc.testCase))
		}

		for _, profile := range tc.otherPowerProfiles.Items {
			err = r.Client.Create(context.TODO(), &profile)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error creating PowerProfile object '%s'", tc.testCase, profile.Name))
			}
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.extendedPowerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		server.Close()

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.node.Name,
		}, updatedNode)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Node object", tc.testCase))
		}

		resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, tc.extendedPowerProfile.Name))
		if _, exists := updatedNode.Status.Capacity[resourceName]; exists != tc.expectedExtendedResourcesToBeCreated {
			t.Errorf("%s - Failed: Expected extended resources to be created to be %v, got %v", tc.testCase, tc.expectedExtendedResourcesToBeCreated, exists)
		}

		updatedPowerProfile := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.extendedPowerProfile.Name,
			Namespace: PowerProfileNamespace,
		}, updatedPowerProfile)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile object", tc.testCase))
		}

		if updatedPowerProfile.Spec.Max != tc.expectedMaxFrequencyValue {
			t.Errorf("%s - Failed: Expected max frequency value to be %v, got %v", tc.testCase, tc.expectedMaxFrequencyValue, tc.expectedMaxFrequencyValue)
		}

		if updatedPowerProfile.Spec.Min != tc.expectedMinFrequencyValue {
			t.Errorf("%s - Failed: Expected min frequency value to be %v, got %v", tc.testCase, tc.expectedMinFrequencyValue, updatedPowerProfile.Spec.Min)
		}

		powerProfileList := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfileList)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list", tc.testCase))
		}

		if len(powerProfileList.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfileList.Items))
		}
	}
}

func TestIncorrectEppValue(t *testing.T) {
	tcases := []struct {
		testCase                             string
		powerProfile                         *powerv1alpha1.PowerProfile
		node                                 *corev1.Node
		otherPowerProfiles                   *powerv1alpha1.PowerProfileList
		expectedNumberOfPowerProfiles        int
		expectedExtendedResourcesToBeCreated bool
	}{
		{
			testCase: "Test Case 1",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance",
					Epp:  "incorrect",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{},
			},
			expectedNumberOfPowerProfiles:        0,
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 2",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance",
					Epp:  "incorrect",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node1",
							Epp:  "performance",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles:        1,
			expectedExtendedResourcesToBeCreated: false,
		},
		{
			testCase: "Test Case 3",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance-example-node1",
					Epp:  "performance-example-node1",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
			},
			otherPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance",
							Epp:  "performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node2",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "performance-example-node2",
							Epp:  "performance",
						},
					},
				},
			},
			expectedNumberOfPowerProfiles:        2,
			expectedExtendedResourcesToBeCreated: false,
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.powerProfile)
		if err != nil {
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listener", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Node object", tc.testCase))
		}

		for _, profile := range tc.otherPowerProfiles.Items {
			err = r.Client.Create(context.TODO(), &profile)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error creating PowerProfile object '%s'", tc.testCase, profile.Name))
			}
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.powerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		server.Close()

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.node.Name,
		}, updatedNode)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Node object", tc.testCase))
		}

		resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, tc.powerProfile.Name))
		if _, exists := updatedNode.Status.Capacity[resourceName]; exists != tc.expectedExtendedResourcesToBeCreated {
			t.Errorf("%s - Failed: Expected extended resources to be created to be %v, got %v", tc.testCase, tc.expectedExtendedResourcesToBeCreated, exists)
		}

		powerProfileList := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfileList)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list", tc.testCase))
		}

		if len(powerProfileList.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfileList.Items))
		}

		powerProfile := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.powerProfile.Name,
			Namespace: PowerProfileNamespace,
		}, powerProfile)
		if !errors.IsNotFound(err) {
			t.Errorf("%s - Failed: Expected PowerProfile '%s' to not exist", tc.testCase, tc.powerProfile.Name)
		}
	}
}

func TestExtendedPowerProfileDeletion(t *testing.T) {
	tcases := []struct {
		testCase                         string
		extendedPowerProfile             *powerv1alpha1.PowerProfile
		basePowerProfile                 *powerv1alpha1.PowerProfile
		node                             *corev1.Node
		powerWorkload                    *powerv1alpha1.PowerWorkload
		expectedExtendedResourcesToExist map[string]bool
	}{
		{
			testCase: "Test Case 1",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "performance",
				},
			},
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance",
					Epp:  "performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						"cpu:":                        *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/performance": *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/performance-example-node1": *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance-example-node1-workload",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "performance-example-node1-workload",
					PowerNodeSelector: map[string]string{
						"exmple-node": "true",
					},
					Node: powerv1alpha1.NodeInfo{
						Name: "exmple-node1",
					},
					PowerProfile: "performance-example-node1",
				},
			},
			expectedExtendedResourcesToExist: map[string]bool{
				"performance":               true,
				"performance-example-node1": false,
			},
		},
		{
			testCase: "Test Case 2",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-performance-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-performance-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "balance_performance",
				},
			},
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-performance",
					Epp:  "balance_performance",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						"cpu:":                                *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/balance-performance": *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/balance-performance-example-node1": *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-performance-example-node1-workload",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "balance-performance-example-node1-workload",
					PowerNodeSelector: map[string]string{
						"exmple-node": "true",
					},
					Node: powerv1alpha1.NodeInfo{
						Name: "exmple-node1",
					},
					PowerProfile: "balance-performance-example-node1",
				},
			},
			expectedExtendedResourcesToExist: map[string]bool{
				"balance-performance":               true,
				"balance-performance-example-node1": false,
			},
		},
		{
			testCase: "Test Case 3",
			extendedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-power-example-node1",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-power-example-node1",
					Max:  3700,
					Min:  3400,
					Epp:  "balance_power",
				},
			},
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-power",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "balance-power",
					Epp:  "balance-power",
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						"cpu:":                          *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/balance-power": *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/balance-power-example-node1": *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "balance-power-example-node1-workload",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					Name: "balance-power-example-node1-workload",
					PowerNodeSelector: map[string]string{
						"exmple-node": "true",
					},
					Node: powerv1alpha1.NodeInfo{
						Name: "exmple-node1",
					},
					PowerProfile: "balance-power-example-node1",
				},
			},
			expectedExtendedResourcesToExist: map[string]bool{
				"balance-power":               true,
				"balance-power-example-node1": false,
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.extendedPowerProfile)
		if err != nil {
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listener", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Node object", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.basePowerProfile)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating base PowerProfile object", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.powerWorkload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating PowerWorkload object", tc.testCase))
		}

		err = r.Client.Delete(context.TODO(), tc.extendedPowerProfile)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error deleting PowerProfile object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.extendedPowerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		server.Close()

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.node.Name,
		}, updatedNode)

		for powerProfileName, shouldExist := range tc.expectedExtendedResourcesToExist {
			resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, powerProfileName))
			if _, exists := updatedNode.Status.Capacity[resourceName]; exists != shouldExist {
				t.Errorf("%s - Failed: Expected extended resource '%s' to exist to be %v, got %v", tc.testCase, resourceName, shouldExist, exists)
			}
		}

		extendedPowerProfile := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.extendedPowerProfile.Name,
			Namespace: PowerProfileNamespace,
		}, extendedPowerProfile)
		if !errors.IsNotFound(err) {
			t.Errorf("%s - Failed: Expected exteended PowerProfile '%s' to not exist", tc.testCase, tc.extendedPowerProfile.Name)
		}

		basePowerProfile := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.basePowerProfile.Name,
			Namespace: PowerProfileNamespace,
		}, basePowerProfile)
		if err != nil {
			if errors.IsNotFound(err) {
				t.Errorf("%s - Failed: Expected base PowerProfile '%s' to exist", tc.testCase, tc.basePowerProfile.Name)
			} else {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving base PoewrProfile object '%s'", tc.testCase, tc.basePowerProfile.Name))
			}
		}

		powerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.powerWorkload.Name,
			Namespace: PowerProfileNamespace,
		}, powerWorkload)
		if !errors.IsNotFound(err) {
			t.Errorf("%s - Failed: Expected PowerWorkload '%s' to not exist", tc.testCase, tc.powerWorkload.Name)
		}
	}
}

func TestBasePowerProfileDeletion(t *testing.T) {
	tcases := []struct {
		testCase                         string
		basePowerProfile                 *powerv1alpha1.PowerProfile
		extendedPowerProfiles            *powerv1alpha1.PowerProfileList
		node                             *corev1.Node
		powerWorkloads                   *powerv1alpha1.PowerWorkloadList
		expectedProfilesToExist          []string
		expectedProfilesToNotExist       []string
		expectedWorkloadsToExist         []string
		expectedWorkloadsToNotExist      []string
		expectedExtendedResourcesToExist map[string]bool
		expectedNumberOfPowerProfiles    int
		expectedNumberOfPowerWorkloads   int
	}{
		{
			testCase: "Test Case 1",
			basePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "performance",
					Namespace: PowerProfileNamespace,
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "performance",
					Epp:  "performance",
				},
			},
			extendedPowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1",
							Namespace: PowerProfileNamespace,
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
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						"cpu:":                        *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/performance": *resource.NewQuantity(42, resource.DecimalSI),
						"power.intel.com/performance-example-node1": *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			powerWorkloads: &powerv1alpha1.PowerWorkloadList{
				Items: []powerv1alpha1.PowerWorkload{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "performance-example-node1-workload",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerWorkloadSpec{
							Name: "performance-example-node1-workload",
							PowerNodeSelector: map[string]string{
								"example-node": "true",
							},
							Node: powerv1alpha1.NodeInfo{
								Name: "example-node1",
							},
							PowerProfile: "performance-example-node1",
						},
					},
				},
			},
			expectedProfilesToNotExist: []string{
				"performance",
				"performance-example-node1",
			},
			expectedWorkloadsToNotExist: []string{
				"performance-example-node1-workload",
			},
			expectedExtendedResourcesToExist: map[string]bool{
				"performance":               false,
				"performance-example-node1": false,
			},
			expectedNumberOfPowerProfiles:  0,
			expectedNumberOfPowerWorkloads: 0,
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.basePowerProfile)
		if err != nil {
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listener", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Node object", tc.testCase))
		}

		for _, extendedProfile := range tc.extendedPowerProfiles.Items {
			err = r.Client.Create(context.TODO(), &extendedProfile)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error creating extended PowerProfile object", tc.testCase))
			}
		}

		err = r.Client.Delete(context.TODO(), tc.basePowerProfile)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error deleting PowerProfile object", tc.testCase))
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.basePowerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		for _, powerProfileName := range tc.expectedProfilesToNotExist {
			if powerProfileName != tc.basePowerProfile.Name {
				extendedReq := reconcile.Request{
					NamespacedName: client.ObjectKey{
						Name:      powerProfileName,
						Namespace: PowerProfileName,
					},
				}

				_, err = r.Reconcile(extendedReq)
				if err != nil {
					t.Error(err)
					t.Fatal(fmt.Sprintf("%s - error reconciling extended PowerProfile", tc.testCase))
				}
			}
		}

		server.Close()

		updatedNode := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.node.Name,
		}, updatedNode)

		for powerProfileName, shouldExist := range tc.expectedExtendedResourcesToExist {
			resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, powerProfileName))
			if _, exists := updatedNode.Status.Capacity[resourceName]; exists != shouldExist {
				t.Errorf("%s - Failed: Expected extended resource '%s' to exist to be %v, got %v", tc.testCase, resourceName, shouldExist, exists)
			}
		}

		for _, powerProfileName := range tc.expectedProfilesToExist {
			profile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerProfileName,
				Namespace: PowerProfileNamespace,
			}, profile)
			if err != nil {
				t.Errorf("%s - Failed: Expected PowerProfile '%s' to exist", tc.testCase, powerProfileName)
			}
		}

		for _, powerProfileName := range tc.expectedProfilesToExist {
			profile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerProfileName,
				Namespace: PowerProfileNamespace,
			}, profile)
			if !errors.IsNotFound(err) {
				t.Errorf("%s - Failed: Expected PowerProfile '%s' to not exist", tc.testCase, powerProfileName)
			}
		}

		for _, powerWorkloadName := range tc.expectedWorkloadsToExist {
			workload := &powerv1alpha1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerWorkloadName,
				Namespace: PowerProfileNamespace,
			}, workload)
			if err != nil {
				t.Errorf("%s - Failed: Expected PowerWorkload '%s' to exist", tc.testCase, powerWorkloadName)
			}
		}

		powerProfileList := &powerv1alpha1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfileList)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile list", tc.testCase))
		}

		if len(powerProfileList.Items) != tc.expectedNumberOfPowerProfiles {
			t.Errorf("%s - Failed: Expected number of PowerProfiles to be %v, got %v", tc.testCase, tc.expectedNumberOfPowerProfiles, len(powerProfileList.Items))
		}

		powerWorkloadList := &powerv1alpha1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), powerWorkloadList)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving PowerWorkload list", tc.testCase))
		}
	}
}

func TestPowerProfileMinMaxValues(t *testing.T) {
	tcases := []struct{
		testCase string
		performanceBasePowerProfile *powerv1alpha1.PowerProfile
		basePowerProfiles *powerv1alpha1.PowerProfileList
		node *corev1.Node
		performancePowerProfileName string
		expectedNumberOfPowerProfiles int
		expectedPowerProfileMaxMinValue map[string]int
	}{
		{
			testCase: "Test Case 1",
			performanceBasePowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
                                                        Name: "performance",
                                                        Namespace: PowerProfileNamespace,
                                                },
                                                Spec: powerv1alpha1.PowerProfileSpec{
                                                        Name: "performance",
                                                        Epp: "performance",
                                                },
			},
			basePowerProfiles: &powerv1alpha1.PowerProfileList{
				Items: []powerv1alpha1.PowerProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "balance-performance",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-performance",
							Epp: "balance_performance",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "balance-power",
							Namespace: PowerProfileNamespace,
						},
						Spec: powerv1alpha1.PowerProfileSpec{
							Name: "balance-power",
							Epp: "balance_power",
						},
					},
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-node1",
				},
				Status: corev1.NodeStatus{
					Capacity: map[corev1.ResourceName]resource.Quantity{
						CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
					},
				},
			},
			performancePowerProfileName: "performance-example-node1",
			expectedNumberOfPowerProfiles: 6,
			expectedPowerProfileMaxMinValue: map[string]int{
				"balance-performance-example-node1": 1000,
				"balance-power-example-node1": 2000,
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.node.Name)
		AppQoSClientAddress = "http://localhost:5000"

		r, err := createPowerProfileReconcileObject(tc.performanceBasePowerProfile)
		if err != nil {
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		server, err := createPowerProfileListeners([]appqos.PowerProfile{})
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Listener", tc.testCase))
		}

		err = r.Client.Create(context.TODO(), tc.node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating Node object", tc.testCase))
		}

		for _, profile := range tc.basePowerProfiles.Items {
			err = r.Client.Create(context.TODO(), &profile)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error creating PowerProfile object '%s'", tc.testCase, profile.Name))
			}
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.performanceBasePowerProfile.Name,
				Namespace: PowerProfileNamespace,
			},
		}

		_, err = r.Reconcile(req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		for _, profile := range tc.basePowerProfiles.Items {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      profile.Name,
					Namespace: PowerProfileNamespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
			}
		}

		server.Close()

		performancePowerProfile := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.performancePowerProfileName,
			Namespace: PowerProfileNamespace,
		}, performancePowerProfile)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving performance PowerProfile", tc.testCase))
		}

		for profileName, value := range tc.expectedPowerProfileMaxMinValue {
			powerProfile := &powerv1alpha1.PowerProfile{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name: profileName,
				Namespace: PowerProfileNamespace,
			}, powerProfile)
			if err != nil {
				t.Error(err)
				t.Fatal(fmt.Sprintf("%s - error retrieving PowerProfile '%s'", tc.testCase, profileName))
			}

			if (performancePowerProfile.Spec.Max - value) != powerProfile.Spec.Max {
				t.Errorf("%s - Failed: Expected PowerProfile '%s' Max value to be %v, got %v", tc.testCase, profileName, (performancePowerProfile.Spec.Max - value), powerProfile.Spec.Max)
			}

			if (performancePowerProfile.Spec.Min - value) != powerProfile.Spec.Min {
				t.Errorf("%s - Failed: Expected PowerProfile '%s' Min value to be %v, got %v", tc.testCase, profileName, (performancePowerProfile.Spec.Min - value), powerProfile.Spec.Min)
			}
		}
	}
}
