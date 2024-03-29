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
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kristofferahl/aeto/internal/pkg/aws"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/util"

	acmawsv1alpha1 "github.com/kristofferahl/aeto/apis/acm.aws/v1alpha1"
	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	eventv1alpha1 "github.com/kristofferahl/aeto/apis/event/v1alpha1"
	route53awsv1alpha1 "github.com/kristofferahl/aeto/apis/route53.aws/v1alpha1"
	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	acmawscontrollers "github.com/kristofferahl/aeto/controllers/acm.aws"
	corecontrollers "github.com/kristofferahl/aeto/controllers/core"
	eventcontrollers "github.com/kristofferahl/aeto/controllers/event"
	route53awscontrollers "github.com/kristofferahl/aeto/controllers/route53.aws"
	sustainabilitycontrollers "github.com/kristofferahl/aeto/controllers/sustainability"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(corev1alpha1.AddToScheme(scheme))
	utilruntime.Must(route53awsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(acmawsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(eventv1alpha1.AddToScheme(scheme))
	utilruntime.Must(sustainabilityv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	var operatorNamespace string
	var operatorReconcileInterval time.Duration
	var operatorEnabledControllers string
	var operatorMaxTenantResourceSets int

	// Kubebuilder flags
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)

	// Operator flags
	flag.StringVar(&operatorNamespace, "operator-namespace", "aeto", "The operator namespace.")
	flag.DurationVar(&operatorReconcileInterval, "operator-reconcile-interval", 30*time.Minute, "The interval of the reconciliation loop")
	flag.IntVar(&operatorMaxTenantResourceSets, "operator-max-tenant-resourcesets", 3, "The maximum number of resourcesets kept for each tenant")

	// Parse flags
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Operator environment overrides
	operatorNamespace = config.StringEnvVar("OPERATOR_NAMESPACE", operatorNamespace)
	operatorReconcileInterval = config.DurationEnvVar("OPERATOR_RECONCILE_INTERVAL", operatorReconcileInterval)
	operatorMaxTenantResourceSets = config.IntEnvVar("OPERATOR_MAX_TENANT_RESOURCESETS", operatorMaxTenantResourceSets)
	operatorEnabledControllers = config.StringEnvVar("OPERATOR_ENABLED_CONTROLLERS", strings.Join([]string{
		"Tenant",
		"ResourceTemplate",
		"Blueprint",
		"ResourceSet",
		"HostedZone",
		"Certificate",
		"CertificateConnector",
		"EventStreamChunk",
		"SavingsPolicy",
	}, ","))

	enabledControllers := strings.Split(strings.TrimLeft(strings.TrimRight(operatorEnabledControllers, ","), ","), ",")
	if len(enabledControllers) < 1 {
		setupLog.Error(fmt.Errorf("no controllers enabled"), "bootstrap failed")
		os.Exit(1)
	}

	setupLog.Info("bootstrapping operator", "controllers", enabledControllers)

	// Configure operator
	config.Operator = config.OperatorConfig{
		ReconcileInterval:     operatorReconcileInterval,
		Namespace:             operatorNamespace,
		MaxTenantResourceSets: operatorMaxTenantResourceSets,
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "a7b5e012.aeto.net",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if awsRegion := config.StringEnvVar("AWS_REGION", ""); awsRegion == "" {
		setupLog.Error(fmt.Errorf("required environment variable AWS_REGION has no value set"), "bootstrap failed")
		os.Exit(1)
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		setupLog.Error(err, "unable to load AWS credentials")
		os.Exit(1)
	}

	awsClients := aws.Clients{
		Log:     ctrl.Log.WithName("aws-client"),
		Config:  awsConfig,
		Route53: route53.NewFromConfig(awsConfig),
	}

	dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create dynamic client")
		os.Exit(1)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create discovery client")
		os.Exit(1)
	}

	k8sClient := kubernetes.NewClient(mgr.GetClient(), dynamicClient, discoveryClient)

	if util.SliceContainsString(enabledControllers, "Tenant") {
		if err = (&corecontrollers.TenantReconciler{
			Scheme:   mgr.GetScheme(),
			Client:   k8sClient,
			Recorder: mgr.GetEventRecorderFor("tenant-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Tenant")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "ResourceTemplate") {
		if err = (&corecontrollers.ResourceTemplateReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ResourceTemplate")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "Blueprint") {
		if err = (&corecontrollers.BlueprintReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Blueprint")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "ResourceSet") {
		if err = (&corecontrollers.ResourceSetReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ResourceSet")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "HostedZone") {
		if err = (&route53awscontrollers.HostedZoneReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
			AWS:    awsClients,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "HostedZone")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "Certificate") {
		if err = (&acmawscontrollers.CertificateReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
			AWS:    awsClients,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Certificate")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "CertificateConnector") {
		if err = (&acmawscontrollers.CertificateConnectorReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "CertificateConnector")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "EventStreamChunk") {
		if err = (&eventcontrollers.EventStreamChunkReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "EventStreamChunk")
			os.Exit(1)
		}
	}
	if util.SliceContainsString(enabledControllers, "SavingsPolicy") {
		if err = (&sustainabilitycontrollers.SavingsPolicyReconciler{
			Scheme: mgr.GetScheme(),
			Client: k8sClient,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "SavingsPolicy")
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
