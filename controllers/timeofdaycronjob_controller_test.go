package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/podstate"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var queueTime = 10 * time.Second

var podSpec = corev1.PodSpec{
	NodeName: "IncorrectNode",
	Containers: []corev1.Container{
		{
			Name: "test-container-1",
			Resources: corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
					corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
					corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceName("cpu"):                         *resource.NewQuantity(3, resource.DecimalSI),
					corev1.ResourceName("memory"):                      *resource.NewQuantity(200, resource.DecimalSI),
					corev1.ResourceName("power.intel.com/performance"): *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		},
	},
	EphemeralContainers: []corev1.EphemeralContainer{},
}

var podStaus = corev1.PodStatus{
	Phase:    corev1.PodRunning,
	QOSClass: corev1.PodQOSGuaranteed,
	ContainerStatuses: []corev1.ContainerStatus{
		{
			Name:        "example-container-1",
			ContainerID: "docker://abcdefg",
		},
	},
}

var guaranteedPod = powerv1.GuaranteedPod{
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

var defaultSharedProf = &powerv1.PowerProfile{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "shared-TestNode",
		Namespace: IntelPowerNamespace,
	},
	Spec: powerv1.PowerProfileSpec{
		Name: "shared-TestNode",
		Epp:  "power",
		Max:  1000,
		Min:  1000,
	},
}

var defaultSharedWork = &powerv1.PowerWorkload{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "shared-TestNode",
		Namespace: IntelPowerNamespace,
	},
	Spec: powerv1.PowerWorkloadSpec{
		Name:     "shared-TestNode",
		AllCores: true,
		Node: powerv1.WorkloadNode{
			Name:       "TestNode",
			Containers: []powerv1.Container{},
			CpuIds:     []uint{},
		},
		PowerProfile: "shared-TestNode",
	},
}

func createTODCronReconcilerObject(objs []client.Object) (*TimeOfDayCronJobReconciler, error) {
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
	var state *podstate.State
	var err error
	if state, err = podstate.NewState(); err != nil {
		return nil, err
	}
	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).WithStatusSubresource(objs...).Build()
	// Create a ReconcileNode object with the scheme and fake client.
	r := &TimeOfDayCronJobReconciler{cl, ctrl.Log.WithName("testing"), s, state, nil}

	return r, nil
}

// used to solve any rounding errors
func addSeconds(queueTime time.Duration, loc *time.Location) (int, int, int) {
	var hour int
	var minute int
	var second int
	if loc == nil {
		loc = time.Local
	}
	minRounder := time.Now().In(loc).Second() + int(queueTime.Seconds())
	if minRounder >= 60 {
		minute = time.Now().In(loc).Add(1 * time.Minute).Minute()
		second = minRounder - 60
	} else if minRounder < 0 {
		// for negative values
		minute = time.Now().In(loc).Add(-1 * time.Minute).Minute()
		second = 60 + minRounder
	} else {
		second = time.Now().In(loc).Add(queueTime).Second()
		minute = time.Now().In(loc).Minute()
	}
	if minute >= 60 {
		hour = time.Now().In(loc).Hour()
		hour += 1
		minute = minute - 60

	} else if minute < 0 {
		hour = time.Now().In(loc).Hour()
		hour -= 1
		minute = 60 + minute
	} else {
		hour = time.Now().In(loc).Hour()
	}
	fmt.Printf("values are %d %d %d with location %v \n", hour, minute, second, loc)
	return hour, minute, second
}
func TestTimeOfDayCronJob_Reconcile_CronProfile(t *testing.T) {
	zone := "Eire"
	profile := "performance"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	clientObjs := []client.Object{
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
		defaultSharedProf,
		defaultSharedWork,
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "performance",
				Epp:  "performance",
			},
		},
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance-TestNode",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "performance-TestNode",
				Node: powerv1.WorkloadNode{
					Name:       "TestNode",
					Containers: []powerv1.Container{},
					CpuIds:     []uint{},
				},
			},
		},
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDayCronJobSpec{
				Hour:     hour,
				Minute:   minute,
				Second:   second,
				TimeZone: &zone,
				Profile:  &profile,
			},
		},
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	workloadReq := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "shared-TestNode",
			Namespace: "intel-power",
		},
	}
	nodemk := new(hostMock)
	poolmk := new(poolMock)
	poolmk.On("SetPowerProfile", mock.Anything).Return(nil)
	nodemk.On("GetSharedPool").Return(poolmk)
	freqSetmk := new(frequencySetMock)
	nodemk.On("GetFreqRanges").Return(power.CoreTypeList{freqSetmk})
	freqSetmk.On("GetMax").Return(uint(9000000))
	freqSetmk.On("GetMin").Return(uint(100000))
	r, err := createTODCronReconcilerObject(clientObjs)
	r.PowerLibrary = nodemk
	assert.NoError(t, err)
	// this just prevents a feature not enabled error from the library
	_, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	// ensure the workload has the correct initial profile
	workload := powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), workloadReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, workload.Spec.AllCores)
	assert.Equal(t, workload.Spec.PowerProfile, "shared-TestNode")
	// initiate the cron job
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// wait till the job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// ensure the workload profile has been changed
	workload = powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), workloadReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, workload.Spec.AllCores)
	assert.Equal(t, workload.Spec.PowerProfile, profile)

}

