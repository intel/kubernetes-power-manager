/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createProfileReconcilerObject(objs []runtime.Object) (*PowerProfileReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(s).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &PowerProfileReconciler{cl, ctrl.Log.WithName("testing"), s, nil}

	return r, nil
}

func TestPowerProfileCreationNonPowerProfileNotInLibrary(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Max|Min non zero, epp performance",
			nodeName:    "TestNode",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
						Max:  3600,
						Min:  3200,
						Epp:  "performance",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
		{
			testCase:    "Test Case 2 - Max|Min zero, epp performance",
			nodeName:    "TestNode",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
						Max:  0,
						Min:  0,
						Epp:  "performance",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
		{
			testCase:    "Test Case 3 - Max|Min non zero, epp empty",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  3600,
						Min:  3200,
						Epp:  "",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		nodemk := new(nodeMock)
		nodemk.On("GetProfile", mock.Anything).Return(nil)
		nodemk.On("AddProfile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&mockProfile{}, nil)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		workload := &powerv1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      fmt.Sprintf("%s-%s", tc.profileName, tc.nodeName),
			Namespace: "default",
		}, workload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Power Workload Object", tc.testCase))
		}

		node := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.nodeName,
		}, node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Node object", tc.testCase))
		}

		resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, tc.profileName))
		if _, exists := node.Status.Capacity[resourceName]; !exists {
			t.Errorf("%s - Failed: Expected Extended Resource '%s' to be created", tc.testCase, fmt.Sprintf("%s%s", ExtendedResourcePrefix, tc.profileName))
		}
	}
}

func TestPowerProfileCreationNonPowerProfileInLibrary(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Max|Min non zero, epp performance",
			nodeName:    "TestNode",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
						Max:  3600,
						Min:  3200,
						Epp:  "performance",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
		{
			testCase:    "Test Case 2 - Max|Min zero, epp performance",
			nodeName:    "TestNode",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "performance",
						Max:  0,
						Min:  0,
						Epp:  "performance",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
		{
			testCase:    "Test Case 3 - Max|Min non zero, epp empty",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  3600,
						Min:  3200,
						Epp:  "",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		nodemk.On("GetProfile", mock.Anything).Return(&mockProfile{})
		nodemk.On("UpdateProfile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		workload := &powerv1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      fmt.Sprintf("%s-%s", tc.profileName, tc.nodeName),
			Namespace: "default",
		}, workload)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Power Workload Object", tc.testCase))
		}
	}
}

func TestPowerProfileCreationMaxMinValuesZero(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Max value zero",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  0,
						Min:  3200,
						Epp:  "",
					},
				},
			},
		},
		{
			testCase:    "Test Case 2 - Min value zero",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  3600,
						Min:  0,
						Epp:  "",
					},
				},
			},
		},
		{
			testCase:    "Test Case 3 - Max/Min value zero",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  0,
						Min:  0,
						Epp:  "",
					},
				},
			},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Power Workload Objects", tc.testCase))
		}

		if len(workloads.Items) > 0 {
			t.Errorf("%s - Failed - Expected number of Power Workload Objects to be zero", tc.testCase)
		}
	}
}

func TestPowerProfileCreationIncorrectEppValue(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Epp value incorrect",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  3600,
						Min:  3200,
						Epp:  "incorrect",
					},
				},
			},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		profile := &powerv1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.profileName,
			Namespace: "default",
		}, profile)
		if err == nil {
			t.Errorf("%s Failed - Expected Power Profile %s to not exist", tc.testCase, tc.profileName)
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Power Workload Objects", tc.testCase))
		}

		if len(workloads.Items) > 0 {
			t.Errorf("%s - Failed - Expected number of Power Workload Objects to be zero", tc.testCase)
		}
	}
}

func TestSharedPowerProfileCreationProfileDoesNotExistInLibrary(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Profile does not exists in Power Library",
			nodeName:    "TestNode",
			profileName: "shared",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "shared",
						Max:  800,
						Min:  800,
						Epp:  "power",
					},
				},
			},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		nodemk.On("GetProfile", mock.Anything).Return(nil)
		nodemk.On("AddProfile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&mockProfile{}, nil)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Power Workload Objects", tc.testCase))
		}

		if len(workloads.Items) > 0 {
			t.Errorf("%s - Failed - Expected number of Power Workload Objects to be zero", tc.testCase)
		}
	}
}

