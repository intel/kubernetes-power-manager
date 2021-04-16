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
	"k8s.io/apimachinery/pkg/api/resource"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	ExtendedResourcePrefix = "power.intel.com/"
	//GoldResource resource.ResourceName = ""power.intel.com/gold"
	//SilverResource resource.ResourceName = ""power.intel.com/silver"
	//BronzeResource resource.ResourceName = ""power.intel.com/bronze"
)

/*
var extendedResourceQuantity map[string] = map[string]int64{
	// CHANGE TO BE A REP OF THE NUMBER OF CORES
	"gold": 400,
	"silver": 800,
	"bronze": 101,
}
*/

var extendedResourcePercentage map[string]float64 = map[string]float64{
	"gold": 40.0,
	"silver": 80.0,
	"bronze": 100.0,
}

// PowerConfigReconciler reconciles a PowerConfig object
type PowerConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	State  *state.PowerNodeData
	AppQoSClient *appqos.AppQoSClient
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerconfigs/status,verbs=get;update;patch

func (r *PowerConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerconfig", req.NamespacedName)

	config := &powerv1alpha1.PowerConfig{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, config)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PowerConfig not found")
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Error retreiving PowerConfig")
		return ctrl.Result{}, err
	}

	labelledNodeList := &corev1.NodeList{}
	listOption := client.MatchingLabels{}
	listOption = config.Spec.PowerNodeSelector

	err = r.Client.List(context.TODO(), labelledNodeList, client.MatchingLabels(listOption))
	if err != nil {
		logger.Info("Failed to list Nodes with PowerNodeSelector", listOption)
		return ctrl.Result{}, err
	}

	for _, node := range labelledNodeList.Items {
		r.State.UpdatePowerNodeData(node.Name)
	}

	config.Status.Nodes = r.State.PowerNodeList
	err = r.Client.Status().Update(context.TODO(), config)
	if err != nil {
		logger.Error(err, "Failed to update PowerConfig")
		return ctrl.Result{}, nil
	}

	for _, nodeName := range r.State.PowerNodeList {
		nodeAddress, err := r.getPodAddress(nodeName, req)
		if err != nil {
			// Continue with other nodes if there's a failure with this one
			logger.Error(err, fmt.Sprintf("Failed to get IP address for node: %s", nodeName))
			continue
		}

		defaultPool, _, err := r.AppQoSClient.GetPoolByName(nodeAddress, "Default")
		if err != nil {
			logger.Error(err, "Error getting Default pool")
			return ctrl.Result{}, nil
		}

		numPools := len(*defaultPool.Cores)
		logger.Info(fmt.Sprintf("NUMBER OF CORES: %v", numPools))

		node := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
                	Name: nodeName,
        	}, node)
		if err != nil {
                	logger.Error(err, "Failed to get node")
                	return ctrl.Result{}, nil
        	}

		for _, profileName := range config.Spec.PowerProfiles {
			numProfileResources := int64((float64(numPools) / 100) * extendedResourcePercentage[profileName])
			//numProfileResources := numPools * extendedResourcePercentage[profileName]
			profilesAvailable := resource.NewQuantity(numProfileResources, resource.DecimalSI)
			extendedResourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, profileName))
			node.Status.Capacity[extendedResourceName] = *profilesAvailable
		}

		err = r.Client.Status().Update(context.TODO(), node)
		if err != nil {
			logger.Error(err, "Failed updating node")
			continue
		}
	}
/*
	node := &corev1.Node{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Name: "cascade-lake",
	}, node)
	if err != nil {
		logger.Error(err, "Failed to get node")
		return ctrl.Result{}, nil
	}

	for _, profileName := range config.Spec.PowerProfiles {
		//logger.Info(fmt.Sprintf("%s: %d", profileName, extendedResourceQuantity[profileName]))
		numExtendedResources := resource.NewQuantity(extendedResourceQuantity[profileName], resource.DecimalSI)
		extendedResourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, profileName))
		node.Status.Capacity[extendedResourceName] = *numExtendedResources
	}

	err = r.Client.Status().Update(context.TODO(), node)
	if err != nil {
		logger.Error(err, "Failed updating node")
		return ctrl.Result{}, nil
	}
*/

	return ctrl.Result{}, nil
}

func (r *PowerConfigReconciler) getPodAddress(nodeName string, req ctrl.Request) (string, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerworkload", req.NamespacedName)

	// TODO: DELETE WHEN APPQOS CONTAINERIZED
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

func (r *PowerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerConfig{}).
		Complete(r)
}