func TestTimeOfDayCronJob_Reconcile_CronPods(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	clientObjs := []client.Object{
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
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "balance-performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "balance-performance",
				Epp:  "balance-performance",
			},
		},
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "balance-performance-TestNode",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "balance-performance-TestNode",
				Node: powerv1.WorkloadNode{
					Name:       "TestNode",
					Containers: []powerv1.Container{},
					CpuIds:     []uint{},
				},
			},
		},
		defaultSharedProf,
		defaultSharedWork,
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "performance",
				Epp:  "performance",
			},
		},
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance-TestNode",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "performance-TestNode",
				Node: powerv1.WorkloadNode{
					Name: "TestNode",
					Containers: []powerv1.Container{
						{
							Pod:           "test-pod-1",
							ExclusiveCPUs: []uint{3, 4},
						},
					},
					CpuIds: []uint{3, 4},
				},
				PowerProfile: "performance",
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-pod-1",
				Namespace:   IntelPowerNamespace,
				UID:         "abcdefg",
				Labels:      map[string]string{"power": "true"},
				Annotations: map[string]string{"jibber": "jabber"},
			},
			Spec: podSpec,

			Status: podStaus,
		},
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Status: powerv1.TimeOfDayCronJobStatus{},
			Spec: powerv1.TimeOfDayCronJobSpec{
				Hour:     hour,
				Minute:   minute,
				Second:   second,
				TimeZone: &zone,
				Pods: &[]powerv1.PodInfo{
					{
						Labels: metav1.LabelSelector{MatchLabels: map[string]string{"power": "true"}},
						Target: "balance-performance",
					},
				},
			},
		},
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	performanceReq := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "performance-TestNode",
			Namespace: IntelPowerNamespace,
		},
	}
	balancePerformanceReq := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "balance-performance-TestNode",
			Namespace: IntelPowerNamespace,
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	// adding pods to the internal state
	err = r.State.UpdateStateGuaranteedPods(guaranteedPod)
	assert.Nil(t, err)
	// ensure pod is in the correct workload
	workload := powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), performanceReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, findPodInWorkload(workload, "test-pod-1"))
	// reconcile job and wait for schedule time
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	// ensure initial workload no longer has pod
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	workload = powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), performanceReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.False(t, findPodInWorkload(workload, "test-pod-1"))
	assert.False(t, findPodInWorkload(workload, "test-pod-1"))

	// ensure new workload has pod
	workload = powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), balancePerformanceReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, findPodInWorkload(workload, "test-pod-1"))

}

func findPodInWorkload(workload powerv1.PowerWorkload, podName string) bool {
	for _, container := range workload.Spec.Node.Containers {
		if container.Pod == podName {
			return true
		}
	}
	return false
}

