package controllers

import (
	"context"
	"strings"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func buildCStatesReconcilerObject(objs []runtime.Object, powerLibMock power.Host) *CStatesReconciler {
	log.SetLogger(zap.New(
		zap.UseDevMode(true),
		func(opts *zap.Options) {
			opts.TimeEncoder = zapcore.ISO8601TimeEncoder
		},
	),
	)
	schm := runtime.NewScheme()
	err := powerv1.AddToScheme(schm)
	if err != nil {
		return nil
	}
	client := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(schm).Build()
	reconciler := &CStatesReconciler{
		Client:       client,
		Log:          ctrl.Log.WithName("testing"),
		Scheme:       scheme.Scheme,
		PowerLibrary: powerLibMock,
	}

	return reconciler
}

func TestCStates_Reconcile(t *testing.T) {
	cStatesObj := &powerv1.CStates{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1",
			Namespace: IntelPowerNamespace,
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

	powerLibMock := new(hostMock)
	sharedPoolMock := new(poolMock)
	powerLibMock.On("GetSharedPool").Return(sharedPoolMock)
	exclusivePool := new(poolMock)
	exclusivePool.On("Name").Return("exclusive")
	powerLibMock.On("GetAllExclusivePools").Return(&power.PoolList{exclusivePool})
	powerLibMock.On("GetExclusivePool", "performance").Return(exclusivePool)
	mockedCore := new(coreMock)
	mockedCore.On("GetID").Return(uint(3))
	powerLibMock.On("GetAllCpus").Return(&power.CpuList{mockedCore})

	// validate
	powerLibMock.On("ValidateCStates", power.CStates(cStatesObj.Spec.SharedPoolCStates)).Return(nil)
	powerLibMock.On("ValidateCStates", power.CStates(cStatesObj.Spec.ExclusivePoolCStates["performance"])).Return(nil)
	powerLibMock.On("ValidateCStates", power.CStates(cStatesObj.Spec.IndividualCoreCStates["3"])).Return(nil)

	// reset calls
	sharedPoolMock.On("SetCStates", power.CStates(nil)).Return(error(nil))
	exclusivePool.On("SetCStates", power.CStates(nil)).Return(error(nil))
	mockedCore.On("SetCStates", power.CStates(nil)).Return(error(nil))

	// set calls
	sharedPoolMock.On("SetCStates", power.CStates(cStatesObj.Spec.SharedPoolCStates)).Return(error(nil))
	exclusivePool.On("SetCStates", power.CStates(cStatesObj.Spec.ExclusivePoolCStates["performance"])).Return(error(nil))
	mockedCore.On("SetCStates", power.CStates(cStatesObj.Spec.IndividualCoreCStates["3"])).Return(error(nil))

	r := buildCStatesReconcilerObject(objs, powerLibMock)
	assert.NotNil(t, r)
	ctx := context.Background()
	req := reconcile.Request{NamespacedName: client.ObjectKey{
		Namespace: IntelPowerNamespace,
		Name:      "node1",
	}}
	t.Setenv("NODE_NAME", "node1")

	result, err := r.Reconcile(ctx, req)

	assert.NotNil(t, result)
	assert.NoError(t, err)

	powerLibMock.AssertExpectations(t)
	sharedPoolMock.AssertExpectations(t)
	exclusivePool.AssertExpectations(t)
	mockedCore.AssertExpectations(t)

	// simulate CRD intended for a different node
	t.Setenv("NODE_NAME", "node2")
	powerLibMock = new(hostMock)
	r.PowerLibrary = powerLibMock

	_, err = r.Reconcile(ctx, req)
	assert.NoError(t, err)
	powerLibMock.AssertExpectations(t)

	// simulate request for node not in the cluster
	// expecting error
	req.Name = "notInCluster"
	cStatesObj.Name = "notInCluster"
	objs = []runtime.Object{powerNodesObj, cStatesObj, powerProfilesObj}

	r = buildCStatesReconcilerObject(objs, powerLibMock)
	assert.NotNil(t, r)

	_, err = r.Reconcile(ctx, req)
	assert.True(t, errors.IsBadRequest(err))
}

func FuzzCStatesReconciler(f *testing.F) {
	powerLibMock := new(hostMock)
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
		_, err := r.Reconcile(context.Background(), req)
		if err != nil {
			t.Errorf("error reconciling: %s", err)
		}
	})
}

func setupFuzz(t *testing.T, nodeName string, namespace string, extraNode bool, node2name string, runningOnTargetNode bool, powerLib power.Host) (*CStatesReconciler, reconcile.Request) {
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
	nodeName = strings.ReplaceAll(nodeName, " ", "")
	node2name = strings.ReplaceAll(node2name, " ", "")
	nodeName = strings.ReplaceAll(nodeName, "\t", "")
	node2name = strings.ReplaceAll(node2name, "\t", "")
	nodeName = strings.ReplaceAll(nodeName, "\000", "")
	node2name = strings.ReplaceAll(node2name, "\000", "")
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

func TestCstate_Reconcile_SetupPass(t *testing.T) {
	schm := runtime.NewScheme()
	err := powerv1.AddToScheme(schm)
	assert.Nil(t, err)
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects([]client.Object{}...).WithStatusSubresource([]client.Object{}...).Build()
	r := &CStatesReconciler{
		Client:       client,
		Log:          ctrl.Log.WithName("testing"),
		Scheme:       schm,
		PowerLibrary: new(hostMock),
	}
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(cacheMk))
	err = (&CStatesReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)
}

// tests failure for SetupWithManager function
func TestCstate_Reconcile_SetupFail(t *testing.T) {
	powerLibMock := new(hostMock)
	r := buildCStatesReconcilerObject([]runtime.Object{}, powerLibMock)
	mgr, _ := ctrl.NewManager(&rest.Config{}, ctrl.Options{
		Scheme: scheme.Scheme,
	})

	err := (&CStatesReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
