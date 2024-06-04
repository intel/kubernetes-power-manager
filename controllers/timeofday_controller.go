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
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intel/kubernetes-power-manager/pkg/util"
)

// TimeOfDayReconciler reconciles a TimeOfDay object
type TimeOfDayReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=power.intel.com,resources=timeofdays,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=power.intel.com,resources=timeofdays/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=use

func (r *TimeOfDayReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	logger := r.Log.WithValues("timeofday", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		err := fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{Requeue: false}, err
	}
	nodeName := os.Getenv("NODE_NAME")
	if req.Name != nodeName {
		return ctrl.Result{}, nil
	}
	//enforces HH:MM:SS time format
	timeRegex := regexp.MustCompile("(0[0-9]|1[0-9]|2[0-3]):[0-5][0-9](:[0-5][0-9])?")

	timeOfDayList := &powerv1.TimeOfDayList{}
	logger.V(5).Info("retrieving time-of-day objects from the lists")
	err = r.Client.List(c, timeOfDayList)
	if err != nil {
		logger.Error(err, "error retrieving the time-of-day list")
		return ctrl.Result{}, err
	}

	logger.V(5).Info("confirming that there can only be one time-of-day object")
	if len(timeOfDayList.Items) > 1 {
		err = apierrors.NewServiceUnavailable("cannot have more than one time-of-day")
		logger.Error(err, "error reconciling time-of-day")
		return ctrl.Result{}, err
	}

	timeOfDay := &powerv1.TimeOfDay{}
	defer func() { _ = writeUpdatedStatusErrsIfRequired(c, r.Status(), timeOfDay, err) }()

	err = r.Client.Get(c, req.NamespacedName, timeOfDay)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info("deleting time-of-day cron jobs from cluster")
			err = r.Client.DeleteAllOf(c, &powerv1.TimeOfDayCronJob{}, client.InNamespace(IntelPowerNamespace))
			if err != nil {
				logger.Error(err, "error deleting the time-of-day cron jobs")
				return ctrl.Result{}, err
			}
		}
		logger.Error(err, "error retrieving time-of-day")
		return ctrl.Result{}, err
	}

	// Validate incoming values from time-of-day manifest
	timeZone := timeOfDay.Spec.TimeZone
	logger.V(5).Info(fmt.Sprintf("validated timezone for time-of-day, values are: %s", timeZone))

	if timeZone != "" {
		_, err = time.LoadLocation(timeZone)
		logger.Info(fmt.Sprintf("timezone is %s\n", timeZone))
		if err != nil {
			err = apierrors.NewServiceUnavailable("invalid timezone, refer to the IANA timezone database for a list of valid timezones")
			logger.Error(err, "error creating time-of-day")
			return ctrl.Result{}, err
		}
	}

	var cronJobNames []string
	logger.V(5).Info("creating time-of-day Cronjobs")
	for _, scheduleInfo := range timeOfDay.Spec.Schedule {
		if !timeRegex.MatchString(scheduleInfo.Time) {
			err = apierrors.NewServiceUnavailable("the time filed must be in format HH:MM:SS or HH:MM and cannot be empty")
			logger.Error(err, "error creating the time-of-day schedule")
			return ctrl.Result{}, err
		}

		scheduledTime := strings.Split(scheduleInfo.Time, ":")
		if len(scheduledTime) == 2 {
			scheduledTime = append(scheduledTime, "00")
		}

		hr, _ := strconv.Atoi(scheduledTime[0])
		min, _ := strconv.Atoi(scheduledTime[1])
		sec, _ := strconv.Atoi(scheduledTime[2])
		if scheduleInfo.PowerProfile != nil && timeOfDay.Spec.ReservedCPUs == nil {
			err = apierrors.NewServiceUnavailable("profile detected with no reserved CPUs set")
			logger.Error(err, "error creating the time-of-day schedule")
			return ctrl.Result{}, err
		}

		cronJobName := fmt.Sprintf("%s-%d-%d-%d", timeOfDay.Name, hr, min, sec)

		cronJob := &powerv1.TimeOfDayCronJob{}
		err = r.Client.Get(c, client.ObjectKey{
			Name:      cronJobName,
			Namespace: IntelPowerNamespace,
		}, cronJob)
		// if cronjob doesn't exist create one
		logger.V(5).Info("creating the cron job if one doesn't exist")
		if err != nil {

			if apierrors.IsNotFound(err) {
				// passing spec values from timeofday object to cronjob
				cronJobSpec := &powerv1.TimeOfDayCronJobSpec{
					Hour:         hr,
					Minute:       min,
					Second:       sec,
					TimeZone:     &timeOfDay.Spec.TimeZone,
					Profile:      scheduleInfo.PowerProfile,
					Pods:         scheduleInfo.Pods,
					ReservedCPUs: timeOfDay.Spec.ReservedCPUs,
					CState:       scheduleInfo.CState,
				}

				cronJob = &powerv1.TimeOfDayCronJob{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: IntelPowerNamespace,
						Name:      cronJobName,
					},
				}
				// creating the new cronjob
				cronJob.Spec = *cronJobSpec
				err = r.Client.Create(c, cronJob)

				if err != nil {
					logger.Error(err, "error creating the time-of-day cron job")
					return ctrl.Result{}, err
				}

				logger.Info(fmt.Sprintf("cron job for %d:%d successfully created", hr, min))
			}
		}

		cronJobNames = append(cronJobNames, cronJobName)
	}

	cronJobList := &powerv1.TimeOfDayCronJobList{}
	err = r.Client.List(c, cronJobList, client.InNamespace(IntelPowerNamespace))
	if err != nil {
		logger.Error(err, "error retrieving the cron job list")
		return ctrl.Result{}, err
	}

	err = r.cleanUpCronJobs(cronJobList.Items, cronJobNames)
	if err != nil {
		logger.Error(err, "error reconciling time-of-day")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TimeOfDayReconciler) cleanUpCronJobs(cronJobs []powerv1.TimeOfDayCronJob, expectedCronJobs []string) error {
	for _, cronJob := range cronJobs {
		if !util.StringInStringList(cronJob.Name, expectedCronJobs) {
			err := r.Client.Delete(context.TODO(), &cronJob)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TimeOfDayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.TimeOfDay{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