func TestTimeOfDayCronJob_Reconcile_Cstates(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	cMapShared := make(map[string]bool)
	cMapShared["C1"] = false
	cMapShared["C6"] = false
	cMapExclusiveInner := make(map[string]bool)
	cMapExclusiveInner["C1E"] = false
	cMapExclusiveInner["C1"] = true
	cMapExclusiveOuter := make(map[string]map[string]bool)
	cMapExclusiveOuter["performance"] = cMapExclusiveInner
	cSpec := powerv1.CStatesSpec{
		SharedPoolCStates:    cMapShared,
		ExclusivePoolCStates: cMapExclusiveOuter,
	}
	clientObjs := []client.Object{
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
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDayCronJobSpec{
				Hour:     hour,
				Minute:   minute,
				Second:   second,
				TimeZone: &zone,
				CState:   &cSpec,
			},
		},
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: IntelPowerNamespace,
		},
	}
	cstateReq := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "TestNode",
			Namespace: IntelPowerNamespace,
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	// ensure no initial C-State object exists
	cstate := powerv1.CStates{}
	err = r.Client.Get(context.TODO(), cstateReq.NamespacedName, &cstate)
	assert.Empty(t, cstate.Name)
	assert.Error(t, err)
	// reconcile job and wait for the scheduled time
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// ensure C-State was created and has the correct values
	cstate = powerv1.CStates{}
	err = r.Client.Get(context.TODO(), cstateReq.NamespacedName, &cstate)
	assert.NoError(t, err)
	assert.False(t, cstate.Spec.SharedPoolCStates["C1"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.True(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1"])
	assert.False(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1E"])

}

// tests setting C-States when they already exist
func TestTimeOfDayCronJob_Reconcile_ExistingCstates(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	cMapShared := make(map[string]bool)
	cMapShared["C1"] = false
	cMapShared["C6"] = false
	cMapExclusiveInner := make(map[string]bool)
	cMapExclusiveInner["C1E"] = false
	cMapExclusiveInner["C1"] = true
	cMapExclusiveOuter := make(map[string]map[string]bool)
	cMapExclusiveOuter["performance"] = cMapExclusiveInner
	cSpec := powerv1.CStatesSpec{
		SharedPoolCStates:    cMapShared,
		ExclusivePoolCStates: cMapExclusiveOuter,
	}
	clientObjs := []client.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodename,
			},
			Status: corev1.NodeStatus{
				Capacity: map[corev1.ResourceName]resource.Quantity{
					CPUResource: *resource.NewQuantity(42, resource.DecimalSI),
				},
			},
		},
		&powerv1.CStates{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodename,
				Namespace: "intel-power",
			},
			Spec: powerv1.CStatesSpec{
				ExclusivePoolCStates: map[string]map[string]bool{"performance": {"C1": false, "C1E": true}},
			},
		},
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDayCronJobSpec{
				Hour:     hour,
				Minute:   minute,
				Second:   second,
				TimeZone: &zone,
				CState:   &cSpec,
			},
		},
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: IntelPowerNamespace,
		},
	}
	cstateReq := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "TestNode",
			Namespace: IntelPowerNamespace,
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	// reconcile job and wait for schedule time
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// ensure C-State was created and has correct values
	cstate := powerv1.CStates{}
	err = r.Client.Get(context.TODO(), cstateReq.NamespacedName, &cstate)
	assert.NoError(t, err)
	assert.False(t, cstate.Spec.SharedPoolCStates["C1"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.True(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1"])
	assert.False(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1E"])
}

// tests setting workload when one exists
func TestTimeOfDayCronJob_Reconcile_NoExistingWorkload_Lib_Err(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	profile := "shared-TestNode"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	clientObjs := []client.Object{
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
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "balance-performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "balance-performance",
				Epp:  "balance-performance",
				Max:  3300,
				Min:  3000,
			},
		},
		defaultSharedProf,
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "performance",
				Epp:  "performance",
				Max:  3700,
				Min:  3500,
			},
		},
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDayCronJobSpec{
				Hour:         hour,
				Minute:       minute,
				Second:       second,
				TimeZone:     &zone,
				Profile:      &profile,
				ReservedCPUs: &[]uint{0},
			},
		},
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	workloadReq := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "shared-TestNode",
			Namespace: "intel-power",
		},
	}
	_, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	nodemk := new(hostMock)
	poolmk := new(poolMock)
	nodemk.On("GetSharedPool").Return(poolmk)
	freqSetmk := new(frequencySetMock)
	nodemk.On("GetFreqRanges").Return(power.CoreTypeList{freqSetmk})
	freqSetmk.On("GetMax").Return(uint(9000000))
	freqSetmk.On("GetMin").Return(uint(100000))
	poolmk.On("SetPowerProfile", mock.Anything).Return(fmt.Errorf("forced library err"))
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	// initiate cron job
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "forced library err")
	// ensure the workload profile has been created
	workload := powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), workloadReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, workload.Spec.AllCores)
	assert.Equal(t, workload.Spec.PowerProfile, profile)
}

