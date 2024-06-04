package controllers

import (
	"context"
	"fmt"
	"strconv"
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

const (
	sharedPoolErrorMessage               string = "shared pool set-up failed during %s"
	exclusivePoolErrorMessage            string = "exclusive pool %s set-up failed during %s"
	individualPoolErrorMessage           string = "individual CPU %s set-up failed during %s"
	invalidIntegerInCoreNameErrorMessage string = "%s must be an integer core ID"
	invalidCoreIdErrorMessage            string = "invalid core id ID: %s"
	controllerVerifyMethodName           string = "verify"
	controllerRestoreMethodName          string = "restore"
	controllerApplyMethodName            string = "apply"
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

func initializeMocksForPartialHandling(hasSharedPool bool, exclusivePools, individualCores map[string]map[string]bool,
	invalidIndividualCores map[string]bool) (*poolMock, map[string]*poolMock, map[string]*coreMock, *hostMock) {
	var sharedPoolMock *poolMock
	powerLibMock := new(hostMock)

	if hasSharedPool {
		sharedPoolMock = new(poolMock)
		powerLibMock.On("GetSharedPool").Return(sharedPoolMock)
	}

	mockedExclusivePoolMappings := make(map[string]*poolMock)
	mockedPoolList := power.PoolList{}

	for poolName := range exclusivePools {
		mockedPool := new(poolMock)
		mockedPool.On("Name").Return(poolName)
		powerLibMock.On("GetExclusivePool", poolName).Return(mockedPool)
		mockedExclusivePoolMappings[poolName] = mockedPool
		mockedPoolList = append(mockedPoolList, mockedPool)
	}

	powerLibMock.On("GetAllExclusivePools").Return(&mockedPoolList)

	mockedCPUMappings := make(map[string]*coreMock)
	mockedCPUList := power.CpuList{}
	for cpuID := range individualCores {
		mockedCore := new(coreMock)
		coreIDInt, _ := strconv.Atoi(cpuID)
		mockedCore.On("GetID").Return(uint(coreIDInt))
		mockedCPUMappings[cpuID] = mockedCore
		// we ignore the invalid cores as this will trigger
		// L182 in the controller. We'll be in a situation where we have
		// CPU cores that are not recognized by the library
		// which is used to trigger test case  11 below
		if !invalidIndividualCores[cpuID] {
			mockedCPUList = append(mockedCPUList, mockedCore)
		}
	}
	powerLibMock.On("GetAllCpus").Return(&mockedCPUList)

	return sharedPoolMock, mockedExclusivePoolMappings, mockedCPUMappings, powerLibMock
}

func assignMockMethodsForSharedPoolHandling(mock *hostMock, sharedPoolMock *poolMock,
	reconcileMethodAtError string, sharedPoolCStates map[string]bool, withSharedPoolError bool) (*hostMock, *poolMock) {

	if withSharedPoolError && reconcileMethodAtError == controllerVerifyMethodName {
		mock.On("ValidateCStates", power.CStates(sharedPoolCStates)).
			Return(errors.NewBadRequest(fmt.Sprintf(sharedPoolErrorMessage, controllerVerifyMethodName))).
			Once()
	} else {
		mock.On("ValidateCStates", power.CStates(sharedPoolCStates)).Return(nil).Once()
	}

	if withSharedPoolError && reconcileMethodAtError == controllerRestoreMethodName {
		sharedPoolMock.On("SetCStates", power.CStates(nil)).
			Return(errors.NewBadRequest(fmt.Sprintf(sharedPoolErrorMessage, controllerRestoreMethodName)))
	} else {
		sharedPoolMock.On("SetCStates", power.CStates(nil)).Return(error(nil))
	}

	if withSharedPoolError && reconcileMethodAtError == controllerApplyMethodName {
		sharedPoolMock.On("SetCStates", power.CStates(sharedPoolCStates)).
			Return(errors.NewBadRequest(fmt.Sprintf(sharedPoolErrorMessage, controllerApplyMethodName)))
	} else {
		sharedPoolMock.On("SetCStates", power.CStates(sharedPoolCStates)).Return(error(nil))
	}

	return mock, sharedPoolMock
}

func assignMockMethodsForExclPoolHandling(mock *hostMock, exclusivePoolMocks map[string]*poolMock,
	reconcileMethodAtError string, exclusivePoolCStates map[string]map[string]bool,
	exclusivePoolsWithError map[string]bool) (*hostMock, map[string]*poolMock) {
	for poolName, mockedPool := range exclusivePoolMocks {
		if reconcileMethodAtError == controllerVerifyMethodName && exclusivePoolsWithError[poolName] {
			mock.On("ValidateCStates", power.CStates(exclusivePoolCStates[poolName])).
				Return(errors.NewBadRequest(
					fmt.Sprintf(exclusivePoolErrorMessage, poolName, controllerVerifyMethodName)))
		} else {
			mock.On("ValidateCStates", power.CStates(exclusivePoolCStates[poolName])).
				Return(nil)
		}

		if reconcileMethodAtError == controllerRestoreMethodName && exclusivePoolsWithError[poolName] {
			mockedPool.On("SetCStates", power.CStates(nil)).
				Return(errors.NewBadRequest(
					fmt.Sprintf(exclusivePoolErrorMessage, poolName, controllerRestoreMethodName)))
		} else {
			mockedPool.On("SetCStates", power.CStates(nil)).Return(error(nil))
		}

		if reconcileMethodAtError == controllerApplyMethodName && exclusivePoolsWithError[poolName] {
			mockedPool.On("SetCStates", power.CStates(exclusivePoolCStates[poolName])).
				Return(errors.NewBadRequest(
					fmt.Sprintf(exclusivePoolErrorMessage, poolName, controllerApplyMethodName)))
		} else {
			mockedPool.On("SetCStates", power.CStates(exclusivePoolCStates[poolName])).
				Return(error(nil))
		}

	}

	return mock, exclusivePoolMocks
}

func assignMockMethodsForIndvdPoolHandling(mock *hostMock, individualPoolMocks map[string]*coreMock,
	reconcileMethodAtError string, individualPoolCStates map[string]map[string]bool,
	individualPoolsWithErrors map[string]bool) (*hostMock, map[string]*coreMock) {

	for cpuID, mockedPool := range individualPoolMocks {
		if reconcileMethodAtError == controllerVerifyMethodName && individualPoolsWithErrors[cpuID] {
			mock.On("ValidateCStates", power.CStates(individualPoolCStates[cpuID])).
				Return(errors.NewBadRequest(
					fmt.Sprintf(individualPoolErrorMessage, cpuID, controllerVerifyMethodName)))
		} else {
			mock.On("ValidateCStates", power.CStates(individualPoolCStates[cpuID])).
				Return(nil)
		}

		if reconcileMethodAtError == controllerRestoreMethodName && individualPoolsWithErrors[cpuID] {
			mockedPool.On("SetCStates", power.CStates(nil)).
				Return(errors.NewBadRequest(
					fmt.Sprintf(individualPoolErrorMessage, cpuID, controllerRestoreMethodName)))
		} else {
			mockedPool.On("SetCStates", power.CStates(nil)).Return(error(nil))
		}

		if reconcileMethodAtError == controllerApplyMethodName && individualPoolsWithErrors[cpuID] {
			mockedPool.On("SetCStates", power.CStates(individualPoolCStates[cpuID])).
				Return(errors.NewBadRequest(
					fmt.Sprintf(individualPoolErrorMessage, cpuID, controllerApplyMethodName)))
		} else {
			mockedPool.On("SetCStates", power.CStates(individualPoolCStates[cpuID])).
				Return(error(nil))
		}

	}

	return mock, individualPoolMocks
}

func TestCStates_Reconcile_WithPartialSetup(t *testing.T) {
	twoPowerNodeSetup := &powerv1.PowerNodeList{
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
	twoProfileSetup := &powerv1.PowerProfileList{
		Items: []powerv1.PowerProfile{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "balance-performance",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "performance",
				},
			},
		},
	}

	// please make sure to not use the same c-state config for more that one CPUID/exclusive pool name
	// as the test logic can't handle this well: i.e. "1": {"C1": true}, "2": {"C3": true}, "3": {"C1": true}
	// CPUs 1,3 have the same C-states. The controller code will handle them correctly, but we are unable
	// to properly mock such calls (have two consecutive calls to the same method return different errors)
	cStatesSetup := &powerv1.CStates{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1",
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.CStatesSpec{
			SharedPoolCStates:     power.CStates{"C1": true},
			ExclusivePoolCStates:  map[string]map[string]bool{"performance": {"C2": false}, "balance-performance": {"C1": true, "C3": true}},
			IndividualCoreCStates: map[string]map[string]bool{"0": {"C1": true, "C2": true}, "1": {"C1": true}, "2": {"C3": true}, "3": {"C0": true, "C1": true}, "4": {"C4": true}},
		},
	}

	cStatesSetupWithInvalidCPUIds := &powerv1.CStates{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1",
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.CStatesSpec{
			SharedPoolCStates:     power.CStates{"C1": true},
			ExclusivePoolCStates:  map[string]map[string]bool{"balance-performance": {"C2": false}},
			IndividualCoreCStates: map[string]map[string]bool{"C0": {"C1": true, "C2": true}, "1": {"C1": true}},
		},
	}

	cStateControllerInputs := []struct {
		name                     string
		cStatesSetup             *powerv1.CStates
		powerNodesSetup          *powerv1.PowerNodeList
		powerProfileSetup        *powerv1.PowerProfileList
		withSharedPoolError      bool
		exclusivePoolsWithError  map[string]bool
		individualPoolsWithError map[string]bool
		reconcileMethodAtError   string
		invalidIndividualCores   map[string]bool
		expectError              bool
		reconcileErrorMessages   []string
	}{
		{
			name:                   "Test 1 - verify fails with shared pool error",
			cStatesSetup:           cStatesSetup,
			powerNodesSetup:        twoPowerNodeSetup,
			powerProfileSetup:      twoProfileSetup,
			withSharedPoolError:    true,
			reconcileMethodAtError: controllerVerifyMethodName,
			expectError:            true,
			reconcileErrorMessages: []string{fmt.Sprintf(sharedPoolErrorMessage, controllerVerifyMethodName)},
		},
		{
			name:                    "Test 2 - verify fails with shared & exclusive pool errors",
			cStatesSetup:            cStatesSetup,
			powerNodesSetup:         twoPowerNodeSetup,
			powerProfileSetup:       twoProfileSetup,
			withSharedPoolError:     true,
			exclusivePoolsWithError: map[string]bool{"performance": true},
			reconcileMethodAtError:  controllerVerifyMethodName,
			expectError:             true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(sharedPoolErrorMessage, controllerVerifyMethodName),
				fmt.Sprintf(exclusivePoolErrorMessage, "performance", controllerVerifyMethodName),
			},
		},
		{
			name:                     "Test 3 - verify fails with shared, exclusive & individual pool errors",
			cStatesSetup:             cStatesSetup,
			powerNodesSetup:          twoPowerNodeSetup,
			powerProfileSetup:        twoProfileSetup,
			withSharedPoolError:      true,
			exclusivePoolsWithError:  map[string]bool{"performance": true},
			individualPoolsWithError: map[string]bool{"2": true, "1": true, "4": true},
			reconcileMethodAtError:   controllerVerifyMethodName,
			expectError:              true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(sharedPoolErrorMessage, controllerVerifyMethodName),
				fmt.Sprintf(exclusivePoolErrorMessage, "performance", controllerVerifyMethodName),
				fmt.Sprintf(individualPoolErrorMessage, "2", controllerVerifyMethodName),
				fmt.Sprintf(individualPoolErrorMessage, "1", controllerVerifyMethodName),
				fmt.Sprintf(individualPoolErrorMessage, "4", controllerVerifyMethodName),
			},
		},
		{
			name:                   "Test 4 - restore fails with shared pool error",
			cStatesSetup:           cStatesSetup,
			powerNodesSetup:        twoPowerNodeSetup,
			powerProfileSetup:      twoProfileSetup,
			withSharedPoolError:    true,
			reconcileMethodAtError: controllerRestoreMethodName,
			expectError:            true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(sharedPoolErrorMessage, controllerRestoreMethodName),
			},
		},
		{
			name:                "Test 5 - restore fails with shared &  pool errors",
			cStatesSetup:        cStatesSetup,
			powerNodesSetup:     twoPowerNodeSetup,
			powerProfileSetup:   twoProfileSetup,
			withSharedPoolError: true,
			exclusivePoolsWithError: map[string]bool{
				"performance":         true,
				"balance-performance": true},
			reconcileMethodAtError: controllerRestoreMethodName,
			expectError:            true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(exclusivePoolErrorMessage, "performance", controllerRestoreMethodName),
				fmt.Sprintf(exclusivePoolErrorMessage, "balance-performance", controllerRestoreMethodName),
			},
		},
		{
			name:                "Test 6 - restore fails with shared & individual pool errors",
			cStatesSetup:        cStatesSetup,
			powerNodesSetup:     twoPowerNodeSetup,
			powerProfileSetup:   twoProfileSetup,
			withSharedPoolError: true,
			individualPoolsWithError: map[string]bool{
				"0": true, "3": true, "4": true},
			reconcileMethodAtError: controllerRestoreMethodName,
			expectError:            true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(sharedPoolErrorMessage, controllerRestoreMethodName),
				fmt.Sprintf(individualPoolErrorMessage, "0", controllerRestoreMethodName),
				fmt.Sprintf(individualPoolErrorMessage, "3", controllerRestoreMethodName),
				fmt.Sprintf(individualPoolErrorMessage, "4", controllerRestoreMethodName),
			},
		},
		{
			name:                   "Test 7 - apply fails with shared pool error",
			cStatesSetup:           cStatesSetup,
			powerNodesSetup:        twoPowerNodeSetup,
			powerProfileSetup:      twoProfileSetup,
			withSharedPoolError:    true,
			reconcileMethodAtError: controllerApplyMethodName,
			expectError:            true,
			reconcileErrorMessages: []string{fmt.Sprintf(sharedPoolErrorMessage, controllerApplyMethodName)},
		},
		{
			name:                    "Test 8 - apply fails with shared & exclusive pool errors",
			cStatesSetup:            cStatesSetup,
			powerNodesSetup:         twoPowerNodeSetup,
			powerProfileSetup:       twoProfileSetup,
			withSharedPoolError:     true,
			exclusivePoolsWithError: map[string]bool{"balance-performance": true},
			reconcileMethodAtError:  controllerApplyMethodName,
			expectError:             true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(sharedPoolErrorMessage, controllerApplyMethodName),
				fmt.Sprintf(exclusivePoolErrorMessage, "balance-performance", controllerApplyMethodName),
			},
		},
		{
			name:                     "Test 9 - verify fails with shared, exclusive and individual pool errors",
			cStatesSetup:             cStatesSetup,
			powerNodesSetup:          twoPowerNodeSetup,
			powerProfileSetup:        twoProfileSetup,
			withSharedPoolError:      true,
			exclusivePoolsWithError:  map[string]bool{"balance-performance": true},
			individualPoolsWithError: map[string]bool{"1": true},
			reconcileMethodAtError:   controllerApplyMethodName,
			expectError:              true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(sharedPoolErrorMessage, controllerApplyMethodName),
				fmt.Sprintf(exclusivePoolErrorMessage, "balance-performance", controllerApplyMethodName),
				fmt.Sprintf(individualPoolErrorMessage, "1", controllerApplyMethodName),
			},
		},
		{
			name:                    "Test 10 - apply fails with invalid cState name",
			cStatesSetup:            cStatesSetupWithInvalidCPUIds,
			powerNodesSetup:         twoPowerNodeSetup,
			powerProfileSetup:       twoProfileSetup,
			exclusivePoolsWithError: map[string]bool{"balance-performance": true},
			reconcileMethodAtError:  controllerApplyMethodName,
			expectError:             true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(exclusivePoolErrorMessage, "balance-performance", controllerApplyMethodName),
				fmt.Sprintf(invalidIntegerInCoreNameErrorMessage, "C0"),
			},
		},
		{
			name:                   "Test 11 - apply fails with non-existent cState in library",
			cStatesSetup:           cStatesSetup,
			powerNodesSetup:        twoPowerNodeSetup,
			powerProfileSetup:      twoProfileSetup,
			invalidIndividualCores: map[string]bool{"2": true, "4": true},
			expectError:            true,
			reconcileErrorMessages: []string{
				fmt.Sprintf(invalidCoreIdErrorMessage, "2"),
				fmt.Sprintf(invalidCoreIdErrorMessage, "4"),
			},
		},
	}

	for _, tt := range cStateControllerInputs {
		objs := []runtime.Object{
			tt.cStatesSetup, tt.powerProfileSetup, tt.powerNodesSetup,
		}

		sharedPoolMock, mockedExclusivePoolMappings, mockedIndividualCPUMappings, powerLibMock :=
			initializeMocksForPartialHandling(tt.cStatesSetup.Spec.SharedPoolCStates != nil,
				tt.cStatesSetup.Spec.ExclusivePoolCStates,
				tt.cStatesSetup.Spec.IndividualCoreCStates, tt.invalidIndividualCores)

		powerLibMock, sharedPoolMock = assignMockMethodsForSharedPoolHandling(powerLibMock, sharedPoolMock,
			tt.reconcileMethodAtError, tt.cStatesSetup.Spec.SharedPoolCStates, tt.withSharedPoolError)

		powerLibMock, mockedExclusivePoolMappings = assignMockMethodsForExclPoolHandling(powerLibMock, mockedExclusivePoolMappings,
			tt.reconcileMethodAtError, tt.cStatesSetup.Spec.ExclusivePoolCStates, tt.exclusivePoolsWithError)

		powerLibMock, mockedIndividualCPUMappings = assignMockMethodsForIndvdPoolHandling(powerLibMock, mockedIndividualCPUMappings,
			tt.reconcileMethodAtError, tt.cStatesSetup.Spec.IndividualCoreCStates, tt.individualPoolsWithError)

		r := buildCStatesReconcilerObject(objs, powerLibMock)
		assert.NotNil(t, r)
		ctx := context.Background()
		req := reconcile.Request{NamespacedName: client.ObjectKey{
			Namespace: IntelPowerNamespace,
			Name:      "node1",
		}}
		t.Setenv("NODE_NAME", "node1")

		result, err := r.Reconcile(ctx, req)

		if !tt.expectError {
			assert.NotNil(t, result)
			assert.NoError(t, err)
			powerLibMock.AssertExpectations(t)
			sharedPoolMock.AssertExpectations(t)
			for _, exclusivePoolMock := range mockedExclusivePoolMappings {
				exclusivePoolMock.AssertExpectations(t)
			}
			for _, individualCPUMock := range mockedIndividualCPUMappings {
				individualCPUMock.AssertExpectations(t)
			}
		} else {
			assert.NotNil(t, result)
			assert.Error(t, err)
			for _, message := range tt.reconcileErrorMessages {
				assert.True(t, strings.Contains(err.Error(), message))
			}

		}

	}
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
