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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
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

// basic exclusive pool scenario
func TestPowerProfileExclusivePoolCreation(t *testing.T) {
	nodeName := "TestNode"
	clientObjs := []runtime.Object{
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name:     "performance",
				Max:      3600,
				Min:      3200,
				Epp:      "performance",
				Governor: "powersave",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Status: corev1.NodeStatus{
				Capacity: map[corev1.ResourceName]resource.Quantity{
					CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
				},
			},
		},
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "performance",
			Namespace: IntelPowerNamespace,
		},
	}
	t.Setenv("NODE_NAME", nodeName)
	r, err := createProfileReconcilerObject(clientObjs)
	if err != nil {
		t.Error(err)
		t.Fatalf("error creating reconciler object")
	}
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	r.PowerLibrary = host
	host.AddExclusivePool("performance")
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

}

// basic shared pool scenario
func TestPowerProfileSharedPoolCreation(t *testing.T) {
	clientObjs := []runtime.Object{
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-TestNode",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{},
		},
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name:   "shared",
				Max:    3600,
				Min:    3200,
				Shared: true,
				Epp:    "",
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
	}
	nodemk := new(hostMock)
	poolmk := new(poolMock)
	poolmk.On("SetPowerProfile", mock.Anything).Return(nil)
	nodemk.On("GetSharedPool").Return(poolmk)

	t.Setenv("NODE_NAME", "TestNode")
	r, err := createProfileReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = nodemk
	assert.Nil(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "shared",
			Namespace: IntelPowerNamespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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

		host, teardown, err := fullDummySystem()
		assert.Nil(t, err)
		defer teardown()
		r.PowerLibrary = host

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
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
			Namespace: IntelPowerNamespace,
		}, workload)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Object", tc.testCase)
		}

		node := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: tc.nodeName,
		}, node)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Node object", tc.testCase)
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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

		host, teardown, err := fullDummySystem()
		assert.Nil(t, err)
		defer teardown()
		r.PowerLibrary = host

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
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
			Namespace: IntelPowerNamespace,
		}, workload)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Object", tc.testCase)
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		nodemk := new(hostMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Objects", tc.testCase)
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
						Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		nodemk := new(hostMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		profile := &powerv1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.profileName,
			Namespace: IntelPowerNamespace,
		}, profile)
		if err == nil {
			t.Errorf("%s Failed - Expected Power Profile %s to not exist", tc.testCase, tc.profileName)
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Objects", tc.testCase)
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
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:   "shared",
						Max:    800,
						Min:    800,
						Shared: true,
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
		host, teardown, err := fullDummySystem()
		assert.Nil(t, err)
		defer teardown()
		r.PowerLibrary = host
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error reconciling object", tc.testCase)
		}

		workloads := &powerv1.PowerWorkloadList{}
		err = r.Client.List(context.TODO(), workloads)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Objects", tc.testCase)
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		pool := new(poolMock)
		pool.On("Remove").Return(nil)
		nodemk := new(hostMock)
		nodemk.On("GetExclusivePool", tc.profileName).Return(pool)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
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
			Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error retrieving Node object", tc.testCase)
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
						Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		nodemk := new(hostMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:   "shared",
						Max:    100,
						Min:    100,
						Shared: true,
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

		nodemk := new(hostMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		nodemk := new(hostMock)
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Errorf("%s Failed - expected reconciler to not have failed", tc.testCase)
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
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
		}

		nodemk := new(hostMock)
		pool := new(poolMock)
		nodemk.On("GetExclusivePool", tc.profileName).Return(pool)
		pool.On("Remove").Return(fmt.Errorf("test error"))
		r.PowerLibrary = nodemk

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		assert.Error(t, err)

	}
}

