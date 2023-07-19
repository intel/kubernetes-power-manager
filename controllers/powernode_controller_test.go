package controllers

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createPowerNodeReconcilerObject(objs []runtime.Object) (*PowerNodeReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &PowerNodeReconciler{cl, ctrl.Log.WithName("testing"), s, nil}

	return r, nil
}

var defaultNode = &powerv1.PowerNode{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "TestNode",
		Namespace: IntelPowerNamespace,
	},
	Spec: powerv1.PowerNodeSpec{},
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
		Name:      "performance-Testnode",
		Namespace: IntelPowerNamespace,
	},
	Spec: powerv1.PowerWorkloadSpec{
		Name:         "performance-Testnode",
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

//tests scenarios where the powernode is incorrect or does not exist
func TestPowerNodeInvalidNodes(t *testing.T) {
	tcases := []struct {
		testCase      string
		nodeName      string
		powerNodeName string
		clientObjs    []runtime.Object
	}{
		defaultObs,
		{
			testCase:      "Test Case 1 - PowerNode not for this Node",
			nodeName:      "TestNode",
			powerNodeName: "DifferentNode",
			clientObjs: []runtime.Object{
				&powerv1.PowerNode{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "DifferentNode",
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
	pool, err := dummyFilesystemHost.AddExclusivePool("performance-Testnode")
	assert.Nil(t, err)
	prof, err := power.NewPowerProfile("performance", 10000, 10000, "powersave", "")
	assert.Nil(t, err)
	pool.SetPowerProfile(prof)
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createPowerNodeReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating reconciler object", tc.testCase)
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
			t.Errorf("%s Failed - expected reconciler to not have failed", tc.testCase)
		}
	}
}

// test to check for errors returned by the client
// uses an errclient which mocks client calls
func TestPowerNodeClientErrs(t *testing.T) {
	tcases := []struct {
		testCase      string
		nodeName      string
		powerNodeName string
		convertClient func(client.Client) client.Client
		clientErr     string
	}{
		{
			testCase:      "Test Case 1 - Invalid Get requests",
			nodeName:      "TestNode",
			powerNodeName: "TestNode",
			convertClient: func(c client.Client) client.Client {
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
			convertClient: func(c client.Client) client.Client {
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
			convertClient: func(c client.Client) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					node := args.Get(2).(*powerv1.PowerNode)
					*node = *defaultNode
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
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createPowerNodeReconcilerObject([]runtime.Object{})
		assert.Nil(t, err)
		r.PowerLibrary = dummyFilesystemHost
		r.Client = tc.convertClient(r.Client)
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

// positive and negative test casses for the SetupWithManager function
func TestPowerNodeReconcileSetupPass(t *testing.T) {
	r, err := createPowerNodeReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(v1alpha1.ControllerConfigurationSpec{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	err = (&PowerNodeReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}
func TestPowerNodeReconcileSetupFail(t *testing.T) {
	r, err := createPowerNodeReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr, _ := ctrl.NewManager(&rest.Config{}, ctrl.Options{
		Scheme: scheme.Scheme,
	})

	err = (&PowerNodeReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
