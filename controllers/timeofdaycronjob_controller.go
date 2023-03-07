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

	"time"

	"github.com/go-logr/logr"
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
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
	jobActiveTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), hr, min, 0, 0, location)
	wait := jobActiveTime.Sub(time.Now().In(location))
	//calculating when next to schedule the job
	nextActiveTime := jobActiveTime.Add(24 * time.Hour)
	nextWait := nextActiveTime.Sub(time.Now().In(location))
	// cronjob missed deadline
	if wait.Round(1*time.Minute).Minutes() < 0 {
		logger.Info(fmt.Sprintf("cronjob missed deadline by %s. Scheduling for tommorow", wait.String()))
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: time.Now().In(location)}
		if err := r.Status().Update(c, cronJob); err != nil {
			logger.Error(err, "cannot update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: nextWait}, nil
	}
	// cronjob just created
	if cronJob.Status.LastScheduleTime == nil {
		logger.Info(fmt.Sprintf("telling reconciler to wait %s", wait.String()))
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: time.Now().In(location)}
		if err := r.Status().Update(c, cronJob); err != nil {
			logger.Error(err, "cannot update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: wait}, nil

	} else {
		//cronjob ready for application
		if wait.Round(1*time.Minute).Minutes() == 0 {
			logger.Info(fmt.Sprintf("cronjob ready to be applied"))
			if cronJob.Spec.Profile != nil {
				// check if shared workload exists
				// if not create one
				//logger.Info(fmt.Sprintf("searching for workload with allcores or creating one"))
				workloadList := &powerv1.PowerWorkloadList{}
				err = r.Client.List(context.TODO(), workloadList)
				if err != nil {
					logger.Error(err, "error retrieving workloads")
					return ctrl.Result{}, err
				}
				//if an active workload exists with allcores set to true it must be shared
				var workloadMatch *powerv1.PowerWorkload
				for _, workload := range workloadList.Items {
					if workload.Spec.AllCores {
						workloadMatch = &workload
						break
					}
				}
				// a shared workload does not exist so make one
				if workloadMatch == nil {
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
					//logger.Info(fmt.Sprintf("workload successfully created "))
					if err != nil {
						logger.Error(err, "error creating Shared Pool in Power Library")
						return ctrl.Result{}, err
					}
				} else {
					// A shared workload exists so we swap out the profile attached
					//logger.Info(fmt.Sprintf("A shared workload already exists so we need to change it"))
					//logger.Info(fmt.Sprintf("retrieved workload %s",workload_match.Name))
					workloadMatch.Spec.PowerProfile = *cronJob.Spec.Profile
					logger.Info(fmt.Sprintf("setting profile %s", *cronJob.Spec.Profile))

					if err := r.Client.Update(c, workloadMatch); err != nil {
						logger.Error(err, "cannot update workload")
						return ctrl.Result{}, err
					}

				}
				logger.Info(fmt.Sprintf("new shared pool applied"))
			}
			if cronJob.Spec.CState != nil {
				cstate := &powerv1.CStates{}
				err = r.Client.Get(context.TODO(), client.ObjectKey{
					Name:      nodeName,
					Namespace: IntelPowerNamespace,
				}, cstate)
				if err != nil {
					//if cstate does not exist
					//logger.Info(fmt.Sprintf("creating new cstate"))
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
					//logger.Info(fmt.Sprintf("cstate %s already exists",cstate.Name))
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
				logger.Info(fmt.Sprintf("cstate successfully applied"))

			}
			//logic for tuning individual pods
			if cronJob.Spec.Pods != nil {
				logger.Info(fmt.Sprintf("changing profile for exclusive pods"))
				workloadFrom := powerv1.PowerWorkload{}
				workloadTo := powerv1.PowerWorkload{}
				//looping over each pod to tune
				for podName, profToProf := range *cronJob.Spec.Pods {
					for from, to := range profToProf {
						//useful check to see if we've already retrieved the workload in an earlier loop
						if workloadFrom.Name != from {
							//logger.Info(fmt.Sprintf("retrieving %s and %s",from+"-"+nodeName,to+"-"+nodeName))
							err = r.Client.Get(context.TODO(), client.ObjectKey{
								Name:      from + "-" + nodeName,
								Namespace: IntelPowerNamespace,
							}, &workloadFrom)
							if err != nil {
								logger.Error(err, "error retrieving workload")
								continue
							}
						}
						//same check as before
						if workloadTo.Name != to {
							err = r.Client.Get(context.TODO(), client.ObjectKey{
								Name:      to + "-" + nodeName,
								Namespace: IntelPowerNamespace,
							}, &workloadTo)
							if err != nil {
								logger.Error(err, "error retrieving workload")
								continue
							}
						}
						//looping over the container field of the workload
						for i, container := range workloadFrom.Spec.Node.Containers {
							//logger.Info(fmt.Sprintf("value of i is %d",i))
							if container.Pod == podName {
								//logger.Info(fmt.Sprintf("found %s and %s",container.Pod,pod_name))
								// first we set the profile on the container to it's new value
								container.PowerProfile = to
								// copying container to its' new workload
								workloadTo.Spec.Node.Containers = append(workloadTo.Spec.Node.Containers, container)
								//getting cores to be removed from one workload and added to another
								coresToSwap := workloadFrom.Spec.Node.Containers[i].ExclusiveCPUs
								//append cores to one workload and shrink the list in the other
								workloadTo.Spec.Node.CpuIds = append(workloadTo.Spec.Node.CpuIds, coresToSwap...)
								workloadFrom.Spec.Node.Containers[i] = workloadFrom.Spec.Node.Containers[len(workloadFrom.Spec.Node.Containers)-1]
								workloadFrom.Spec.Node.Containers = workloadFrom.Spec.Node.Containers[:len(workloadFrom.Spec.Node.Containers)-1]
								updatedWorkloadCPUList := getNewWorkloadCPUList(coresToSwap, workloadFrom.Spec.Node.CpuIds)
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
								break
							}
						}
					}
				}
			}

			//reschedule for tomorrow
			cronJob.Status.LastSuccessfulTime = &metav1.Time{Time: time.Now().In(location)}
			cronJob.Status.LastScheduleTime = &metav1.Time{Time: time.Now().In(location)}
			logger.Info(fmt.Sprintf("telling reconciler to wait till %s", nextWait.String()))
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
