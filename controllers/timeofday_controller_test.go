package controllers

import (
	"context"
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
	"strconv"
	"testing"
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
func TestTimeOfDay(t *testing.T) {
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
			Name:      "timeofday-test",
			Namespace: "intel-power",
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
	//time of day creation
	t.Setenv("NODE_NAME", testNode)
	r, err := createTimeOfDayReconcilerObject(clientObjs)
	assert.NoError(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "timeofday-test",
			Namespace: "intel-power",
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
			Name:      "timeofday-test",
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

func FuzzTimeOfDayController(f *testing.F) {
	f.Fuzz(func(t *testing.T, timeZone string, time1 string, time2 int, time3 int) {
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
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
			Spec: powerv1.TimeOfDaySpec{
				TimeZone:     timeZone,
				ReservedCPUs: &[]uint{0, 1},
				Schedule: []powerv1.ScheduleInfo{
					{
						Time: time1,
					}, {
						Time: strconv.Itoa(time2) + ":" + strconv.Itoa(time3),
					},
				},
			},
		}

		clientObjs := []runtime.Object{
			nodesObj, todObj,
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      "timeofday-test",
				Namespace: "intel-power",
			},
		}
		r, err := createTimeOfDayReconcilerObject(clientObjs)

		if err != nil {
			t.Error(err)
		}
		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Error(err)
		}
	})
}
