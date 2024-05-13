package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type reservedPoolMocks struct {
	node             *hostMock
	shared           *poolMock
	performance      *poolMock
	exclusiveRserved *poolMock
	reserved         *poolMock
}

// used to remove a call from a predefined mock
func popCall(calls []*mock.Call, method string) []*mock.Call {
	for i, call := range calls {
		if call.Method == method {
			calls[i] = calls[len(calls)-1]
			return calls[:len(calls)-1]
		}
	}
	return calls
}

// creates a basic template for reaching reserved pool related segments
func mocktemplate() reservedPoolMocks {
	nodemk := new(hostMock)
	sharedPoolmk := new(poolMock)
	perfPoolmk := new(poolMock)
	exclusiveReservedmk := new(poolMock)
	reservedmk := new(poolMock)
	nodemk.On("GetReservedPool").Return(reservedmk)
	nodemk.On("GetSharedPool").Return(sharedPoolmk)
	nodemk.On("GetAllExclusivePools").Return(&power.PoolList{exclusiveReservedmk})
	nodemk.On("AddExclusivePool", mock.Anything).Return(exclusiveReservedmk, nil)
	nodemk.On("GetExclusivePool", mock.Anything).Return(perfPoolmk)
	sharedPoolmk.On("Cpus").Return(&power.CpuList{})
	sharedPoolmk.On("MoveCpuIDs", mock.Anything).Return(nil)
	sharedPoolmk.On("SetCpuIDs", mock.Anything).Return(nil)
	sharedPoolmk.On("SetPowerProfile", mock.Anything).Return(nil)
	reservedmk.On("MoveCpuIDs", mock.Anything).Return(nil)
	reservedmk.On("SetCpuIDs", mock.Anything).Return(nil)
	exclusiveReservedmk.On("Name").Return("TestNode-reserved-[0]")
	exclusiveReservedmk.On("Remove").Return(nil)
	exclusiveReservedmk.On("SetCpuIDs", mock.Anything).Return(nil)
	exclusiveReservedmk.On("SetPowerProfile", mock.Anything).Return(nil)
	perfPoolmk.On("GetPowerProfile").Return(new(profMock))
	return reservedPoolMocks{
		node:             nodemk,
		shared:           sharedPoolmk,
		performance:      perfPoolmk,
		exclusiveRserved: exclusiveReservedmk,
		reserved:         reservedmk,
	}
}
func TestPowerWorkload_Reconcile(t *testing.T) {
	testNode := "TestNode"
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
	pwrWorkloadObj := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "performance-TestNode",
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name:              "performance-TestNode",
			AllCores:          false,
			PowerNodeSelector: map[string]string{"powernode": "selector"},
			PowerProfile:      "performance",
			Node: powerv1.WorkloadNode{
				Name:   testNode,
				CpuIds: []uint{4, 5},
			},
		},
	}
	sharedSkeleton := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-" + testNode,
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name:     "shared-" + testNode,
			AllCores: true,
			ReservedCPUs: []powerv1.ReservedSpec{
				{Cores: []uint{0, 1}, PowerProfile: "performance"},
				{Cores: []uint{2}, PowerProfile: "performance"},
			},
		},
	}
	tcases := []struct {
		testCase     string
		workloadName string
		clientObjs   []runtime.Object
		getNodemk    func() *hostMock
		validateErr  func(r *PowerWorkloadReconciler, e error) bool
	}{
		{
			testCase:     "Test Case 1 - pool does not exist",
			workloadName: "performance-TestNode",
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "does not exist in the power library")
			},
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				nodemk.On("GetExclusivePool", mock.Anything).Return(nil)
				return nodemk
			},
			clientObjs: []runtime.Object{
				pwrWorkloadObj,
				nodeObj,
			},
		},
		{
			testCase:     "Test Case 2 - workload creation",
			workloadName: "shared-" + testNode,
			getNodemk:    func() *hostMock { return new(hostMock) },
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				assert.Nil(t, e)
				req := reconcile.Request{
					NamespacedName: client.ObjectKey{
						Name:      "performance-TestNode",
						Namespace: IntelPowerNamespace,
					},
				}

				_, err := r.Reconcile(context.TODO(), req)
				return assert.NoError(t, err)
			},
			clientObjs: []runtime.Object{
				pwrWorkloadObj,
				&powerv1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared-" + testNode,
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerWorkloadSpec{
						Name:              "shared-" + testNode,
						AllCores:          true,
						ReservedCPUs:      []powerv1.ReservedSpec{{Cores: []uint{0, 1}}},
						PowerNodeSelector: map[string]string{"powernode": "selector"},
						PowerProfile:      "shared",
						Node: powerv1.WorkloadNode{
							Name:   testNode,
							CpuIds: []uint{5, 3},
						},
					},
				},
				nodeObj,
			},
		},
		{
			testCase:     "Test Case 3 - pool deletion err",
			workloadName: "performance-TestNode",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				poolmk := new(poolMock)
				nodemk.On("GetExclusivePool", mock.Anything).Return(poolmk)
				poolmk.On("Remove").Return(errors.New("pool removal err"))
				return nodemk
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "pool removal err")
			},
		},
		{
			testCase:     "Test Case 4 - shared workload deletion",
			workloadName: "shared",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				reservedmk := new(poolMock)
				poolmk := new(poolMock)
				nodemk.On("GetSharedPool").Return(poolmk)
				poolmk.On("SetPowerProfile", nil).Return(nil)
				nodemk.On("GetReservedPool").Return(reservedmk)
				exclusiveReservedmk := new(poolMock)
				nodemk.On("GetAllExclusivePools").Return(&power.PoolList{exclusiveReservedmk})
				exclusiveReservedmk.On("Name").Return("TestNode-reserved-[0]")
				exclusiveReservedmk.On("Remove").Return(nil)
				exclusiveReservedmk.On("SetPowerProfile", mock.Anything).Return(nil)
				exclusiveReservedmk.On("Cpus").Return(&power.CpuList{})
				exclusiveReservedmk.On("Remove").Return(nil)
				reservedmk.On("MoveCpus", mock.Anything).Return(nil)
				poolmk.On("Cpus").Return(&power.CpuList{})
				sharedPowerWorkloadName = "shared"
				return nodemk
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				assert.Nil(t, e)
				return assert.Empty(t, sharedPowerWorkloadName)
			},
		},
		{
			testCase:     "Test Case 5 - shared workload on wrong node",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				sharedPowerWorkloadName = "shared"
				return new(hostMock)
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{sharedSkeleton},
		},
		{
			testCase:     "Test Case 6 - shared workload already exists",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				return new(hostMock)
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "a shared power workload already exists")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 7 - set cpu error",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				poolmk := new(poolMock)
				sharedPoolmk := new(poolMock)
				dummyPoolmk := new(poolMock)
				profmk := new(profMock)
				nodemk.On("GetReservedPool").Return(poolmk)
				nodemk.On("GetExclusivePool", mock.Anything).Return(dummyPoolmk)
				nodemk.On("GetSharedPool").Return(sharedPoolmk)
				dummyPoolmk.On("GetPowerProfile").Return(profmk)
				sharedPoolmk.On("SetPowerProfile", mock.Anything).Return(nil)
				poolmk.On("SetCpuIDs", mock.Anything).Return(fmt.Errorf("set cpu error"))
				sharedPowerWorkloadName = ""
				return nodemk
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "set cpu error")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 8 - shared pool creation",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				template := mocktemplate()
				return template.node
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				assert.Nil(t, e)
				return assert.Equal(t, "shared-"+testNode, sharedPowerWorkloadName)
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 9 - reserved setProfile error recovery failure",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				template := mocktemplate()
				// have 2 calls causign an issue
				template.exclusiveRserved.ExpectedCalls = popCall(template.exclusiveRserved.ExpectedCalls, "SetPowerProfile")
				template.reserved.ExpectedCalls = popCall(template.reserved.ExpectedCalls, "MoveCpuIDs")
				template.exclusiveRserved.On("SetPowerProfile", mock.Anything).Return(fmt.Errorf("set profile err"))
				template.reserved.On("MoveCpuIDs", mock.Anything).Return(fmt.Errorf("recovery failed"))
				return template.node
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "recovery failed")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 10 - reserved setCpu error recovery failure",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				template := mocktemplate()
				template.exclusiveRserved.ExpectedCalls = popCall(template.exclusiveRserved.ExpectedCalls, "SetCpuIDs")
				template.reserved.ExpectedCalls = popCall(template.reserved.ExpectedCalls, "MoveCpuIDs")
				template.exclusiveRserved.On("SetCpuIDs", mock.Anything).Return(fmt.Errorf("set profile err"))
				template.reserved.On("MoveCpuIDs", mock.Anything).Return(fmt.Errorf("recovery failed"))
				return template.node
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "recovery failed")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 11 - reserved SetCpuIDs() and pseudoReservedPool.Remove() errors",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				template := mocktemplate()
				template.exclusiveRserved.ExpectedCalls = popCall(template.exclusiveRserved.ExpectedCalls, "SetCpuIDs")
				template.reserved.ExpectedCalls = popCall(template.reserved.ExpectedCalls, "MoveCpuIDs")
				template.exclusiveRserved.On("SetCpuIDs", mock.Anything).Return(fmt.Errorf("set profile err"))
				template.exclusiveRserved.On("Remove", mock.Anything).Return(fmt.Errorf("remove pool error"))
				template.reserved.On("MoveCpuIDs", mock.Anything).Return(fmt.Errorf("recovery failed"))
				return template.node
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "recovery failed")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 12 - reserved recovery",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				template := mocktemplate()
				exclusiveReservedmk2 := new(poolMock)
				template.exclusiveRserved.ExpectedCalls = popCall(template.exclusiveRserved.ExpectedCalls, "SetCpuIDs")
				template.reserved.ExpectedCalls = popCall(template.reserved.ExpectedCalls, "MoveCpuIDs")
				template.node.ExpectedCalls = popCall(template.node.ExpectedCalls, "GetAllExclusivePools")
				template.node.ExpectedCalls = popCall(template.node.ExpectedCalls, "AddExclusivePool")
				template.node.On("GetAllExclusivePools").Return(&power.PoolList{exclusiveReservedmk2, template.exclusiveRserved})
				template.node.On("AddExclusivePool", "TestNode-reserved-[2]").Return(exclusiveReservedmk2, nil)
				template.node.On("AddExclusivePool", "TestNode-reserved-[0 1]").Return(template.exclusiveRserved, nil)
				exclusiveReservedmk2.On("Name").Return("TestNode-reserved-[2]")
				exclusiveReservedmk2.On("Remove").Return(nil)
				exclusiveReservedmk2.On("SetCpuIDs", mock.Anything).Return(nil)
				exclusiveReservedmk2.On("SetPowerProfile", mock.Anything).Return(nil)
				template.exclusiveRserved.On("SetCpuIDs", mock.Anything).Return(fmt.Errorf("set profile err"))
				template.reserved.On("MoveCpuIDs", mock.Anything).Return(nil)
				return template.node
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "error(s) encountered establishing reserved pool")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 13 - SetPowerProfile() and pseudoReservedPool.Remove() errors",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				template := mocktemplate()
				exclusiveReservedmk2 := new(poolMock)
				template.exclusiveRserved.ExpectedCalls = popCall(template.exclusiveRserved.ExpectedCalls, "SetCpuIDs")
				template.reserved.ExpectedCalls = popCall(template.reserved.ExpectedCalls, "MoveCpuIDs")
				template.node.ExpectedCalls = popCall(template.node.ExpectedCalls, "GetAllExclusivePools")
				template.node.ExpectedCalls = popCall(template.node.ExpectedCalls, "AddExclusivePool")
				template.node.On("GetAllExclusivePools").Return(&power.PoolList{exclusiveReservedmk2, template.exclusiveRserved})
				template.node.On("AddExclusivePool", "TestNode-reserved-[2]").Return(exclusiveReservedmk2, nil)
				template.node.On("AddExclusivePool", "TestNode-reserved-[0 1]").Return(template.exclusiveRserved, nil)
				exclusiveReservedmk2.On("Name").Return("TestNode-reserved-[2]")
				exclusiveReservedmk2.On("Remove").Return(nil).Once()
				exclusiveReservedmk2.On("Remove").Return(fmt.Errorf("remove error")).Once()
				exclusiveReservedmk2.On("SetCpuIDs", mock.Anything).Return(nil)
				exclusiveReservedmk2.On("SetPowerProfile", mock.Anything).Return(fmt.Errorf("set profile err"))
				template.exclusiveRserved.On("SetCpuIDs", mock.Anything).Return(fmt.Errorf("set CPU ids err"))
				template.exclusiveRserved.On("Remove").Return(nil)
				template.reserved.On("MoveCpuIDs", mock.Anything).Return(nil)
				return template.node
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "error(s) encountered establishing reserved pool")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
		{
			testCase:     "Test Case 14 - nil GetExclusivePool() response and pseudoReservedPool.Remove() error",
			workloadName: "shared-" + testNode,
			getNodemk: func() *hostMock {
				template := mocktemplate()
				exclusiveReservedmk2 := new(poolMock)
				template.node.ExpectedCalls = popCall(template.node.ExpectedCalls, "GetAllExclusivePools")
				template.node.ExpectedCalls = popCall(template.node.ExpectedCalls, "AddExclusivePool")
				template.node.ExpectedCalls = popCall(template.node.ExpectedCalls, "GetExclusivePool")
				template.node.On("GetAllExclusivePools").Return(&power.PoolList{exclusiveReservedmk2, template.exclusiveRserved})
				template.node.On("AddExclusivePool", "TestNode-reserved-[2]").Return(exclusiveReservedmk2, nil)
				template.node.On("AddExclusivePool", "TestNode-reserved-[0 1]").Return(template.exclusiveRserved, nil)
				template.node.On("GetExclusivePool", "performance").Return(template.performance).Once()
				template.node.On("GetExclusivePool", "performance").Return(nil).Once()
				template.node.On("GetExclusivePool", "").Return(template.performance).Once()

				exclusiveReservedmk2.On("Name").Return("TestNode-reserved-[2]")
				exclusiveReservedmk2.On("Remove").Return(nil).Once()
				exclusiveReservedmk2.On("Remove").Return(fmt.Errorf("remove error 2")).Once()
				exclusiveReservedmk2.On("SetCpuIDs", mock.Anything).Return(nil)
				exclusiveReservedmk2.On("SetPowerProfile", mock.Anything).Return(nil)

				return template.node
			},
			validateErr: func(r *PowerWorkloadReconciler, e error) bool {
				return assert.ErrorContains(t, e, "error(s) encountered establishing reserved pool")
			},
			clientObjs: []runtime.Object{sharedSkeleton, nodeObj},
		},
	}
	for _, tc := range tcases {
		t.Log(tc.testCase)
		t.Setenv("NODE_NAME", testNode)
		r, err := createWorkloadReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}
		nodemk := tc.getNodemk()
		if tc.testCase == "Test Case 2 - workload creation" {
			host, teardown, err := fullDummySystem()
			assert.Nil(t, err)
			defer teardown()
			sharedProf, err := power.NewPowerProfile("shared", 1000, 1000, "powersave", "power")
			assert.Nil(t, err)
			assert.Nil(t, host.GetSharedPool().SetPowerProfile(sharedProf))
			perf, err := host.AddExclusivePool("performance")
			assert.Nil(t, err)
			pool, err := host.AddExclusivePool("shared")
			pool.SetPowerProfile(sharedProf)
			assert.Nil(t, err)
			assert.Nil(t, host.GetSharedPool().SetCpuIDs([]uint{2, 3, 4, 5, 6, 7}))
			assert.Nil(t, perf.SetCpuIDs([]uint{2, 3}))
			r.PowerLibrary = host
		} else {
			r.PowerLibrary = nodemk
		}
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.workloadName,
				Namespace: IntelPowerNamespace,
			},
		}
		_, err = r.Reconcile(context.TODO(), req)
		tc.validateErr(r, err)
		nodemk.AssertExpectations(t)

	}
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
	assert.ErrorContains(t, err, "incorrect namespace")
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
			ReservedCPUs:      []powerv1.ReservedSpec{{Cores: []uint{0, 1}}},
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

	mkwriter := new(mockResourceWriter)
	mkwriter.On("Update", mock.Anything, mock.Anything).Return(nil)
	mkcl := new(errClient)
	mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		node := args.Get(2).(*powerv1.PowerWorkload)
		*node = *pwrWorkloadObj
	})
	mkcl.On("List", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client list error"))
	mkcl.On("Status").Return(mkwriter)
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
	mkcl.On("Status").Return(mkwriter)
	r.Client = mkcl

	sharedPowerWorkloadName = "shared"
	req.Name = workloadName
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "client delete error")
}