func TestAcpiDriver(t *testing.T) {
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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
						Namespace: IntelPowerNamespace,
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

		host, teardown, err := setupDummyFiles(86, 1, 2, map[string]string{
			"driver": "acpi-cpufreq", "max": "3700", "min": "1000",
			"epp": "performance", "governor": "performance",
			"package": "0", "die": "0", "available_governors": "powersave performance",
			"uncore_max": "2400000", "uncore_min": "1200000",
			"cstates": "intel_idle"})
		assert.Nil(t, err)
		defer teardown()
		r.PowerLibrary = host

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
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
			Namespace: IntelPowerNamespace,
		}, workload)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving Power Workload Object", tc.testCase)
		}
	}
}

// tests that force an error from a library function
// validateErr required as some instances result in nil being returned by reconciler
// to prevent requeueing
func TestPowerProfileLibraryErrs(t *testing.T) {
	tcases := []struct {
		testCase      string
		profileName   string
		powerNodeName string
		getNodemk     func() *hostMock
		validateErr   func(e error) bool
		clientObjs    []runtime.Object
	}{
		{
			testCase:      "Test Case 1 - exclusive pool does not exist",
			profileName:   "",
			powerNodeName: "TestNode",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				nodemk.On("GetExclusivePool", mock.Anything).Return(nil)
				return nodemk
			},
			validateErr: func(e error) bool {
				return assert.Error(t, e)
			},
			clientObjs: []runtime.Object{},
		},
		{
			testCase:      "Test Case 2 -error setting profile",
			profileName:   "shared",
			powerNodeName: "TestNode",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				poolmk := new(poolMock)
				poolmk.On("SetPowerProfile", mock.Anything).Return(fmt.Errorf("unit test set profile error"))
				nodemk.On("GetSharedPool").Return(poolmk)
				return nodemk
			},
			validateErr: func(e error) bool {
				return assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:   "shared",
						Max:    3600,
						Min:    3200,
						Shared: true,
						Epp:    "",
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
			testCase:    "Test Case 3 - Pool creation error",
			profileName: "performance",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				nodemk.On("GetExclusivePool", mock.Anything).Return(nil)
				nodemk.On("AddExclusivePool", mock.Anything).Return(nil, fmt.Errorf("Pool creation err"))
				return nodemk
			},
			validateErr: func(e error) bool {
				return assert.ErrorContains(t, e, "Pool creation err")
			},
			clientObjs: []runtime.Object{
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
			testCase:    "Test Case 4 - Set power profile error",
			profileName: "performance",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				poolmk := new(poolMock)
				nodemk.On("GetExclusivePool", mock.Anything).Return(nil)
				nodemk.On("AddExclusivePool", mock.Anything).Return(poolmk, nil)
				poolmk.On("SetPowerProfile", mock.Anything).Return(fmt.Errorf("Set profile err"))
				return nodemk
			},
			validateErr: func(e error) bool {
				return assert.ErrorContains(t, e, "Set profile err")
			},
			clientObjs: []runtime.Object{
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
	_, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", "TestNode")
		r, err := createProfileReconcilerObject(tc.clientObjs)
		assert.Nil(t, err)
		r.PowerLibrary = tc.getNodemk()
		assert.Nil(t, err)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		tc.validateErr(err)
	}
	teardown()
	tc := tcases[1]
	t.Setenv("NODE_NAME", "TestNode")
	r, err := createProfileReconcilerObject(tc.clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = tc.getNodemk()
	assert.Nil(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      tc.profileName,
			Namespace: IntelPowerNamespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	tc.validateErr(err)
}

// covers epp not supported error logs
// does not result in returned error as this is recoverable
func TestFeatureNotSupportedErr(t *testing.T) {
	tcases := []struct {
		testCase    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Shared profile",
			profileName: "shared",
			clientObjs: []runtime.Object{
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:   "shared",
						Max:    3600,
						Min:    3200,
						Shared: true,
						Epp:    "power",
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
			testCase:    "Test Case 2 - Exclusive profile",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{},
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
	setupDummyFiles(1, 1, 1, map[string]string{})
	t.Setenv("NODE_NAME", "TestNode")
	for _, tc := range tcases {
		r, err := createProfileReconcilerObject(tc.clientObjs)
		assert.Nil(t, err)
		nodemk := new(hostMock)
		poolmk := new(poolMock)
		nodemk.On("GetExclusivePool", mock.Anything).Return(poolmk)
		r.PowerLibrary = nodemk
		assert.Nil(t, err)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		assert.Nil(t, err)
	}

}

// tests errors returned by the reconciler client using the errclient mock
func TestPowerProfileClientErrs(t *testing.T) {
	tcases := []struct {
		testCase      string
		profileName   string
		powerNodeName string
		convertClient func(client.Client) client.Client
		clientErr     string
	}{
		{
			testCase:      "Test Case 1 - Invalid Get requests",
			profileName:   "",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client get error"))
				return mkcl
			},
			clientErr: "client get error",
		},
		{
			testCase:      "Test Case 2 - Client delete workload error",
			profileName:   "performance",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerProfile")).Return(errors.NewNotFound(schema.GroupResource{}, "profile"))
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerWorkload")).Return(nil)
				mkcl.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client delete error"))
				return mkcl
			},
			clientErr: "client delete error",
		},
		{
			testCase:      "Test Case 3 - Client create error",
			profileName:   "performance",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client) client.Client {
				mkcl := new(errClient)
				mkwriter := new(mockResourceWriter)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerProfile")).Return(nil).Run(func(args mock.Arguments) {
					pod := args.Get(2).(*powerv1.PowerProfile)
					*pod = *defaultProf
				})
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.Node")).Return(nil).Run(func(args mock.Arguments) {
					pod := args.Get(2).(*corev1.Node)
					*pod = corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "TestNode",
						},
						Status: corev1.NodeStatus{
							Capacity: map[corev1.ResourceName]resource.Quantity{
								CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
							},
						},
					}
				})
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerWorkload")).Return(errors.NewNotFound(schema.GroupResource{}, "profile"))
				mkcl.On("Status").Return(mkwriter)
				mkwriter.On("Update", mock.Anything, mock.Anything).Return(nil)
				mkcl.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client create error"))
				return mkcl
			},
			clientErr: "client create error",
		},
		{
			testCase:      "Test Case 4 - Client profile delete error",
			profileName:   "performance",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerProfile")).Return(errors.NewNotFound(schema.GroupResource{}, "profile"))
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerWorkload")).Return(nil)
				mkcl.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client delete error"))
				return mkcl
			},
			clientErr: "client delete error",
		},
	}

	dummyFilesystemHost, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	dummyFilesystemHost.AddExclusivePool("performance")
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", "TestNode")

		r, err := createProfileReconcilerObject([]runtime.Object{})
		assert.Nil(t, err)
		r.PowerLibrary = dummyFilesystemHost
		r.Client = tc.convertClient(r.Client)
		assert.Nil(t, err)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		assert.ErrorContains(t, err, tc.clientErr)
	}
}

// tests exclusive and shared profiles requesting invalid governors
func TestPowerProfileUnsupportedGovernor(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - invalid exclusive governor",
			nodeName:    "TestNode",
			profileName: "performance",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "performance",
						Max:      3600,
						Min:      3200,
						Epp:      "performance",
						Governor: "made up",
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
			testCase:    "Test Case 2 - invalid shared governor",
			nodeName:    "TestNode",
			profileName: "shared",
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "shared",
						Max:      10000,
						Min:      10000,
						Shared:   true,
						Governor: "made up",
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

		host, teardown, err := fullDummySystem()
		assert.Nil(t, err)
		defer teardown()
		r.PowerLibrary = host

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		assert.ErrorContains(t, err, "not supported")
	}

}

// tests positive and negative cases for SetupWithManager function
func TestPowerProfileReconcileSetupPass(t *testing.T) {
	r, err := createProfileReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(v1alpha1.ControllerConfigurationSpec{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	err = (&PowerProfileReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}
func TestPowerProfileReconcileSetupFail(t *testing.T) {
	r, err := createProfileReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr, _ := ctrl.NewManager(&rest.Config{}, ctrl.Options{
		Scheme: scheme.Scheme,
	})

	err = (&PowerProfileReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
