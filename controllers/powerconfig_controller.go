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
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intel/kubernetes-power-manager/pkg/state"
	"github.com/intel/kubernetes-power-manager/pkg/util"
)

const (
	ExtendedResourcePrefix = "power.intel.com/"
	NodeAgentDSName        = "power-node-agent"
	IntelPowerNamespace    = "intel-power"
)

var NodeAgentDaemonSetPath = "/power-manifests/power-node-agent-ds.yaml"

// PowerConfigReconciler reconciles a PowerConfig object
type PowerConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	State  *state.PowerNodeData
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=use

func (r *PowerConfigReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerconfig", req.NamespacedName)

	if req.Namespace != IntelPowerNamespace {
		err := fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{Requeue: false}, err
	}

	configs := &powerv1.PowerConfigList{}
	logger.V(5).Info("retrieving the power config list")
	err := r.Client.List(c, configs)
	if err != nil {
		logger.Error(err, "error retrieving the power config list")
		return ctrl.Result{}, err
	}

	config := &powerv1.PowerConfig{}
	logger.V(5).Info("retrieving the power config")
	err = r.Client.Get(c, req.NamespacedName, config)
	if err != nil {
		logger.V(5).Info("failed retrieving the power config, checking if exists")
		if errors.IsNotFound(err) {
			// Power config was deleted, if the number of power configs is > 0, don't delete the power profiles
			if len(configs.Items) == 0 {
				powerProfiles := &powerv1.PowerProfileList{}
				err = r.Client.List(c, powerProfiles)
				logger.V(5).Info("retrieving all power profiles in the cluster")
				if err != nil {
					logger.Error(err, "error retrieving the power profiles")
					return ctrl.Result{}, err
				}

				for _, profile := range powerProfiles.Items {
					err = r.Client.Delete(c, &profile)
					logger.V(5).Info(fmt.Sprintf("deleting power profile %s", profile.Name))
					if err != nil {
						logger.Error(err, fmt.Sprintf("error deleting power profile '%s' from cluster", profile.Name))
						return ctrl.Result{}, err
					}
				}

				// Make sure all power workloads have been removed
				powerWorkloads := &powerv1.PowerWorkloadList{}
				err = r.Client.List(c, powerWorkloads)
				logger.V(5).Info("retrieving all power workloads in the cluster")
				if err != nil {
					logger.Error(err, "error retrieving the power workloads")
					return ctrl.Result{}, err
				}

				for _, workload := range powerWorkloads.Items {
					logger.V(5).Info(fmt.Sprintf("deleting power workload %s", workload.Name))
					err = r.Client.Delete(c, &workload)
					if err != nil {
						logger.Error(err, fmt.Sprintf("error deleting power workload '%s' from cluster", workload.Name))
						return ctrl.Result{}, err
					}
				}

				powerNodes := &powerv1.PowerNodeList{}
				err = r.Client.List(c, powerNodes)
				logger.V(5).Info("retrieving all power nodes in the cluster")
				if err != nil {
					logger.Error(err, "error retrieving power nodes")
					return ctrl.Result{}, err
				}

				for _, node := range powerNodes.Items {
					logger.V(5).Info(fmt.Sprintf("deleting power nodes %s", node.Name))
					err = r.Client.Delete(c, &node)
					if err != nil {
						logger.Error(err, fmt.Sprintf("error deleting power node '%s' from cluster", node.Name))
						return ctrl.Result{}, err
					}
				}

				daemonSet := &appsv1.DaemonSet{}
				logger.V(5).Info("retrieving the power node-agent daemonSet")
				err = r.Client.Get(c, client.ObjectKey{
					Name:      NodeAgentDSName,
					Namespace: IntelPowerNamespace,
				}, daemonSet)
				if err != nil {
					if !errors.IsNotFound(err) {
						logger.Error(err, "error retrieving the power node-agent daemonSet")
						return ctrl.Result{}, err
					}
				} else {
					err = r.Client.Delete(c, daemonSet)
					if err != nil {
						logger.Error(err, "error deleting the power node-agent daemonset")
						return ctrl.Result{}, err
					}
				}
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "error retrieving the power config")
		return ctrl.Result{}, err
	}

	if len(configs.Items) > 1 {
		logger.V(5).Info("checking to make sure there is only one power config")
		moreThanOneConfigError := errors.NewServiceUnavailable("cannot have more than one power config")
		logger.Error(moreThanOneConfigError, "error reconciling the power config")

		err = r.Client.Delete(c, config)
		if err != nil {
			logger.Error(err, "error deleting the power config")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Create power node-agent daemonSet
	logger.V(5).Info("creating the power node-agent daemonSet")
	err = r.createDaemonSetIfNotPresent(c, config, NodeAgentDaemonSetPath, &logger)
	if err != nil {
		logger.Error(err, "error creating the power node-agent")
		return ctrl.Result{}, err
	}

	labelledNodeList := &corev1.NodeList{}
	listOption := config.Spec.PowerNodeSelector

	// Searching for Custom Devices in PowerConfig
	customDevices := config.Spec.CustomDevices
	if len(customDevices) > 0 {
		logger.V(5).Info("the behaviour of the power node agent will be affected by the following devices.",
			"Custom Devices", customDevices)
	}

	logger.V(5).Info("confirming desired nodes match the power node selector")
	err = r.Client.List(c, labelledNodeList, client.MatchingLabels(listOption))
	if err != nil {
		logger.Info("failed to list nodes with power node selector", listOption)
		return ctrl.Result{}, err
	}

	for _, node := range labelledNodeList.Items {
		logger.V(5).Info("updating the node name")
		r.State.UpdatePowerNodeData(node.Name)

		powerNode := &powerv1.PowerNode{}
		err = r.Client.Get(c, client.ObjectKey{
			Namespace: IntelPowerNamespace,
			Name:      node.Name,
		}, powerNode)

		logger.V(5).Info(fmt.Sprintf("creating the power node CRD %s", node.Name))
		if err != nil {
			if errors.IsNotFound(err) {
				powerNode = &powerv1.PowerNode{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: IntelPowerNamespace,
						Name:      node.Name,
					},
				}

				powerNodeSpec := &powerv1.PowerNodeSpec{
					NodeName:      node.Name,
					CustomDevices: customDevices,
				}

				powerNode.Spec = *powerNodeSpec
				err = r.Client.Create(c, powerNode)
				if err != nil {
					logger.Error(err, "error creating the power node CRD")
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, err
			}
		}

		patch := client.MergeFrom(powerNode.DeepCopy())
		powerNode.Spec.CustomDevices = customDevices
		err = r.Client.Patch(c, powerNode, patch)
		if err != nil {
			logger.Error(err, "failed to update power node with custom devices.")
			return ctrl.Result{}, err
		}
	}

	patch := client.MergeFrom(config.DeepCopy())
	config.Status.Nodes = r.State.PowerNodeList
	config.Spec.CustomDevices = customDevices
	err = r.Client.Status().Patch(c, config, patch)
	if err != nil {
		logger.Error(err, "failed to update the power config")
		return ctrl.Result{}, err
	}

	// Create the power profiles that were requested in the power config if it doesn't exist
	// Delete any power profiles that are not being requested but exist
	for _, profile := range config.Spec.PowerProfiles {
		logger.V(5).Info(fmt.Sprintf("checking if the power profile exists %s", profile))
		profileFromCluster := &powerv1.PowerProfile{}
		err = r.Client.Get(c, client.ObjectKey{
			Name:      profile,
			Namespace: IntelPowerNamespace,
		}, profileFromCluster)
		if err != nil {
			if errors.IsNotFound(err) {
				// Power profile does not exist, so we need to create it
				logger.V(5).Info(fmt.Sprintf("creating power profile %s", profile))
				epp := strings.Replace(profile, "-", "_", 1)
				powerProfileSpec := &powerv1.PowerProfileSpec{
					Name: profile,
					Epp:  epp,
				}
				powerProfile := &powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: IntelPowerNamespace,
						Name:      profile,
					},
				}
				powerProfile.Spec = *powerProfileSpec
				err = r.Client.Create(c, powerProfile)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error creating power profile '%s'", profile))
					return ctrl.Result{}, err
				}
			} else {
				logger.Error(err, fmt.Sprintf("error retrieving power profile '%s'", profile))
				return ctrl.Result{}, err
			}
		}

		// If the power profile/workload was successfull retrieved, we don't need to do anything
	}

	powerProfiles := &powerv1.PowerProfileList{}
	logger.V(5).Info("retrieving the list of power profiles")
	err = r.Client.List(c, powerProfiles)
	if err != nil {
		logger.Error(err, "error retrieving the power profile list")
		return ctrl.Result{}, err
	}

	// Check power profiles for any that are no longer requested; only check base profiles
	for _, profile := range powerProfiles.Items {
		logger.V(5).Info("checking if the power profile exists and is not requested")
		convertedName := strings.Replace(profile.Spec.Name, "-", "_", 1)
		if _, exists := profilePercentages[convertedName]; exists {
			if !util.StringInStringList(profile.Spec.Name, config.Spec.PowerProfiles) {
				err = r.Client.Delete(c, &profile)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error deleting power profile '%s'", profile.Spec.Name))
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{RequeueAfter: time.Second * 5}, nil
}

func (r *PowerConfigReconciler) createDaemonSetIfNotPresent(c context.Context, powerConfig *powerv1.PowerConfig, path string, logger *logr.Logger) error {
	logger.V(5).Info("creating the daemonSet")

	daemonSet := &appsv1.DaemonSet{}
	var err error

	err = r.Client.Get(c, client.ObjectKey{
		Name:      NodeAgentDSName,
		Namespace: IntelPowerNamespace,
	}, daemonSet)
	if err != nil {
		if errors.IsNotFound(err) {
			daemonSet, err = createDaemonSetFromManifest(path)
			if err != nil {
				logger.Error(err, "error creating the daemonSet")
				return err
			}
			if len(powerConfig.Spec.PowerNodeSelector) != 0 {
				daemonSet.Spec.Template.Spec.NodeSelector = powerConfig.Spec.PowerNodeSelector
			}
			err = r.Client.Create(c, daemonSet)
			if err != nil {
				logger.Error(err, "error creating the daemonSet")
				return err
			}
			logger.V(5).Info("new power node-agent daemonSet created")
			return nil
		}
	}

	// If the daemonSet already exists and is different than the selected nodes, update it
	if !reflect.DeepEqual(daemonSet.Spec.Template.Spec.NodeSelector, powerConfig.Spec.PowerNodeSelector) {
		logger.V(5).Info("updating the existing daemonSet")
		daemonSet.Spec.Template.Spec.NodeSelector = powerConfig.Spec.PowerNodeSelector
		err = r.Client.Update(c, daemonSet)
		if err != nil {
			logger.Error(err, "error updating the power node-agent daemonSet")
			return err
		}
	}

	return nil
}

func createDaemonSetFromManifest(path string) (*appsv1.DaemonSet, error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(yamlFile, nil, nil)
	if err != nil {
		return nil, err
	}

	nodeAgentDaemonSet := obj.(*appsv1.DaemonSet)
	return nodeAgentDaemonSet, nil
}

func (r *PowerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.PowerConfig{}).
		Complete(r)
}
