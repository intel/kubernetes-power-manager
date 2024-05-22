package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
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

func createUncoreReconcilerObject(objs []runtime.Object) (*UncoreReconciler, error) {
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
	r := &UncoreReconciler{cl, ctrl.Log.WithName("testing"), scheme.Scheme, nil}

	return r, nil
}

// tests setting a system wide uncore
func TestUncore_Reconcile_SystemUncore(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	max := uint(2400000)
	min := uint(1200000)
	 	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				SysMax: &max,
				SysMin: &min,
			},
		},
	}
	r, err := createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	r.PowerLibrary = host
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodename,
			Namespace: "intel-power",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	err = checkUncoreValues("testing/cpus", "00", "00", fmt.Sprint(max), fmt.Sprint(min))
	assert.Nil(t, err)

}

// tests tuning a specific die
func TestUncore_Reconcile_TestDieTuning(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	max := uint(2200000)
	min := uint(1300000)
	pkg := uint(0)
	die := uint(1)
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err := createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	r.PowerLibrary = host
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodename,
			Namespace: "intel-power",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	err = checkUncoreValues("testing/cpus", "00", "01", fmt.Sprint(max), fmt.Sprint(min))
	assert.Nil(t, err)
	err = checkUncoreValues("testing/cpus", "00", "00", "2400000", "1200000")
	assert.Nil(t, err)
}

// tests tuning a specific package
func TestUncore_Reconcile_PackageTuning(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	max := uint(2200000)
	min := uint(1400000)
	pkg := uint(0)
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err := createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	r.PowerLibrary = host
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodename,
			Namespace: "intel-power",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	err = checkUncoreValues("testing/cpus", "00", "00", fmt.Sprint(max), fmt.Sprint(min))
	assert.Nil(t, err)
	err = checkUncoreValues("testing/cpus", "00", "01", fmt.Sprint(max), fmt.Sprint(min))
	assert.Nil(t, err)

}

// tests invalid uncore fields
func TestUncore_Reconcile_InvalidUncores(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	// uncore does not exist
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodename,
			Namespace: "intel-power",
		},
	}
	r, err := createUncoreReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	// mising min
	max := uint(2400000)
	pkg := uint(0)
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Max: &max},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host

	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "max, min and package fields must not be empty")
	// missing max
	min := uint(1200000)
	pkg = uint(0)
	die := uint(0)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "max, min and package fields must not be empty")
	// missing package
	max = uint(2400000)
	min = uint(1200000)
	die = uint(0)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Die: &die, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "max, min and package fields must not be empty")
	// no die selector or system wide values
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "no system wide or per die min/max values were provided")
	// no package or die on selector
	max = uint(2400000)
	min = uint(1200000)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Max: &max, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "max, min and package fields must not be empty")
	// no die selector or system wide values
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "no system wide or per die min/max values were provided")
	// package that does not exist
	max = uint(2400000)
	min = uint(1200000)
	pkg = uint(100000)
	die = uint(0)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "invalid package")
	// same but for package wide tuning
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "invalid package")
	// die that does not exist
	max = uint(2400000)
	min = uint(1200000)
	pkg = uint(0)
	die = uint(100000)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "invalid die")
	// large sys max
	max = uint(20000000000)
	min = uint(1200000)
	pkg = uint(0)
	die = uint(0)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				SysMax: &max,
				SysMin: &min,
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "specified Max frequency is higher than")
	// large package max
	max = uint(20000000000)
	min = uint(1200000)
	pkg = uint(0)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "specified Max frequency is higher than")
	// large die max
	max = uint(20000000000)
	min = uint(1200000)
	pkg = uint(0)
	die = uint(0)
	clientObjs = []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err = createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	r.PowerLibrary = host
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "specified Max frequency is higher than")
}

// tests requests for the wrong node and namespace
func TestUncore_Reconcile_InvalidRequests(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	// incorrect namespace
	r, err := createUncoreReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodename,
			Namespace: "somespace",
		},
	}
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "incorrect namespace")
	// incorrect node
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "wrong-node",
			Namespace: "intel-power",
		},
	}
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

}

// test for a file system with missing files (ie. some broken kernel module etc)
func TestUncore_Reconcile_InvalidFileSystem(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodename,
			Namespace: "intel-power",
		},
	}
	max := uint(2400000)
	min := uint(1200000)
	pkg := uint(0)
	die := uint(1)
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg, Die: &die, Max: &max, Min: &min},
				},
			},
		},
	}
	r, err := createUncoreReconcilerObject(clientObjs)
	assert.Nil(t, err)
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	r.PowerLibrary = host
	err = os.RemoveAll("./testing")
	assert.Nil(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "no such file or directory")

}

// tests failed client Get function call
func TestUncore_Reconcile_UnexpectedClientErr(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodename,
			Namespace: "intel-power",
		},
	}
	r, err := createUncoreReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	host, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	r.PowerLibrary = host

	mkwriter := new(mockResourceWriter)
	mkwriter.On("Update", mock.Anything, mock.Anything).Return(nil)
	mkcl := new(errClient)
	mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client get error"))
	mkcl.On("Status").Return(mkwriter)
	r.Client = mkcl
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "client get error")
}

// tests positive and negative cases for SetupWithManager function
func TestUncore_Reconcile_SetupPass(t *testing.T) {
	r, err := createUncoreReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(cacheMk))
	err = (&UncoreReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}

func TestUncore_Reconcile_SetupFail(t *testing.T) {
	r, err := createUncoreReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("setup fail"))

	err = (&UncoreReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}

// fuzzing function for uncore
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
	f.Add("node1", "intel-power", true, "node2", true, uint(1400000), uint(2400000), uint(0))
	_, teardown, err := fullDummySystem()
	assert.Nil(f, err)
	defer teardown()
	f.Fuzz(func(t *testing.T, nodeName string, namespace string, extraNode bool, node2Name string, runningOnTargetNode bool, min uint, max uint, die_and_package uint) {

		r, req := setupUncoreFuzz(t, nodeName, namespace, extraNode, node2Name, runningOnTargetNode, min, max, die_and_package, hostmk)
		if r == nil {
			// if r is nil setupFuzz must have panicked, so we ignore it
			return
		}
		r.Reconcile(context.Background(), req)

	})
}

// go test -fuzz FuzzUncoreReconciler -run=FuzzUncoreReconciler
// sets up fuzzing and discards invalid inputs
func setupUncoreFuzz(t *testing.T, nodeName string, namespace string, extraNode bool, node2Name string,
	runningOnTargetNode bool, min uint, max uint, die_and_package uint, powerLib power.Host) (*UncoreReconciler, reconcile.Request) {
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
	// prevents empty and null terminated strings
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

// used to validate test outcomes
func checkUncoreValues(basepath string, pkg string, die string, max string, min string) error {
	realMax, err := os.ReadFile(fmt.Sprintf("%s/intel_uncore_frequency/package_%s_die_%s/max_freq_khz", basepath, pkg, die))
	if err != nil {
		return err
	}
	realMin, err := os.ReadFile(fmt.Sprintf("%s/intel_uncore_frequency/package_%s_die_%s/min_freq_khz", basepath, pkg, die))
	if err != nil {
		return err
	}
	if max != string(realMax) || min != string(realMin) {
		return errors.New("min/max values in filesystem provided are unexpected")
	}
	return nil

}
