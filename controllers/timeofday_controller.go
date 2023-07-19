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
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// Only power and performance profiles supported for now

//+kubebuilder:rbac:groups=power.intel.com,resources=timeofdays,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=power.intel.com,resources=timeofdays/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TimeOfDay object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.4/pkg/reconcile
func (r *TimeOfDayReconciler) Reconcile(c context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("timeofday", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		logger.Error(fmt.Errorf("incorrect namespace"), "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{}, nil
	}
	//enforces HH:MM:SS time format
	timeRegex := regexp.MustCompile("(0[0-9]|1[0-9]|2[0-3]):[0-5][0-9](:[0-5][0-9])?")

	timeOfDayList := &powerv1.TimeOfDayList{}
	logger.V(5).Info("Retrieving TimeOfDay objects from lists")
	err := r.Client.List(c, timeOfDayList)
	if err != nil {
		logger.Error(err, "Error retrieving TimeOfDayList")
		return ctrl.Result{}, err
	}

	logger.V(5).Info("Confirming that there can only be one TimeOfDay object")
	if len(timeOfDayList.Items) > 1 {
		err := errors.NewServiceUnavailable("Cannot have more than one TimeOfDay")
		logger.Error(err, "Error reconciling TimeOfDay")
		return ctrl.Result{}, err
	}

	timeOfDay := &powerv1.TimeOfDay{}
	err = r.Client.Get(c, req.NamespacedName, timeOfDay)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(5).Info("Deleting TimeOfDay Cronjobs from Cluster")
			err = r.Client.DeleteAllOf(c, &powerv1.TimeOfDayCronJob{}, client.InNamespace(IntelPowerNamespace))
			if err != nil {
				logger.Error(err, "Error deleting TimeOfDay CronJobs")
				return ctrl.Result{}, err
			}
		}
		logger.Error(err, "Error retrieving TimeOfDay")
		return ctrl.Result{}, err
	}

	// Validate incoming values from TimeOfDay manifest
	timeZone := timeOfDay.Spec.TimeZone
	logger.V(5).Info("Validated timezone for TimeOfDay, values are: %s", timeZone)

	if timeZone != "" {
		_, err := time.LoadLocation(timeZone)
		logger.Info(fmt.Sprintf("timezone is %s\n", timeZone))
		if err != nil {
			err := errors.NewServiceUnavailable("Invalid timezone, refer to the IANA timezone database for a list of valid timezones")
			logger.Error(err, "Error creating TimeOfDay")
			return ctrl.Result{}, err
		}
	}

	var cronJobNames []string
	logger.V(5).Info("Creating TimeOfDay Cronjobs")
	for _, scheduleInfo := range timeOfDay.Spec.Schedule {
		if !timeRegex.MatchString(scheduleInfo.Time) {
			err := errors.NewServiceUnavailable("Time filed must be in format HH:MM:SS or HH:MM and cannot be empty")
			logger.Error(err, "Error creating TimeOfDay schedule")
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
			err := errors.NewServiceUnavailable("profile detected with no reserved CPUs set")
			logger.Error(err, "Error creating TimeOfDay schedule")
			return ctrl.Result{}, err
		}

		cronJobName := fmt.Sprintf("%s-%d-%d-%d", timeOfDay.Name, hr, min, sec)

		cronJob := &powerv1.TimeOfDayCronJob{}
		err = r.Client.Get(c, client.ObjectKey{
			Name:      cronJobName,
			Namespace: IntelPowerNamespace,
		}, cronJob)
		// if cronjob doesn't exist create one
		logger.V(5).Info("Creating Cronjob if one doesn't exist")
		if err != nil {

			if errors.IsNotFound(err) {
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
				//logger.Info(fmt.Sprintf("created cronjob %s\n",cronJob.Name))

				if err != nil {
					logger.Error(err, "Error creating TimeOfDay CronJob")
					return ctrl.Result{}, err
				}

				logger.Info(fmt.Sprintf("CronJob for %d:%d successfully created", hr, min))
			}
		}

		cronJobNames = append(cronJobNames, cronJobName)
	}

	cronJobList := &powerv1.TimeOfDayCronJobList{}
	err = r.Client.List(c, cronJobList, client.InNamespace(IntelPowerNamespace))
	if err != nil {
		logger.Error(err, "Error retrieving CronJobList")
		return ctrl.Result{}, err
	}

	err = r.cleanUpCronJobs(cronJobList.Items, cronJobNames)
	if err != nil {
		logger.Error(err, "Error reconciling TimeOfDay")
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
		Complete(r)
}
