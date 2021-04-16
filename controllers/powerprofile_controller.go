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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	corev1 "k8s.io/api/core/v1"

	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
)

const (
	PowerPodNameConst = "PowerPod"
)

// PowerProfileReconciler reconciles a PowerProfile object
type PowerProfileReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	AppQoSClient *appqos.AppQoSClient
	State        *state.PowerNodeData
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

			obseleteProfiles, err := r.findObseleteProfiles(req)
			if err != nil {
				return ctrl.Result{}, err
			}

			for address, profileID := range obseleteProfiles {
				err = r.AppQoSClient.DeletePowerProfile(address, profileID)
				if err != nil {
					logger.Error(err, "Failed to delete profile from AppQoS")
					return ctrl.Result{}, err
				}
			}

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	// Loop through Power Nodes to check which AppQos instances have this PowerProfile already
	// If an instance has it, update it, otherwise create it

	for _, nodeName := range r.State.PowerNodeList {
		nodeAddress, err := r.getPodAddress(nodeName, req)
		if err != nil {
			// Continue with other Nodes if there's a failure on one

			logger.Error(err, fmt.Sprintf("Failed to get IP address for node: %s", nodeName))
			continue
		}

		powerProfileFromAppQoS, err := r.AppQoSClient.GetProfileByName(req.NamespacedName.Name, nodeAddress)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Error retrieving Power Profiles from AppQoS from node %s", nodeName))
			continue
		}

		powerProfile := &appqos.PowerProfile{}
		powerProfile.Name = &req.NamespacedName.Name
		powerProfile.MinFreq = &profile.Spec.Min
		powerProfile.MaxFreq = &profile.Spec.Max
		powerProfile.Epp = &profile.Spec.Epp

		if !reflect.DeepEqual(powerProfileFromAppQoS, &appqos.PowerProfile{}) {
			// Update PowerProfile

			appqosPutResp, err := r.AppQoSClient.PutPowerProfile(powerProfile, nodeAddress, *powerProfileFromAppQoS.ID)
			if err != nil {
				logger.Error(err, appqosPutResp)
				continue
			}
		} else {
			// Create PowerProfile

			appqosPostResp, err := r.AppQoSClient.PostPowerProfile(powerProfile, nodeAddress)
			if err != nil {
				logger.Error(err, appqosPostResp)
				continue
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *PowerProfileReconciler) findObseleteProfiles(req ctrl.Request) (map[string]int, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)

	obseleteProfiles := make(map[string]int, 0)

	for _, nodeName := range r.State.PowerNodeList {
		address, err := r.getPodAddress(nodeName, req)
		if err != nil {
			return nil, err
		}

		profile, err := r.AppQoSClient.GetProfileByName(req.NamespacedName.Name, address)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Error retrieving Power Profiles from AppQoS from node %s", nodeName))
			return nil, err
		}

		if reflect.DeepEqual(profile, &appqos.PowerProfile{}) {
			logger.Info("PowerProfile not found on AppQoS instance")
			continue
		}

		obseleteProfiles[address] = *profile.ID
	}

	return obseleteProfiles, nil
}

func (r *PowerProfileReconciler) getPodAddress(nodeName string, req ctrl.Request) (string, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)

	// DELETE
	if 1 == 1 {
		node := &corev1.Node{}
		err := r.Client.Get(context.TODO(), client.ObjectKey{
			Name: nodeName,
		}, node)
		if err != nil {
			return "", err
		}
		address := fmt.Sprintf("%s%s%s", "https://", node.Status.Addresses[0].Address, ":5000")
		return address, nil
	}

	pods := &corev1.PodList{}
	err := r.Client.List(context.TODO(), pods, client.MatchingLabels(client.MatchingLabels{"name": PowerPodNameConst}))
	if err != nil {
		logger.Error(err, "Failed to list AppQoS pods")
		return "", nil
	}

	appqosPod, err := util.GetPodFromNodeName(pods, nodeName)
	if err != nil {
		appqosNode := &corev1.Node{}
		err := r.Client.Get(context.TODO(), client.ObjectKey{
			Name: nodeName,
		}, appqosNode)
		if err != nil {
			logger.Error(err, "Error getting AppQoS node")
			return "", err
		}

		appqosPod, err = util.GetPodFromNodeAddresses(pods, appqosNode)
		if err != nil {
			return "", err
		}
	}

	var podIP string
	notFoundError := errors.NewServiceUnavailable("pod address not available")
	if appqosPod.Status.PodIP != "" {
		podIP = appqosPod.Status.PodIP
	} else if len(appqosPod.Status.PodIPs) != 0 {
		podIP = appqosPod.Status.PodIPs[0].IP
	} else {
		return "", notFoundError
	}

	if len(appqosPod.Spec.Containers) == 0 {
		return "", notFoundError
	}

	if len(appqosPod.Spec.Containers[0].Ports) == 0 {
		return "", notFoundError
	}

	addressPrefix := r.AppQoSClient.GetAddressPrefix()
	address := fmt.Sprintf("%s%s%s%d", addressPrefix, podIP, ":", appqosPod.Spec.Containers[0].Ports[0].ContainerPort)
	return address, nil
}

// SetupWithManager specifies how the controller is built and watch a CR and other resources that are owned and managed by the controller
func (r *PowerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerProfile{}).
		Complete(r)
}
