/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
)

// UncoreReconciler reconciles a Uncore object
type UncoreReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PowerLibrary power.Host
}

//+kubebuilder:rbac:groups=power.intel.com,resources=uncores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=power.intel.com,resources=uncores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=power.intel.com,resources=uncores/finalizers,verbs=update
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=use

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Uncore object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *UncoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	nodeName := os.Getenv("NODE_NAME")
	// uncore is not for this node
	if req.Name != nodeName {
		return ctrl.Result{}, nil
	}
	logger := r.Log.WithValues("uncore", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		err = fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{Requeue: false}, err
	}
	logger.Info("Reconciling uncore")
	uncore := &powerv1.Uncore{}
	defer func() { _ = writeUpdatedStatusErrsIfRequired(ctx, r.Status(), uncore, err) }()
	// resets all values to allow for CRD updates
	err = r.PowerLibrary.Topology().SetUncore(nil)
	if err != nil {
		logger.Error(err, "could not reset uncore for topology")
		return ctrl.Result{}, err
	}
	for _, pkg := range *r.PowerLibrary.Topology().Packages() {
		if err := pkg.SetUncore(nil); err != nil {
			logger.Error(err, "could not reset uncore on package")
			return ctrl.Result{}, err
		}
		for _, die := range *pkg.Dies() {
			if err := die.SetUncore(nil); err != nil {
				logger.Error(err, "could not reset uncore on die")
				return ctrl.Result{}, err
			}
		}
	}
	err = r.Client.Get(context.TODO(), req.NamespacedName, uncore)
	if err != nil {
		// uncore deleted so we can ignore here since everything is already reset
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil

		} else {
			logger.Error(err, "could not retrieve uncore specification")
			return ctrl.Result{Requeue: false}, err
		}
	}
	// setting system wide uncore
	if uncore.Spec.SysMax != nil && uncore.Spec.SysMin != nil {
		p_uncore, err := power.NewUncore(*uncore.Spec.SysMin, *uncore.Spec.SysMax)
		if err != nil {
			logger.Error(err, "error creating uncore")
			return ctrl.Result{Requeue: false}, err
		}
		err = r.PowerLibrary.Topology().SetUncore(p_uncore)
		if err != nil {
			logger.Error(err, "error setting uncore")
			return ctrl.Result{Requeue: false}, err
		}
	}
	// setting die/package specific uncore
	if uncore.Spec.DieSelectors != nil {
		for _, dieselect := range *uncore.Spec.DieSelectors {
			if dieselect.Max == nil || dieselect.Min == nil || dieselect.Package == nil {
				err = apierrors.NewServiceUnavailable("die selector max, min and package fields must not be empty")
				logger.Error(err, "max, min and package values must be set for die selector")
				return ctrl.Result{Requeue: false}, err
			}
			p_uncore, err := power.NewUncore(*dieselect.Min, *dieselect.Max)
			if err != nil {
				logger.Error(err, "error creating uncore")
				return ctrl.Result{Requeue: false}, err
			}
			// package tuning
			if dieselect.Die == nil {
				pkg := r.PowerLibrary.Topology().Package(*dieselect.Package)
				// used to prevent invalid package or die input causing a panic
				if pkg == nil {
					err = apierrors.NewServiceUnavailable(fmt.Sprintf("invalid package: %d", *dieselect.Package))
					return ctrl.Result{Requeue: false}, err
				}
				err = pkg.SetUncore(p_uncore)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error setting uncore for package %d and die %d", dieselect.Package, dieselect.Die))
					return ctrl.Result{Requeue: false}, err
				}
			} else { // die tuning
				pkg := r.PowerLibrary.Topology().Package(*dieselect.Package)
				if pkg == nil {
					err = apierrors.NewServiceUnavailable(fmt.Sprintf("invalid package: %d", *dieselect.Package))
					return ctrl.Result{Requeue: false}, err
				}
				die := pkg.Die(*dieselect.Die)
				if die == nil {
					err = apierrors.NewServiceUnavailable(fmt.Sprintf("invalid die: %d", *dieselect.Die))
					return ctrl.Result{Requeue: false}, err
				}
				err = die.SetUncore(p_uncore)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error setting uncore for package %d and die %d", dieselect.Package, dieselect.Die))
					return ctrl.Result{Requeue: false}, err
				}
			}
		}
	}
	if uncore.Spec.DieSelectors == nil && uncore.Spec.SysMax == nil && uncore.Spec.SysMin == nil {
		err = apierrors.NewServiceUnavailable("no system wide or per die min/max values were provided")
		logger.Error(err, "error setting uncore values")
		return ctrl.Result{Requeue: false}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UncoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.Uncore{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
