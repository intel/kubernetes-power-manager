package controllers

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	// "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// WIP timeofday controller unit tests
func createTimeOfDayReconcilerObject(objs []runtime.Object) (*TimeOfDayReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(s).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &TimeOfDayReconciler{cl, ctrl.Log.WithName("testing"), s}

	return r, nil
}
func TestTimeOfDay_Reconcile(t *testing.T) {
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
	todObj := &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     "Eire",
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "09:00",
				},
				{
					Time: "09:05",
				},
				{
					Time: "09:15",
				},
			},
		},
	}

	nodesObj := &corev1.NodeList{
		Items: []corev1.Node{*nodeObj},
	}

	clientObjs := []runtime.Object{
		todObj, nodesObj,
	}
	// time of day creation
	t.Setenv("NODE_NAME", testNode)
	r, err := createTimeOfDayReconcilerObject(clientObjs)
	assert.NoError(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      testNode,
			Namespace: IntelPowerNamespace,
		},
	}
	nodemk := new(hostMock)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	nodemk.AssertExpectations(t)
	jobs := &powerv1.TimeOfDayCronJobList{}
	err = r.Client.List(context.TODO(), jobs)
	assert.NoError(t, err)
	assert.Len(t, jobs.Items, 3)

	// timeofday deletion
	jobs = &powerv1.TimeOfDayCronJobList{}
	err = r.Client.Delete(context.TODO(), todObj)
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	err = r.Client.List(context.TODO(), jobs)
	assert.NoError(t, err)
	assert.Len(t, jobs.Items, 0)
	// incorrect format error
	todObj = &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     "Eire",
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "25:61",
				},
			},
		},
	}
	clientObjs = []runtime.Object{
		todObj, nodesObj,
	}
	r, err = createTimeOfDayReconcilerObject(clientObjs)
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "time filed must be in format HH:MM:SS or HH:MM")
	// time overflow
	todObj = &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     "Eire",
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "25:61",
				},
			},
		},
	}
	clientObjs = []runtime.Object{
		todObj, nodesObj,
	}
	r, err = createTimeOfDayReconcilerObject(clientObjs)
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.Error(t, err)

}

// go test -fuzz FuzzTimeOfDayController -run=FuzzTimeOfDayController
func FuzzTimeOfDayController(f *testing.F) {
	f.Add("Eire", "12:30:24", uint(19), uint(45), "performance", "balance-power", "shared", "power", "bigger", "C4", "25")
	f.Fuzz(func(t *testing.T, timeZone string, time1 string, time2 uint, time3 uint, prof1 string, prof2 string, prof3 string, label1 string, label2 string, cstate string, corevalue string) {
		testNode := "TestNode"
		t.Setenv("NODE_NAME", testNode)
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

		todObj := &powerv1.TimeOfDay{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testNode,
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDaySpec{
				TimeZone:     timeZone,
				ReservedCPUs: &[]uint{0, 1},
				Schedule: []powerv1.ScheduleInfo{
					{
						Time:         time1,
						PowerProfile: &prof1,
						Pods: &[]powerv1.PodInfo{
							{Labels: metav1.LabelSelector{MatchLabels: map[string]string{label1: "true"}}, Target: prof3},
							{Labels: metav1.LabelSelector{MatchLabels: map[string]string{label2: "false"}}, Target: prof3},
						},
						CState: &powerv1.CStatesSpec{
							SharedPoolCStates: map[string]bool{cstate: true},
							ExclusivePoolCStates: map[string]map[string]bool{
								prof1: {"C1E": false, "C6": false, "C1": false},
								prof2: {"C1E": true, "C6": false},
							},
							IndividualCoreCStates: map[string]map[string]bool{
								"200":     {"C1E": true, "C6": false},
								"-4":      {"C1E": false, "C6": false},
								corevalue: {"C1E": false, "C6": false, "CIE": true},
							},
						},
					},
					{
						Time:         strconv.Itoa(int(time2)) + ":" + strconv.Itoa(int(time3)),
						PowerProfile: &prof2,
						Pods: &[]powerv1.PodInfo{
							{Labels: metav1.LabelSelector{MatchLabels: map[string]string{label2: "true"}}, Target: prof1},
							{Labels: metav1.LabelSelector{MatchLabels: map[string]string{label1: "false"}}, Target: prof2},
						},
						CState: &powerv1.CStatesSpec{
							SharedPoolCStates: map[string]bool{},
							ExclusivePoolCStates: map[string]map[string]bool{
								prof1: {cstate: false},
								prof2: {"C1E": true, "C6": false},
							},
							IndividualCoreCStates: map[string]map[string]bool{
								"3": {"C1E": true, "C6": false},
								"8": {"C1E": false, "C6": false},
							},
						},
					},
				},
			},
		}

		clientObjs := []runtime.Object{
			nodesObj, todObj,
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      testNode,
				Namespace: "intel-power",
			},
		}
		r, err := createTimeOfDayReconcilerObject(clientObjs)
		if err != nil {
			t.Error(err)
		}
		r.Reconcile(context.TODO(), req)

	})
}

