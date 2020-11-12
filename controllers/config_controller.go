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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=power.intel.com,resources=configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=configs/status,verbs=get;update;patch

func (r *ConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("config", req.NamespacedName)

	config := &powerv1alpha1.Config{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, config)
	if err != nil {
		logger.Error(err, "failed to get Config instance")
		return ctrl.Result{}, err
	}

	logger.Info("Config for this Config")
	logger.Info(fmt.Sprintf("Profile: %s", config.Spec.Profile))

	logger.Info("Nodes effected:")
	for _, n := range config.Spec.Nodes {
		logger.Info(fmt.Sprintf("- %s", n))
	}

	logger.Info("CPUs effected:")
	for _, c := range config.Spec.CpuIds {
		logger.Info(fmt.Sprintf("- %s", c))
	}

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

	return ctrl.Result{}, nil
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.Config{}).
		Complete(r)
}
