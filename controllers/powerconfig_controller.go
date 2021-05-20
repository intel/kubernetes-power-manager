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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
)

const (
	AppQoSLabel            = "feature.node.kubernetes.io/appqos-node"
	AppQoSDaemonSetPath    = "/power-manifests/appqos-ds.yaml"
	AppQoSPodPath          = "/power-manifests/appqos-pod-template.yaml"
	NodeAgentDaemonSetPath = "/power-manifests/power-node-agent-ds.yaml"
	ExtendedResourcePrefix = "power.intel.com/"
	AppQoSPodLabel         = "appqos-pod"
)

var extendedResourcePercentage map[string]float64 = map[string]float64{
	// performance          ===>  priority level 0
	// balance_performance  ===>  priority level 1
	// balance_power        ===>  priority level 2
	// power                ===>  priority level 3

	"performance":         40.0,
	"balance_performance": 80.0,
	"balance_power":       100.0,
	"power":               100.0,
}

// PowerConfigReconciler reconciles a PowerConfig object
type PowerConfigReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	State        *state.PowerNodeData
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

	/* CAN DELETE - NODE DOESN'T NEED TO BE LABELLED PRIOR TO DS CREATION */
	/* CAN MOVE INTO THE NODENAME FOR LOOP */
	for _, node := range labelledNodeList.Items {
		r.State.UpdatePowerNodeData(node.Name)

		err = r.createPodIfNotPresent(node.Name, AppQoSPodPath)
		if err != nil {
			return ctrl.Result{}, err
		}

		/*
			appqosNode := &corev1.Node{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{
				Name: node.Name,
			}, appqosNode)
			if err != nil {
				logger.Error(err, "Error getting node")
				return ctrl.Result{}, nil
			}

			appqosNode.Labels["feature.node.kubernetes.io/appqos-node"] = "true"
			err = r.Client.Update(context.TODO(), appqosNode)
			if err != nil {
				logger.Error(err, "Error updating node's label")
				return ctrl.Result{}, nil
			}
		*/
	}

	config.Status.Nodes = r.State.PowerNodeList
	err = r.Client.Status().Update(context.TODO(), config)
	if err != nil {
		logger.Error(err, "Failed to update PowerConfig")
		return ctrl.Result{}, nil
	}

	for _, nodeName := range r.State.PowerNodeList {
		appqosNotReadyError := fmt.Sprintf("Failed to get IP address for node: %s", nodeName)
		nodeAddress, err := r.getPodAddress(nodeName)
		if err != nil {
			// Requeue after 5 seconds to give the AppQoS Pod time to start up

			logger.Info(appqosNotReadyError)
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		// We need to make sure that the appqos pod is running, as it causes problems if it is not
		appqosRunning := r.checkAppQoSRunning(nodeName)
		if !appqosRunning {
			logger.Info("AppQoS not running yet")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// Don't need to determine if it's the Default or the Shared pool because there shouldn't
		// be a Shared pool at this stage
		defaultPool, err := r.AppQoSClient.GetPoolByName(nodeAddress, "Default")
		if err != nil {
			// Requeue after 5 seconds to give the AppQoS Pod time to start up

			//logger.Info(appqosNotReadyError)
			logger.Info("Failed in GetPoolByName")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		numPools := len(*defaultPool.Cores)

		node := &corev1.Node{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name: nodeName,
		}, node)
		if err != nil {
			logger.Error(err, "Failed to get node")
			return ctrl.Result{}, nil
		}

		for _, profileName := range config.Spec.PowerProfiles {
			if profilePercentage, exists := extendedResourcePercentage[profileName]; exists {
				numProfileResources := int64((float64(numPools) / 100) * profilePercentage)
				profilesAvailable := resource.NewQuantity(numProfileResources, resource.DecimalSI)
				extendedResourceName := corev1.ResourceName(fmt.Sprintf("%s%s", ExtendedResourcePrefix, profileName))
				node.Status.Capacity[extendedResourceName] = *profilesAvailable
			} else {
				logger.Info(fmt.Sprintf("Skipping profile '%s', profile must be 'performance', 'balance_performance', 'balance_power', or 'power'", profileName))
			}
		}

		err = r.Client.Status().Update(context.TODO(), node)
		if err != nil {
			logger.Error(err, "Failed updating node")
			continue
		}

		powerNode := &powerv1alpha1.PowerNode{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: req.NamespacedName.Namespace,
			Name:      nodeName,
		}, powerNode)

		if err != nil {
			if errors.IsNotFound(err) {
				powerNode = &powerv1alpha1.PowerNode{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: req.NamespacedName.Namespace,
						Name:      nodeName,
					},
				}

				powerNodeSpec := &powerv1alpha1.PowerNodeSpec{
					NodeName: nodeName,
				}

				powerNode.Spec = *powerNodeSpec
				err = r.Client.Create(context.TODO(), powerNode)
				if err != nil {
					logger.Error(err, "Error creating PowerNode CRD")
					return ctrl.Result{}, err
				}
			}

			return ctrl.Result{}, err
		}
	}

	logger.Info("We are finished")

	/*
	err = r.createDaemonSetIfNotPresent(config, NodeAgentDaemonSetPath)
	if err != nil {
		return ctrl.Result{}, nil
	}
	*/

	return ctrl.Result{}, nil
}

