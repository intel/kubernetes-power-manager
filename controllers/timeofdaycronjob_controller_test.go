package controllers

import (
	"context"
	"testing"
	"time"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
func TestTODCronProfile(t *testing.T) {
	zone := "Eire"
	profile := "performance"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	clientObjs := []runtime.Object{
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
				Name:      "shared-TestNode",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "shared-TestNode",
				Epp:  "power",
			},
		},
		&powerv1.PowerWorkload{
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
				Hour:     time.Now().In(loc).Hour(),
				Minute:   time.Now().In(loc).Add(1 * time.Minute).Minute(),
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
	r, err := createTODCronReconcilerObject(clientObjs)
	assert.NoError(t, err)
	//ensure workload has correct initial profile
	workload := powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), workloadReq.NamespacedName, &workload)
	assert.NoError(t, err)
	assert.True(t, workload.Spec.AllCores)
	assert.Equal(t, workload.Spec.PowerProfile, "shared-TestNode")
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

func TestTODCronPods(t *testing.T) {
	TestNode := "TestNode"
	t.Setenv("NODE_NAME", TestNode)
	zone := "Eire"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	podMap := make(map[string]map[string]string)
	profMap := make(map[string]string)
	profMap["performance"] = "balance-performance"
	podMap["test-pod-1"] = profMap
	clientObjs := []runtime.Object{
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
				Name:      "shared-TestNode",
				Namespace: IntelPowerNamespace,
			},
			Spec: powerv1.PowerProfileSpec{
				Name: "shared-TestNode",
				Epp:  "power",
			},
		},
		&powerv1.PowerWorkload{
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
				Name:      "test-pod-1",
				Namespace: "default",
				UID:       "abcdefg",
			},
			Spec: corev1.PodSpec{
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
			},
			Status: corev1.PodStatus{
				Phase:    corev1.PodRunning,
				QOSClass: corev1.PodQOSGuaranteed,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:        "example-container-1",
						ContainerID: "docker://abcdefg",
					},
				},
			},
		},
		&powerv1.TimeOfDayCronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDayCronJobSpec{
				Hour:     time.Now().In(loc).Hour(),
				Minute:   time.Now().In(loc).Add(1 * time.Minute).Minute(),
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

func TestTODCstates(t *testing.T) {
	TestNode := "TestNode"
	t.Setenv("NODE_NAME", TestNode)
	zone := "Eire"
	loc, err := time.LoadLocation(zone)
	assert.NoError(t, err)
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
				Hour:     time.Now().In(loc).Hour(),
				Minute:   time.Now().In(loc).Add(1 * time.Minute).Minute(),
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
