package main

import (
	"flag"
	"log"
	"strings"

	apps "github.com/SAP/cloud-robotics/src/go/pkg/apis/apps/v1alpha1"
	cfg "github.com/SAP/cloud-robotics/src/go/pkg/apis/config/v1alpha1"
	registry "github.com/SAP/cloud-robotics/src/go/pkg/apis/registry/v1alpha1"
	"github.com/SAP/cloud-robotics/src/go/pkg/controller/tenant"
	cert "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	dns "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	networking "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	certDir                = flag.String("cert-dir", "/tls", "directory for TLS certificate")
	domain                 = flag.String("domain", "", "root domain of the cluster")
	webhookPort            = flag.Int("webhook-port", 9876, "listening port for tenant webhook")
	defaultGateway         = flag.String("default-gateway", "", "Default Istio Gateway of the cluster")
	tenantSpecificGateways = flag.String("tenant-specific-gateways", "false", "use tenant specific Istio Gateways")
)

func main() {
	// Parse command line flags
	flag.Parse()
	gatewaysEnabled := bool(strings.ToLower(*tenantSpecificGateways) == "true")

	// Prepare new scheme for manager
	sc := runtime.NewScheme()
	scheme.AddToScheme(sc)
	cfg.AddToScheme(sc)
	cert.AddToScheme(sc)
	dns.AddToScheme(sc)
	networking.AddToScheme(sc)
	apps.AddToScheme(sc)
	registry.AddToScheme(sc)

	// Create new manager
	ctrllog.SetLogger(zap.New())
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{Scheme: sc, Port: *webhookPort})
	if err != nil {
		log.Fatalf("Unable to create manager: %s", err)
	}

	// Add tenant-controller
	log.Print("Setting up tenant-controller")
	if *domain == "" {
		log.Fatal("domain flag must not be empty")
	}
	log.Printf("Cluster root domain is %s", *domain)
	err = tenant.Add(mgr, *domain, *defaultGateway, gatewaysEnabled)
	if err != nil {
		log.Fatalf("Unable to add tenant-controller to manager: %s", err)
	}

	// Setup webhook
	log.Print("Setting up webhook server")
	hookserver := mgr.GetWebhookServer()
	hookserver.CertDir = *certDir

	log.Print("Registering validating tenant webhook")
	hookserver.Register("/tenant/validate", tenant.NewTenantValidationWebhook(mgr.GetClient(), gatewaysEnabled))

	// Start the controller
	log.Print("Starting controller manager")
	err = mgr.Start(signals.SetupSignalHandler())
	if err != nil {
		log.Fatalf("Unable to start controller manager: %s", err)
	} else {
		log.Print("Shutting down")
	}
}
