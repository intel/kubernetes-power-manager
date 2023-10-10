package controllers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createWorkloadReconcilerObject(objs []runtime.Object) (*PowerWorkloadReconciler, error) {
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
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(s).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &PowerWorkloadReconciler{cl, ctrl.Log.WithName("testing"), s, nil}

	return r, nil
}

func TestPowerWorkload_Reconcile(t *testing.T) {
	testNode := "TestNode"
	workloadName := "performance-TestNode"
	pwrWorkloadObj := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: IntelPowerNamespace,
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
		t.Fatalf("error creating the reconciler object")
	}

	nodemk := new(hostMock)

	nodemk.On("GetExclusivePool", mock.Anything).Return(nil)
	r.PowerLibrary = nodemk

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      workloadName,
			Namespace: IntelPowerNamespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	nodemk.AssertExpectations(t)
	// workload created - pool does exist in library
	// using dummy file system because nested function calls are hard to mock
	exclusiveWorkload := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name:              workloadName,
			AllCores:          false,
			PowerNodeSelector: map[string]string{"powernode": "selector"},
			PowerProfile:      "performance",
			Node: powerv1.WorkloadNode{
				Name:   testNode,
				CpuIds: []uint{2, 3},
			},
		},
	}
	sharedWorkload := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-" + testNode,
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name:              "shared-" + testNode,
			AllCores:          true,
			ReservedCPUs:      []uint{0, 1},
			PowerNodeSelector: map[string]string{"powernode": "selector"},
			PowerProfile:      "shared",
			Node: powerv1.WorkloadNode{
				Name:   testNode,
				CpuIds: []uint{2, 3},
			},
		},
	}
	clientObjs = []runtime.Object{
		sharedWorkload, exclusiveWorkload, nodesObj,
	}
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	sharedProf, err := power.NewPowerProfile("shared", 10000, 10000, "powersave", "power")

	assert.Nil(t, err)

	host.GetSharedPool().SetPowerProfile(sharedProf)
	r, err = createWorkloadReconcilerObject(clientObjs)
	if err != nil {
		t.Error(err)
		t.Fatalf("error creating the reconciler object")
	}
	r.PowerLibrary = host
	host.AddExclusivePool("performance")
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "shared-" + testNode,
			Namespace: IntelPowerNamespace,
		},
	}
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      workloadName,
			Namespace: IntelPowerNamespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	nodemk.AssertExpectations(t)
	//workload deletion - remove from library - exclusive pool -- error
	r, err = createWorkloadReconcilerObject([]runtime.Object{})
	if err != nil {
		t.Error(err)
		t.Fatal("error creating the reconciler object")
	}
	sharedPowerWorkloadName = "something"
	nodemk = new(hostMock)
	poolmk := new(poolMock)
	nodemk.On("GetExclusivePool", mock.Anything).Return(poolmk)
	poolmk.On("Remove").Return(errors.New("err"))

	r.PowerLibrary = nodemk
	_, err = r.Reconcile(context.TODO(), req)

	assert.Error(t, err)
	nodemk.AssertExpectations(t)
	assert.Equal(t, "something", sharedPowerWorkloadName)

	// workload deletetion - shared pool
	sharedPowerWorkloadName = "shared"

	r, err = createWorkloadReconcilerObject([]runtime.Object{})
	assert.NoError(t, err, "failed to create the reconciler object")

	nodemk = new(hostMock)
	reservedmk := new(poolMock)
	nodemk.On("GetSharedPool").Return(poolmk)
	poolmk.On("SetPowerProfile", nil).Return(nil)
	nodemk.On("GetReservedPool").Return(reservedmk)
	reservedmk.On("MoveCpus", mock.Anything).Return(nil)
	poolmk.On("Cpus").Return(&power.CpuList{})
	r.PowerLibrary = nodemk
	req.Name = "shared"

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	nodemk.AssertExpectations(t)
	assert.Empty(t, sharedPowerWorkloadName)

	// not running on node with stuff
	pwrWorkloadObj.Spec.AllCores = true
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj})
	assert.NoError(t, err, "failed to create the reconciler object")
	r.PowerLibrary = new(hostMock)

	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

	// shared workload already exists
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj, nodesObj})
	assert.NoError(t, err, "failed to create the reconciler object")
	r.PowerLibrary = new(hostMock)

	sharedPowerWorkloadName = "shared"
	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	assert.Error(t, r.Client.Get(context.TODO(), req.NamespacedName, &powerv1.PowerWorkload{}))

	// error adding shared pool
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj, nodesObj})
	assert.NoError(t, err, "failed to create the reconciler object")

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
	assert.NoError(t, err, "failed to create the reconciler object")

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

