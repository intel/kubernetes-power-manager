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
	"reflect"
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	freq "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/frequency"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/configstate"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	ConfigState configstate.Configs
}

const (
	SHARED_CONFIG_NAME string = "shared-config"
)

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
		//	effectedCpus := configState[req.NamespacedName.Name]
			effectedCPUs := r.ConfigState.RemoveCPUFromState(req.NamespacedName.Name)

			logger.Info(fmt.Sprintf("CPUs that were tuned by the deleted Config: %v", effectedCPUs))
		//	delete(configState, req.NamespacedName.Name)

			sharedConfig := &powerv1alpha1.Config{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: req.NamespacedName.Namespace,
				Name:      SHARED_CONFIG_NAME,
			}, sharedConfig)
			if err != nil {
				if errors.IsNotFound(err) {
					logger.Info(fmt.Sprintf("Shared Config could not be found, cannot reset frequency of CPUs %v", effectedCPUs))
					return ctrl.Result{}, nil
				}

				return ctrl.Result{}, err
			}

			maxCpuFrequency := sharedConfig.Spec.Profile.Spec.Max
			minCpuFrequency := sharedConfig.Spec.Profile.Spec.Min

			// For now just log that the CPU of the group of CPUs is being updating.
			// When frequency tuning is implemented, may need to update them individually.
			logger.Info(fmt.Sprintf("Updating max frequency of CPUs %v to %dMHz", effectedCPUs, maxCpuFrequency))
			logger.Info(fmt.Sprintf("Updating min frequency of CPUs %v to %dMHz", effectedCPUs, minCpuFrequency))

			return ctrl.Result{}, nil
		}

		logger.Error(err, "failed to get Config instance")
		return ctrl.Result{}, err
	}

	cpusEffected := config.Spec.CpuIds
	logger.Info(fmt.Sprintf("CPUs now effected: %v", cpusEffected))
	maxCpuFrequency := config.Spec.Profile.Spec.Max
	minCpuFrequency := config.Spec.Profile.Spec.Min

	// Check to see if this is a Creation or Update
	if oldConfig, exists := r.ConfigState.Configs[req.NamespacedName.Name]; exists {
		// The Config has been update
		oldConfigCpusSorted := oldConfig.CPUs
		newConfigCpusSorted := cpusEffected
		sort.Strings(oldConfigCpusSorted)
		sort.Strings(newConfigCpusSorted)
		changedCpus := make([]string, 0)

		if !reflect.DeepEqual(oldConfigCpusSorted, newConfigCpusSorted) {
			logger.Info("CPUs for this Config have changed, updating...")

			if len(oldConfigCpusSorted) > len(newConfigCpusSorted) {
				// CPUs have been removed from this Config, need to revert their frequency back to the shared frequency
				for _, id := range oldConfigCpusSorted {
					if !idInConfig(id, newConfigCpusSorted) {
						changedCpus = append(changedCpus, id)
					}
				}

				// Need changed CPUs for frequency resetting
				oldConfig.CPUs = newConfigCpusSorted
				logger.Info(fmt.Sprintf("Reverting CPU(s) back to Shared frequency: %v", changedCpus))

				// Set changedCpus to empty so their frequencies don't accidently get reset after this
				changedCpus = make([]string, 0)
			} else {
				// TODO: check if necessary
				for _, id := range cpusEffected {
					if !idInConfig(id, oldConfig.CPUs) {
						changedCpus = append(changedCpus, id)
					}
				}

				oldConfig.CPUs = newConfigCpusSorted
			}

			r.ConfigState.UpdateConfigState(req.NamespacedName.Name, oldConfig)
			//configState[req.NamespacedName.Name] = oldConfig
		}

		// If the max/min frequencies of the Config have changed, we need to alter every CPU's frequency
		// If not, only the changed CPUs need to be altered

		if maxCpuFrequency != oldConfig.Max {
			logger.Info(fmt.Sprintf("Updating maximum frequency of every CPU in Config: %v", r.ConfigState.Configs[req.NamespacedName.Name]))
		} else {
			logger.Info(fmt.Sprintf("Updating maximum frequency of changed CPUs: %v", changedCpus))
		}

		if minCpuFrequency != oldConfig.Min {
			logger.Info(fmt.Sprintf("Updating minimum frequency of every CPU in Config: %v", r.ConfigState.Configs[req.NamespacedName.Name]))
		} else {
			logger.Info(fmt.Sprintf("Updating minimum frequency of changed CPUs: %v", changedCpus))
		}

		return ctrl.Result{}, nil
	}

	// Update the frequency of the cores
	err = freq.AdjustCpuFrequency(cpusEffected, maxCpuFrequency, freq.MAX_FREQ_PATH)
	if err != nil {
		logger.Error(err, "Error occured while updating Max frequency")
		return ctrl.Result{}, nil
	}

	err = freq.AdjustCpuFrequency(cpusEffected, minCpuFrequency, freq.MIN_FREQ_PATH)
	if err != nil {
		logger.Error(err, "Error occured while updating Min frequency")
                return ctrl.Result{}, nil
	}
	/*
	for _, cpu := range cpusEffected {
		err = freq.AdjustCpuFrequency(cpu, maxCpuFrequency, freq.MAX_FREQ_PATH)
		if err != nil {
			logger.Error(err, "Error occured while updating Max frequency")
			return ctrl.Result{}, nil
		}

		err = freq.AdjustCpuFrequency(cpu, minCpuFrequency, freq.MIN_FREQ_PATH)
		if err != nil {
			logger.Error(err, "Error occured while updating Min frequency")
			return ctrl.Result{}, nil
		}
	}*/

	cs := configstate.ConfigState{}
	cs.CPUs = cpusEffected
	cs.Max = maxCpuFrequency
	cs.Min = minCpuFrequency
	r.ConfigState.UpdateConfigState(req.NamespacedName.Name, cs)
	//configState[req.NamespacedName.Name] = cs

	return ctrl.Result{}, nil
}

func idInConfig(newId string, config []string) bool {
	for _, id := range config {
		if newId == id {
			return true
		}
	}

	return false
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.Config{}).
		Complete(r)
}