// test setting tod for an earlier point in the day
func TestTimeOfDayCronJob_Reconcile_MissedDeadline(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	profile := "shared-TestNode"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime*-1, loc)
	clientObjs := []client.Object{
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
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDayCronJobSpec{
				Hour:         hour,
				Minute:       minute,
				Second:       second,
				TimeZone:     &zone,
				Profile:      &profile,
				ReservedCPUs: &[]uint{0},
			},
		},
		defaultSharedWork,
		defaultProfile,
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	// initiate cron job
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// ensure cron job was requeued for tommorow
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	assert.GreaterOrEqual(t, int(res.RequeueAfter.Seconds()), 8600)
}

// checks for error cases with shared pool aplication
func TestTimeOfDayCronJob_Reconcile_ErrsSharedPoolExists(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	profile := "performance"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	cronjob := &powerv1.TimeOfDayCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDayCronJobSpec{
			TimeZone:     &zone,
			Profile:      &profile,
			ReservedCPUs: &[]uint{0},
		},
	}
	performanceWorkload := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "performance-TestNode",
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name: "performance-TestNode",
			Node: powerv1.WorkloadNode{
				Name:       "TestNode",
				Containers: []powerv1.Container{},
				CpuIds:     []uint{},
			},
		},
	}
	clientObjs := []client.Object{
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
		defaultSharedProf,
		defaultSharedWork,
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "performance",
				Epp:  "performance",
				Max:  3500,
				Min:  3200,
			},
		},
		performanceWorkload,
	}
	tcases := []struct {
		testCase      string
		getNodemk     func() *hostMock
		convertClient func(client.Client, powerv1.TimeOfDayCronJob) client.Client
		validateErr   func(e error) bool
	}{
		{
			testCase: "Test Case 1: set profile err",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				poolmk := new(poolMock)
				nodemk.On("GetSharedPool").Return(poolmk)
				freqSetmk := new(frequencySetMock)
				nodemk.On("GetFreqRanges").Return(power.CoreTypeList{freqSetmk})
				freqSetmk.On("GetMax").Return(uint(9000000))
				freqSetmk.On("GetMin").Return(uint(100000))
				poolmk.On("SetPowerProfile", mock.Anything).Return(fmt.Errorf("forced library err"))
				return nodemk
			},
			convertClient: func(c client.Client, cron powerv1.TimeOfDayCronJob) client.Client {
				return c
			},
			validateErr: func(e error) bool {
				return assert.ErrorContains(t, e, "forced library err")
			},
		},
		{
			testCase: "Test Case 2: client list err",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				return nodemk
			},
			convertClient: func(c client.Client, cron powerv1.TimeOfDayCronJob) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.TimeOfDayCronJob")).Return(nil).Run(func(args mock.Arguments) {
					job := args.Get(2).(*powerv1.TimeOfDayCronJob)
					cron.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
					*job = cron
				})
				mkcl.On("Status").Return(c.Status())
				mkcl.On("List", mock.Anything, mock.Anything).Return(fmt.Errorf("client list error"))
				return mkcl
			},
			validateErr: func(e error) bool {
				return assert.ErrorContains(t, e, "client list error")
			},
		},
		{
			testCase: "Test Case 3: client get err",
			getNodemk: func() *hostMock {
				nodemk := new(hostMock)
				return nodemk
			},
			convertClient: func(c client.Client, cron powerv1.TimeOfDayCronJob) client.Client {
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.TimeOfDayCronJob")).Return(nil).Run(func(args mock.Arguments) {
					job := args.Get(2).(*powerv1.TimeOfDayCronJob)
					cron.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
					*job = cron
				})
				mkcl.On("Status").Return(c.Status())
				mkcl.On("List", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*powerv1.PowerWorkloadList)
					*list = powerv1.PowerWorkloadList{Items: []powerv1.PowerWorkload{*performanceWorkload, *defaultSharedWork}}
				})
				mkcl.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client get error"))
				return mkcl
			},
			validateErr: func(e error) bool {
				return assert.ErrorContains(t, e, "client get error")
			},
		},
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	_, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	for _, tc := range tcases {
		hour, minute, second := addSeconds(queueTime, loc)
		cronjob.Spec.Hour = hour
		cronjob.Spec.Minute = minute
		cronjob.Spec.Second = second
		clientObjs = append(clientObjs, cronjob)
		r, err := createTODCronReconcilerObject(clientObjs)
		assert.NoError(t, err)
		r.PowerLibrary = tc.getNodemk()
		// this just prevents a feature not enabled error from the library
		res, err := r.Reconcile(context.TODO(), req)
		assert.NoError(t, err)
		r.Client = tc.convertClient(r.Client, *cronjob)
		// wait till job needs to run
		t.Logf("requeue after %f", res.RequeueAfter.Seconds())
		time.Sleep(res.RequeueAfter)
		_, err = r.Reconcile(context.TODO(), req)
		assert.True(t, tc.validateErr(err))
		// remove cron job so next can be added
		clientObjs = clientObjs[0 : len(clientObjs)-1]

	}
}

