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
	"sort"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/power-optimization-library/pkg/power"
)

// PowerNodeReconciler reconciles a PowerNode object
type PowerNodeReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PowerLibrary power.Host
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes/status,verbs=get;update;patch

func (r *PowerNodeReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	logger := r.Log.WithValues("powernode", req.NamespacedName)
	logger.V(5).Info("Checking if PowerNode and Node Name match")
	nodeName := os.Getenv("NODE_NAME")
	if nodeName != req.NamespacedName.Name {
		// PowerNode is not on this Node

		return ctrl.Result{}, nil
	}

	powerProfileStrings := make([]string, 0)
	powerWorkloadStrings := make([]string, 0)
	powerContainers := make([]powerv1.Container, 0)

	powerNode := &powerv1.PowerNode{}
	logger.V(5).Info("Retrieving Power Node instance")
	err := r.Client.Get(context.TODO(), req.NamespacedName, powerNode)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(5).Info("Power Node not found, requeueing")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		return ctrl.Result{}, err
	}

	powerProfiles := &powerv1.PowerProfileList{}
	logger.V(5).Info("Retrieving PowerProfileList")
	err = r.Client.List(context.TODO(), powerProfiles)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		return ctrl.Result{}, err
	}

	for _, profile := range powerProfiles.Items {
		logger.V(5).Info("Retrieving Profile information from the Power Librarys")
		pool := r.PowerLibrary.GetExclusivePool(profile.Spec.Name)
		if pool == nil {
			continue
		}
		profileFromLibrary := pool.GetPowerProfile()
		if profileFromLibrary == nil {
			continue
		}

		profileString := fmt.Sprintf("%s: %v || %v || %s", profileFromLibrary.Name(), profileFromLibrary.MaxFreq(), profileFromLibrary.MinFreq(), profileFromLibrary.Epp())
		powerProfileStrings = append(powerProfileStrings, profileString)
	}

	powerWorkloads := &powerv1.PowerWorkloadList{}
	logger.V(5).Info("Retrieving PowerWorkloadList")
	err = r.Client.List(context.TODO(), powerWorkloads)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		return ctrl.Result{}, err
	}

	for _, workload := range powerWorkloads.Items {
		logger.V(5).Info("Checking if workload is shared or on the wrong node")
		if workload.Spec.AllCores || workload.Spec.Node.Name != nodeName {
			continue
		}

		poolFromLibrary := r.PowerLibrary.GetExclusivePool(workload.Spec.Name)
		logger.V(5).Info("Retrieving workload information from Power Library")
		if poolFromLibrary == nil {
			continue
		}

		logger.V(5).Info("Retrieving Power Profile information for workload")
		cores := prettifyCoreList(poolFromLibrary.Cpus().IDs())
		profile := poolFromLibrary.GetPowerProfile()
		workloadString := fmt.Sprintf("%s: %s || %s", poolFromLibrary.Name(), profile.Name(), cores)
		powerWorkloadStrings = append(powerWorkloadStrings, workloadString)

		for _, container := range workload.Spec.Node.Containers {
			logger.V(5).Info("Configuring the Power Container information")
			container.Workload = workload.Name
			powerContainers = append(powerContainers, container)
		}
	}

	logger.V(5).Info("Setting the PowerNode Spec - PowerProfiles, PowerWorkloads, PowerContainers")
	powerNode.Spec.PowerProfiles = powerProfileStrings
	powerNode.Spec.PowerWorkloads = powerWorkloadStrings
	powerNode.Spec.PowerContainers = powerContainers

	logger.V(5).Info("Setting the Shared pool, Shared Cores, Shared profiles and reserved system CPUs")
	sharedPool := r.PowerLibrary.GetSharedPool()
	sharedCores := sharedPool.Cpus().IDs()
	sharedProfile := sharedPool.GetPowerProfile()
	reservedSystemCpus := r.PowerLibrary.GetReservedPool().Cpus().IDs()

	logger.V(5).Info("Configurating the cores to the SharedPool")
	powerNode.Spec.PowerContainers = powerContainers
	if len(sharedCores) > 0 && sharedProfile != nil {
		cores := prettifyCoreList(sharedCores)
		powerNode.Spec.SharedPool = fmt.Sprintf("%s || %v || %v || %s", sharedProfile.Name(), sharedProfile.MaxFreq(), sharedProfile.MinFreq(), cores)
	}

	if len(reservedSystemCpus) > 0 {
		logger.V(5).Info("Configurating the cores to the ReservedPool")
		cores := prettifyCoreList(reservedSystemCpus)
		powerNode.Spec.UneffectedCores = cores
	}

	err = r.Client.Update(context.TODO(), powerNode)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 5}, nil
}

func prettifyCoreList(cores []uint) string {
	prettified := ""
	sort.Slice(cores, func(i, j int) bool { return cores[i] < cores[j] })
	for i := 0; i < len(cores); i++ {
		start := i
		end := i

		for end < len(cores)-1 {
			if cores[end+1]-cores[end] == 1 {
				end++
			} else {
				break
			}
		}

		if end-start > 0 {
			prettified += fmt.Sprintf("%d-%d", cores[start], cores[end])
		} else {
			prettified += fmt.Sprintf("%d", cores[start])
		}

		if end < len(cores)-1 {
			prettified += ","
		}

		i = end
	}

	return prettified
}

func (r *PowerNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.PowerNode{}).
		Complete(r)
}
