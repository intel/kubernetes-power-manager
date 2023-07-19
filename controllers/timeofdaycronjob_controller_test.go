package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var queueTime = (10 * time.Second)

// WIP timeofdaycronjob controller unit tests
func createTODCronReconcilerObject(objs []runtime.Object) (*TimeOfDayCronJobReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(s).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &TimeOfDayCronJobReconciler{cl, ctrl.Log.WithName("testing"), s, nil}

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
func TestTODCronProfile(t *testing.T) {
	zone := "Eire"
	profile := "performance"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	clientObjs := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodename",
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
				Name:      "balance-performance-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "balance-performance-nodename",
				Node: powerv1.WorkloadNode{
					Name:       "nodename",
					Containers: []powerv1.Container{},
					CpuIds:     []uint{},
				},
			},
		},
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "shared-nodename",
				Epp:  "power",
			},
		},
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name:     "shared-nodename",
				AllCores: true,
				Node: powerv1.WorkloadNode{
					Name:       "nodename",
					Containers: []powerv1.Container{},
					CpuIds:     []uint{},
				},
				PowerProfile: "shared-nodename",
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
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "performance-nodename",
				Node: powerv1.WorkloadNode{
					Name:       "nodename",
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
			Name:      "shared-nodename",
			Namespace: "intel-power",
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	//ensure workload has correct initial profile
	workload := powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), workloadReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, workload.Spec.AllCores)
	assert.Equal(t, workload.Spec.PowerProfile, "shared-nodename")
	//initiate cronjob
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//ensure workload profile has been changed
	workload = powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), workloadReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, workload.Spec.AllCores)
	assert.Equal(t, workload.Spec.PowerProfile, profile)

}

func TestCronPods(t *testing.T) {
	nodename := "nodename"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	podMap := map[string]map[string]string{"test-pod-1": {"performance": "balance-performance"}, "test-pod-2": {"don't-exist": "performance"}, "test-pod-3": {"performance": "don't-exist"}}
	podSpec := corev1.PodSpec{
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
	podStaus := corev1.PodStatus{
		Phase:    corev1.PodRunning,
		QOSClass: corev1.PodQOSGuaranteed,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name:        "example-container-1",
				ContainerID: "docker://abcdefg",
			},
		},
	}
	clientObjs := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodename",
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
				Name:      "balance-performance-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "balance-performance-nodename",
				Node: powerv1.WorkloadNode{
					Name:       "nodename",
					Containers: []powerv1.Container{},
					CpuIds:     []uint{},
				},
			},
		},
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "shared-nodename",
				Epp:  "power",
			},
		},
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name:     "shared-nodename",
				AllCores: true,
				Node: powerv1.WorkloadNode{
					Name:       "nodename",
					Containers: []powerv1.Container{},
					CpuIds:     []uint{},
				},
				PowerProfile: "shared-nodename",
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
		&powerv1.PowerWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "performance-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerWorkloadSpec{
				Name: "performance-nodename",
				Node: powerv1.WorkloadNode{
					Name: "nodename",
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
				Name:      "test-pod-1",
				Namespace: IntelPowerNamespace,
				UID:       "abcdefg",
			},
			Spec: podSpec,

			Status: podStaus,
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-2",
				Namespace: IntelPowerNamespace,
				UID:       "abcdefg",
			},
			Spec:   podSpec,
			Status: podStaus,
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-3",
				Namespace: IntelPowerNamespace,
				UID:       "abcdefg",
			},
			Spec:   podSpec,
			Status: podStaus,
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
				Pods:     &podMap,
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
			Name:      "performance-nodename",
			Namespace: IntelPowerNamespace,
		},
	}
	balancePerformanceReq := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "balance-performance-nodename",
			Namespace: IntelPowerNamespace,
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	//ensure pod is in correct workload
	workload := powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), performanceReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, findPodInWorkload(workload, "test-pod-1"))
	//reconcile job and wait for schedule time
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	//ensure initial workload no longer has pod
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	workload = powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), performanceReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.False(t, findPodInWorkload(workload, "test-pod-1"))
	assert.False(t, findPodInWorkload(workload, "test-pod-1"))

	//ensure new workload has pod
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

