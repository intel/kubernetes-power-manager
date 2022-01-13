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
	"io/ioutil"
	"os"
	"reflect"
	rt "runtime"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MaxFrequencyFile         = "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"
	MinFrequencyFile         = "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_min_freq"
	MinimumRequiredFrequency = 2000
)

var AppQoSClientAddress = "https://localhost:5000"

// performance          ===>  priority level 0
// balance_performance  ===>  priority level 1
// balance_power        ===>  priority level 2
// power                ===>  priority level 3

var extendedResourcePercentage map[string]float64 = map[string]float64{
	"performance":         .40,
	"balance-performance": .60,
	"balance-power":       .80,
	"power":               1.0,
}

var extendedPowerProfileMaxMinDifference map[string]map[string]int = map[string]map[string]int{
	"performance": map[string]int{
		"baseMax":  500,
		"baseMin":  700,
		"modifier": 0,
	},
	"balance-performance": map[string]int{
		"baseMax":  700,
		"baseMin":  900,
		"modifier": 100,
	},
	"balance-power": map[string]int{
		"baseMax":  900,
		"baseMin":  1100,
		"modifier": 200,
	},
}

var basePowerProfileToEppValue map[string]string = map[string]string{
	// The Kubernetes CRD naming convention doesn't allow underscores

	"performance":         "performance",
	"balance-performance": "balance_performance",
	"balance-power":       "balance_power",
	"power":               "power",
}

var allowedEppValues map[string]struct{} = map[string]struct{}{
	// Empty structs hold no memory value

	"performance":         struct{}{},
	"balance_performance": struct{}{},
	"balance_power":       struct{}{},
	"power":               struct{}{},
}

// PowerProfileReconciler reconciles a PowerProfile object
type PowerProfileReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	AppQoSClient *appqos.AppQoSClient
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles/status,verbs=get;update;patch