func TestTimeOfDay_Reconcile_InvalidTODRequests(t *testing.T) {
	// incorrect node
	testNode := "TestNode"
	t.Setenv("NODE_NAME", testNode)
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
	todObj := &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     "Eire",
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "22:31",
				},
			},
		},
	}
	clientObjs := []runtime.Object{
		todObj, nodesObj,
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      testNode,
			Namespace: "made-up",
		},
	}
	r, err := createTimeOfDayReconcilerObject(clientObjs)
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "incorrect namespace")
	// ensure object was not created
	dummyObject := powerv1.TimeOfDay{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, &dummyObject)
	assert.ErrorContains(t, err, "not found")
	// invalid timezone
	todObj = &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     "made up",
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "22:31",
				},
			},
		},
	}
	clientObjs = []runtime.Object{
		todObj, nodesObj,
	}
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      testNode,
			Namespace: IntelPowerNamespace,
		},
	}
	r, err = createTimeOfDayReconcilerObject(clientObjs)
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "invalid timezone")
	// multiple TODs
	todObj1 := &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     "Eire",
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "22:31",
				},
			},
		},
	}
	todObj2 := &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: "default",
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     "Eire",
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "22:31",
				},
			},
		},
	}
	clientObjs = []runtime.Object{
		todObj1, todObj2, nodesObj,
	}
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      testNode,
			Namespace: IntelPowerNamespace,
		},
	}
	r, err = createTimeOfDayReconcilerObject(clientObjs)
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "cannot have more than one time-of-day")
}

func TestTimeOfDay_Reconcile_ClientErrs(t *testing.T) {
	timeZone := "Eire"
	profile := "performance"
	testNode := "TestNode"
	t.Setenv("NODE_NAME", testNode)
	todObj := &powerv1.TimeOfDay{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNode,
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDaySpec{
			TimeZone:     timeZone,
			ReservedCPUs: &[]uint{0, 1},
			Schedule: []powerv1.ScheduleInfo{
				{
					Time: "22:31",
				},
			},
		},
	}
	rogueCronjob := &powerv1.TimeOfDayCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rogue",
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.TimeOfDayCronJobSpec{
			Hour:     22,
			Minute:   23,
			TimeZone: &timeZone,
			Profile:  &profile,
		},
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      testNode,
			Namespace: IntelPowerNamespace,
		},
	}
	r, err := createTimeOfDayReconcilerObject([]runtime.Object{})
	assert.NoError(t, err)
	mkcl := new(errClient)
	mkcl.On("List", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client list error"))
	r.Client = mkcl
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "client list error")
	// cron job cleanup error
	r, err = createTimeOfDayReconcilerObject([]runtime.Object{})
	assert.NoError(t, err)

	mkwriter := new(mockResourceWriter)
	mkwriter.On("Update", mock.Anything, mock.Anything).Return(nil)
	mkcl = new(errClient)
	mkcl.On("List", mock.Anything, mock.AnythingOfType("*v1.TimeOfDayList")).Return(nil).Run(func(args mock.Arguments) {
		todList := args.Get(1).(*powerv1.TimeOfDayList)
		*todList = powerv1.TimeOfDayList{Items: []powerv1.TimeOfDay{*todObj}}
	})
	mkcl.On("List", mock.Anything, mock.AnythingOfType("*v1.TimeOfDayCronJobList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		todList := args.Get(1).(*powerv1.TimeOfDayCronJobList)
		*todList = powerv1.TimeOfDayCronJobList{Items: []powerv1.TimeOfDayCronJob{*rogueCronjob}}
	})
	mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.TimeOfDay")).Return(nil).Run(func(args mock.Arguments) {
		tod := args.Get(2).(*powerv1.TimeOfDay)
		*tod = *todObj
	})
	mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.TimeOfDayCronJob")).Return(nil).Run(func(args mock.Arguments) {
		todCron := args.Get(2).(*powerv1.TimeOfDayCronJob)
		*todCron = powerv1.TimeOfDayCronJob{}
	})
	mkcl.On("Delete", mock.Anything, mock.Anything).Return(fmt.Errorf("client delete error"))
	mkcl.On("Status").Return(mkwriter)
	r.Client = mkcl
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "client delete error")
	// ensure typical case holds
	r, err = createTimeOfDayReconcilerObject([]runtime.Object{})
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	dummy := powerv1.TimeOfDayCronJob{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: IntelPowerNamespace, Name: "rogue"}, &dummy)
	assert.ErrorContains(t, err, "not found")

}

func TestTimeOfDay_Reconcile_SetupPass(t *testing.T) {
	r, err := createTimeOfDayReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(cacheMk))
	err = (&TimeOfDayReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}

func TestTimeOfDay_Reconcile_SetupFail(t *testing.T) {
	r, err := createTimeOfDayReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("setup fail"))

	err = (&TimeOfDayReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
