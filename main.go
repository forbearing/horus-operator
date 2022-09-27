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
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	networkingv1alpha1 "github.com/forbearing/horus-operator/apis/networking/v1alpha1"
	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	networkingcontrollers "github.com/forbearing/horus-operator/controllers/networking"
	storagecontrollers "github.com/forbearing/horus-operator/controllers/storage"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/version"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	//managerLog   = logr.New(log.NewDelegatingLogSink(log.NullLogSink{})).WithValues("operator", "horus-operator") // manager logger remove all key/value.
	backupLog    = ctrl.Log.WithValues("Group", types.GroupStorage, "Kind", types.KindBackup)
	restoreLog   = ctrl.Log.WithValues("Group", types.GroupStorage, "Kind", types.KindRestore)
	cloneLog     = ctrl.Log.WithValues("Group", types.GroupStorage, "Kind", types.KindClone)
	migrationLog = ctrl.Log.WithValues("Group", types.GroupStorage, "Kind", types.KindMigration)
	trafficLog   = ctrl.Log.WithValues("Group", types.GroupNetworking, "Kind", types.KindTraffic)
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
	flag.IntVar(&logLevel, "log-level", int(zapcore.InfoLevel), "set log level, higher levels are more important")
	flag.BoolVar(&printVersion, "version", false, "print version and exist")
	flag.BoolVar(&pprofActive, "pprof", false, "enable pprof endpoint")
	flag.BoolVar(&webhookEnabled, "enable-webhook", false, "enable CRD conversion webhook")
	flag.Parse()

	// Logging setup
	if err := customSetupLogging(zapcore.Level(logLevel), logEncoder); err != nil {
		setupLog.Error(err, "Unable to setup the logger")
		os.Exit(1)
	}
	// Print version information
	if printVersion {
		version.PrintVersionWriter(os.Stdout, "text")
		os.Exit(0)
	}
	version.PrintVersionLogs(setupLog)

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
		setupLog.Error(err, "unable to create controller", "controller", types.KindBackup)
		os.Exit(1)
	}
	if err = (&storagecontrollers.RestoreReconciler{
		Client: mgr.GetClient(),
		Log:    restoreLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", types.KindRestore)
		os.Exit(1)
	}
	if err = (&storagecontrollers.CloneReconciler{
		Client: mgr.GetClient(),
		Log:    cloneLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", types.KindClone)
		os.Exit(1)
	}
	if err = (&storagecontrollers.MigrationReconciler{
		Client: mgr.GetClient(),
		Log:    migrationLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", types.KindMigration)
		os.Exit(1)
	}
	if err = (&networkingcontrollers.TrafficReconciler{
		Client: mgr.GetClient(),
		Log:    trafficLog,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", types.KindTraffic)
		os.Exit(1)
	}
	// Enable dynamic admission webhooks.
	if webhookEnabled {
		if err = (&networkingv1alpha1.Traffic{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Traffic")
			os.Exit(1)
		}
		if err = (&storagev1alpha1.Backup{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Backup")
			os.Exit(1)
		}
		if err = (&storagev1alpha1.Restore{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Restore")
			os.Exit(1)
		}
		if err = (&storagev1alpha1.Clone{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Clone")
			os.Exit(1)
		}
		if err = (&storagev1alpha1.Migration{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Migration")
			os.Exit(1)
		}
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

func customSetupLogging(logLevel zapcore.Level, logEncoder string) error {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	//encoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder

	var encoder zapcore.Encoder
	switch logEncoder {
	case "console":
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	default:
		return fmt.Errorf("unknow log encoder: %s", logEncoder)
	}
	ctrl.SetLogger(ctrlzap.New(
		ctrlzap.Encoder(encoder),
		ctrlzap.Level(logLevel),
		ctrlzap.StacktraceLevel(zapcore.PanicLevel)))
	return nil
}

//func customSetupHealthChecks(mgr manager.Manager) {
//}
//func customSetupEndpoints(pprofActive bool, mgr manager.Manager) {
//}
