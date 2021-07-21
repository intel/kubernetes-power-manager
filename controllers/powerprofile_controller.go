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
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AppQoSClientAddress = "https://localhost:5000"
	MaxFrequencyFile = "/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq"
)

var extendedResourcePercentage map[string]float64 = map[string]float64{
        // performance          ===>  priority level 0
        // balance_performance  ===>  priority level 1
        // balance_power        ===>  priority level 2
        // power                ===>  priority level 3

        "performance":         .40,
        "balance_performance": .80,
        "balance_power":       1.0,
        "power":               1.0,
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

	profile := &powerv1alpha1.PowerProfile{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			// When a PowerProfile cannot be found, we assume it has been deleted. We need to check if there is a
			// corresponding PowerWorkload and, if there is, delete that too. We leave the cleanup of requesting the
			// frequency resets of the effected CPUs to the PowerWorkload controller. We also need to check to see
			// if there are any AppQoS instances on other nodes

			profileFromAppQoS, err := r.AppQoSClient.GetProfileByName(req.NamespacedName.Name, AppQoSClientAddress)
			if err != nil {
				logger.Error(err, "error retrieving PowerProfile from AppQoS instance")
				return ctrl.Result{}, err
			}

			err = r.AppQoSClient.DeletePowerProfile(AppQoSClientAddress, *profileFromAppQoS.ID)
			if err != nil {
				logger.Error(err, "error deleting PowerProfile from AppQoS instance")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	if _, exists := extendedResourcePercentage[profile.Spec.Name]; !exists && profile.Spec.Epp != "power" {
		logger.Info("PowerProfile is not a base profile or designated as a Shared Profile, skipping...")
		return ctrl.Result{}, nil
	}

	// Update the PowerProfile with the correct min/max values and name for the given node
	nodeName := os.Getenv("NODE_NAME")
	profileName := fmt.Sprintf("%s-%s", profile.Spec.Name, nodeName)

	maximumFrequencyByte, err := ioutil.ReadFile(MaxFrequencyFile)
        if err != nil {
                logger.Error(err, "error reading maximum frequency from file")
                return ctrl.Result{}, err
        }

        maximumFrequencyString := string(maximumFrequencyByte)
        maximumFrequency, err := strconv.Atoi(strings.Split(maximumFrequencyString, "\n")[0])
        if err != nil {
                logger.Error(err, "error reading maximum frequency value")
        }

        maximumFrequency = maximumFrequency/1000
        minimumFrequency := maximumFrequency-400

	// Check to see if this PowerProfile has already been created for this Node
	profileForNode := &powerv1alpha1.PowerProfile{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: req.NamespacedName.Namespace,
		Name: profileName,
	}, profileForNode)

	if err != nil {
		if errors.IsNotFound(err) {
			if profile.Spec.Epp != "power" {
				powerProfile := &powerv1alpha1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: req.NamespacedName.Namespace,
						Name: profileName,
					},
				}

				powerProfileSpec := &powerv1alpha1.PowerProfileSpec{
					Name: profileName,
					Max: maximumFrequency,
					Min: minimumFrequency,
					Epp: profile.Spec.Epp,
				}

				powerProfile.Spec = *powerProfileSpec
				err = r.Client.Create(context.TODO(), powerProfile)
				if err != nil {
					logger.Error(err, "error creating PowerProfile CRD")
					return ctrl.Result{}, err
				}

				err = r.createExtendedResources(nodeName, profileName, profile.Spec.Epp)
				if err != nil {
					logger.Error(err, "error creating Extended resources")
					return ctrl.Result{}, err
				}
			} else {
				err = r.createExtendedResources(nodeName, profile.Spec.Name, profile.Spec.Epp)
				if err != nil {
                                        logger.Error(err, "error creating Extended resources")
                                        return ctrl.Result{}, err
                                }
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	// TODO: Do check for update
	if _, exists := extendedResourcePercentage[profileName]; !exists {
		powerProfile := &appqos.PowerProfile{}
		powerProfile.Name = &profileName
		powerProfile.Epp = &profile.Spec.Epp
		if profile.Spec.Epp == "power" {
			powerProfile.MinFreq = &profile.Spec.Min
			powerProfile.MaxFreq = &profile.Spec.Max
		} else {
			powerProfile.MinFreq = &minimumFrequency
			powerProfile.MaxFreq = &maximumFrequency
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

func (r *PowerProfileReconciler) createExtendedResources(nodeName string, profileName string, eppValue string) error {
	node := &corev1.Node{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil {
		return err
	}

	numCPUsOnNode := float64(rt.NumCPU())
	numExtendedResources := int64(numCPUsOnNode * extendedResourcePercentage[eppValue])
	profilesAvailable := resource.NewQuantity(numExtendedResources, resource.DecimalSI)
	extendedResourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, profileName))
	node.Status.Capacity[extendedResourceName] = *profilesAvailable

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
