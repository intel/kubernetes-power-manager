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
	"performance": {
		"resource":   .40,
		"difference": 0.0,
	},
	"balance_performance": {
		"resource":   .60,
		"difference": .25,
	},
	"balance_power": {
		"resource":   .80,
		"difference": .50,
	},
	"power": {
		"resource":   1.0,
		"difference": 0.0,
	},
	// We have the empty string here so users can create power profiles that are not associated with SST-CP
	"": {
		"resource": 1.0,
	},
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
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=use

// Reconcile method that implements the reconcile loop
func (r *PowerProfileReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		err := fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{Requeue: false}, err
	}
	logger.Info("reconciling the power profile")

	// Node name is passed down via the downwards API and used to make sure the PowerProfile is for this node
	nodeName := os.Getenv("NODE_NAME")

	profile := &powerv1.PowerProfile{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, profile)
	logger.V(5).Info("retrieving the power profile instances")
	if err != nil {
		if errors.IsNotFound(err) {
			// First we need to remove the profile from the Power library, this will in turn remove the pool,
			// which will also move the cores back to the Shared/Default pool and reconfigure them. We then
			// need to remove the Power Workload from the cluster, which in this case will do nothing as
			// everything has already been removed. Finally, we remove the Extended Resources from the Node
			// first we make sure the profile isn't the one used by the shared pool
			if r.PowerLibrary.GetSharedPool().GetPowerProfile() != nil && req.Name == r.PowerLibrary.GetSharedPool().GetPowerProfile().Name() {
				err := r.PowerLibrary.GetSharedPool().SetPowerProfile(nil)
				if err != nil {
					logger.Error(err, "error setting nil profile")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			pool := r.PowerLibrary.GetExclusivePool(req.Name)
			if pool == nil {
				logger.Info("attempted to remove the non existing pool", "pool", req.Name)
				return ctrl.Result{Requeue: false}, err
			}
			err = pool.Remove()
			if err != nil {
				logger.Error(err, "error deleting the power profile from the library")
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
					logger.Error(err, fmt.Sprintf("error deleting the power workload '%s' from the cluster", powerWorkloadName))
					return ctrl.Result{}, err
				}
			} else {
				err = r.Client.Delete(context.TODO(), powerWorkload)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error deleting the power workload '%s' from the cluster", powerWorkloadName))
					return ctrl.Result{}, err
				}
			}

			// Remove the extended resources for this power profile from the node
			err = r.removeExtendedResources(nodeName, req.NamespacedName.Name, &logger)
			if err != nil {
				logger.Error(err, "error removing the extended resources from the node")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	// Make sure the EPP value is one of the four correct ones or empty in the case of a user-created profile
	logger.V(5).Info("confirming EPP value is one of the correct values")
	if _, exists := profilePercentages[profile.Spec.Epp]; !exists {
		incorrectEppErr := errors.NewServiceUnavailable(fmt.Sprintf("EPP value not allowed: %v - deleting the power profile CRD", profile.Spec.Epp))
		logger.Error(incorrectEppErr, "error reconciling the power profile")

		err = r.Client.Delete(context.TODO(), profile)
		if err != nil {
			logger.Error(err, fmt.Sprintf("error deleting the power profile %s with the incorrect EPP value %s", profile.Spec.Name, profile.Spec.Epp))
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	logger.V(5).Info("making sure max value is higher than the min value")
	if profile.Spec.Max < profile.Spec.Min {
		maxLowerThanMaxError := errors.NewServiceUnavailable("max frequency value cannot be lower than the minimum frequency value")
		logger.Error(maxLowerThanMaxError, fmt.Sprintf("error creating the profile '%s'", profile.Spec.Name))
		return ctrl.Result{Requeue: false}, maxLowerThanMaxError
	}

	absoluteMaximumFrequency, absoluteMinimumFrequency, err := getMaxMinFrequencyValues()
	logger.V(5).Info("retrieving the maximum possible frequency and minimum possible frequency from the system")
	if err != nil {
		logger.Error(err, "error retrieving the frequency values from the node")
		return ctrl.Result{Requeue: false}, err
	}

	// If the profile is shared then the associated pool will not be created in the power library
	if profile.Spec.Shared {
		if profile.Spec.Max < absoluteMinimumFrequency || profile.Spec.Min < absoluteMinimumFrequency {
			frequencyTooLowError := errors.NewServiceUnavailable(fmt.Sprintf("maximum or minimum frequency value cannot be below %d", absoluteMinimumFrequency))
			logger.Error(frequencyTooLowError, "error creating the shared power profile")
			return ctrl.Result{Requeue: false}, frequencyTooLowError
		}
		actualEpp := profile.Spec.Epp
		if !power.IsFeatureSupported(power.EPPFeature) && actualEpp != "" {
			err = fmt.Errorf("EPP is not supported but %s provides one, setting EPP to ''", profile.Name)
			logger.Error(err, "invalid EPP")
			actualEpp = ""
		}
		if !checkGovs(profile.Spec.Governor) {
			err = fmt.Errorf("governor %s is not supported, please use one of the following %v''", profile.Spec.Governor, power.GetAvailableGovernors())
			logger.Error(err, "invalid governor")
			return ctrl.Result{Requeue: false}, err
		}
		powerProfile, err := power.NewPowerProfile(profile.Spec.Name, uint(profile.Spec.Min), uint(profile.Spec.Max), profile.Spec.Governor, actualEpp)
		if err != nil {
			logger.Error(err, "could not set the power profile for the shared pool")
			return ctrl.Result{Requeue: false}, err
		}
		err = r.PowerLibrary.GetSharedPool().SetPowerProfile(powerProfile)
		if err != nil {
			logger.Error(err, "could not set the power profile for the shared pool")
			return ctrl.Result{Requeue: false}, err
		}

		logger.V(5).Info(fmt.Sprintf("shared power profile successfully created: name - %s max - %d min - %d EPP - %s", profile.Spec.Name, profile.Spec.Max, profile.Spec.Min, profile.Spec.Epp))
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
			cannotBeZeroError := errors.NewServiceUnavailable("max or min frequency cannot be zero")
			logger.Error(cannotBeZeroError, fmt.Sprintf("error creating the profile '%s'", profile.Spec.Name))
			return ctrl.Result{Requeue: false}, cannotBeZeroError
		}

		profileFromLibrary := r.PowerLibrary.GetExclusivePool(profile.Spec.Name)
		actualEpp := profile.Spec.Epp
		if !power.IsFeatureSupported(power.EPPFeature) && actualEpp != "" {
			err = fmt.Errorf("EPP is not supported but %s provides one, setting EPP to ''", profile.Name)
			logger.Error(err, "invalid EPP")
			actualEpp = ""
		}
		if !checkGovs(profile.Spec.Governor) {
			err = fmt.Errorf("governor %s is not supported, please use one of the following %v''", profile.Spec.Governor, power.GetAvailableGovernors())
			logger.Error(err, "invalid governor")
			return ctrl.Result{Requeue: false}, err
		}
		powerProfile, err := power.NewPowerProfile(profile.Spec.Name, uint(profileMinFreq), uint(profileMaxFreq), profile.Spec.Governor, actualEpp)
		if err != nil {
			logger.Error(err, "could not create the power profile")
			return ctrl.Result{Requeue: false}, err
		}
		if profileFromLibrary == nil {
			pool, err := r.PowerLibrary.AddExclusivePool(profile.Spec.Name)
			if err != nil {
				logger.Error(err, "failed to create the power profile")
				return ctrl.Result{}, err
			}
			err = pool.SetPowerProfile(powerProfile)
			if err != nil {
				logger.Error(err, fmt.Sprintf("error adding the profile '%s' to the power library for host '%s'", profile.Spec.Name, nodeName))
				return ctrl.Result{}, err
			}

			// Create the extended resources for the profile
			err = r.createExtendedResources(nodeName, profile.Spec.Name, profile.Spec.Epp, &logger)
			if err != nil {
				logger.Error(err, "error creating the extended resources for the base profile")
				return ctrl.Result{}, err
			}
		} else {
			err = r.PowerLibrary.GetExclusivePool(profile.Spec.Name).SetPowerProfile(powerProfile)
			logger.V(5).Info(fmt.Sprintf("updating the power profile '%s' to the power library for node '%s'", profile.Spec.Name, nodeName))
			if err != nil {
				logger.Error(err, fmt.Sprintf("error updating the profile '%s' to the power library for node '%s'", profile.Spec.Name, nodeName))
				return ctrl.Result{}, err
			}
		}

		logger.V(5).Info(fmt.Sprintf("power profile successfully created: name - %s max - %d min - %d EPP - %s", profile.Spec.Name, profileMaxFreq, profileMinFreq, profile.Spec.Epp))
	}

	workloadName := fmt.Sprintf("%s-%s", profile.Spec.Name, nodeName)
	logger.V(5).Info(fmt.Sprintf("configuring the workload name: %s", workloadName))
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
				logger.Error(err, fmt.Sprintf("error creating the power workload '%s'", workloadName))
				return ctrl.Result{}, err
			}

			logger.V(5).Info("power workload successfully created", "name", workloadName, "profile", profile.Spec.Name)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// If the workload already exists then the power profile was just updated and the power library will take care of reconfiguring cores
	return ctrl.Result{}, nil
}

func (r *PowerProfileReconciler) createExtendedResources(nodeName string, profileName string, eppValue string, logger *logr.Logger) error {
	node := &corev1.Node{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil {
		return err
	}

	numCPUsOnNode := float64(len(*r.PowerLibrary.GetAllCpus()))
	logger.V(5).Info("configuring based on the percentage associated to the specific power profile")
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

func (r *PowerProfileReconciler) removeExtendedResources(nodeName string, profileName string, logger *logr.Logger) error {
	node := &corev1.Node{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil {
		return err
	}

	logger.V(5).Info("removing the extended resources")
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

func checkGovs(profileGovernor string) bool {
	if profileGovernor == "" {
		return true
	}
	for _, gov := range power.GetAvailableGovernors() {
		if gov == profileGovernor {
			return true
		}
	}
	return false
}