// tests error cases with pod tuning
func TestTimeOfDayCronJob_Reconcile_ErrsPodTuning(t *testing.T) {
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	cronjob := &powerv1.TimeOfDayCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
		Spec: powerv1.TimeOfDayCronJobSpec{
			TimeZone: &zone,
			Pods: &[]powerv1.PodInfo{
				{
					Labels: metav1.LabelSelector{MatchLabels: map[string]string{"power": "true"}},
					Target: "balance-performance",
				},
			},
		},
	}
	performanceWorkload := &powerv1.PowerWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "performance-TestNode",
			Namespace: IntelPowerNamespace,
		},
		Spec: powerv1.PowerWorkloadSpec{
			Name: "performance-TestNode",
			Node: powerv1.WorkloadNode{
				Name: "TestNode",
				Containers: []powerv1.Container{
					{
						Pod:           "test-pod-1",
						ExclusiveCPUs: []uint{3, 4},
					},
				},
				CpuIds: []uint{3, 4},
			},
			PowerProfile: "performance",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod-1",
			Namespace:   IntelPowerNamespace,
			UID:         "abcdefg",
			Labels:      map[string]string{"power": "true"},
			Annotations: map[string]string{},
		},
		Spec: podSpec,

		Status: podStaus,
	}
	clientObjs := []client.Object{
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
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "balance-performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "balance-performance",
				Epp:  "balance-performance",
			},
		},
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "balance-performance-TestNode",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "balance-performance-TestNode",
				Node: powerv1.WorkloadNode{
					Name:       "TestNode",
					Containers: []powerv1.Container{},
					CpuIds:     []uint{},
				},
			},
		},
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "performance",
				Epp:  "performance",
			},
		},
		performanceWorkload,
		pod,
	}
	tcases := []struct {
		testCase      string
		getNodemk     func(r *TimeOfDayCronJobReconciler) *hostMock
		convertClient func(client.Client, powerv1.TimeOfDayCronJob) client.Client
		validateErr   func(e error) bool
	}{
		{
			testCase: "Test Case 1: internal state mismatch",
			getNodemk: func(r *TimeOfDayCronJobReconciler) *hostMock {
				nodemk := new(hostMock)
				return nodemk
			},
			convertClient: func(c client.Client, cron powerv1.TimeOfDayCronJob) client.Client {
				return c
			},
			validateErr: func(e error) bool {
				return assert.Nil(t, err)
			},
		},
		{
			testCase: "Test Case 2: client list err",
			getNodemk: func(r *TimeOfDayCronJobReconciler) *hostMock {
				nodemk := new(hostMock)
				r.State.UpdateStateGuaranteedPods(guaranteedPod)
				return nodemk
			},
			convertClient: func(c client.Client, cron powerv1.TimeOfDayCronJob) client.Client {
				mkwriter := new(mockResourceWriter)
				mkwriter.On("Update", mock.Anything, mock.Anything).Return(nil)
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.TimeOfDayCronJob")).Return(nil).Run(func(args mock.Arguments) {
					job := args.Get(2).(*powerv1.TimeOfDayCronJob)
					cron.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
					*job = cron
				})
				mkcl.On("List", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("client list error"))
				mkcl.On("Status").Return(mkwriter)
				return mkcl
			},
			validateErr: func(e error) bool {
				return assert.ErrorContains(t, e, "client list error")
			},
		},
		{
			testCase: "Test Case 3: client workload retrieval err",
			getNodemk: func(r *TimeOfDayCronJobReconciler) *hostMock {
				nodemk := new(hostMock)
				r.State.UpdateStateGuaranteedPods(guaranteedPod)
				return nodemk
			},
			convertClient: func(c client.Client, cron powerv1.TimeOfDayCronJob) client.Client {
				mkwriter := new(mockResourceWriter)
				mkwriter.On("Update", mock.Anything, mock.Anything).Return(nil)
				mkcl := new(errClient)
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.TimeOfDayCronJob")).Return(nil).Run(func(args mock.Arguments) {
					job := args.Get(2).(*powerv1.TimeOfDayCronJob)
					cron.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
					*job = cron
				})
				mkcl.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*corev1.PodList)
					*list = corev1.PodList{Items: []corev1.Pod{*pod}}
				})
				mkcl.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1.PowerWorkload")).Return(fmt.Errorf("client get error"))
				mkcl.On("Status").Return(mkwriter)

				return mkcl
			},
			validateErr: func(e error) bool {
				return assert.ErrorContains(t, e, "client get error")
			},
		},
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	_, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	for _, tc := range tcases {
		hour, minute, second := addSeconds(queueTime, loc)
		cronjob.Spec.Hour = hour
		cronjob.Spec.Minute = minute
		cronjob.Spec.Second = second
		clientObjs = append(clientObjs, cronjob)
		r, err := createTODCronReconcilerObject(clientObjs)
		assert.NoError(t, err)
		r.PowerLibrary = tc.getNodemk(r)
		// this just prevents a feature not enabled error from the library
		res, err := r.Reconcile(context.TODO(), req)
		assert.NoError(t, err)
		r.Client = tc.convertClient(r.Client, *cronjob)
		// wait till job needs to run
		t.Logf("requeue after %f", res.RequeueAfter.Seconds())
		time.Sleep(res.RequeueAfter)
		_, err = r.Reconcile(context.TODO(), req)
		assert.True(t, tc.validateErr(err))
		// remove cronjob so next can be added
		clientObjs = clientObjs[0 : len(clientObjs)-1]

	}
}

