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
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const SHARED_CONFIG_NAME string = "shared-config"

var configState map[string][]string = make(map[string][]string)

// +kubebuilder:rbac:groups=power.intel.com,resources=configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=configs/status,verbs=get;update;patch

func (r *ConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("config", req.NamespacedName)

	config := &powerv1alpha1.Config{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, config)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Config deleted: %s; cleaning up...", req.NamespacedName.Name))
			effectedCpus := configState[req.NamespacedName.Name]

			logger.Info(fmt.Sprintf("CPUs that were tuned by the deleted Config: %s", effectedCpus))
			delete(configState, req.NamespacedName.Name)
			logger.Info(fmt.Sprintf("%v", configState))

			sharedConfig := &powerv1alpha1.Config{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name:      SHARED_CONFIG_NAME,
			}, sharedConfig)
			if err != nil {
				if errors.IsNotFound(err) {
					logger.Info(fmt.Sprintf("Shared Config could not be found, cannot reset frequency of CPUs %s", effectedCpus))
					return ctrl.Result{}, nil
				}

				return ctrl.Result{}, err
			}

			maxCpuFrequency := sharedConfig.Spec.Profile.Spec.Max
			minCpuFrequency := sharedConfig.Spec.Profile.Spec.Min

			// For now just log that the CPU of the group of CPUs is being updating.
			// When frequency tuning is implemented, may need to update them individually.
			logger.Info(fmt.Sprintf("Updating max frequency of CPUs %v to %dMHz", effectedCpus, maxCpuFrequency))
			logger.Info(fmt.Sprintf("Updating min frequency of CPUs %v to %dMHz", effectedCpus, minCpuFrequency))

			return ctrl.Result{}, nil
		}

		logger.Error(err, "failed to get Config instance")
		return ctrl.Result{}, err
	}

	cpusEffected := config.Spec.CpuIds
	maxCpuFrequency := config.Spec.Profile.Spec.Max
	minCpuFrequency := config.Spec.Profile.Spec.Min

	// For now just log that the CPU of the group of CPUs is being updating.
	// When frequency tuning is implemented, may need to update them individually.
	logger.Info(fmt.Sprintf("Updating max frequency of CPUs %v to %dMHz", cpusEffected, maxCpuFrequency))
	logger.Info(fmt.Sprintf("Updating min frequency of CPUs %v to %dMHz", cpusEffected, minCpuFrequency))
	configState[req.NamespacedName.Name] = cpusEffected

	logger.Info(fmt.Sprintf("%v", configState))

	/*
		logger.Info("Config for this Config")
		logger.Info(fmt.Sprintf("Profile: %v", config.Spec.Profile))

		logger.Info("Nodes effected:")
		for _, n := range config.Spec.Nodes {
			logger.Info(fmt.Sprintf("- %s", n))
		}

		logger.Info("CPUs effected:")
		for _, c := range config.Spec.CpuIds {
			logger.Info(fmt.Sprintf("- %s", c))
		}
	*/

	/* profile := &powerv1alpha1.Profile{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, profile)
	if err != nil {
		logger.Error(err, "failed to get Profile instance")
		return ctrl.Result{}, err
	}

	logger.Info("Profile configuration:")
	logger.Info(fmt.Sprintf("Name: %s", profile.Spec.Name))
	logger.Info(fmt.Sprintf("Max: %d", profile.Spec.Max))
	logger.Info(fmt.Sprintf("Min: %d", profile.Spec.Min))
	*/
	/*
		profileSpec := &powerv1alpha1.ProfileSpec{"NewProfile", 2800, 2200, true}
		profile := &powerv1alpha1.Profile{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name: "newprofile",
			},
		}
		profile.Spec = *profileSpec
		r.Client.Create(context.TODO(), profile)
	*/
	return ctrl.Result{}, nil
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.Config{}).
		Complete(r)
}
