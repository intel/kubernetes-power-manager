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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	cgp "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cgroupsparser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PowerProfileReconciler reconciles a PowerProfile object
type PowerProfileReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles/status,verbs=get;update;patch

// Reconcile method that implements the reconcile loop
func (r *PowerProfileReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)
	logger.Info("Reconciling PowerProfile")

	profile := &powerv1alpha1.PowerProfile{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			// When a PowerProfile cannot be found, we assume it has been deleted. We need to check if there is a
			// corresponding PowerWorkload and, if there is, delete that too. We leave the cleanup of requesting the
			// frequency resets of the effected CPUs to the PowerWorkload controller.

			logger.Info(fmt.Sprintf("PowerProfile %v has been deleted, cleaning up...", req.NamespacedName))
			workload := &powerv1alpha1.PowerWorkload{}
			workloadName := fmt.Sprintf("%s%s", req.NamespacedName.Name, WorkloadNameSuffix)
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name:      workloadName,
			}, workload)
			if err != nil {
				if errors.IsNotFound(err) {
					// No PowerWorkload was found so nothing to do
					return ctrl.Result{}, nil
				}

				logger.Error(err, "error while trying to retrieve PowerWorkload")
				return ctrl.Result{}, err
			}

			// PowerWorkload exists so must cleanup
			err = r.Client.Delete(context.TODO(), workload)
			if err != nil {
				logger.Error(err, "error while trying to delete PowerWorkload")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	// Check if a PowerWorkload already exists for this PowerProfile, meaning we just need to update it
	workload := &powerv1alpha1.PowerWorkload{}
	workloadName := fmt.Sprintf("%s%s", req.NamespacedName.Name, WorkloadNameSuffix)
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: req.NamespacedName.Namespace,
		Name:      workloadName,
	}, workload)
	if err != nil {
		if errors.IsNotFound(err) {
			// This is a new PowerProfile, so we may need to create the corresponding PowerWorkload.
			// If the PowerProfile is the designated Shared configuration for all of the shared-pool cores,
			// this controller is responsible for creating the associated PowerWorkload. If it's an
			// Exclusive PowerProfile, PowerWorkload creation is left to the PowerPod controller when the PowerProfile is requested.
			// The Shared configuration is recognised by having the name "Shared".

			if profile.Spec.Name == "Shared" {
				logger.Info("Shared PowerProfile detected, creating corresponding PowerWorkload")
				// TODO: Update with correct value when pakcage has been developed
				nodes := []string{"Placeholder"}
				cpuIDs, _ := cgp.GetSharedPool()
				workloadSpec := &powerv1alpha1.PowerWorkloadSpec{
					Nodes:        nodes,
					CpuIds:       cpuIDs,
					PowerProfile: *profile,
				}
				workload := &powerv1alpha1.PowerWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: req.NamespacedName.Namespace,
						Name:      workloadName,
					},
				}
				workload.Spec = *workloadSpec
				err = r.Client.Create(context.TODO(), workload)
				if err != nil {
					logger.Error(err, "error while trying to create PowerWorkload for PowerProfile 'Shared'")
					return ctrl.Result{}, err
				}
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error while trying to retrieve PowerWorkload")
		return ctrl.Result{}, err
	}

	workload.Spec.PowerProfile = *profile
	err = r.Client.Update(context.TODO(), workload)
	if err != nil {
		logger.Error(err, "error while trying to update PowerWorkload")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager specifies how the controller is built and watch a CR and other resources that are owned and managed by the controller
func (r *PowerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerProfile{}).
		Complete(r)
}
