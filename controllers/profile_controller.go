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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProfileReconciler reconciles a Profile object
type ProfileReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=power.intel.com,resources=profiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=profiles/status,verbs=get;update;patch

func (r *ProfileReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("profile", req.NamespacedName)
	logger.Info("Reconciling Profile")

	/*
		// Make sure there is a Shared Profile available
		profileList := &powerv1alpha1.ProfileList{}
		err := r.Client.List(context.TODO(), profileList)
		if err != nil {
			logger.Info("Something went wrong")
			return ctrl.Result{}, err
		}

		// I don't think there needs to be a Shared Profile, only that when there is, its Config gets created by the Profile controller instead of the Pod controller
			if sharedProfileNotFound(profileList) {
				errorMsg := "Shared Profile is required but was not found"
				return ctrl.Result{}, errors.NewServiceUnavailable(errorMsg)
			}
	*/

	profile := &powerv1alpha1.Profile{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			// Whe na Profile cannot be found, we assume it has been deleted. We need to check if there is a
			// corresponding Config and, if there is, delete that too. We leave the cleanup of resetting the
			// frequencies of the effected CPUs to the Config controller.

			logger.Info(fmt.Sprintf("Profile %v has been deleted, cleaning up...", req.NamespacedName))
			config := &powerv1alpha1.Config{}
			configName := fmt.Sprintf("%s-config", req.NamespacedName.Name)
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name:      configName,
			}, config)
			if err != nil {
				if errors.IsNotFound(err) {
					//No Config was found, so no cleanup is necessary
					return ctrl.Result{}, nil
				}

				logger.Error(err, "Something went wrong")
				return ctrl.Result{}, err
			}

			// Config exists so must cleanup
			err = r.Client.Delete(context.TODO(), config)
			if err != nil {
				logger.Error(err, "Deletion failed")
				return ctrl.Result{}, err
			}

			// Returning a nil error will stop the requeue
			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	logger.Info("Specs for Profile:")
	logger.Info(fmt.Sprintf("Name: %s", profile.Spec.Name))
	logger.Info(fmt.Sprintf("Max: %d", profile.Spec.Max))
	logger.Info(fmt.Sprintf("Min: %d", profile.Spec.Min))

	config := &powerv1alpha1.Config{}
	configName := fmt.Sprintf("%s-config", req.NamespacedName.Name)
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: req.NamespacedName.Namespace,
		Name:      configName,
	}, config)
	if err != nil {
		if errors.IsNotFound(err) {
			// This is a fresh Profile, so we may need to create a corresponding Config.
			// If the Profile is the designated Shared configuration for all of the shared-pool cores,
			// this controller is responsible for creating the associated Config. If it's an
			// Exclusive Profile, Config creation is left to the Pod controller when the Profile is requested.
			// The Shared configuration is recognised by having the name "Shared".

			if profile.Spec.Name == "Shared" {
				logger.Info("This Profile has been designated as the Shared Profile...")
				logger.Info("Creating Config for Shared Profile...")
				// TODO: Update with package that Conor is working on
				nodes := []string{"Placeholder"}
				// TODO: Update with package that Conor is working on
				cpuIDs := []string{"1-32"}
				configSpec := &powerv1alpha1.ConfigSpec{
					Nodes:   nodes,
					CpuIds:  cpuIDs,
					Profile: *profile,
				}
				config := &powerv1alpha1.Config{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: req.NamespacedName.Namespace,
						Name:      configName,
					},
				}
				config.Spec = *configSpec
				err = r.Client.Create(context.TODO(), config)
				if err != nil {
					logger.Error(err, fmt.Sprintf("Failed to create config for %s Profile", profile.Name))
				}

				return ctrl.Result{}, nil
			}
		} else {
			logger.Error(err, "Some other error")
			return ctrl.Result{}, err
		}
	}

	// If the Config for the supplied Profile already exists, we assume it has been updated and update
	// the Config. We leave all of the State updating to the Config controller.
	logger.Info(fmt.Sprintf("%s Config already exists, updating...", configName))
	// TODO: Update with package that Conor is working on
	nodes := []string{"Placeholder"}
	// TODO: Update with package that Conor is working on
	cpuIDs := []string{"1-32"}
	updatedSpec := &powerv1alpha1.ConfigSpec{
		Nodes:   nodes,
		CpuIds:  cpuIDs,
		Profile: *profile,
	}
	config.Spec = *updatedSpec
	err = r.Client.Update(context.TODO(), config)
	if err != nil {
		logger.Error(err, "Error while updating config")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.Profile{}).
		Complete(r)
}
