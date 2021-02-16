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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	//cgp "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cgroupsparser"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/newstate"
)

// PowerProfileReconciler reconciles a PowerProfile object
type PowerProfileReconciler struct {
	client.Client
	Log          	logr.Logger
	Scheme       	*runtime.Scheme
	AppQoSClient 	*appqos.AppQoSClient
	State		*newstate.PowerNodeData
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles/status,verbs=get;update;patch

// Reconcile method that implements the reconcile loop
func (r *PowerProfileReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)
	logger.Info("Reconciling PowerProfile")

	logger.Info(fmt.Sprintf("State: %v", r.State.PowerNodeList))
	if 1 == 1 {
		return ctrl.Result{}, nil
	}

	/*app, e := r.AppQoSClient.GetApps("https://localhoste:5000")
	if e != nil {
		logger.Error(e, "Error retrieving App")
		return ctrl.Result{}, nil
	}
	logger.Info(fmt.Sprintf("Apps: %v", app))
	if 1 == 1 {
		return ctrl.Result{}, nil
	}*/
	/*
		pools, er := r.AppQoSClient.GetPools("https://localhost:5000")
		if er != nil {
			logger.Error(er, "Error retreiving pools")
			return ctrl.Result{}, nil
		}

		for _, pool := range pools {
			if *pool.Name == "Default" {
				logger.Info(fmt.Sprintf("Cores: %v", *pool.Cores))
			}
		}
		//logger.Info(fmt.Sprintf("POOLS: %v", pools))

		if 1 == 1 {
			return ctrl.Result{}, nil
		}
	*/

	profile := &powerv1alpha1.PowerProfile{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			// When a PowerProfile cannot be found, we assume it has been deleted. We need to check if there is a
			// corresponding PowerWorkload and, if there is, delete that too. We leave the cleanup of requesting the
			// frequency resets of the effected CPUs to the PowerWorkload controller.

			powerProfileFromAppqos, err := GetPowerProfileByName(profile.Spec.Name, "https://localhost:5000", r.AppQoSClient)
			fmt.Printf("Profile: %s--%v\n", profile.Spec.Name, powerProfileFromAppqos)
			if err != nil {
				logger.Error(err, "Error retreiving PowerProfile")
				return ctrl.Result{}, nil
			}

			if powerProfileFromAppqos != nil {
				err = r.AppQoSClient.DeleteApp("https://localhost:5000", *powerProfileFromAppqos.ID)
				if err != nil {
					logger.Error(err, "Error deleting PowerProfile")
					return ctrl.Result{}, nil
				}
			}

			/*
				logger.Info(fmt.Sprintf("PowerProfile %v has been deleted, cleaning up...", req.NamespacedName))
				workload := &powerv1alpha1.PowerWorkload{}
				workloadName := fmt.Sprintf("%s%s", req.NamespacedName.Name, WorkloadNameSuffix)
				err = r.Client.Get(context.TODO(), client.ObjectKey{
					Namespace: req.NamespacedName.Namespace,
					Name:      workloadName,
				}, workload)
				if err != nil {
					if errors.IsNotFound(err) {
						// No PowerWorkload was found so nothing to do
						return ctrl.Result{}, nil
					}

					logger.Error(err, "error while trying to retrieve PowerWorkload")
					return ctrl.Result{}, err
				}

				// PowerWorkload exists so must cleanup
				err = r.Client.Delete(context.TODO(), workload)
				if err != nil {
					logger.Error(err, "error while trying to delete PowerWorkload")
					return ctrl.Result{}, err
				}
			*/

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	// Check if the PowerProfile exists in the AppQoS instance
	profileFromAppQoS, err := GetPowerProfileByName(profile.Spec.Name, "https://localhost:5000", r.AppQoSClient)
	if err != nil {
		logger.Error(err, "Error retreiving PowerProfile")
		return ctrl.Result{}, nil
	}

	if profileFromAppQoS != nil {
		// Updating PowerProfile
		logger.Info("Updating")
		updatedProfile := &appqos.App{}
		updatedProfile.Name = profileFromAppQoS.Name
		updatedProfile.Cores = &[]int{3, 4, 5}
		updatedProfile.Pids = profileFromAppQoS.Pids
		updatedProfile.PoolID = profileFromAppQoS.PoolID
		appqosPutString, err := r.AppQoSClient.PutApp(updatedProfile, "https://localhost:5000", *profileFromAppQoS.ID)
		if err != nil {
			logger.Error(err, appqosPutString)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}

	logger.Info("Creating")
	// CHANGE TO POWER PROFILE STUFF
	app := &appqos.App{}
	app.Name = &profile.Spec.Name
	app.Cores = &[]int{1, 2, 3}
	app.Pids = &[]int{64855}
	appqosPostString, err := r.AppQoSClient.PostApp(app, "https://localhost:5000")
	if err != nil {
		logger.Error(err, appqosPostString)
		return ctrl.Result{}, nil
	}

	/*
		// Check if a PowerWorkload already exists for this PowerProfile, meaning we just need to update it
		workload := &powerv1alpha1.PowerWorkload{}
		workloadName := fmt.Sprintf("%s%s", req.NamespacedName.Name, WorkloadNameSuffix)
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: req.NamespacedName.Namespace,
			Name:      workloadName,
		}, workload)
		if err != nil {
			if errors.IsNotFound(err) {
				// TODO: change this comment
				// This is a new PowerProfile, so we may need to create the corresponding PowerWorkload.
				// If the PowerProfile is the designated Shared configuration for all of the shared-pool cores,
				// this controller is responsible for creating the associated PowerWorkload. If it's an
				// Exclusive PowerProfile, PowerWorkload creation is left to the PowerPod controller when the PowerProfile is requested.
				// The Shared configuration is recognised by having the name "Shared".

				app := &appqos.App{}
				app.Name = &profile.Spec.Name
				app.Cores = &[]int{1,2,3,4}
				app.Pids = &[]int{38893}
				postStr, err := r.AppQoSClient.PostApp(app, "https://localhost:5000")
				if err != nil {
					logger.Error(err, postStr)
					return ctrl.Result{}, nil
				}

				/*
				if profile.Spec.Name == "Shared" {
					logger.Info("Shared PowerProfile detected, creating corresponding PowerWorkload")
					// TODO: Update with correct value when pakcage has been developed
					nodes := []string{"Placeholder"}
					cpuIDs, _ := cgp.GetSharedPool()
					workloadSpec := &powerv1alpha1.PowerWorkloadSpec{
						Nodes:        nodes,
						CpuIds:       cpuIDs,
						PowerProfile: *profile,
					}
					workload := &powerv1alpha1.PowerWorkload{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: req.NamespacedName.Namespace,
							Name:      workloadName,
						},
					}
					workload.Spec = *workloadSpec
					err = r.Client.Create(context.TODO(), workload)
					if err != nil {
						logger.Error(err, "error while trying to create PowerWorkload for PowerProfile 'Shared'")
						return ctrl.Result{}, err
					}
				}
				/



				return ctrl.Result{}, nil
			}

			logger.Error(err, "error while trying to retrieve PowerWorkload")
			return ctrl.Result{}, err
		}
	*/

	/*
		workload.Spec.PowerProfile = *profile
		err = r.Client.Update(context.TODO(), workload)
		if err != nil {
			logger.Error(err, "error while trying to update PowerWorkload")
			return ctrl.Result{}, err
		}
	*/

	return ctrl.Result{}, nil
}

/*
func (r *PowerProfileReconciler) findObceleteProfiles(req ctrl.Request) (map[string]string, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)

	obsleteProfiles := make(map[string]string, 0)

	for _, name := range r.PowerNodeData.PowerNodeList {
		address, err := r.getPodAddress(nodeName)
		if err != nil {
			return nil, err
		}

		activeProfiles, err := r.AppQoSClient.GetPowerProfiles(address)
		if err != nil {
			logger.Info("Could not GET PowerProfiles.", "Error:", err)
			return nil, err
		}

		profile := appqos.FindProfileByName(activeProfiles, req.NamespacedName.Name)
		if profile.Name == "" {
			logger.Info("PowerProfile not found on AppQoS instance")
			continue
		}

		obseleteProfiles[address] = profile.ID
	}
}
*/

// TODO: DELETE WHEN HARDWARE COMES THROUGH
func (r *PowerProfileReconciler) findObceleteProfiles(req ctrl.Request) (map[string]string, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)

	obsleteProfiles := make(map[string]string, 0)

	for _, name := range r.PowerNodeData.PowerNodeList {
		address, err := r.getPodAddress(nodeName)
		if err != nil {
			return nil, err
		}

		activeProfiles, err := r.AppQoSClient.GetApps(address)
		if err != nil {
			logger.Info("Could not GET PowerProfiles.", "Error:", err)
			return nil, err
		}

		profile := appqos.FindProfileByName(activeProfiles, req.NamespacedName.Name)
		if profile.Name == "" {
			logger.Info("PowerProfile not found on AppQoS instance")
			continue
		}

		obseleteProfiles[address] = profile.ID
	}
}

func (r *PowerProfileReconciler) getPodAddress(nodeName string) (string, error) {
	_ = context.Background()
        logger := r.Log.WithValues("powerprofile", req.NamespacedName)

	if 1 == 1 {
		return "https://localhost:5000", nil
	}

	pods := &corev1.PodList{}
	err := r.Client.List(context.TODO(), pods, client.MatchingLabels(client.MatchLabels{"name": PowerPodNameConst}))
	if err != nil {
		logger.Error(err, "Failed to list AppQoS pods")
		return "", nil
	}

	//appqosPod, err := 
}

// SetupWithManager specifies how the controller is built and watch a CR and other resources that are owned and managed by the controller
func (r *PowerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerProfile{}).
		Complete(r)
}
