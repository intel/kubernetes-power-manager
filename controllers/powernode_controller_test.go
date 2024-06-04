package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/podstate"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createPowerNodeReconcilerObject(objs []runtime.Object) (*PowerNodeReconciler, error) {
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
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
	state, err := podstate.NewState()
	if err != nil {
		return nil, err
	}
	// Create a ReconcileNode object with the scheme and fake client.
	r := &PowerNodeReconciler{cl, ctrl.Log.WithName("testing"), s, state, map[string]corev1.Pod{}, nil}

	return r, nil
}

var defaultNode = &powerv1.PowerNode{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "TestNode",
		Namespace: IntelPowerNamespace,
	},
	Spec: powerv1.PowerNodeSpec{
		CustomDevices: []string{"device-plugin"},
	},
}
var defaultProf = &powerv1.PowerProfile{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "perfromance",
		Namespace: IntelPowerNamespace,
	},
	Spec: powerv1.PowerProfileSpec{
		Name: "performance",
		Epp:  "",
		Max:  3600,
		Min:  3200,
	},
}
var defaultWload = &powerv1.PowerWorkload{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "performance-TestNode",
		Namespace: IntelPowerNamespace,
	},
	Spec: powerv1.PowerWorkloadSpec{
		Name:         "performance-TestNode",
		PowerProfile: "performance",
		Node: powerv1.WorkloadNode{
			Name: "TestNode",
			Containers: []powerv1.Container{
				{Name: "fake container"},
			},
		},
	},
}
var defaultObs = struct {
	testCase      string
	nodeName      string
	powerNodeName string
	clientObjs    []runtime.Object
}{
	testCase:      "Test Case 1 - Default Scenario",
	nodeName:      "TestNode",
	powerNodeName: "TestNode",
	clientObjs: []runtime.Object{
		defaultNode,
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
		defaultProf,
		defaultWload,
	},
}

var nodeGuaranteedPod = powerv1.GuaranteedPod{
	Node:      "TestNode",
	Name:      "test-pod-1",
	Namespace: IntelPowerNamespace,
	UID:       "abcdefg",
	Containers: []powerv1.Container{
		{
			Name:          "test-container-1",
			Id:            "abcdefg",
			Pod:           "test-pod-1",
			ExclusiveCPUs: []uint{3, 4},
			PowerProfile:  "performance",
			Workload:      "performance-TestNode",
		},
	},
}

