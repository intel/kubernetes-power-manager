package controllers

import (
	"context"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
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
	r := &UncoreReconciler{cl, ctrl.Log.WithName("testing"), s,nil}

	return r, nil
}

func TestDieSelectMinError(t *testing.T){
	max:=uint(2400000)
	pkg:=uint(0)
	die:=uint(0)
	uncoreName:=""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name: uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg,Die: &die,Max: &max},
				},
			},
		},
	}

	hostmk := new(hostMock)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary=hostmk
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

func TestDieSelectMaxError(t *testing.T){
	min:=uint(1200000)
	pkg:=uint(0)
	die:=uint(0)
	uncoreName:=""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name: uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Package: &pkg,Die: &die,Min: &min},
				},
			},
		},
	}

	hostmk := new(hostMock)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary=hostmk
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
func TestDieSelectPackageError(t *testing.T){
	max:=uint(2400000)
	min:=uint(1200000)
	die:=uint(0)
	uncoreName:=""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name: uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
				DieSelectors: &[]powerv1.DieSelector{
					{Die: &die,Max: &max,Min: &min},
				},
			},
		},
	}

	hostmk := new(hostMock)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary=hostmk
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
func TestNoSelectorsOrSystemValues(t *testing.T){
	uncoreName:=""
	clientObjs := []runtime.Object{
		&powerv1.Uncore{
			ObjectMeta: metav1.ObjectMeta{
				Name: uncoreName,
				Namespace: "intel-power",
			},
			Spec: powerv1.UncoreSpec{
			},
		},
	}

	hostmk := new(hostMock)
	r, err := createUncoreReconcilerObject(clientObjs)
	r.PowerLibrary=hostmk
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