func TestPowerWorkload_Reconcile_DetectCoresRemoved(t *testing.T) {
	orig := []uint{1, 2, 3, 4}
	updated := []uint{1, 2, 4, 5}

	expectedResult := []uint{3}
	result := detectCoresRemoved(orig, updated, &logr.Logger{})
	assert.ElementsMatch(t, result, expectedResult)
}

func TestPowerWorkload_Reconcile_DetectCoresAdded(t *testing.T) {
	orig := []uint{1, 2, 3, 4}
	updated := []uint{1, 2, 4, 5}

	expectedResult := []uint{5}
	result := detectCoresAdded(orig, updated, &logr.Logger{})
	assert.ElementsMatch(t, result, expectedResult)
}

func TestPowerWorkload_Reconcile_WrongNamespace(t *testing.T) {
	// ensure request for wrong namespace is ignored

	r, err := createWorkloadReconcilerObject([]runtime.Object{})
	if err != nil {
		t.Error(err)
		t.Fatalf("error creating reconciler object")
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "test-workload",
			Namespace: "MADE-UP",
		},
	}
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
}

func TestPowerWorkload_Reconcile_ClientErrs(t *testing.T) {
	// error getting power nodes
	testNode := "TestNode"
	workloadName := "performance-TestNode"
	pwrWorkloadObj := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name:              "",
			AllCores:          true,
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

	t.Setenv("NODE_NAME", testNode)

	r, err := createWorkloadReconcilerObject([]runtime.Object{})
	if err != nil {
		t.Error(err)
		t.Fatalf("error creating the reconciler object")
	}
	mkcl := new(errClient)
	mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		node := args.Get(2).(*powerv1.PowerWorkload)
		*node = *pwrWorkloadObj
	})
	mkcl.On("List", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client list error"))
	r.Client = mkcl
	nodemk := new(hostMock)

	r.PowerLibrary = nodemk

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      workloadName,
			Namespace: IntelPowerNamespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "client list error")
	nodemk.AssertExpectations(t)
	// error deleting duplicate shared pool
	r, err = createWorkloadReconcilerObject([]runtime.Object{pwrWorkloadObj, nodesObj})
	assert.Nil(t, err)
	r.PowerLibrary = new(hostMock)
	mkcl = new(errClient)
	mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		wload := args.Get(2).(*powerv1.PowerWorkload)
		*wload = *pwrWorkloadObj
	})
	mkcl.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		node := args.Get(1).(*corev1.NodeList)
		*node = *nodesObj
	})
	mkcl.On("Delete", mock.Anything, mock.Anything).Return(fmt.Errorf("client delete error"))
	r.Client = mkcl

	sharedPowerWorkloadName = "shared"
	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "client delete error")
}

func TestPowerWorkload_Reconcile_SetupPass(t *testing.T) {
	r, err := createPowerNodeReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(cacheMk))
	err = (&PowerWorkloadReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}
func TestPowerWorkload_Reconcile_SetupFail(t *testing.T) {
	r, err := createPowerNodeReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("setup fail"))

	err = (&PowerWorkloadReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