func (r *PowerConfigReconciler) checkAppQoSRunning(nodeName string) bool {
	_ = context.Background()
        logger := r.Log.WithName("checkAppQoSRunning")

        pods := &corev1.PodList{}
        err := r.Client.List(context.TODO(), pods, client.MatchingLabels(client.MatchingLabels{"name": AppQoSPodLabel}))
        if err != nil {
                logger.Error(err, "Failed to list AppQoS pods")
                return false
        }

	appqosPod, err := util.GetPodFromNodeName(pods, nodeName)
        if err != nil {
                appqosNode := &corev1.Node{}
                err := r.Client.Get(context.TODO(), client.ObjectKey{
                        Name: nodeName,
                }, appqosNode)
                if err != nil {
                        logger.Error(err, "Error getting AppQoS node")
                        return false
                }

                appqosPod, err = util.GetPodFromNodeAddresses(pods, appqosNode)
                if err != nil {
                        return false
                }
        }

	if appqosPod.Status.Phase != corev1.PodRunning {
		return false
	}

	return true
}

func (r *PowerConfigReconciler) getPodAddress(nodeName string) (string, error) {
	_ = context.Background()
	logger := r.Log.WithName("getPodAddress")

	pods := &corev1.PodList{}
	err := r.Client.List(context.TODO(), pods, client.MatchingLabels(client.MatchingLabels{"name": AppQoSPodLabel}))
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

	addressPrefix := r.AppQoSClient.GetAddressPrefix()
	podHostname := appqosPod.Spec.Hostname
	podSubdomain := appqosPod.Spec.Subdomain
	fullHostname := fmt.Sprintf("%s%s.%s:%d", addressPrefix, podHostname, podSubdomain, appqosPod.Spec.Containers[0].Ports[0].ContainerPort)

	return fullHostname, nil
}

func (r *PowerConfigReconciler) createDaemonSetIfNotPresent(powerConfig *powerv1alpha1.PowerConfig, path string) error {
	logger := r.Log.WithName("createDaemonSetIfNotPresent")

	daemonSet, err := newDaemonSet(path)
	if err != nil {
		logger.Error(err, "Error creating DaemonSet")
		return err
	}

	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: daemonSet.GetObjectMeta().GetNamespace(),
		Name:      daemonSet.GetObjectMeta().GetName(),
	}, daemonSet)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Client.Create(context.TODO(), daemonSet)
			if err != nil {
				logger.Error(err, "Error creating DaemonSet")
				return err
			}
		}

		return err
	}

	return nil
}

func newDaemonSet(path string) (*appsv1.DaemonSet, error) {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(yamlFile, nil, nil)
	if err != nil {
		return nil, err
	}

	appqosDaemonSet := obj.(*appsv1.DaemonSet)
	return appqosDaemonSet, nil
}

func (r *PowerConfigReconciler) createPodIfNotPresent(nodeName string, path string) error {
	logger := r.Log.WithName("createPodIfNotPresent")

	pod, err := newPod(path)
	if err != nil {
		logger.Error(err, "Error creating Pod")
		return err
	}

	pod.Name = fmt.Sprintf("%s%s", pod.Name, nodeName)
	pod.Spec.Hostname = fmt.Sprintf("%s%s", pod.Spec.Hostname, nodeName)

	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: pod.GetObjectMeta().GetNamespace(),
		Name:      pod.GetObjectMeta().GetName(),
	}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Client.Create(context.TODO(), pod)
			if err != nil {
				logger.Error(err, "Error creating Pod")
				return err
			}
		}

		return err
	}

	return nil
}

func newPod(path string) (*corev1.Pod, error) {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(yamlFile, nil, nil)
	if err != nil {
		return nil, err
	}

	appqosPod := obj.(*corev1.Pod)
	return appqosPod, nil
}

func (r *PowerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerConfig{}).
		Complete(r)
}
