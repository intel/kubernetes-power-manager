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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	"time"

	"github.com/go-logr/logr"
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/podstate"
	"github.com/intel/power-optimization-library/pkg/power"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// TimeOfDayCronJobReconciler reconciles a TimeOfDayCronJob object
type TimeOfDayCronJobReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	State        *podstate.State
	PowerLibrary power.Host
}

// +kubebuilder:rbac:groups=power.intel.com,resources=timeofdaycronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=timeofdaycronjobs/status,verbs=get;update;patch
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TimeOfDayCronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.4/pkg/reconcile
func (r *TimeOfDayCronJobReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("timeofdaycronjob", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		logger.Error(fmt.Errorf("incorrect namespace"), "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{}, nil
	}
	logger.Info("Reconciling TimeOfDayCronJob")

	cronJob := &powerv1.TimeOfDayCronJob{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, cronJob)

	if err != nil {
		logger.Error(err, "Error retrieving CronJob")
		return ctrl.Result{}, nil
	}

	//	setting up location
	var location *time.Location
	if cronJob.Spec.TimeZone != nil {
		location, err = time.LoadLocation(*cronJob.Spec.TimeZone)
		if err != nil {
			location = time.Local
		}
	} else {
		location = time.Local
	}
	nodeName := os.Getenv("NODE_NAME")
	// reading schedule
	hr := cronJob.Spec.Hour
	min := cronJob.Spec.Minute
	sec := cronJob.Spec.Second
	jobActiveTime := time.Date(time.Now().In(location).Year(), time.Now().In(location).Month(), time.Now().In(location).Day(), hr, min, sec, 0, location)
	wait := jobActiveTime.Sub(time.Now().In(location))
	//calculating when next to schedule the job
	nextActiveTime := jobActiveTime.Add(24 * time.Hour)
	logger.V(5).Info(fmt.Sprintf("Next active time is: %s", nextActiveTime))
	nextWait := nextActiveTime.Sub(time.Now().In(location))
	// cronjob missed deadline
	if wait.Seconds() <= 0 && cronJob.Status.LastScheduleTime == nil {
		logger.Info(fmt.Sprintf("cronjob missed deadline by %s. Scheduling for tommorow", wait.String()))
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: time.Now().In(location)}
		if err := r.Status().Update(c, cronJob); err != nil {
			logger.Error(err, "cannot update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: nextWait}, nil
	}

	if cronJob.Status.LastScheduleTime == nil {
		// cronjob just created
		logger.V(5).Info("Reconciling newly created Cronjob")
		logger.Info(fmt.Sprintf("telling reconciler to wait %s", wait.String()))
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: time.Now().In(location)}
		if err := r.Status().Update(c, cronJob); err != nil {
			logger.Error(err, "cannot update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: wait}, nil

	} else {
		//cronjob ready for application
		if wait.Seconds() <= 0 {
			logger.V(5).Info("Cronjob ready to be applied")
			if cronJob.Spec.Profile != nil {
				var workloadMatch *powerv1.PowerWorkload
				var profileMaxFreq int
				var profileMinFreq int
				// check if shared workload exists
				// if not create one
				logger.V(5).Info("Checking for existing shared workload")
				workloadList := &powerv1.PowerWorkloadList{}
				err = r.Client.List(context.TODO(), workloadList)
				if err != nil {
					logger.Error(err, "error retrieving workloads")
					return ctrl.Result{}, err
				}
				//if an active workload exists with allcores set to true it must be shared
				for _, workload := range workloadList.Items {
					if workload.Spec.AllCores {
						workloadMatch = &workload
						break
					}
				}
				// a shared workload does not exist so make one
				if workloadMatch == nil {
					if cronJob.Spec.ReservedCPUs == nil {
						err := fmt.Errorf("reserved CPU field left blank")
						logger.Error(err, "reservedCPUs must be set")
						return ctrl.Result{Requeue: false}, err
					}
					logger.V(5).Info("Creating shared workload as none exists")
					workloadName := fmt.Sprintf("shared-%s", nodeName)
					workload := &powerv1.PowerWorkload{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: IntelPowerNamespace,
							Name:      workloadName,
						},
						Spec: powerv1.PowerWorkloadSpec{
							Name:         workloadName,
							AllCores:     true,
							ReservedCPUs: *cronJob.Spec.ReservedCPUs,
							Node: powerv1.WorkloadNode{
								Name: nodeName,
							},
							PowerProfile: *cronJob.Spec.Profile,
						},
					}
					if err = r.Client.Create(context.TODO(), workload); err != nil {
						logger.Error(err, "error creating workload")
						return ctrl.Result{}, err
					}
					workloadMatch = workload
				}
				// A shared workload exists so we attach it to the profile
				logger.V(5).Info("Modifying shared workload")
				workloadMatch.Spec.PowerProfile = *cronJob.Spec.Profile
				logger.V(5).Info(fmt.Sprintf("Setting profile %s", *cronJob.Spec.Profile))
				prof := &powerv1.PowerProfile{}
				if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: *cronJob.Spec.Profile, Namespace: IntelPowerNamespace}, prof); err != nil {
					logger.Error(err, "cannot retrieve profile")
					return ctrl.Result{Requeue: false}, err
				}
				var absoluteMaximumFrequency, absoluteMinimumFrequency int
				if absoluteMaximumFrequency, absoluteMinimumFrequency, err = getMaxMinFrequencyValues(); err != nil {
					logger.Error(err, "error retrieving frequency values from Node")
					return ctrl.Result{}, nil
				}
				if prof.Spec.Epp != "" && prof.Spec.Max == 0 && prof.Spec.Min == 0 {
					profileMaxFreq = int(float64(absoluteMaximumFrequency) - (float64((absoluteMaximumFrequency - absoluteMinimumFrequency)) * profilePercentages[prof.Spec.Epp]["difference"]))
					profileMinFreq = int(profileMaxFreq) - 200
				} else {
					profileMaxFreq = prof.Spec.Max
					profileMinFreq = prof.Spec.Min
				}
				powerProfile, err := power.NewPowerProfile(prof.Spec.Name, uint(profileMinFreq), uint(profileMaxFreq), prof.Spec.Governor, prof.Spec.Epp)
				if err != nil {
					logger.Error(err, "could not set power profile for shared pool")
					return ctrl.Result{}, nil
				}
				err = r.PowerLibrary.GetSharedPool().SetPowerProfile(powerProfile)
				if err != nil {
					logger.Error(err, "could not set power profile for shared pool")
					return ctrl.Result{}, nil
				}
				if err := r.Client.Update(c, workloadMatch); err != nil {
					logger.Error(err, "cannot update workload")
					return ctrl.Result{}, err
				}

				logger.V(5).Info("New shared pool applied")
			}
			if cronJob.Spec.CState != nil {
				cstate := &powerv1.CStates{}
				err = r.Client.Get(context.TODO(), client.ObjectKey{
					Name:      nodeName,
					Namespace: IntelPowerNamespace,
				}, cstate)
				if errors.IsNotFound(err) {
					//if cstate does not exist
					logger.V(5).Info("Creating new cstate")
					newCstate := &powerv1.CStates{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: IntelPowerNamespace,
							Name:      nodeName,
						},
						Spec: powerv1.CStatesSpec{
							SharedPoolCStates:     cronJob.Spec.CState.SharedPoolCStates,
							ExclusivePoolCStates:  cronJob.Spec.CState.ExclusivePoolCStates,
							IndividualCoreCStates: cronJob.Spec.CState.IndividualCoreCStates,
						},
					}
					if err = r.Client.Create(context.TODO(), newCstate); err != nil {
						logger.Error(err, "error creating workload")
						return ctrl.Result{}, err
					}

				} else {
					//if cstate already exists
					logger.V(5).Info(fmt.Sprintf("Modifying cstate %s", cstate.Name))
					newSpec := powerv1.CStatesSpec{
						SharedPoolCStates:     cronJob.Spec.CState.SharedPoolCStates,
						ExclusivePoolCStates:  cronJob.Spec.CState.ExclusivePoolCStates,
						IndividualCoreCStates: cronJob.Spec.CState.IndividualCoreCStates,
					}
					cstate.Spec = newSpec
					if err := r.Client.Update(c, cstate); err != nil {
						logger.Error(err, "cannot update cstate")
						return ctrl.Result{}, err
					}
				}
				logger.V(5).Info("Cstate successfully applied")

			}
			//logic for tuning individual pods
			if cronJob.Spec.Pods != nil {
				logger.V(5).Info("Changing profile for exclusive pods")
				workloadFrom := powerv1.PowerWorkload{}
				workloadTo := powerv1.PowerWorkload{}
				//looping over each pod to tune
				for _, podInfo := range *cronJob.Spec.Pods {
					var selector labels.Selector
					if selector, err = metav1.LabelSelectorAsSelector(&podInfo.Labels); err != nil {
						logger.Error(err, "error parsing pod label info")
						return ctrl.Result{Requeue: false}, err
					}
					listOptions := client.ListOptions{
						LabelSelector: selector,
					}
					powerpods := &corev1.PodList{}
					if err := r.Client.List(context.TODO(), powerpods, &listOptions); err != nil {
						logger.Error(err, "retrieving pods")
						return ctrl.Result{}, err
					}
					for _, pod := range powerpods.Items {
						podName := pod.Name
						podState := r.State.GetPodFromState(pod.Name, pod.Namespace)
						if podState.Name != pod.Name {
							logger.Error(err, fmt.Sprintf("mismatch between pod name and internal state name. %s and %s", podState.Name, pod.Name))
							return ctrl.Result{}, nil
						}
						var from string
						for i, container := range podState.Containers {
							if container.Workload != "" {
								from = container.Workload
								podState.Containers[i].Workload = podInfo.Target + "-" + nodeName
							}
						}
						if err = r.State.UpdateStateGuaranteedPods(podState); err != nil {
							logger.Error(err, "error updating internal state")
							return ctrl.Result{}, err
						}
						//useful check to see if we've already retrieved the workload in an earlier loop
						if workloadFrom.Name != from {
							err = r.Client.Get(context.TODO(), client.ObjectKey{
								Name:      from,
								Namespace: IntelPowerNamespace,
							}, &workloadFrom)
							if err != nil {
								logger.Error(err, fmt.Sprintf("error retrieving workload %s", from))
								return ctrl.Result{Requeue: false}, err
							}
						}
						//same check as before
						if workloadTo.Name != podInfo.Target+"-"+nodeName {
							err = r.Client.Get(context.TODO(), client.ObjectKey{
								Name:      podInfo.Target + "-" + nodeName,
								Namespace: IntelPowerNamespace,
							}, &workloadTo)
							if err != nil {
								logger.Error(err, fmt.Sprintf("error retrieving workload %s", (podInfo.Target+"-"+nodeName)))
								return ctrl.Result{Requeue: false}, err
							}
						}
						//looping over the container field of the workload
						for i, container := range workloadFrom.Spec.Node.Containers {
							if container.Pod == podName {
								logger.V(5).Info(fmt.Sprintf("Found %s for tuning", container.Pod))
								// first we set the profile on the container to it's new value
								container.PowerProfile = podInfo.Target
								// copying container to its' new workload
								workloadTo.Spec.Node.Containers = append(workloadTo.Spec.Node.Containers, container)
								//getting cores to be removed from one workload and added to another
								coresToSwap := workloadFrom.Spec.Node.Containers[i].ExclusiveCPUs
								//append cores to one workload and shrink the list in the other
								workloadTo.Spec.Node.CpuIds = append(workloadTo.Spec.Node.CpuIds, coresToSwap...)
								workloadFrom.Spec.Node.Containers[i] = workloadFrom.Spec.Node.Containers[len(workloadFrom.Spec.Node.Containers)-1]
								workloadFrom.Spec.Node.Containers = workloadFrom.Spec.Node.Containers[:len(workloadFrom.Spec.Node.Containers)-1]
								updatedWorkloadCPUList := getNewWorkloadCPUList(coresToSwap, workloadFrom.Spec.Node.CpuIds, &logger)
								workloadFrom.Spec.Node.CpuIds = updatedWorkloadCPUList
								//update both workloads to bring changes into affect
								if err := r.Client.Update(c, &workloadFrom); err != nil {
									logger.Error(err, "cannot update workload")
									return ctrl.Result{}, err
								}
								if err := r.Client.Update(c, &workloadTo); err != nil {
									logger.Error(err, "cannot update workload")
									return ctrl.Result{}, err
								}

							}
						}
						pod.ObjectMeta.Annotations["PM-updated"] = fmt.Sprint(time.Now().Unix())
						pod.ObjectMeta.Annotations["PM-altered"] = podInfo.Target
						if err = r.Client.Update(context.TODO(), &pod); err != nil {
							logger.Error(err, "Could not update pod")
							return ctrl.Result{}, err
						}
					}
				}
			}

			//reschedule for tomorrow
			cronJob.Status.LastSuccessfulTime = &metav1.Time{Time: time.Now().In(location)}
			cronJob.Status.LastScheduleTime = &metav1.Time{Time: time.Now().In(location)}
			logger.V(5).Info(fmt.Sprintf("Telling reconciler to wait till %s", nextWait.String()))
			if err := r.Status().Update(c, cronJob); err != nil {
				logger.Error(err, "cannot update status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: nextWait}, nil
		}

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TimeOfDayCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// this predicate prevents an unwanted reconcile when updating a cronjob to reschedule
	predicate := predicate.GenerationChangedPredicate{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.TimeOfDayCronJob{}).
		WithEventFilter(predicate).
		Complete(r)
}