// uses dummy sysfs so must be run in isolation from other fuzzers
// go test -fuzz FuzzPowerWorkloadController -run=FuzzPowerWorkloadController -parallel=1
func FuzzPowerWorkloadController(f *testing.F) {
	f.Add("TestNode", "performance", 3600, 3200, "performance", "powersave", false, false, uint(44), "performance", uint(1), uint(5), uint(2), uint(7))
	f.Fuzz(func(t *testing.T, nodeName string, prof string, maxVal int, minVal int, epp string, governor string, shared bool, allcores bool, reservedCore uint, reservedProfile string, wCore1 uint, wCore2 uint, nCore1, nCore2 uint) {
		nodeName = strings.ReplaceAll(nodeName, " ", "")
		nodeName = strings.ReplaceAll(nodeName, "\t", "")
		nodeName = strings.ReplaceAll(nodeName, "\000", "")
		if len(nodeName) == 0 {
			return
		}
		t.Setenv("NODE_NAME", nodeName)

		clientObjs := []runtime.Object{
			&powerv1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prof,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerProfileSpec{
					Name:     prof,
					Max:      maxVal,
					Min:      minVal,
					Epp:      epp,
					Governor: governor,
					Shared:   shared,
				},
			},
			&powerv1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prof + "-" + nodeName,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerWorkloadSpec{
					Name:     prof + "-" + nodeName,
					AllCores: allcores,
					ReservedCPUs: []powerv1.ReservedSpec{
						{
							Cores:        []uint{reservedCore},
							PowerProfile: reservedProfile,
						},
					},
					PowerNodeSelector: map[string]string{"kubernetes.io/hostname": nodeName},
					Node: powerv1.WorkloadNode{
						Name: nodeName,
						Containers: []powerv1.Container{
							{
								Name:          "test-container-1",
								ExclusiveCPUs: []uint{wCore1, wCore2},
								PowerProfile:  prof,
							},
						},
						CpuIds: []uint{nCore1, nCore2},
					},
					PowerProfile: prof,
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
		r, err := createWorkloadReconcilerObject(clientObjs)
		assert.Nil(t, err)
		host, teardown, err := fullDummySystem()
		assert.Nil(t, err)
		defer teardown()
		r.PowerLibrary = host
		host.GetReservedPool().SetCpuIDs([]uint{})
		host.AddExclusivePool(prof)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      prof + "-" + nodeName,
				Namespace: IntelPowerNamespace,
			},
		}

		r.Reconcile(context.TODO(), req)

	})
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