// Reconcile method that implements the reconcile loop
func (r *PowerProfileReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)
	logger.Info("Reconciling PowerProfile")

	// Node name is passed down via the downwards API and used to make sure the PowerProfile is for this node
	nodeName := os.Getenv("NODE_NAME")

	profile := &powerv1alpha1.PowerProfile{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			// When a PowerProfile cannot be found, we assume it has been deleted. We need to check if there is a
			// corresponding PowerWorkload in App QoS and, if there is, delete that too. We leave the cleanup of requesting the
			// frequency resets of the effected CPUs to the PowerWorkload controller. We also need to check to see
			// if there are any AppQoS instances on other nodes

			profileFromAppQoS, err := r.AppQoSClient.GetProfileByName(req.NamespacedName.Name, AppQoSClientAddress)
			if err != nil {
				logger.Error(err, "error retrieving PowerProfile from AppQoS instance")
				return ctrl.Result{}, err
			}

			// Make sure the profile existed in AppQoS, if not we don't have to delete it
			if !reflect.DeepEqual(*profileFromAppQoS, appqos.PowerProfile{}) {
				err = r.AppQoSClient.DeletePowerProfile(AppQoSClientAddress, *profileFromAppQoS.ID)
				if err != nil {
					logger.Error(err, "error deleting PowerProfile from AppQoS instance")
					return ctrl.Result{}, err
				}
			}

			// Remove the Extended Resources for this PowerProfile from the Node
			err = r.removeExtendedResources(nodeName, req.NamespacedName.Name)
			if err != nil {
				logger.Error(err, "error removing Extended Resources from node")
				return ctrl.Result{}, err
			}

			if _, exists := extendedResourcePercentage[req.NamespacedName.Name]; exists {
				// If this PowerProfile was a base profile, delete all appropriate extended profiles

				profileList := &powerv1alpha1.PowerProfileList{}
				err = r.Client.List(context.TODO(), profileList)
				if err != nil {
					logger.Error(err, "error retrieving PowerProfile list")
					return ctrl.Result{}, err
				}

				for _, profileFromList := range profileList.Items {
					if profileFromList.Spec.Epp == basePowerProfileToEppValue[req.NamespacedName.Name] {
						// Only need to delete the PowerProfile CRD, profile will be deleted from AppQoS then

						err = r.Client.Delete(context.TODO(), &profileFromList)
						if err != nil {
							logger.Error(err, "error deleting PowerProfile")
							return ctrl.Result{}, err
						}
					}
				}
			}

			if strings.HasSuffix(req.NamespacedName.Name, nodeName) {
				workloadName := fmt.Sprintf("%s%s", req.NamespacedName.Name, WorkloadNameSuffix)
				powerWorkload := &powerv1alpha1.PowerWorkload{}
				err = r.Client.Get(context.TODO(), client.ObjectKey{
					Name:      workloadName,
					Namespace: req.NamespacedName.Namespace,
				}, powerWorkload)
				if err != nil {
					if errors.IsNotFound(err) {
						logger.Info("No PowerWorkload associated with this PowerProfile")
					} else {
						logger.Error(err, "error retrieving PowerWorkload")
						return ctrl.Result{}, err
					}
				} else {
					err = r.Client.Delete(context.TODO(), powerWorkload)
					if err != nil {
						logger.Error(err, "error deleting PowerWorkload")
						return ctrl.Result{}, err
					}
				}
			}

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	// Make sure the EPP value is one of the four correct ones
	if _, exists := allowedEppValues[profile.Spec.Epp]; !exists {
		incorrectEppErr := errors.NewServiceUnavailable(fmt.Sprintf("EPP value not allowed: %v - deleting PowerProfile CRD", profile.Spec.Epp))
		logger.Error(incorrectEppErr, "error reconciling PowerProfile")

		err = r.Client.Delete(context.TODO(), profile)
		if err != nil {
			logger.Error(err, fmt.Sprintf("error deleting PowerProfile %s with incorrect EPP value %s", profile.Spec.Name, profile.Spec.Epp))
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if _, exists := extendedResourcePercentage[profile.Spec.Name]; !exists && profile.Spec.Epp != "power" {
		logger.Info("PowerProfile is not a base profile or designated as a Shared Profile, skipping...")
		return ctrl.Result{}, nil
	}

	// Update the PowerProfile with the correct min/max values and name for the given node
	profileName := fmt.Sprintf("%s-%s", profile.Spec.Name, nodeName)

	absoluteMaximumFrequencyByte, err := ioutil.ReadFile(MaxFrequencyFile)
	if err != nil {
		logger.Error(err, "error reading maximum frequency from file")
		return ctrl.Result{}, err
	}
	absoluteMaximumFrequencyString := string(absoluteMaximumFrequencyByte)
	absoluteMaximumFrequency, err := strconv.Atoi(strings.Split(absoluteMaximumFrequencyString, "\n")[0])
	if err != nil {
		logger.Error(err, "error reading maximum frequency value")
		return ctrl.Result{}, err
	}

	absoluteMinimumFrequencyByte, err := ioutil.ReadFile(MinFrequencyFile)
	if err != nil {
		logger.Error(err, "error reading minimum frequency from file")
		return ctrl.Result{}, err
	}
	absoluteMinimumFrequencyString := string(absoluteMinimumFrequencyByte)
	absoluteMinimumFrequency, err := strconv.Atoi(strings.Split(absoluteMinimumFrequencyString, "\n")[0])
	if err != nil {
		logger.Error(err, "error reading minimum frequency value")
		return ctrl.Result{}, err
	}

	absoluteMaximumFrequency = absoluteMaximumFrequency / 1000
	absoluteMinimumFrequency = absoluteMinimumFrequency / 1000

	if absoluteMaximumFrequency < MinimumRequiredFrequency {
		maximumFrequencyTooLowError := errors.NewServiceUnavailable(fmt.Sprintf("Maximum CPU Frequency cannot be below %v", MinimumRequiredFrequency))
		logger.Error(maximumFrequencyTooLowError, "maximum frequency too low")
		return ctrl.Result{}, nil
	}

	frequencyModifier := float64((absoluteMaximumFrequency - MinimumRequiredFrequency) / 800)
	frequencyModifier = frequencyModifier * float64(extendedPowerProfileMaxMinDifference[profile.Spec.Name]["modifier"])
	maximumFrequencyModifier := extendedPowerProfileMaxMinDifference[profile.Spec.Name]["baseMax"] + int(frequencyModifier)
	minimumFrequencyModifier := extendedPowerProfileMaxMinDifference[profile.Spec.Name]["baseMin"] + int(frequencyModifier)
	maximumValueForProfile := absoluteMaximumFrequency - maximumFrequencyModifier
	minimumValueForProfile := absoluteMaximumFrequency - minimumFrequencyModifier

	if maximumValueForProfile < absoluteMinimumFrequency {
		maximumValueForProfile = absoluteMinimumFrequency
	}

	if minimumValueForProfile < absoluteMinimumFrequency {
		maximumValueForProfile = absoluteMinimumFrequency
	}

	// Check to see if the extended PowerProfile has already been created for this Node
	if profile.Spec.Epp != "power" {
		profileForNode := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: req.NamespacedName.Namespace,
			Name:      profileName,
		}, profileForNode)

		if err != nil {
			if errors.IsNotFound(err) {
				powerProfile := &powerv1alpha1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: req.NamespacedName.Namespace,
						Name:      profileName,
					},
				}

				powerProfileSpec := &powerv1alpha1.PowerProfileSpec{
					Name: profileName,
					Max:  maximumValueForProfile,
					Min:  minimumValueForProfile,
					Epp:  profile.Spec.Epp,
				}

				powerProfile.Spec = *powerProfileSpec
				err = r.Client.Create(context.TODO(), powerProfile)
				if err != nil {
					logger.Error(err, "error creating PowerProfile CRD")
					return ctrl.Result{}, err
				}

				err = r.createExtendedResources(nodeName, profileName, req.NamespacedName.Name)
				if err != nil {
					logger.Error(err, "error creating Extended resources")
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, err
			}
		}
	}

	// Create the Extended Resources for the base profile as well so the Pod controller can determine the Profile if necessary
	err = r.createExtendedResources(nodeName, profile.Spec.Name, req.NamespacedName.Name)
	if err != nil {
		logger.Error(err, "error creating extended resources for base profile")
		return ctrl.Result{}, err
	}

	if _, exists := extendedResourcePercentage[profileName]; !exists {
		powerProfile := &appqos.PowerProfile{}
		if profile.Spec.Epp == "power" {
			powerProfile.Name = &profile.Spec.Name
		} else {
			powerProfile.Name = &profileName
		}
		powerProfile.Epp = &profile.Spec.Epp
		if profile.Spec.Epp == "power" {
			powerProfile.MinFreq = &profile.Spec.Min
			powerProfile.MaxFreq = &profile.Spec.Max
		} else {
			powerProfile.MinFreq = &minimumValueForProfile
			powerProfile.MaxFreq = &maximumValueForProfile
		}

		// Create PowerProfile

		appqosPostResp, err := r.AppQoSClient.PostPowerProfile(powerProfile, AppQoSClientAddress)
		if err != nil {
			logger.Error(err, appqosPostResp)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *PowerProfileReconciler) createExtendedResources(nodeName string, profileName string, baseProfile string) error {
	node := &corev1.Node{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil {
		return err
	}

	numCPUsOnNode := float64(rt.NumCPU())
	numExtendedResources := int64(numCPUsOnNode * extendedResourcePercentage[baseProfile])
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

// SetupWithManager specifies how the controller is built and watch a CR and other resources that are owned and managed by the controller
func (r *PowerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerProfile{}).
		Complete(r)
}
