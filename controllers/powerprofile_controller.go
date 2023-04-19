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
	rt "runtime"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MaxFrequencyFile = "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"
	MinFrequencyFile = "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_min_freq"
)

// performance          ===>  priority level 0
// balance_performance  ===>  priority level 1
// balance_power        ===>  priority level 2
// power                ===>  priority level 3

var profilePercentages map[string]map[string]float64 = map[string]map[string]float64{
	"performance": map[string]float64{
		"resource":   .40,
		"difference": 0.0,
	},
	"balance_performance": map[string]float64{
		"resource":   .60,
		"difference": .25,
	},
	"balance_power": map[string]float64{
		"resource":   .80,
		"difference": .50,
	},
	"power": map[string]float64{
		"resource":   1.0,
		"difference": 0.0,
	},
	// We have the empty string here so users can create Power Profiles that are not associated with SST-CP
	"": map[string]float64{},
}

// PowerProfileReconciler reconciles a PowerProfile object
type PowerProfileReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PowerLibrary power.Host
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles/status,verbs=get;update;patch

// Reconcile method that implements the reconcile loop
func (r *PowerProfileReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)
	logger.Info("Reconciling PowerProfile")

	// Node name is passed down via the downwards API and used to make sure the PowerProfile is for this node
	nodeName := os.Getenv("NODE_NAME")

	profile := &powerv1.PowerProfile{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			// First we need to remove the profile from the Power library, this will in turn remove the pool,
			// which will also move the cores back to the Shared/Default pool and reconfigure them. We then
			// need to remove the Power Workload from the cluster, which in this case will do nothing as
			// everything has already been removed. Finally, we remove the Extended Resources from the Node
			pool := r.PowerLibrary.GetExclusivePool(req.Name)
			if pool == nil {
				logger.Info("Attempted to remove non existing pool", "pool", req.Name)
			}
			err = pool.Remove()
			if err != nil {
				logger.Error(err, "error deleting Power Profile From Library")
				return ctrl.Result{}, err
			}

			powerWorkloadName := fmt.Sprintf("%s-%s", req.NamespacedName.Name, nodeName)
			powerWorkload := &powerv1.PowerWorkload{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name:      powerWorkloadName,
				Namespace: req.NamespacedName.Namespace,
			}, powerWorkload)
			if err != nil {
				if !errors.IsNotFound(err) {
					logger.Error(err, fmt.Sprintf("error deleting PowerWorkload '%s' from cluster", powerWorkloadName))
					return ctrl.Result{}, err
				}
			} else {
				err = r.Client.Delete(context.TODO(), powerWorkload)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error deleting Power Workload '%s' from cluster", powerWorkloadName))
					return ctrl.Result{}, err
				}
			}

			// Remove the Extended Resources for this PowerProfile from the Node
			err = r.removeExtendedResources(nodeName, req.NamespacedName.Name)
			if err != nil {
				logger.Error(err, "error removing Extended Resources from node")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	// Make sure the EPP value is one of the four correct ones or empty in the case of a user-created profile
	if _, exists := profilePercentages[profile.Spec.Epp]; !exists {
		incorrectEppErr := errors.NewServiceUnavailable(fmt.Sprintf("EPP value not allowed: %v - deleting PowerProfile CRD", profile.Spec.Epp))
		logger.Error(incorrectEppErr, "error reconciling PowerProfile")

		err = r.Client.Delete(context.TODO(), profile)
		if err != nil {
			logger.Error(err, fmt.Sprintf("error deleting PowerProfile %s with incorrect EPP value %s", profile.Spec.Name, profile.Spec.Epp))
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if profile.Spec.Max < profile.Spec.Min {
		maxLowerThanMaxError := errors.NewServiceUnavailable("Max frequency value cannot be lower than Minimum frequency value")
		logger.Error(maxLowerThanMaxError, fmt.Sprintf("error creating Profile '%s'", profile.Spec.Name))
		return ctrl.Result{}, nil
	}

	absoluteMaximumFrequency, absoluteMinimumFrequency, err := getMaxMinFrequencyValues()
	if err != nil {
		logger.Error(err, "error retrieving frequency values from Node")
		return ctrl.Result{}, nil
	}

	// If the Profile is shared (epp == power) then the associated Pool will not be created in the Power Library
	if profile.Spec.Epp == "power" {

		if profile.Spec.Max < absoluteMinimumFrequency || profile.Spec.Min < absoluteMinimumFrequency {
			frequencyTooLowError := errors.NewServiceUnavailable(fmt.Sprintf("Maximum or Minimum frequency value cannot be below %d", absoluteMinimumFrequency))
			logger.Error(frequencyTooLowError, "error creating Shared Power Profile")
			return ctrl.Result{}, nil
		}
		actualEpp := profile.Spec.Epp
		if isEppSupported() {
			actualEpp = ""
		}
		powerProfile, _ := power.NewPowerProfile(profile.Spec.Name, uint(profile.Spec.Min), uint(profile.Spec.Max), profile.Spec.Governor, actualEpp)
		err = r.PowerLibrary.GetSharedPool().SetPowerProfile(powerProfile)
		if err != nil {
			logger.Error(err, "could not set power profile for shared pool")
			return ctrl.Result{}, nil
		}
		logger.Info(fmt.Sprintf("Shared Power Profile successfully created: name - %s max - %d Min - %d EPP - %s", profile.Spec.Name, profile.Spec.Max, profile.Spec.Min, profile.Spec.Epp))
		return ctrl.Result{}, nil
	} else {
		var profileMaxFreq int
		var profileMinFreq int
		if profile.Spec.Epp != "" && profile.Spec.Max == 0 && profile.Spec.Min == 0 {
			profileMaxFreq = int(float64(absoluteMaximumFrequency) - (float64((absoluteMaximumFrequency - absoluteMinimumFrequency)) * profilePercentages[profile.Spec.Epp]["difference"]))
			profileMinFreq = int(profileMaxFreq) - 200
		} else {
			profileMaxFreq = profile.Spec.Max
			profileMinFreq = profile.Spec.Min
		}

		if profileMaxFreq == 0 || profileMinFreq == 0 {
			cannotBeZeroError := errors.NewServiceUnavailable("max or Min frequency cannot be zero")
			logger.Error(cannotBeZeroError, "error creating Profile '%s'", profile.Spec.Name)
			return ctrl.Result{}, nil
		}

		profileFromLibrary := r.PowerLibrary.GetExclusivePool(profile.Spec.Name)
		actualEpp := profile.Spec.Epp
		if isEppSupported() {
			actualEpp = ""
		}
		powerProfile, _ := power.NewPowerProfile(profile.Spec.Name, uint(profileMinFreq), uint(profileMaxFreq), profile.Spec.Governor, actualEpp)
		if profileFromLibrary == nil {
			pool, err := r.PowerLibrary.AddExclusivePool(profile.Spec.Name)
			if err != nil {
				logger.Error(err, "failed to create power profile")
				return ctrl.Result{}, err
			}
			err = pool.SetPowerProfile(powerProfile)
			if err != nil {
				logger.Error(err, fmt.Sprintf("error adding Profile '%s' to Power Library for Host '%s'", profile.Spec.Name, nodeName))
				return ctrl.Result{}, err
			}

			// Create the Extended Resources for the profile
			err = r.createExtendedResources(nodeName, profile.Spec.Name, profile.Spec.Epp)
			if err != nil {
				logger.Error(err, "error creating extended resources for base profile")
				return ctrl.Result{}, err
			}
		} else {
			err = r.PowerLibrary.GetExclusivePool(profile.Spec.Name).SetPowerProfile(powerProfile)
			if err != nil {
				logger.Error(err, fmt.Sprintf("error updating Profile '%s' to Power Library for Node '%s'", profile.Spec.Name, nodeName))
				return ctrl.Result{}, err
			}
		}

		logger.Info(fmt.Sprintf("Power Profile successfully created: Name - %s Max - %d Min - %d EPP - %s", profile.Spec.Name, profileMaxFreq, profileMinFreq, profile.Spec.Epp))
	}

	workloadName := fmt.Sprintf("%s-%s", profile.Spec.Name, nodeName)
	workload := &powerv1.PowerWorkload{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Name:      workloadName,
		Namespace: req.NamespacedName.Namespace,
	}, workload)
	if err != nil {
		if errors.IsNotFound(err) {
			powerWorkloadSpec := &powerv1.PowerWorkloadSpec{
				Name: workloadName,
				Node: powerv1.WorkloadNode{
					Name: nodeName,
				},
				PowerProfile: profile.Spec.Name,
			}

			powerWorkload := &powerv1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workloadName,
					Namespace: req.NamespacedName.Namespace,
				},
			}
			powerWorkload.Spec = *powerWorkloadSpec

			err = r.Client.Create(context.TODO(), powerWorkload)
			if err != nil {
				logger.Error(err, fmt.Sprintf("error creating Power Workload '%s'", workloadName))
				return ctrl.Result{}, err
			}

			logger.Info(fmt.Sprintf("Power Workload successfully created: Name - %s Profile - %s", workloadName, profile.Spec.Name))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// If the workload already exists then the Power Profile was just updated and the Power Library will take care of reconfiguring cores
	return ctrl.Result{}, nil
}

func (r *PowerProfileReconciler) createExtendedResources(nodeName string, profileName string, eppValue string) error {
	node := &corev1.Node{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil {
		return err
	}

	numCPUsOnNode := float64(rt.NumCPU())
	numExtendedResources := int64(numCPUsOnNode * profilePercentages[eppValue]["resource"])
	profilesAvailable := resource.NewQuantity(numExtendedResources, resource.DecimalSI)
	extendedResourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, profileName))
	node.Status.Capacity[extendedResourceName] = *profilesAvailable

	err = r.Client.Status().Update(context.TODO(), node)
	if err != nil {
		return err
	}

	return nil
}

