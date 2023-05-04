package controllers

import (
	"context"
	"strings"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createUncoreReconcilerObject(objs []runtime.Object) (*UncoreReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(s).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &UncoreReconciler{cl, ctrl.Log.WithName("testing"), scheme.Scheme, nil}

	return r, nil
}

func TestDieSelectMinError(t *testing.T) {
	t.Setenv("NODE_NAME", "")
	max := uint(2400000)
	pkg := uint(0)
	die := uint(0)
	uncoreName := ""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Max: &max},
				},
			},
		},
	}

	hostmk := new(hostMock)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary = hostmk
	assert.NoError(t, err)
	mocktop := new(mockCpuTopology)
	mockpkg := new(mockCpuPackage)
	mockdie := new(mockCpuDie)
	pkglst := mockpkg.MakeList()
	dielst := mockdie.MakeList()
	mocktop.On("SetUncore", mock.Anything).Return(nil)
	hostmk.On("Topology").Return(mocktop)
	mocktop.On("Packages").Return(&pkglst)
	mocktop.On("Package", mock.Anything).Return(mockpkg)
	mockpkg.On("SetUncore", mock.Anything).Return(nil)

	mockpkg.On("Dies").Return(&dielst)
	mockpkg.On("Die", mock.Anything).Return(mockdie)
	mockdie.On("SetUncore", mock.Anything).Return(nil)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      uncoreName,
			Namespace: "intel-power",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Error(t, err)
	hostmk.AssertExpectations(t)
}

func TestDieSelectMaxError(t *testing.T) {
	t.Setenv("NODE_NAME", "")
	min := uint(1200000)
	pkg := uint(0)
	die := uint(0)
	uncoreName := ""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Min: &min},
				},
			},
		},
	}

	hostmk := new(hostMock)
	mocktop := new(mockCpuTopology)
	mockpkg := new(mockCpuPackage)
	mockdie := new(mockCpuDie)
	pkglst := mockpkg.MakeList()
	dielst := mockdie.MakeList()
	mocktop.On("SetUncore", mock.Anything).Return(nil)
	hostmk.On("Topology").Return(mocktop)
	mocktop.On("Packages").Return(&pkglst)
	mocktop.On("Package", mock.Anything).Return(mockpkg)
	mockpkg.On("SetUncore", mock.Anything).Return(nil)

	mockpkg.On("Dies").Return(&dielst)
	mockpkg.On("Die", mock.Anything).Return(mockdie)
	mockdie.On("SetUncore", mock.Anything).Return(nil)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary = hostmk
	assert.NoError(t, err)
	// hostmk.On("Topology").Return(power.Topology)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      uncoreName,
			Namespace: "intel-power",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Error(t, err)
	hostmk.AssertExpectations(t)
}
func TestDieSelectPackageError(t *testing.T) {
	t.Setenv("NODE_NAME", "")
	max := uint(2400000)
	min := uint(1200000)
	die := uint(0)
	uncoreName := ""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Die: &die, Max: &max, Min: &min},
				},
			},
		},
	}

	hostmk := new(hostMock)
	mocktop := new(mockCpuTopology)
	mockpkg := new(mockCpuPackage)
	mockdie := new(mockCpuDie)
	pkglst := mockpkg.MakeList()
	dielst := mockdie.MakeList()
	mocktop.On("SetUncore", mock.Anything).Return(nil)
	hostmk.On("Topology").Return(mocktop)
	mocktop.On("Packages").Return(&pkglst)
	mocktop.On("Package", mock.Anything).Return(mockpkg)
	mockpkg.On("SetUncore", mock.Anything).Return(nil)

	mockpkg.On("Dies").Return(&dielst)
	mockpkg.On("Die", mock.Anything).Return(mockdie)
	mockdie.On("SetUncore", mock.Anything).Return(nil)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary = hostmk
	assert.NoError(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      uncoreName,
			Namespace: "intel-power",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Error(t, err)
	hostmk.AssertExpectations(t)
}
func TestNoSelectorsOrSystemValues(t *testing.T) {
	t.Setenv("NODE_NAME", "")
	uncoreName := ""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{},
		},
	}

	hostmk := new(hostMock)
	mocktop := new(mockCpuTopology)
	mockpkg := new(mockCpuPackage)
	mockdie := new(mockCpuDie)
	pkglst := mockpkg.MakeList()
	dielst := mockdie.MakeList()
	mocktop.On("SetUncore", mock.Anything).Return(nil)
	hostmk.On("Topology").Return(mocktop)
	mocktop.On("Packages").Return(&pkglst)
	mocktop.On("Package", mock.Anything).Return(mockpkg)
	mockpkg.On("SetUncore", mock.Anything).Return(nil)

	mockpkg.On("Dies").Return(&dielst)
	mockpkg.On("Die", mock.Anything).Return(mockdie)
	mockdie.On("SetUncore", mock.Anything).Return(nil)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary = hostmk
	assert.NoError(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      uncoreName,
			Namespace: "intel-power",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Error(t, err)
	hostmk.AssertExpectations(t)
}

