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

func (r *PowerConfigReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerconfig", req.NamespacedName)

	configs := &powerv1.PowerConfigList{}
	err := r.Client.List(context.TODO(), configs)
	if err != nil {
		logger.Error(err, "error retrieving PowerConfigList")
		return ctrl.Result{}, err
	}

	config := &powerv1.PowerConfig{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, config)
	if err != nil {
		if errors.IsNotFound(err) {
			// PowerConfig was deleted, if the number PowerConfigs is > 0, don't delete the PowerProfiles
			if len(configs.Items) == 0 {
				powerProfiles := &powerv1.PowerProfileList{}
				err = r.Client.List(context.TODO(), powerProfiles)
				if err != nil {
					logger.Error(err, "error retrieving PowerProfiles")
					return ctrl.Result{}, err
				}

				for _, profile := range powerProfiles.Items {
					err = r.Client.Delete(context.TODO(), &profile)
					if err != nil {
						logger.Error(err, fmt.Sprintf("error deleting Power Profile '%s' from cluster", profile.Name))
						return ctrl.Result{}, err
					}
				}

				// Make sure all PowerWorkloads have been removed
				powerWorkloads := &powerv1.PowerWorkloadList{}
				err = r.Client.List(context.TODO(), powerWorkloads)
				if err != nil {
					logger.Error(err, "error retrieving PowerWorkloads")
					return ctrl.Result{}, err
				}

				for _, workload := range powerWorkloads.Items {
					err = r.Client.Delete(context.TODO(), &workload)
					if err != nil {
						logger.Error(err, fmt.Sprintf("error deleting Power Workload '%s' from cluster", workload.Name))
						return ctrl.Result{}, err
					}
				}

				powerNodes := &powerv1.PowerNodeList{}
				err = r.Client.List(context.TODO(), powerNodes)
				if err != nil {
					logger.Error(err, "error retrieving PowerNodes")
					return ctrl.Result{}, err
				}

				for _, node := range powerNodes.Items {
					err = r.Client.Delete(context.TODO(), &node)
					if err != nil {
						logger.Error(err, fmt.Sprintf("error deleting PowerNode '%s' from cluster", node.Name))
						return ctrl.Result{}, err
					}
				}

				daemonSet := &appsv1.DaemonSet{}
				err = r.Client.Get(context.TODO(), client.ObjectKey{
					Name:      NodeAgentDSName,
					Namespace: IntelPowerNamespace,
				}, daemonSet)
				if err != nil {
					if !errors.IsNotFound(err) {
						logger.Error(err, "error retrieving Power Node Agent DaemonSet")
						return ctrl.Result{}, err
					}
				} else {
					err = r.Client.Delete(context.TODO(), daemonSet)
					if err != nil {
						logger.Error(err, "error deleting Power Node Agent Daemonset")
						return ctrl.Result{}, err
					}
				}
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "Error retreiving PowerConfig")
		return ctrl.Result{}, err
	}

	if len(configs.Items) > 1 {
		moreThanOneConfigError := errors.NewServiceUnavailable("Cannot have more than one PowerConfig")
		logger.Error(moreThanOneConfigError, "error reconciling PowerConfig")

		err = r.Client.Delete(context.TODO(), config)
		if err != nil {
			logger.Error(err, "error deleting PowerConfig")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Create PowerNodeAgent DaemonSet
	err = r.createDaemonSetIfNotPresent(config, NodeAgentDaemonSetPath)
	if err != nil {
		logger.Error(err, "Error creating Power Node Agent")
		return ctrl.Result{}, err
	}

	labelledNodeList := &corev1.NodeList{}
	listOption := config.Spec.PowerNodeSelector

	err = r.Client.List(context.TODO(), labelledNodeList, client.MatchingLabels(listOption))
	if err != nil {
		logger.Info("Failed to list Nodes with PowerNodeSelector", listOption)
		return ctrl.Result{}, err
	}

	for _, node := range labelledNodeList.Items {
		r.State.UpdatePowerNodeData(node.Name)

		powerNode := &powerv1.PowerNode{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: IntelPowerNamespace,
			Name:      node.Name,
		}, powerNode)

		if err != nil {
			if errors.IsNotFound(err) {
				powerNode = &powerv1.PowerNode{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: IntelPowerNamespace,
						Name:      node.Name,
					},
				}

				powerNodeSpec := &powerv1.PowerNodeSpec{
					NodeName: node.Name,
				}

				powerNode.Spec = *powerNodeSpec
				err = r.Client.Create(context.TODO(), powerNode)
				if err != nil {
					logger.Error(err, "Error creating PowerNode CRD")
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, err
			}
		}
	}

	config.Status.Nodes = r.State.PowerNodeList
	err = r.Client.Status().Update(context.TODO(), config)
	if err != nil {
		logger.Error(err, "Failed to update PowerConfig")
		return ctrl.Result{}, err
	}

	// Create the PowerProfiles that were requested in the PowerConfig if it doesn't exist
	// Delete any PowerProfiles that are not being requested but exist
	for _, profile := range config.Spec.PowerProfiles {
		profileFromCluster := &powerv1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      profile,
			Namespace: IntelPowerNamespace,
		}, profileFromCluster)
		if err != nil {
			if errors.IsNotFound(err) {
				// PowerProfile does not exist, so we need to create it

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
				err = r.Client.Create(context.TODO(), powerProfile)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error creating PowerProfile '%s'", profile))
					return ctrl.Result{}, err
				}
			} else {
				logger.Error(err, fmt.Sprintf("error retrieving PowerProfile '%s'", profile))
				return ctrl.Result{}, err
			}
		}

		// If the PowerProfile/PowerWorkload was successfull retrieved, we don't need to do anything
	}

	powerProfiles := &powerv1.PowerProfileList{}
	err = r.Client.List(context.TODO(), powerProfiles)
	if err != nil {
		logger.Error(err, "error retrieving PowerProfile List")
		return ctrl.Result{}, err
	}

	// Check PowerProfiles for any that are no longer requested; only check base profiles
	for _, profile := range powerProfiles.Items {
		convertedName := strings.Replace(profile.Spec.Name, "-", "_", 1)
		if _, exists := profilePercentages[convertedName]; exists {
			if !util.StringInStringList(profile.Spec.Name, config.Spec.PowerProfiles) {
				err = r.Client.Delete(context.TODO(), &profile)
				if err != nil {
					logger.Error(err, fmt.Sprintf("error deleting PowerProfile '%s'", profile.Spec.Name))
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{RequeueAfter: time.Second * 5}, nil
}

func (r *PowerConfigReconciler) createDaemonSetIfNotPresent(powerConfig *powerv1.PowerConfig, path string) error {
	logger := r.Log.WithName("createDaemonSetIfNotPresent")

	daemonSet := &appsv1.DaemonSet{}
	var err error

	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Name:      NodeAgentDSName,
		Namespace: IntelPowerNamespace,
	}, daemonSet)
	if err != nil {
		if errors.IsNotFound(err) {
			daemonSet, err = newDaemonSet(path)
			if err != nil {
				logger.Error(err, "Error creating DaemonSet")
				return err
			}
			if len(powerConfig.Spec.PowerNodeSelector) != 0 {
				daemonSet.Spec.Template.Spec.NodeSelector = powerConfig.Spec.PowerNodeSelector
			}
			err = r.Client.Create(context.TODO(), daemonSet)
			if err != nil {
				logger.Error(err, "Error creating DaemonSet")
				return err
			}
			logger.Info("New PowerNodeAgent DaemonSet created")
			return nil
		}
	}

	// If the the DaemonSet already exists and is different than the selected nodes, update it
	if !reflect.DeepEqual(daemonSet.Spec.Template.Spec.NodeSelector, powerConfig.Spec.PowerNodeSelector) {
		daemonSet.Spec.Template.Spec.NodeSelector = powerConfig.Spec.PowerNodeSelector
		err = r.Client.Update(context.TODO(), daemonSet)
		if err != nil {
			logger.Error(err, "error updating PowerNodeAgent DaemonSet")
			return err
		}
	}

	return nil
}

func newDaemonSet(path string) (*appsv1.DaemonSet, error) {
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
