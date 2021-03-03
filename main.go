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

package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	//"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/workloadstate"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/podresourcesclient"

	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/controllers"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/newstate"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(powerv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	//flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":10000", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "6846766c.intel.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	powerNodeState, err := state.NewState()
	if err != nil {
		setupLog.Error(err, "unable to create internal state")
		os.Exit(1)
	}

	//workloadState, err := workloadstate.NewWorkloads()
	//if err != nil {
	//	setupLog.Error(err, "unable to create internal cpu state")
	//	os.Exit(1)
	//}

	appQoSClient := appqos.NewDefaultAppQoSClient()
	//if err != nil {
	//	setupLog.Error(err, "unable to create internal AppQoS client")
	//	os.Exit(1)
	//}

	newstate := newstate.NewPowerNodeData()
	//newstate.UpdatePowerNodeData("worker")

	podResourcesClient, err := podresourcesclient.NewPodResourcesClient()
	if err != nil {
		setupLog.Error(err, "unable to create internal client")
		os.Exit(1)
	}

	if err = (&controllers.PowerNodeReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("PowerNode"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerNode")
		os.Exit(1)
	}
	if err = (&controllers.PowerProfileReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("PowerProfile"),
		Scheme:       mgr.GetScheme(),
		AppQoSClient: appQoSClient,
		State:        newstate,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerProfile")
		os.Exit(1)
	}
	if err = (&controllers.PowerWorkloadReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("PowerWorkload"),
		Scheme:       mgr.GetScheme(),
		AppQoSClient: appQoSClient,
		State:        newstate,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerWorkload")
		os.Exit(1)
	}
	if err = (&controllers.PowerPodReconciler{
		Client:             mgr.GetClient(),
		Log:                ctrl.Log.WithName("controllers").WithName("PowerPod"),
		Scheme:             mgr.GetScheme(),
		State:              *powerNodeState,
		PodResourcesClient: *podResourcesClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerPod")
		os.Exit(1)
	}
	if err = (&controllers.PowerConfigReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("PowerConfig"),
		Scheme: mgr.GetScheme(),
		State:  newstate,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerConfig")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
