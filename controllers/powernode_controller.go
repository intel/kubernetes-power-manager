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
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"

	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podstate"
)

// PowerNodeReconciler reconciles a PowerNode object
type PowerNodeReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	State  podstate.State
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes/status,verbs=get;update;patch

func (r *PowerNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powernode", req.NamespacedName)

	node := &powerv1alpha1.PowerNode{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, node)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
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

	workloadObjects := []powerv1alpha1.Workload{}
	for _, workload := range workloads.Items {
		containers := []powerv1alpha1.ContainerInfo{}
		cores := []int{}
		profile := workload.Spec.PowerProfile
		for _, pod := range r.State.PowerNodeStatus.GuaranteedPods {
			for _, container := range pod.Containers {
				if container.PowerProfile == profile {
					containerInfo := powerv1alpha1.ContainerInfo{}
					containerInfo.Name = container.Name
					containerInfo.ID = container.ID
					containerInfo.Pod = pod.Name
					containers = append(containers, containerInfo)
					cores = append(cores, container.ExclusiveCPUs...)
				}
			}
		}

		sort.Ints(cores)
		workloadObject := powerv1alpha1.Workload{}
		workloadObject.Name = workload.Spec.Name
		workloadObject.PowerProfile = profile
		workloadObject.Containers = containers
		workloadObject.Cores = cores
		workloadObjects = append(workloadObjects, workloadObject)
	}

	node.Status.GuaranteedPods = r.State.PowerNodeStatus.GuaranteedPods
	node.Status.Workloads = workloadObjects
	err = r.Client.Status().Update(context.TODO(), node)
	if err != nil {
		logger.Error(err, "Error updating PowerNode CRD")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 5}, nil
}

func (r *PowerNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerNode{}).
		Complete(r)
}