func TestCronCstates(t *testing.T) {
	nodename := "nodename"
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
	clientObjs := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodename",
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
			Name:      "nodename",
			Namespace: IntelPowerNamespace,
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	//ensure no initial cstate object exists
	cstate := powerv1.CStates{}
	err = r.Client.Get(context.TODO(), cstateReq.NamespacedName, &cstate)
	assert.Empty(t, cstate.Name)
	assert.Error(t, err)
	//reconcile job and wait for schedule time
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// ensure cstate was created and has correct values
	cstate = powerv1.CStates{}
	err = r.Client.Get(context.TODO(), cstateReq.NamespacedName, &cstate)
	assert.NoError(t, err)
	assert.False(t, cstate.Spec.SharedPoolCStates["C1"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.True(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1"])
	assert.False(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1E"])

}
//tests setting c-states when they already exist
func TestCronExistingCstates(t *testing.T) {
	nodename := "nodename"
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
	clientObjs := []runtime.Object{
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
			Name:      "nodename",
			Namespace: IntelPowerNamespace,
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	//reconcile job and wait for schedule time
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	// ensure cstate was created and has correct values
	cstate := powerv1.CStates{}
	err = r.Client.Get(context.TODO(), cstateReq.NamespacedName, &cstate)
	assert.NoError(t, err)
	assert.False(t, cstate.Spec.SharedPoolCStates["C1"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.False(t, cstate.Spec.SharedPoolCStates["C6"])
	assert.True(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1"])
	assert.False(t, cstate.Spec.ExclusivePoolCStates["performance"]["C1E"])
}
//tests setting workload when one exists
func TestCronNoExistingWorkload(t *testing.T) {
	nodename := "nodename"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	profile := "shared-nodename"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime, loc)
	clientObjs := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodename",
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
		&powerv1.PowerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-nodename",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "shared-nodename",
				Epp:  "power",
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
			Name:      "shared-nodename",
			Namespace: "intel-power",
		},
	}
	nodemk := new(hostMock)
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	//initiate cronjob
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//ensure workload profile has been created
	workload := powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), workloadReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, workload.Spec.AllCores)
	assert.Equal(t, workload.Spec.PowerProfile, profile)
}
// test setting tod for an earlier point in the day
func TestCronMissedDeadline(t *testing.T) {
	nodename := "nodename"
	t.Setenv("NODE_NAME", nodename)
	zone := "Eire"
	profile := "shared-nodename"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	hour, minute, second := addSeconds(queueTime*-1, loc)
	clientObjs := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodename",
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
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	//initiate cronjob
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//ensure cronjob was requeued for tommorow
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	assert.GreaterOrEqual(t, int(res.RequeueAfter.Seconds()), 8600)
}

func TestTODInvalidRequests(t *testing.T) {
	// incorrect namespace
	r, err := createTODCronReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "",
			Namespace: "somespace",
		},
	}
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	//incorrect node
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "wrong-node",
			Namespace: "intel-power",
		},
	}
	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)
	// incorrect timezones
	nodename := "nodename"
	t.Setenv("NODE_NAME", nodename)
	zone := "does not exist"
	profile := "shared-nodename"
	hour, minute, second := addSeconds(queueTime, nil)
	cronSpec := &powerv1.TimeOfDayCronJobSpec{
		Hour:         hour,
		Minute:       minute,
		Second:       second,
		TimeZone:     &zone,
		Profile:      &profile,
		ReservedCPUs: &[]uint{0},
	}
	clientObjs := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodename",
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
	}
	req = reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
		},
	}
	nodemk := new(hostMock)

	r, err = createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//nil timezone in spec
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
	//wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	//reserved cpus not set
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
	//wait till job needs to run
	t.Logf("requeue after %f", res.RequeueAfter.Seconds())
	r.PowerLibrary = nodemk
	time.Sleep(res.RequeueAfter)
	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "reserved CPU field left blank")
}

// tests positive and negative cases for SetupWithManager function
func TestCronReconcileSetupPass(t *testing.T) {
	r, err := createTODCronReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(mgrMock)
	mgr.On("GetControllerOptions").Return(v1alpha1.ControllerConfigurationSpec{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	err = (&TimeOfDayCronJobReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)

}
func TestCronReconcileSetupFail(t *testing.T) {
	r, err := createTODCronReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr, _ := ctrl.NewManager(&rest.Config{}, ctrl.Options{
		Scheme: scheme.Scheme,
	})

	err = (&TimeOfDayCronJobReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)

}
