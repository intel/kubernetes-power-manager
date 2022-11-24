package controllers

import (
	"context"
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func buildCStatesReconcilerObject(objs []runtime.Object, powerLibMock power.Node) *CStatesReconciler {
	schm := runtime.NewScheme()
	powerv1.AddToScheme(schm)

	client := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(schm).Build()
	reconciler := &CStatesReconciler{
		Client:       client,
		Log:          ctrl.Log.WithName("testing"),
		Scheme:       scheme.Scheme,
		PowerLibrary: powerLibMock,
	}

	return reconciler
}

func TestCStatesReconciler_Reconcile(t *testing.T) {
	cStatesObj := &powerv1.CStates{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1",
			Namespace: "default",
		},
		Spec: powerv1.CStatesSpec{
			SharedPoolCStates:     power.CStates{"C1": true},
			ExclusivePoolCStates:  map[string]map[string]bool{"performance": {"C2": false}},
			IndividualCoreCStates: map[string]map[string]bool{"3": {"C3": true}},
		},
	}
	powerNodesObj := &powerv1.PowerNodeList{
		Items: []powerv1.PowerNode{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			},
		},
	}
	powerProfilesObj := &powerv1.PowerProfileList{
		Items: []powerv1.PowerProfile{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance",
				},
			},
		},
	}
	objs := []runtime.Object{
		cStatesObj, powerProfilesObj, powerNodesObj,
	}

	powerLibMock := new(nodeMock)
	// verifying cStates exists
	powerLibMock.On("IsCStateValid", mock.Anything).Return(true)

	// reset calls
	powerLibMock.On("ApplyCStatesToCore", mock.Anything, mock.Anything).Return(nil)
	powerLibMock.On("ApplyCStateToPool", mock.Anything, mock.Anything).Return(nil)
	powerLibMock.On("ApplyCStatesToSharedPool", mock.Anything).Return(nil)

	// set values
	powerLibMock.On("ApplyCStatesToCore", 3, power.CStates{"C3": true}).Return(nil)
	powerLibMock.On("ApplyCStateToPool", "performance", power.CStates{"C2": false}).Return(nil)
	powerLibMock.On("ApplyCStatesToSharedPool", power.CStates{"C1": true}).Return(nil)

	r := buildCStatesReconcilerObject(objs, powerLibMock)

	ctx := context.Background()
	req := reconcile.Request{NamespacedName: client.ObjectKey{
		Namespace: "default",
		Name:      "node1",
	}}
	t.Setenv("NODE_NAME", "node1")

	result, err := r.Reconcile(ctx, req)

	assert.NotNil(t, result)
	assert.NoError(t, err)

	powerLibMock.AssertExpectations(t)
	powerLibMock.AssertNotCalled(t, "ApplyCStateToPool", "shared", mock.Anything)
	for _, cState := range []string{"C1", "C2", "C3"} {
		powerLibMock.AssertCalled(t, "IsCStateValid", []string{cState})
	}

	// simulate CRD intended for a different node
	t.Setenv("NODE_NAME", "node2")
	powerLibMock.Calls = []mock.Call{}

	_, err = r.Reconcile(ctx, req)
	assert.NoError(t, err)
	assert.Empty(t, powerLibMock.Calls)

	// simulate request for node not in the cluster
	// expecting error
	req.Name = "notInCluster"
	cStatesObj.Name = "notInCluster"
	objs = []runtime.Object{powerNodesObj, cStatesObj, powerProfilesObj}

	r = buildCStatesReconcilerObject(objs, powerLibMock)
	_, err = r.Reconcile(ctx, req)
	assert.True(t, errors.IsBadRequest(err))
}

func FuzzCStatesReconciler(f *testing.F) {
	powerLibMock := new(nodeMock)
	// verifying cStates exists
	powerLibMock.On("IsCStateValid", mock.Anything).Return(true)

	// reset calls
	powerLibMock.On("ApplyCStatesToCore", mock.Anything, mock.Anything).Return(nil)
	powerLibMock.On("ApplyCStateToPool", mock.Anything, mock.Anything).Return(nil)
	powerLibMock.On("ApplyCStatesToSharedPool", mock.Anything).Return(nil)

	// set values
	powerLibMock.On("ApplyCStatesToCore", mock.Anything, mock.Anything).Return(nil)
	powerLibMock.On("ApplyCStateToPool", mock.Anything, mock.Anything).Return(nil)
	powerLibMock.On("ApplyCStatesToSharedPool", mock.Anything).Return(nil)

	f.Fuzz(func(t *testing.T, nodeName string, namespace string, extraNode bool, node2name string, runningOnTargetNode bool) {

		r, req := setupFuzz(t, nodeName, namespace, extraNode, node2name, runningOnTargetNode, powerLibMock)
		if r == nil {
			// if r is nil setupFuzz must have panicked, so we ignore it
			return
		}
		r.Reconcile(context.Background(), req)
	})
}

func setupFuzz(t *testing.T, nodeName string, namespace string, extraNode bool, node2name string, runningOnTargetNode bool, powerLib power.Node) (*CStatesReconciler, reconcile.Request) {
	defer func(t *testing.T) {
		if r := recover(); r != nil {
			// if setup fails we ignore it
			t.Log("recon creation Panic")
		}
	}(t)

	req := reconcile.Request{NamespacedName: client.ObjectKey{
		Namespace: namespace,
		Name:      nodeName,
	}}

	if len(nodeName) == 0 || len(node2name) == 0 {
		return nil, req
	}

	t.Logf("nodename %v- len %d", nodeName, len(nodeName))
	t.Logf("nodename2 %v- len %d", node2name, len(nodeName))
	if runningOnTargetNode {
		t.Setenv("NODE_NAME", nodeName)
	} else {
		t.Setenv("NODE_NAME", node2name)
	}

	cStatesObj := &powerv1.CStates{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: namespace,
		},
		Spec: powerv1.CStatesSpec{
			SharedPoolCStates:     map[string]bool{"C1": true},
			ExclusivePoolCStates:  map[string]map[string]bool{"performance": {"C2": false}},
			IndividualCoreCStates: map[string]map[string]bool{"3": {"C3": true}},
		},
	}
	powerNodesObj := &powerv1.PowerNodeList{
		Items: []powerv1.PowerNode{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
			},
		},
	}

	if extraNode {
		powerNodesObj.Items = append(powerNodesObj.Items, powerv1.PowerNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: node2name,
			},
		})
	}

	powerProfilesObj := &powerv1.PowerProfileList{
		Items: []powerv1.PowerProfile{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance",
				},
			},
		},
	}

	objs := []runtime.Object{cStatesObj, powerProfilesObj, powerNodesObj}

	return buildCStatesReconcilerObject(objs, powerLib), req
}
