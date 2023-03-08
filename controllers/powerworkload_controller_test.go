package controllers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

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

func createWorkloadReconcilerObject(objs []runtime.Object) (*PowerWorkloadReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(s).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &PowerWorkloadReconciler{cl, ctrl.Log.WithName("testing"), s, nil}

	return r, nil
}

func TestPowerWorkload(t *testing.T) {
	testNode := "TestNode"
	workloadName := "performance-TestNode"
	pwrWorkloadObj := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: "default",
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name:              "",
			AllCores:          false,
			ReservedCPUs:      []uint{0, 1},
			PowerNodeSelector: map[string]string{"powernode": "selector"},
			PowerProfile:      "shared",
			Node: powerv1.WorkloadNode{
				Name:   testNode,
				CpuIds: []uint{2, 3},
			},
		},
	}

	nodeObj := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testNode,
			Labels: map[string]string{"powernode": "selector"},
		},
		Status: corev1.NodeStatus{
			Capacity: map[corev1.ResourceName]resource.Quantity{
				CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
			},
		},
	}
	nodesObj := &corev1.NodeList{
		Items: []corev1.Node{*nodeObj},
	}

	clientObjs := []runtime.Object{
		pwrWorkloadObj, nodesObj,
	}

	// workload creation - pool does not exist in library
	t.Setenv("NODE_NAME", testNode)

	r, err := createWorkloadReconcilerObject(clientObjs)
	if err != nil {
		t.Error(err)
		t.Fatalf("error creating reconciler object")
	}

	nodemk := new(hostMock)

	nodemk.On("GetExclusivePool", mock.Anything).Return(nil)
	r.PowerLibrary = nodemk

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      workloadName,
			Namespace: "default",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	nodemk.AssertExpectations(t)

	//workload deletion - remove from library - exclusive pool -- error
	r, err = createWorkloadReconcilerObject([]runtime.Object{})
	if err != nil {
		t.Error(err)
		t.Fatal("error creating reconciler object")
	}
	sharedPowerWorkloadName = "something"
	nodemk = new(hostMock)
	poolmk := new(poolMock)
	nodemk.On("GetExclusivePool", workloadName).Return(poolmk)
	poolmk.On("Remove").Return(errors.New("err"))

	r.PowerLibrary = nodemk
	_, err = r.Reconcile(context.TODO(), req)

	assert.Error(t, err)
	nodemk.AssertExpectations(t)
	assert.Equal(t, "something", sharedPowerWorkloadName)

	// workload deletetion - shared pool
	sharedPowerWorkloadName = "shared"

	r, err = createWorkloadReconcilerObject([]runtime.Object{})
	assert.NoError(t, err, "Failed to create reconciler object")

	nodemk = new(hostMock)
	nodemk.On("GetSharedPool").Return(poolmk)
	poolmk.On("SetPowerProfile", nil).Return(nil)
	r.PowerLibrary = nodemk
	req.Name = "shared"

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	nodemk.AssertExpectations(t)
	assert.Empty(t, sharedPowerWorkloadName)

	// not running on node with stuff
	pwrWorkloadObj.Spec.AllCores = true
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj})
	assert.NoError(t, err, "Failed to create reconciler object")
	r.PowerLibrary = new(hostMock)

	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

	// shared workload already exists
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj, nodesObj})
	assert.NoError(t, err, "Failed to create reconciler object")
	r.PowerLibrary = new(hostMock)

	sharedPowerWorkloadName = "shared"
	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	assert.Error(t, r.Client.Get(context.TODO(), req.NamespacedName, &powerv1.PowerWorkload{}))

	// error adding shared pool
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj, nodesObj})
	assert.NoError(t, err, "Failed to create reconciler object")

	nodemk = new(hostMock)
	poolmk = new(poolMock)
	nodemk.On("GetReservedPool").Return(poolmk)
	poolmk.On("SetCpuIDs", mock.Anything).Return(fmt.Errorf("scuffed"))
	r.PowerLibrary = nodemk

	sharedPowerWorkloadName = ""
	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	nodemk.AssertExpectations(t)
	poolmk.AssertExpectations(t)
	assert.Error(t, err)

	// successful add shared pool
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj, nodesObj})
	assert.NoError(t, err, "Failed to create reconciler object")

	nodemk = new(hostMock)
	poolmk = new(poolMock)
	r.PowerLibrary = nodemk

	nodemk.On("GetReservedPool").Return(poolmk)
	poolmk.On("SetCpuIDs", mock.Anything).Return(nil)

	sharedPowerWorkloadName = ""
	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	nodemk.AssertExpectations(t)
	assert.Nil(t, err)
	assert.Equal(t, req.Name, sharedPowerWorkloadName)
}
func Test_detectCoresRemoved(t *testing.T) {
	orig := []uint{1, 2, 3, 4}
	updated := []uint{1, 2, 4, 5}

	expectedResult := []uint{3}
	result := detectCoresRemoved(orig, updated)
	assert.ElementsMatch(t, result, expectedResult)
}

func Test_detectCoresAdded(t *testing.T) {
	orig := []uint{1, 2, 3, 4}
	updated := []uint{1, 2, 4, 5}

	expectedResult := []uint{5}
	result := detectCoresAdded(orig, updated)
	assert.ElementsMatch(t, result, expectedResult)
}
