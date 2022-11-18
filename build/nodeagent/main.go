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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/podresourcesclient"

	"github.com/intel/kubernetes-power-manager/controllers"
	"github.com/intel/kubernetes-power-manager/pkg/podstate"
	"github.com/intel/power-optimization-library/pkg/power"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(powerv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":10001", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	nodeName := os.Getenv("NODE_NAME")

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

	powerLibrary, err := power.CreateInstance(nodeName)
	for _, feature := range powerLibrary.GetFeaturesInfo() {
		setupLog.Info(
			"feature status",
			"feature", feature.Name,
			"driver", feature.Driver,
			"available", power.IsFeatureSupported(feature.Feature))
	}

	if err != nil {
		if !power.IsFeatureSupported(power.PStatesFeature) {
			pStatesNotSupported := errors.NewServiceUnavailable("P-States not supported")
			setupLog.Error(pStatesNotSupported, "error setting up P-States in Power Library")
		}

		if !power.IsFeatureSupported(power.CStatesFeature) {
			cStateNotSupported := errors.NewServiceUnavailable("C-States not supported")
			setupLog.Error(cStateNotSupported, "error setting up C-States in Power Library")
		}

		if powerLibrary == nil {
			setupLog.Error(err, "unable to create Power Library instance")
			os.Exit(1)
		}
	}

	powerNodeState, err := podstate.NewState()
	if err != nil {
		setupLog.Error(err, "unable to create internal state")
		os.Exit(1)
	}

	podResourcesClient, err := podresourcesclient.NewPodResourcesClient()
	if err != nil {
		setupLog.Error(err, "unable to create internal client")
		os.Exit(1)
	}

	if err = (&controllers.PowerProfileReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("PowerProfile"),
		Scheme:       mgr.GetScheme(),
		PowerLibrary: powerLibrary,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerProfile")
		os.Exit(1)
	}
	if err = (&controllers.PowerWorkloadReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("PowerWorkload"),
		Scheme:       mgr.GetScheme(),
		PowerLibrary: powerLibrary,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerWorkload")
		os.Exit(1)
	}
	if err = (&controllers.PowerNodeReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("PowerNode"),
		Scheme:       mgr.GetScheme(),
		PowerLibrary: powerLibrary,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PowerNode")
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
	if err = (&controllers.CStatesReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("CState"),
		Scheme:       mgr.GetScheme(),
		PowerLibrary: powerLibrary,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CStates")
		os.Exit(1)
	}
	if err = (&controllers.TimeOfDayReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("TimeOfDay"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TimeOfDay")
		os.Exit(1)
	}
	if err = (&controllers.TimeOfDayCronJobReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("TimeOfDayCronJob"),
		Scheme:       mgr.GetScheme(),
		PowerLibrary: powerLibrary,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TimeOfDayCronJob")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
