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
	"time"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
)

// PowerNodeReconciler reconciles a PowerNode object
type PowerNodeReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	AppQoSClient *appqos.AppQoSClient
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes/status,verbs=get;update;patch

func (r *PowerNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powernode", req.NamespacedName)

	powerProfilesInUse := make(map[string]bool, 0)
	powerWorkloads := make([]powerv1alpha1.WorkloadInfo, 0)
	powerContainers := make([]powerv1alpha1.Container, 0)

	powerNode := &powerv1alpha1.PowerNode{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, powerNode)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		return ctrl.Result{}, err
	}

	profiles := &powerv1alpha1.PowerProfileList{}
	err = r.Client.List(context.TODO(), profiles)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second *5}, nil
		}

		return ctrl.Result{}, err
	}

	workloads := &powerv1alpha1.PowerWorkloadList{}
	err = r.Client.List(context.TODO(), workloads)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		return ctrl.Result{}, err
	}

	for _, profile := range profiles.Items {
		if _, exists := extendedResourcePercentage[profile.Spec.Name]; exists || profile.Spec.Epp == "power" {
			// Base or Shared profile, skip

			continue
		}

		powerProfilesInUse[profile.Name] = false

		workloadName := fmt.Sprintf("%s-workload", profile.Name)
		for _, workload := range workloads.Items {
			if workload.Name == workloadName && workload.Spec.Node.Name == req.NamespacedName.Name {
				powerProfilesInUse[profile.Name] = true
				
				workloadInfo := &powerv1alpha1.WorkloadInfo{}
				workloadInfo.Name = workload.Name
				workloadInfo.CpuIds = workload.Spec.Node.CpuIds
				powerWorkloads = append(powerWorkloads, *workloadInfo)

				for _, container := range workload.Spec.Node.Containers {
					container.Workload = workload.Name
					powerContainers = append(powerContainers, container)
				}

				continue
			}
		}
	}

	defaultPool, err := r.AppQoSClient.GetPoolByName(AppQoSClientAddress, "Default")
	if err != nil {
		logger.Error(err, "error retrieving Default AppQoS Pool")
		return ctrl.Result{}, err
	}

	sharedPool, err := r.AppQoSClient.GetPoolByName(AppQoSClientAddress, "Shared")
	if err != nil {
		logger.Error(err, "error retrieving Shared AppQoS Pool")
		return ctrl.Result{}, err
	}

	sharedPools := make([]powerv1alpha1.SharedPoolInfo, 0)

	if !reflect.DeepEqual(defaultPool, &appqos.Pool{}) {
		defaultPoolInfo := &powerv1alpha1.SharedPoolInfo{
			Name: "Default",
			SharedPoolCpuIds: *defaultPool.Cores,
		}
		sharedPools = append(sharedPools, *defaultPoolInfo)
	}

	if !reflect.DeepEqual(sharedPool, &appqos.Pool{}) {
		sharedPoolInfo := &powerv1alpha1.SharedPoolInfo{
			Name: "Shared",
			SharedPoolCpuIds: *sharedPool.Cores,
		}
		sharedPools = append(sharedPools, *sharedPoolInfo)
	}

	powerNode.Spec.ActiveProfiles = powerProfilesInUse
	powerNode.Spec.ActiveWorkloads = powerWorkloads
	powerNode.Spec.PowerContainers = powerContainers
	powerNode.Spec.SharedPools = sharedPools

	err = r.Client.Update(context.TODO(), powerNode)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 5}, nil
}

func (r *PowerNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerNode{}).
		Complete(r)
}