func (r *PowerProfileReconciler) removeExtendedResources(nodeName string, profileName string) error {
	node := &corev1.Node{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil {
		return err
	}

	newNodeCapacityList := make(map[corev1.ResourceName]resource.Quantity)
	extendedResourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, profileName))
	for resourceFromNode, numberOfResources := range node.Status.Capacity {
		if resourceFromNode == extendedResourceName {
			continue
		}
		newNodeCapacityList[resourceFromNode] = numberOfResources
	}

	node.Status.Capacity = newNodeCapacityList
	err = r.Client.Status().Update(context.TODO(), node)
	if err != nil {
		return err
	}

	return nil
}

func getMaxMinFrequencyValues() (int, int, error) {
	absoluteMaximumFrequencyByte, err := os.ReadFile(MaxFrequencyFile)
	if err != nil {
		return 0, 0, err
	}
	absoluteMaximumFrequencyString := string(absoluteMaximumFrequencyByte)
	absoluteMaximumFrequency, err := strconv.Atoi(strings.Split(absoluteMaximumFrequencyString, "\n")[0])
	if err != nil {
		return 0, 0, err
	}

	absoluteMinimumFrequencyByte, err := os.ReadFile(MinFrequencyFile)
	if err != nil {
		return 0, 0, err
	}
	absoluteMinimumFrequencyString := string(absoluteMinimumFrequencyByte)
	absoluteMinimumFrequency, err := strconv.Atoi(strings.Split(absoluteMinimumFrequencyString, "\n")[0])
	if err != nil {
		return 0, 0, err
	}

	absoluteMaximumFrequency = absoluteMaximumFrequency / 1000
	absoluteMinimumFrequency = absoluteMinimumFrequency / 1000

	return absoluteMaximumFrequency, absoluteMinimumFrequency, nil
}

// SetupWithManager specifies how the controller is built and watch a CR and other resources that are owned and managed by the controller
func (r *PowerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.PowerProfile{}).
		Complete(r)
}

func isEppSupported() bool {
	_, err := os.Stat("/sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference")
	return !os.IsNotExist(err)
}