func FuzzUncoreReconciler(f *testing.F) {
	hostmk := new(hostMock)
	mocktop := new(mockCpuTopology)
	mockpkg := new(mockCpuPackage)
	mockdie := new(mockCpuDie)
	pkglst := mockpkg.MakeList()
	dielst := mockdie.MakeList()
	mocktop.On("SetUncore", mock.Anything).Return(nil)
	hostmk.On("Topology").Return(mocktop)
	mocktop.On("Packages").Return(&pkglst)
	mocktop.On("Package", mock.Anything).Return(mockpkg)
	mockpkg.On("SetUncore", mock.Anything).Return(nil)

	mockpkg.On("Dies").Return(&dielst)
	mockpkg.On("Die", mock.Anything).Return(mockdie)
	mockdie.On("SetUncore", mock.Anything).Return(nil)

	f.Fuzz(func(t *testing.T, nodeName string, namespace string, extraNode bool, node2Name string, runningOnTargetNode bool) {

		r, req := setupUncoreFuzz(t, nodeName, namespace, extraNode, node2Name, runningOnTargetNode, hostmk)
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

func setupUncoreFuzz(t *testing.T, nodeName string, namespace string, extraNode bool, node2Name string, runningOnTargetNode bool, powerLib power.Host) (*UncoreReconciler, reconcile.Request) {
	max := uint(2400000)
	min := uint(1200000)
	die_and_package := uint(0)
	diemin := min - 10000
	diemax := max - 10000
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
	//prevents empty and null terminated strings
	nodeName = strings.ReplaceAll(nodeName, " ", "")
	node2Name = strings.ReplaceAll(node2Name, " ", "")
	nodeName = strings.ReplaceAll(nodeName, "\t", "")
	node2Name = strings.ReplaceAll(node2Name, "\t", "")
	nodeName = strings.ReplaceAll(nodeName, "\000", "")
	node2Name = strings.ReplaceAll(node2Name, "\000", "")
	if len(nodeName) == 0 || len(node2Name) == 0 {
		return nil, req
	}
	t.Logf("%t=%s- len: %d %t", runningOnTargetNode, node2Name, len(node2Name), node2Name == "\000")
	t.Logf("nodename %v- len %d", nodeName, len(nodeName))
	t.Logf("nodename2 %v- len %d", []byte(node2Name), len(node2Name))

	if runningOnTargetNode {
		t.Setenv("NODE_NAME", nodeName)
	} else {
		t.Setenv("NODE_NAME", node2Name)
	}
	UncoreObj := &powerv1.Uncore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: namespace,
		},
		Spec: powerv1.UncoreSpec{
			SysMax: &max,
			SysMin: &min,
			DieSelectors: &[]powerv1.DieSelector{
				{Package: &die_and_package, Die: &die_and_package, Max: &diemax, Min: &diemin},
			},
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
				Name: node2Name,
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

	objs := []runtime.Object{UncoreObj, powerProfilesObj, powerNodesObj}
	rec, _ := createUncoreReconcilerObject(objs)
	rec.PowerLibrary = powerLib
	return rec, req
}