func TestTimeOfDayCronJob_Reconcile_InvalidRequests(t *testing.T) {
	_, teardown, err := fullDummySystem()
	assert.Nil(t, err)
	defer teardown()
	// incorrect namespace
	r, err := createTODCronReconcilerObject([]client.Object{})
	assert.Nil(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "",
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
	assert.ErrorContains(t, err, "wrong-node")
	// incorrect timezones
	nodename := "TestNode"
	t.Setenv("NODE_NAME", nodename)
	zone := "does not exist"
	profile := "shared-TestNode"
	hour, minute, second := addSeconds(queueTime, nil)
	cronSpec := &powerv1.TimeOfDayCronJobSpec{
		Hour:         hour,
		Minute:       minute,
		Second:       second,
		TimeZone:     &zone,
		Profile:      &profile,
		ReservedCPUs: &[]uint{0},
	}
	clientObjs := []client.Object{
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
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: *cronSpec,
		},
		defaultSharedProf,
	}
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	nodemk := new(hostMock)
	poolmk := new(poolMock)
	poolmk.On("SetPowerProfile", mock.Anything).Return(nil)
	nodemk.On("GetSharedPool").Return(poolmk)
	freqSetmk := new(frequencySetMock)
	nodemk.On("GetFreqRanges").Return(power.CoreTypeList{freqSetmk})
	freqSetmk.On("GetMax").Return(uint(9000000))
	freqSetmk.On("GetMin").Return(uint(100000))
	r, err = createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// nil timezone in spec
	hour, minute, second = addSeconds(queueTime, nil)
	cronSpec = &powerv1.TimeOfDayCronJobSpec{
		Hour:         hour,
		Minute:       minute,
		Second:       second,
		Profile:      &profile,
		ReservedCPUs: &[]uint{0},
	}
	clientObjs[1] = &powerv1.TimeOfDayCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
		Spec: *cronSpec,
	}
	r, err = createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	res, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// reserved cpus not set
	zone = "Eire"
	loc, err := time.LoadLocation(zone)
	assert.Nil(t, err)
	hour, minute, second = addSeconds(queueTime, loc)
	cronSpec = &powerv1.TimeOfDayCronJobSpec{
		Hour:     hour,
		Minute:   minute,
		Second:   second,
		TimeZone: &zone,
		Profile:  &profile,
	}
	clientObjs[1] = &powerv1.TimeOfDayCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
		Spec: *cronSpec,
	}
	r, err = createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	res, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "reserved CPU field left blank")
}