func TestPowerProfileDeletion(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Profile performance, ERs present",
			nodeName:    "TestNode",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: "default",
					},
					Spec: powerv1.PowerWorkloadSpec{},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource:                   *resource.NewQuantity(42, resource.DecimalSI),
							"power.intel.com/performance": *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
		{
			testCase:    "Test Case 2 - Profile user-created, ERs not present",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created-TestNode",
						Namespace: "default",
					},
					Spec: powerv1.PowerWorkloadSpec{},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
		{
			testCase:    "Test Case 3 - Profile user-created, ERs not present, workload not present",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		nodemk.On("DeleteProfile", mock.Anything).Return(nil)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error reconciling object", tc.testCase))
		}

		workload := &powerv1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      fmt.Sprintf("%s-%s", tc.profileName, tc.nodeName),
			Namespace: "default",
		}, workload)
		if err == nil {
			t.Errorf("%s Failed - Expected Power Workload Object '%s-%s' to have been deleted", tc.testCase, tc.profileName, tc.nodeName)
		}

		node := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.nodeName,
		}, node)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error retrieving Node object", tc.testCase))
		}

		resourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, tc.profileName))
		if _, exists := node.Status.Capacity[resourceName]; exists {
			t.Errorf("%s - Failed: Expected Extended Resource '%s' to have been deleted", tc.testCase, fmt.Sprintf("%s%s", ExtendedResourcePrefix, tc.profileName))
		}
	}
}

func TestMaxValueLowerThanMinValue(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test 1 - Max value less then Min value",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  2600,
						Min:  2800,
						Epp:  "performance",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Errorf("%s Failed - expected reconciler to not have failed", tc.testCase)
		}
	}
}

func TestSharedFrequencyValuesLessThanAbsoluteValue(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test 1 - Shared Frequency values less than absolute minimum",
			nodeName:    "TestNode",
			profileName: "shared",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "shared",
						Max:  100,
						Min:  100,
						Epp:  "power",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Errorf("%s Failed - expected reconciler to not have failed", tc.testCase)
		}
	}
}

func TestMaxValueZeroMinValueGreaterThanZero(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test 1 - Max value less then Min value",
			nodeName:    "TestNode",
			profileName: "user-created",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "user-created",
						Max:  0,
						Min:  2800,
						Epp:  "performance",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Errorf("%s Failed - expected reconciler to not have failed", tc.testCase)
		}
	}
}

func TestSharedProfileReturningNil(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test 1 - library.AddProfile returns error",
			nodeName:    "TestNode",
			profileName: "shared1",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared1",
						Namespace: "default",
					},
					Spec: powerv1.PowerProfileSpec{
						Name: "shared1",
						Max:  1000,
						Min:  1000,
						Epp:  "power",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		nodemk.On("GetProfile", "shared1").Return(nil)
		nodemk.On("GetProfile", "shared2").Return(&mockProfile{})
		nodemk.On("AddProfile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&mockProfile{}, errors.NewServiceUnavailable("test error"))
		nodemk.On("NewProfile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err == nil {
			t.Errorf("%s Failed - expected reconciler to have failed", tc.testCase)
		}
	}
}

func TestProfileReturnsErrors(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test 1 - Profile deleted, library.DeleteProfile returns error",
			nodeName:    "TestNode",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "TestNode",
					},
					Status: corev1.NodeStatus{
						Capacity: map[corev1.ResourceName]resource.Quantity{
							CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatal(fmt.Sprintf("%s - error creating reconciler object", tc.testCase))
		}

		nodemk := new(nodeMock)
		nodemk.On("DeleteProfile", mock.Anything).Return(errors.NewServiceUnavailable("test error"))
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: "default",
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err == nil {
			t.Errorf("%s Failed - expected reconciler to have failed", tc.testCase)
		}
	}
}
