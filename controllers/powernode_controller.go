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
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/podstate"
	"github.com/intel/power-optimization-library/pkg/power"
)

const queuetime = time.Second * 5

// PowerNodeReconciler reconciles a PowerNode object
type PowerNodeReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	State        *podstate.State
	OrphanedPods map[string]corev1.Pod
	PowerLibrary power.Host
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powernodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=use

func (r *PowerNodeReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	logger := r.Log.WithValues("powernode", req.NamespacedName)
	logger.V(5).Info("checking if power node and node name match")
	nodeName := os.Getenv("NODE_NAME")
	if nodeName != req.NamespacedName.Name {
		// power node is not on this node
		return ctrl.Result{}, nil
	}

	powerProfileStrings := make([]string, 0)
	powerWorkloadStrings := make([]string, 0)
	powerContainers := make([]powerv1.Container, 0)

	powerNode := &powerv1.PowerNode{}
	logger.V(5).Info("retrieving the power node instance")
	err := r.Client.Get(context.TODO(), req.NamespacedName, powerNode)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(5).Info("power node not found, requeueing")
			return ctrl.Result{RequeueAfter: queuetime}, nil
		}
		return ctrl.Result{RequeueAfter: queuetime}, err
	}

	if len(powerNode.Spec.CustomDevices) > 0 {
		logger.V(5).Info("the power node contains the following custom devices.", "Custom Devices", powerNode.Spec.CustomDevices)
	}

	powerProfiles := &powerv1.PowerProfileList{}
	logger.V(5).Info("retrieving the power profile list")
	err = r.Client.List(context.TODO(), powerProfiles)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: queuetime}, nil
		}

		return ctrl.Result{RequeueAfter: queuetime}, err
	}

	for _, profile := range powerProfiles.Items {
		logger.V(5).Info("retrieving profile information from the power library")
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
	logger.V(5).Info("retrieving the power workload list")
	err = r.Client.List(context.TODO(), powerWorkloads)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: queuetime}, nil
		}

		return ctrl.Result{RequeueAfter: queuetime}, err
	}

	for _, workload := range powerWorkloads.Items {
		logger.V(5).Info("checking if the workload is shared or on the wrong node")
		if workload.Spec.AllCores || workload.Spec.Node.Name != nodeName {
			continue
		}
		poolFromLibrary := r.PowerLibrary.GetExclusivePool(workload.Spec.PowerProfile)
		logger.V(5).Info("retrieving the workload information from the power library")
		if poolFromLibrary == nil {
			continue
		}
		// checks for pods that aren't in the pool they should be
		if r.PowerLibrary.GetSharedPool().GetPowerProfile() != nil {
			for _, guaranteedPod := range r.State.GuaranteedPods {
				if err = r.itterPods(nodeName, workload, poolFromLibrary, guaranteedPod, logger); err != nil {
					// erroring out here risks a loop but in that scenario something is seriously wrong with k8s
					// or the managers' internal state
					return ctrl.Result{RequeueAfter: queuetime}, err
				}
			}
		}
		logger.V(5).Info("retrieving the power profile information for workload")
		cores := prettifyCoreList(poolFromLibrary.Cpus().IDs())
		profile := poolFromLibrary.GetPowerProfile()
		workloadString := fmt.Sprintf("%s: %s || %s", poolFromLibrary.Name(), profile.Name(), cores)
		powerWorkloadStrings = append(powerWorkloadStrings, workloadString)

		for _, container := range workload.Spec.Node.Containers {
			logger.V(5).Info("configuring the power container information")
			container.Workload = workload.Name
			powerContainers = append(powerContainers, container)
		}
	}

	logger.V(5).Info("setting the power node spec - profiles, workloads, containers")
	powerNode.Spec.PowerProfiles = powerProfileStrings
	powerNode.Spec.PowerWorkloads = powerWorkloadStrings
	powerNode.Spec.PowerContainers = powerContainers

	logger.V(5).Info("setting the shared pool, shared cores, shared profiles and reserved system CPUs")
	sharedPool := r.PowerLibrary.GetSharedPool()
	sharedCores := sharedPool.Cpus().IDs()
	sharedProfile := sharedPool.GetPowerProfile()
	reservedSystemCpus := r.PowerLibrary.GetReservedPool().Cpus().IDs()

	logger.V(5).Info("configurating the cores to the shared pool")
	powerNode.Spec.PowerContainers = powerContainers
	if len(sharedCores) > 0 && sharedProfile != nil {
		cores := prettifyCoreList(sharedCores)
		powerNode.Spec.SharedPool = fmt.Sprintf("%s || %v || %v || %s", sharedProfile.Name(), sharedProfile.MaxFreq(), sharedProfile.MinFreq(), cores)
	} else {
		powerNode.Spec.SharedPool = ""
	}
	// look for any special reserved pools
	pools := r.PowerLibrary.GetAllExclusivePools()
	powerNode.Spec.ReservedPools = []string{}
	for _, pool := range *pools {
		if strings.Contains(pool.Name(), nodeName+"-reserved-") {
			cores := prettifyCoreList(pool.Cpus().IDs())
			powerNode.Spec.ReservedPools = append(powerNode.Spec.ReservedPools, fmt.Sprintf("%v || %v || %s", pool.GetPowerProfile().MaxFreq(), pool.GetPowerProfile().MinFreq(), cores))
		}
	}
	logger.V(5).Info("configurating the cores to the reserved pool")
	cores := prettifyCoreList(reservedSystemCpus)
	powerNode.Spec.UnaffectedCores = cores
	err = r.Client.Update(context.TODO(), powerNode)
	if err != nil {
		return ctrl.Result{RequeueAfter: queuetime}, err
	}

	return ctrl.Result{RequeueAfter: queuetime}, nil
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