// tests scenarios where the power node is incorrect or does not exist
func TestPowerNode_Reconcile(t *testing.T) {
	tcases := []struct {
		testCase      string
		nodeName      string
		powerNodeName string
		clientObjs    []runtime.Object
	}{
		defaultObs,
		{
			testCase:      "Test Case 1 - default test case",
			nodeName:      "TestNode",
			powerNodeName: "TestNode",
			clientObjs: []runtime.Object{
				&powerv1.PowerNode{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerNodeSpec{},
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
			testCase:      "Test Case 2 - Node not found",
			nodeName:      "TestNode",
			powerNodeName: "TestNode",
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
	dummyFilesystemHost, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	pool, err := dummyFilesystemHost.AddExclusivePool("performance-TestNode")
	assert.Nil(t, err)
	prof, err := power.NewPowerProfile("performance", 1000, 1000, "powersave", "")
	assert.Nil(t, err)
	err = dummyFilesystemHost.GetSharedPool().SetPowerProfile(prof)
	assert.Nil(t, err)
	err = dummyFilesystemHost.GetSharedPool().SetCpuIDs([]uint{2})
	assert.Nil(t, err)
	pool.SetPowerProfile(prof)
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createPowerNodeReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s failed: error creating reconciler object", tc.testCase)
		}
		r.PowerLibrary = dummyFilesystemHost
		assert.Nil(t, err)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.powerNodeName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Errorf("%s failed: expected reconciler to not have failed", tc.testCase)
		}
	}
}

// test to check for errors returned by the client
// uses an errclient which mocks client calls
func TestPowerNode_Reconcile_ClientErrs(t *testing.T) {
	tcases := []struct {
		testCase      string
		nodeName      string
		powerNodeName string
		convertClient func(client.Client, map[string]corev1.Pod) client.Client
		clientErr     string
	}{
		{
			testCase:      "Test Case 1 - Invalid Get requests",
			nodeName:      "TestNode",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client, orphanedPods map[string]corev1.Pod) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client get error"))
				return mkcl
			},
			clientErr: "client get error",
		},
		{
			testCase:      "Test Case 2 - Invalid List requests",
			nodeName:      "TestNode",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client, orphanedPods map[string]corev1.Pod) client.Client {
				mkcl := new(errClient)
				//returns power node
				mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					node := args.Get(2).(*powerv1.PowerNode)
					*node = *defaultNode
				})
				mkcl.On("List", mock.Anything, mock.Anything).Return(fmt.Errorf("client list error"))
				return mkcl
			},
			clientErr: "client list error",
		},
		{
			testCase:      "Test Case 3 - Invalid Update requests",
			nodeName:      "TestNode",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client, orphanedPods map[string]corev1.Pod) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerNode")).Return(nil).Run(func(args mock.Arguments) {
					node := args.Get(2).(*powerv1.PowerNode)
					*node = *defaultNode
				})
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.Pod")).Return(nil).Run(func(args mock.Arguments) {
					pod := args.Get(2).(*corev1.Pod)
					*pod = corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "test-pod-1",
							Namespace:   IntelPowerNamespace,
							UID:         "abcdefg",
							Annotations: make(map[string]string),
						},
					}
				})
				mkcl.On("List", mock.Anything, mock.AnythingOfType("*v1.PowerProfileList")).Return(nil).Run(func(args mock.Arguments) {
					profList := args.Get(1).(*powerv1.PowerProfileList)
					*profList = powerv1.PowerProfileList{Items: []powerv1.PowerProfile{*defaultProf}}
				})
				mkcl.On("List", mock.Anything, mock.AnythingOfType("*v1.PowerWorkloadList")).Return(nil).Run(func(args mock.Arguments) {
					wList := args.Get(1).(*powerv1.PowerWorkloadList)
					*wList = powerv1.PowerWorkloadList{Items: []powerv1.PowerWorkload{*defaultWload}}
				})
				mkcl.On("Update", mock.Anything, mock.AnythingOfType("*v1.Pod")).Return(nil)
				mkcl.On("Update", mock.Anything, mock.Anything).Return(fmt.Errorf("client update error"))
				return mkcl
			},
			clientErr: "client update error",
		},
		{
			testCase:      "Test Case 4 - Invalid pod Update requests",
			nodeName:      "TestNode",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client, orphanedPods map[string]corev1.Pod) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerNode")).Return(nil).Run(func(args mock.Arguments) {
					node := args.Get(2).(*powerv1.PowerNode)
					*node = *defaultNode
				})
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.Pod")).Return(nil).Run(func(args mock.Arguments) {
					pod := args.Get(2).(*corev1.Pod)
					*pod = corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "test-pod-1",
							Namespace:   IntelPowerNamespace,
							UID:         "abcdefg",
							Annotations: map[string]string{"PM-updated": "0"},
						},
					}
					orphanedPods[guaranteedPod.Name] = *pod
				})
				mkcl.On("List", mock.Anything, mock.AnythingOfType("*v1.PowerProfileList")).Return(nil).Run(func(args mock.Arguments) {
					profList := args.Get(1).(*powerv1.PowerProfileList)
					*profList = powerv1.PowerProfileList{Items: []powerv1.PowerProfile{*defaultProf}}
				})
				mkcl.On("List", mock.Anything, mock.AnythingOfType("*v1.PowerWorkloadList")).Return(nil).Run(func(args mock.Arguments) {
					wList := args.Get(1).(*powerv1.PowerWorkloadList)
					*wList = powerv1.PowerWorkloadList{Items: []powerv1.PowerWorkload{*defaultWload}}
				})
				mkcl.On("Update", mock.Anything, mock.Anything).Return(fmt.Errorf("client update error"))
				return mkcl
			},
			clientErr: "client update error",
		},
	}
	dummyFilesystemHost, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	pool, err := dummyFilesystemHost.AddExclusivePool("performance")
	assert.Nil(t, err)
	prof, err := power.NewPowerProfile("performance", 10000, 10000, "powersave", "")
	assert.Nil(t, err)
	pool.SetPowerProfile(prof)
	err = dummyFilesystemHost.GetSharedPool().SetPowerProfile(prof)
	assert.Nil(t, err)
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createPowerNodeReconcilerObject([]runtime.Object{})
		r.State.UpdateStateGuaranteedPods(nodeGuaranteedPod)
		assert.Nil(t, err)
		r.PowerLibrary = dummyFilesystemHost
		r.Client = tc.convertClient(r.Client, r.OrphanedPods)
		assert.Nil(t, err)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.powerNodeName,
				Namespace: IntelPowerNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		assert.ErrorContains(t, err, tc.clientErr)
	}
}

