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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/intel/power-optimization-library/pkg/power"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
)

// CStatesReconciler reconciles a CStates object
type CStatesReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PowerLibrary power.Host
}

//+kubebuilder:rbac:groups=power.intel.com,resources=cstates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=power.intel.com,resources=cstates/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.4/pkg/reconcile
func (r *CStatesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	logger := r.Log.WithValues("cStates", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		logger.Error(fmt.Errorf("incorrect namespace"), "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{}, nil
	}
	nodeName := os.Getenv("NODE_NAME")
	logger.Info("Reconciling C-states")

	cStatesCRD := &powerv1.CStates{}
	cStatesConfigRetrieveError := r.Client.Get(ctx, req.NamespacedName, cStatesCRD)
	if cStatesConfigRetrieveError != nil && !errors.IsNotFound(cStatesConfigRetrieveError) {
		// if it's not found error we want to reset everything on the node which will happen anyway
		return ctrl.Result{}, cStatesConfigRetrieveError
	}

	logger.V(3).Info("Checking if node exists")
	err = r.checkIfNodeExists(ctx, cStatesCRD, &logger)
	if err != nil {
		logger.Error(err, "CRD validation failed")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	// don't do anything else if we're not running on the node specified in the CRD Name
	logger.V(4).Info("Checking if request is intended for this node")
	if req.Name != nodeName {
		return ctrl.Result{}, nil
	}

	logger.V(4).Info("Checking to verify that C-States exist")
	err = r.verifyCSStatesExist(&cStatesCRD.Spec, &logger)
	if err != nil {
		logger.Error(err, "CStates validation failed")
	}
	// prepare system by resetting configuration
	logger.V(4).Info("Resetting system configuration")
	err = r.restoreCStates(ctx, &logger)
	if err != nil {
		logger.Error(err, "failed to restore CStates")
		return ctrl.Result{}, err
	}

	logger.V(4).Info("Applying C-States to CRD Spec")
	err = r.applyCStates(&cStatesCRD.Spec, &logger)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("Successfully Applied new CStates config")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CStatesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.CStates{}).
		Complete(r)
}

func (r *CStatesReconciler) checkIfNodeExists(ctx context.Context, cStatesCRD *powerv1.CStates, logger *logr.Logger) error {
	nodes := &powerv1.PowerNodeList{}
	logger.V(5).Info("retreivingall Node Listed and checking names match CRDs")
	err := r.Client.List(ctx, nodes)
	if err != nil {
		logger.Error(err, "failed to retrieve list of nodes")
		return err
	}

	exists := false
	logger.V(5).Info("The C-Stats CRD names match the list in powerNodes and if true set the bool")
	for _, item := range nodes.Items {
		if item.Name == cStatesCRD.Name {
			exists = true
			break
		}
	}
	if !exists {
		return errors.NewBadRequest("CStates CRD name must match name of one of the powerNodes")
	}
	return nil
}

func (r *CStatesReconciler) verifyCSStatesExist(cStatesSpec *powerv1.CStatesSpec, logger *logr.Logger) error {
	nodeName := os.Getenv("NODE_NAME")
	results := new(multierror.Error)
	logger.V(5).Info("Checking the Power Library to confirm that C-States are in the shared pool")
	err := r.PowerLibrary.ValidateCStates(cStatesSpec.SharedPoolCStates)
	if err != nil {
		results = multierror.Append(results, fmt.Errorf("C-States reconcile on node %s: %w", nodeName, err))
	}
	for poolName, poolCStateList := range cStatesSpec.ExclusivePoolCStates {
		logger.V(5).Info("Checking the Power Library to confirm that C-States are in the exclusive pool", "name", poolName)
		err := r.PowerLibrary.ValidateCStates(poolCStateList)
		if err != nil {
			results = multierror.Append(results, fmt.Errorf("C-States reconcile on node %s: %w", nodeName, err))
		}
	}
	for coreID, coreCStates := range cStatesSpec.IndividualCoreCStates {
		logger.V(5).Info("Checking the Power Library to confirm that S-States are a individual core", "coreID", coreID)
		err := r.PowerLibrary.ValidateCStates(coreCStates)
		if err != nil {
			results = multierror.Append(results, fmt.Errorf("C-States reconcile on node %s: %w", nodeName, err))
		}
	}
	return results.ErrorOrNil()
}

func (r *CStatesReconciler) applyCStates(cStatesSpec *powerv1.CStatesSpec, logger *logr.Logger) error {
	// if CRD is empty or were handling a delete event this function will do nothing
	logger.V(5).Info("Checking if CRD is empty or a deleted event")
	results := new(multierror.Error)
	// individual cores
	for core, cStatesMap := range cStatesSpec.IndividualCoreCStates {
		coreID, err := strconv.Atoi(core)
		if err != nil {
			results = multierror.Append(results, fmt.Errorf("%s must be an integer core ID", core))
			continue
		}
		logger.V(5).Info("Applying C-States to cores", "coreID", coreID)
		coreObj := r.PowerLibrary.GetAllCpus().ByID(uint(coreID))
		if coreObj == nil {
			return fmt.Errorf("invalid core id ID: %d", coreID)
		}
		results = multierror.Append(results, coreObj.SetCStates(cStatesMap))
	}
	// exclusive pools
	for poolName, cStatesMap := range cStatesSpec.ExclusivePoolCStates {
		pool := r.PowerLibrary.GetExclusivePool(poolName)
		if pool == nil {
			return fmt.Errorf("pool with name %s doesn not exist", poolName)
		}
		err := pool.SetCStates(cStatesMap)
		results = multierror.Append(results, err)
		logger.V(5).Info("Applying C-States to exclusive pools", "name", poolName)
	}
	//shared pool
	err := r.PowerLibrary.GetSharedPool().SetCStates(cStatesSpec.SharedPoolCStates)
	results = multierror.Append(results, err)
	logger.V(5).Info("Applying C-States to Shared pool")

	return results.ErrorOrNil()
}

func (r *CStatesReconciler) restoreCStates(ctx context.Context, logger *logr.Logger) error {
	var results *multierror.Error

	logger.V(5).Info("Resetting to default state for the shared pool")
	results = multierror.Append(results, r.PowerLibrary.GetSharedPool().SetCStates(nil))

	for _, pool := range *r.PowerLibrary.GetAllExclusivePools() {
		logger.V(5).Info("Resetting C-States on pool", "name", pool.Name())
		results = multierror.Append(results, pool.SetCStates(nil))
	}

	logger.V(5).Info("Resetting C-States on each core")
	for _, core := range *r.PowerLibrary.GetAllCpus() {
		results = multierror.Append(results, core.SetCStates(nil))
	}
	return results.ErrorOrNil()
}