func (r *PowerNodeReconciler) itterPods(nodeName string, workload powerv1.PowerWorkload, poolFromLibrary power.Pool, guaranteedPod powerv1.GuaranteedPod, logger logr.Logger) error {
	pod := &corev1.Pod{}
	for _, container := range guaranteedPod.Containers {
		if workload.Name == (container.PowerProfile + "-" + nodeName) {
			for _, core := range container.ExclusiveCPUs {
				if !validateCoreIsInCoreList(core, poolFromLibrary.Cpus().IDs()) {
					if err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: guaranteedPod.Namespace, Name: guaranteedPod.Name}, pod); err != nil {
						logger.Error(err, "could not retrieve the pod")
						return err
					}
					if !pod.ObjectMeta.DeletionTimestamp.IsZero() || pod.Status.Phase == corev1.PodSucceeded {
						break
					}
					// this annotation is used to force a reconcile on the pod
					timestamp, exists := r.OrphanedPods[pod.Name].ObjectMeta.Annotations["PM-updated"]
					if exists {
						if t, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
							// gives 20 seconds to ensure pod is not moving from one pool to another
							if (time.Now().Unix() - int64(t)) > 20 {
								logger.V(5).Info(fmt.Sprintf("pod %s found with cores in the wrong pool, updating the pod", guaranteedPod.Name))
								pod.ObjectMeta.Annotations["PM-updated"] = fmt.Sprint(time.Now().Unix())
								err := r.Client.Update(context.TODO(), pod)
								if err != nil {
									logger.Error(err, "could not update the pod")
									return err
								}
								// update success so remove from map
								delete(r.OrphanedPods, pod.Name)
							}
						} else {
							logger.Error(err, fmt.Sprintf("error parsing PM-updated annotation in pod %s", guaranteedPod.Name))
							delete(r.OrphanedPods, pod.Name)
						}
					} else {
						pod.ObjectMeta.Annotations["PM-updated"] = fmt.Sprint(time.Now().Unix())
						r.OrphanedPods[pod.Name] = *pod
					}
					// we've found a problem pod so forego itterating other cores
					return nil
				}
			}
		}
	}
	return nil
}

func (r *PowerNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.LabelChangedPredicate{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.PowerNode{}).
		WithEventFilter(pred).
		Complete(r)
}