// go test -fuzz FuzzPowerNodeController -run=FuzzPowerNodeController -parallel=1
func FuzzPowerNodeController(f *testing.F) {
	f.Add("TestNode", "some-plugin", "performance", "balance-performance", "balance-power", "perfromance-TestNode", "shared-TestNode", "0-44", "reserved-TestNode")
	f.Fuzz(func(t *testing.T, nodeName string, devicePlugin string, prof1 string, prof2 string, prof3 string, workload string, sharedPool string, unaffectedCores string, reservedPools string) {
		nodeName = strings.ReplaceAll(nodeName, " ", "")
		nodeName = strings.ReplaceAll(nodeName, "\t", "")
		nodeName = strings.ReplaceAll(nodeName, "\000", "")
		if len(nodeName) == 0 {
			return
		}
		t.Setenv("NODE_NAME", nodeName)
		container := powerv1.Container{
			Name:          "test-container-1",
			Id:            "abcdefg",
			Pod:           "test-pod-1",
			ExclusiveCPUs: []uint{1, 2, 3},
			PowerProfile:  prof1,
			Workload:      prof1 + nodeName,
		}
		clientObjs := []runtime.Object{
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
			&powerv1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prof1,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerProfileSpec{
					Name: prof1,
				},
			},
			&powerv1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prof2,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerProfileSpec{
					Name: prof2,
				},
			},
			&powerv1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prof3,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerProfileSpec{
					Name: prof3,
				},
			},
			&powerv1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prof1 + "-" + nodeName,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerWorkloadSpec{
					Name:         prof1 + "-" + nodeName,
					PowerProfile: prof1,
					Node: powerv1.WorkloadNode{
						Name:   nodeName,
						CpuIds: []uint{1, 2, 3},
					},
				},
			},
			&powerv1.PowerNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerNodeSpec{
					CustomDevices:   []string{devicePlugin},
					PowerProfiles:   []string{prof1, prof2, prof3},
					PowerWorkloads:  []string{workload},
					SharedPool:      sharedPool,
					UnaffectedCores: unaffectedCores,
					ReservedPools:   []string{reservedPools},
					PowerContainers: []powerv1.Container{
						container,
					},
				},
			},
		}
		pod := powerv1.GuaranteedPod{
			Node:      "TestNode",
			Name:      "test-pod-1",
			Namespace: IntelPowerNamespace,
			UID:       "abcdefg",
			Containers: []powerv1.Container{
				container,
			},
		}
		dummyFilesystemHost, teardown, err := fullDummySystem()
		assert.Nil(t, err)
		defer teardown()

		pool, err1 := dummyFilesystemHost.AddExclusivePool(prof1)
		profile, err2 := power.NewPowerProfile(prof1, 10000, 10000, "powersave", "")
		// continue test without pools
		if err1 == nil && err2 == nil {
			pool.SetPowerProfile(profile)
			dummyFilesystemHost.GetSharedPool().SetPowerProfile(profile)
			dummyFilesystemHost.GetSharedPool().MoveCpuIDs([]uint{0, 1, 2, 3, 4, 5})
			pool.MoveCpuIDs([]uint{1, 2, 3})
			pool, err = dummyFilesystemHost.AddExclusivePool(prof2)
			if err !=nil {
				return
			}
			profile, err = power.NewPowerProfile(prof1, 10000, 10000, "powersave", "")
			if err !=nil {
				return
			}
			pool.SetPowerProfile(profile)
			pool, err = dummyFilesystemHost.AddExclusivePool(prof3)
			if err !=nil {
				return
			}
			profile, err = power.NewPowerProfile(prof1, 10000, 10000, "powersave", "")
			if err !=nil {
				return
			}
			pool.SetPowerProfile(profile)
		}

		r, err := createPowerNodeReconcilerObject(clientObjs)
		assert.Nil(t, err)
		r.State.UpdateStateGuaranteedPods(pod)
		r.PowerLibrary = dummyFilesystemHost

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      nodeName,
				Namespace: IntelPowerNamespace,
			},
		}

		r.Reconcile(context.TODO(), req)

	})
}

// positive and negative test casses for the SetupWithManager function
func TestPowerNode_Reconcile_SetupPass(t *testing.T) {
	r, err := createPowerNodeReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(cacheMk))
	err = (&PowerNodeReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}

func TestPowerNode_Reconcile_SetupFail(t *testing.T) {
	r, err := createPowerNodeReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("setup fail"))

	err = (&PowerNodeReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