// tests positive and negative cases for SetupWithManager function
func TestTimeOfDayCronJob_Reconcile_SetupPass(t *testing.T) {
	r, err := createTODCronReconcilerObject([]client.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(cacheMk))
	err = (&TimeOfDayCronJobReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}
func TestCronReconcileSetupFail(t *testing.T) {
	r, err := createTODCronReconcilerObject([]client.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("setup fail"))

	err = (&TimeOfDayCronJobReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}

// go test -fuzz FuzzTimeOfDayCronController -run=FuzzTimeOfDayCronController -parallel=1
func FuzzTimeOfDayCronController(f *testing.F) {
	f.Add("Eire", 12, 32, 30, "performance", "balance-power", "shared", "power", "bigger", "C4", "25")
	f.Fuzz(func(t *testing.T, timeZone string, hr int, min int, sec int, prof1 string, prof2 string, prof3 string, label1 string, label2 string, cstate string, corevalue string) {
		testNode := "TestNode"
		t.Setenv("NODE_NAME", testNode)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-pod-1",
				Namespace:   IntelPowerNamespace,
				UID:         "abcdefg",
				Labels:      map[string]string{label1: "true"},
				Annotations: map[string]string{"jibber": "jabber"},
			},
			Spec:   podSpec,
			Status: podStaus,
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
		todObj := &powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testNode,
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDayCronJobSpec{
				TimeZone:     &timeZone,
				ReservedCPUs: &[]uint{0, 1},
				Hour:         hr,
				Minute:       min,
				Second:       sec,
				Profile:      &prof1,
				Pods: &[]powerv1.PodInfo{
					{Labels: metav1.LabelSelector{MatchLabels: map[string]string{label1: "true"}}, Target: prof3},
					{Labels: metav1.LabelSelector{MatchLabels: map[string]string{label2: "false"}}, Target: prof2},
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
		}

		clientObjs := []client.Object{
			nodeObj, todObj, pod,
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      testNode,
				Namespace: "intel-power",
			},
		}
		dummyFilesystemHost, teardown, err := fullDummySystem()
		assert.Nil(t, err)
		defer teardown()
		r, err := createTODCronReconcilerObject(clientObjs)
		r.PowerLibrary = dummyFilesystemHost
		if err != nil {
			t.Error(err)
		}
		r.Reconcile(context.TODO(), req)

	})
}
