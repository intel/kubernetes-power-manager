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
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	//cgp "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/cgroupsparser"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/newstate"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/util"
)

const (
	PowerPodNameConst = "PowerPod"
)

// PowerProfileReconciler reconciles a PowerProfile object
type PowerProfileReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	AppQoSClient *appqos.AppQoSClient
	State        *newstate.PowerNodeData
}

// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=power.intel.com,resources=powerprofiles/status,verbs=get;update;patch

// Reconcile method that implements the reconcile loop
func (r *PowerProfileReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)
	logger.Info("Reconciling PowerProfile")

	r.State.AddProfile("NewProfile")
	logger.Info(fmt.Sprintf("State: %v", r.State.ProfileAssociation))
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

			obseleteProfiles, err := r.findObseleteProfiles(req)
			if err != nil {
				return ctrl.Result{}, err
			}

			for address, profileName := range obseleteProfiles {
				// TODO: CHANGE TO POWERPROFILES
				err = r.AppQoSClient.DeleteApp(address, profileName)
				if err != nil {
					logger.Error(err, "Failed to delete profile from AppQoS")
					return ctrl.Result{}, err
				}
			}

			/*
				//powerProfileFromAppqos, err := GetPowerProfileByName(profile.Spec.Name, "https://localhost:5000", r.AppQoSClient)
				profiles, err := r.AppQoSClient.GetApps(address)
				powerProfileFromAppqos := appqos.FindProfileByName(activeProfiles, req.NamespacedName.Name)
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
			*/

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

	// Loop through Power Nodes to check which AppQos instances have this PowerProfile already
	// If an instance has it, update it, otherwise create it

	for _, targetNode := range r.State.PowerNodeList {
		nodeAddress, err := r.getPodAddress(targetNode, req)
		if err != nil {
			// Continue with other Nodes if there's a failure on one
			logger.Error(err, "Failed to get IP address for node: ", targetNode)
			continue
		}
		/*
			allProfilesOnNode, err := r.AppQoSClient.GetPowerProfiles(nodeAddress)
			if err != nil {
				logger.Error(err, "Failed to list PowerProfiles on node: ", targetNode)
				continue
			}
		*/
		allProfilesOnNode, err := r.AppQoSClient.GetApps(nodeAddress)
		if err != nil {
			logger.Error(err, "Failed to list Apps on node: ", targetNode)
			continue
		}
		// TODO: UNCOMMENT WHEN HARDWARE COMES THROUGH
		//powerProfileFromAppqos := appqos.FindProfileByName(allProfilesOnNode, req.NamespacedName.Name)
		powerProfileFromAppQos := appqos.FindAppByName(allProfilesOnNode, req.NamespacedName.Name)

		// TODO: UNCOMMENT WHEN HARDWARE COMES THROUGH
		/*
			if !reflect.DeepEqual(powerProfileFromAppQoS, &appqos.PowerProfile{}) {
				// Update PowerProfile
				updatedProfile := &appqos.PowerProfile{}
				updateProfile.Name = powerProfileFromAppQos.Name
				updateProfile.MinFreq = powerProfileFromAppQos.MinFreq
				updateProfile.MaxFreq = powerProfileFromAppQos.MaxFreq
				updateProfile.Epp = powerProfileFromAppQos.Epp
				appqosPutResp, err := r.AppQoSClient.PutPowerProfile(updateProfile, nodeAddress, *powerProfileFromAppQos.ID)
				if err != nil {
					logger.Error(err, appqosPutResp)
					continue
				}
			}
		*/

		if !reflect.DeepEqual(powerProfileFromAppQos, &appqos.App{}) {
			// Updating PowerProfile
			logger.Info("Updating")
			updatedProfile := &appqos.App{}
			updatedProfile.Name = powerProfileFromAppQos.Name
			updatedProfile.Cores = &[]int{3, 4, 5}
			updatedProfile.Pids = powerProfileFromAppQos.Pids
			updatedProfile.PoolID = powerProfileFromAppQos.PoolID
			appqosPutString, err := r.AppQoSClient.PutApp(updatedProfile, nodeAddress, *powerProfileFromAppQos.ID)
			if err != nil {
				logger.Error(err, appqosPutString)
				continue
			}
			//return ctrl.Result{}, nil
		} else {
			// Creating PowerProfile
			// TODO: UNCOMMENT WHEN HARDWARE COMES THROUGH
			/*
							powerProfile := &appqos.PowerProfile{}
				        	        powerProfile.Name = &req.NamespacedName.Name
				        	        powerProfile.MinFreq = &profile.Spec.Min
					                powerProfile.MaxFreq = &profile.Spec.Max
				        	        powerProfile.Epp = &profile.Spec.Epp
							appqosPostResp, err := r.AppQoSClient.PostPowerProfile(powerProfile, nodeAddress)
							if err != nil {
				                        	logger.Error(err, appqosPostResp)
				                        	continue
				                	}
			*/

			app := &appqos.App{}
			app.Name = &req.NamespacedName.Name
			app.Cores = &[]int{1, 2, 3}
			app.Pids = &[]int{7876}
			appqosPostResp, err := r.AppQoSClient.PostApp(app, nodeAddress)
			if err != nil {
				logger.Error(err, appqosPostResp)
				continue
			}
		}
	}

	return ctrl.Result{}, nil
}

