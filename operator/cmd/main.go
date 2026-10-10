package main

import (
	"flag"
	"os"
	"strings"

	configurationv1alpha1 "github.com/sarunasr/vpc-poc/operator/api/v1alpha1"
	"github.com/sarunasr/vpc-poc/operator/internal/controller"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var metricsAddr, probeAddr, image, collectionVersions string
	var secureMetrics bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Metrics listener address.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Health probe listener address.")
	flag.StringVar(&image, "ansible-image", "ghcr.io/sarunasr/vpc-poc-ansible-ee:1.0.2", "Immutable Ansible execution environment image.")
	flag.StringVar(&collectionVersions, "collection-versions", "homelab.web=1.0.0,homelab.system=1.0.0", "Collections and versions baked into the execution image.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true, "Serve metrics over HTTPS.")
	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(configurationv1alpha1.AddToScheme(scheme))

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Cache:                  cache.Options{DefaultNamespaces: map[string]cache.Config{"infrastructure": {}}},
		Metrics:                metricsserver.Options{BindAddress: metricsAddr, SecureServing: secureMetrics},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         true,
		LeaderElectionID:       "configuration.infra.example.io",
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to create manager")
		os.Exit(1)
	}
	versions := make(map[string]string)
	for _, entry := range strings.Split(collectionVersions, ",") {
		name, version, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok || name == "" || version == "" {
			ctrl.Log.Error(os.ErrInvalid, "invalid --collection-versions value", "value", collectionVersions)
			os.Exit(1)
		}
		versions[name] = version
	}
	if err := (&controller.ConfigurationReconciler{
		Client:             manager.GetClient(),
		Scheme:             manager.GetScheme(),
		Image:              image,
		Reader:             manager.GetAPIReader(),
		LifecycleDir:       "/usr/share/infrastructure-gitops/ansible_collections",
		CollectionVersions: versions,
	}).SetupWithManager(manager); err != nil {
		ctrl.Log.Error(err, "unable to configure controller")
		os.Exit(1)
	}
	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to register health check")
		os.Exit(1)
	}
	if err := manager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to register readiness check")
		os.Exit(1)
	}
	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "manager exited unexpectedly")
		os.Exit(1)
	}
}
