/*
Copyright 2022.

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

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	networkingv1alpha1 "github.com/forbearing/horus-operator/apis/networking/v1alpha1"
	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	networkingcontrollers "github.com/forbearing/horus-operator/controllers/networking"
	storagecontrollers "github.com/forbearing/horus-operator/controllers/storage"
	//+kubebuilder:scaffold:imports
)

var (
	GroupStorage    = storagev1alpha1.GroupVersion.Group
	GroupNetworking = networkingv1alpha1.GroupVersion.Group
	KindBackup      = "Backup"
	KindRestore     = "Restore"
	KindClone       = "Clone"
	KindMigration   = "Migration"
	KindTraffic     = "Traffic"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	//managerLog   = logr.New(log.NewDelegatingLogSink(log.NullLogSink{})).WithValues("operator", "horus-operator") // manager logger remove all key/value.
	backupLog    = ctrl.Log.WithValues("Group", GroupStorage, "Kind", KindBackup)
	restoreLog   = ctrl.Log.WithValues("Group", GroupStorage, "Kind", KindRestore)
	cloneLog     = ctrl.Log.WithValues("Group", GroupStorage, "Kind", KindClone)
	migrationLog = ctrl.Log.WithValues("Group", GroupStorage, "Kind", KindMigration)
	trafficLog   = ctrl.Log.WithValues("Group", GroupNetworking, "Kind", KindTraffic)
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(scheme))
	utilruntime.Must(networkingv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	// Custom flags
	var logEncoder string
	var logLevel int
	var printVersion, pprofActive, webhookEnabled bool
	flag.StringVar(&logEncoder, "log-encoder", "json", "log encoding ('json' or 'console')")
	flag.IntVar(&logLevel, "log-level", int(zapcore.InfoLevel), "set log level")
	flag.BoolVar(&printVersion, "version", false, "print version and exist")
	flag.BoolVar(&pprofActive, "pprof", false, "enable pprof endpoint")
	flag.BoolVar(&webhookEnabled, "enable-webhook", true, "enable CRD conversion webhook")

	opts := zap.Options{
		Development: true, // for test or deployment only, set to true here.
		//TimeEncoder: zapcore.RFC3339NanoTimeEncoder,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	// Parsing flags
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// mgrOpts are the arguments for creating a new manager.
	//
	// Because we have have multigroup enabled,  the manager output log messages
	// with some structured key/values not what we are expect, so reset it with
	// our clean logr.Logger.
	mgrOpts := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ef6e0032.hybfkuf.io",
		//Logger:                 managerLog, // custome manager logger
	}
	// The "NAMESPACE" variable defined in operator deployment manifests will
	// determine which namespace the operator is deploy to.
	// Which namespace the operator is deployed in is important for processing CRDs,
	// and the operator will get the namespace from inside pod by read file "/var/run/secrets/kubernetes.io/serviceaccount/namespace."
	// if no "NAMESPACE" variable provides.
	if operatorNamespace := os.Getenv("NAMESPACE"); len(operatorNamespace) != 0 {
		mgrOpts.Namespace = operatorNamespace
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&storagecontrollers.BackupReconciler{
		Client: mgr.GetClient(),
		Log:    backupLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", KindBackup)
		os.Exit(1)
	}
	if err = (&storagecontrollers.RestoreReconciler{
		Client: mgr.GetClient(),
		Log:    restoreLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", KindRestore)
		os.Exit(1)
	}
	if err = (&storagecontrollers.CloneReconciler{
		Client: mgr.GetClient(),
		Log:    cloneLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", KindClone)
		os.Exit(1)
	}
	if err = (&storagecontrollers.MigrationReconciler{
		Client: mgr.GetClient(),
		Log:    migrationLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", KindMigration)
		os.Exit(1)
	}
	if err = (&networkingcontrollers.TrafficReconciler{
		Client: mgr.GetClient(),
		Log:    trafficLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", KindTraffic)
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