// TODO: UNCOMMENT WHEN HARDWARE COMES THROUGH
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
func (r *PowerProfileReconciler) findObseleteProfiles(req ctrl.Request) (map[string]int, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)

	//obseleteProfiles := make(map[string]string, 0)
	obseleteProfiles := make(map[string]int, 0)

	for _, nodeName := range r.State.PowerNodeList {
		address, err := r.getPodAddress(nodeName, req)
		if err != nil {
			return nil, err
		}

		activeProfiles, err := r.AppQoSClient.GetApps(address)
		if err != nil {
			logger.Info("Could not GET PowerProfiles.", "Error:", err)
			return nil, err
		}

		//profile := appqos.FindProfileByName(activeProfiles, req.NamespacedName.Name)
		profile := appqos.FindAppByName(activeProfiles, req.NamespacedName.Name)
		if *profile.Name == "" {
			logger.Info("PowerProfile not found on AppQoS instance")
			continue
		}

		obseleteProfiles[address] = *profile.ID
	}

	return obseleteProfiles, nil
}

func (r *PowerProfileReconciler) getPodAddress(nodeName string, req ctrl.Request) (string, error) {
	_ = context.Background()
	logger := r.Log.WithValues("powerprofile", req.NamespacedName)

	// TODO: DELETE WHEN APPQOS CONTAINERIZED
	if 1 == 1 {
		return "https://localhost:5000", nil
	}

	pods := &corev1.PodList{}
	err := r.Client.List(context.TODO(), pods, client.MatchingLabels(client.MatchingLabels{"name": PowerPodNameConst}))
	if err != nil {
		logger.Error(err, "Failed to list AppQoS pods")
		return "", nil
	}

	appqosPod, err := util.GetPodFromNodeName(pods, nodeName)
	if err != nil {
		appqosNode := &corev1.Node{}
		err := r.Client.Get(context.TODO(), client.ObjectKey{
			Name: nodeName,
		}, appqosNode)
		if err != nil {
			logger.Error(err, "Error getting AppQoS node")
			return "", err
		}

		appqosPod, err = util.GetPodFromNodeAddresses(pods, appqosNode)
		if err != nil {
			return "", err
		}
	}

	var podIP string
	notFoundError := errors.NewServiceUnavailable("pod address not available")
	if appqosPod.Status.PodIP != "" {
		podIP = appqosPod.Status.PodIP
	} else if len(appqosPod.Status.PodIPs) != 0 {
		podIP = appqosPod.Status.PodIPs[0].IP
	} else {
		return "", notFoundError
	}

	if len(appqosPod.Spec.Containers) == 0 {
		return "", notFoundError
	}

	if len(appqosPod.Spec.Containers[0].Ports) == 0 {
		return "", notFoundError
	}

	addressPrefix := r.AppQoSClient.GetAddressPrefix()
	address := fmt.Sprintf("%s%s%s%d", addressPrefix, podIP, ":", appqosPod.Spec.Containers[0].Ports[0].ContainerPort)
	return address, nil
}

// SetupWithManager specifies how the controller is built and watch a CR and other resources that are owned and managed by the controller
func (r *PowerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1alpha1.PowerProfile{}).
		Complete(r)
}
