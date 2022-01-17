// Copyright 2019 The Cloud Robotics Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	b64 "encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAP/cloud-robotics/src/go/pkg/coretools"
	"github.com/SAP/cloud-robotics/src/go/pkg/kubeutils"
	"github.com/SAP/cloud-robotics/src/go/pkg/robotauth"
	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	robotName            = new(string)
	mainNamespace        string
	robotConfigNamespace string
	domain               = flag.String("domain", "", "Domain name of Cloud Robotics Kyma Cluster")
	tenant               = flag.String("tenant", "", "Tenant name in Cloud Robotics Kyma Cluster")
	robotType            = flag.String("robot-type", "", "Robot type. Optional if the robot is already registered.")
	labels               = flag.String("labels", "", "Robot labels. Optional if the robot is already registered.")
	annotations          = flag.String("annotations", "", "Robot annotations. Optional if the robot is already registered.")
	dockerDataRoot       = flag.String("docker-data-root", "/var/lib/docker", "This should match data-root in /etc/docker/daemon.json.")
	podCIDR              = flag.String("pod-cidr", "192.168.9.0/24",
		"The range of Pod IP addresses in the cluster. This should match the CNI "+
			"configuration (eg Cilium's clusterPoolIPv4PodCIDR). If this is incorrect, "+
			"pods will get 403 Forbidden when trying to reach the metadata server.")

	robotGVR = schema.GroupVersionResource{
		Group:    "registry.cloudrobotics.com",
		Version:  "v1alpha1",
		Resource: "robots",
	}
)

const (
	filesDir          = "/setup-robot-files"
	helmPath          = filesDir + "/helm"
	synkPath          = filesDir + "/synk"
	numDNSRetries     = 6
	numServiceRetries = 6
	// commaSentinel is used when parsing labels or annotations.
	commaSentinel = "_COMMA_SENTINEL_"
	baseNamespace = metav1.NamespaceDefault
)

func parseFlags() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: setup-robot <robot-name> [OPTIONS]")
		fmt.Fprintln(os.Stderr, "  robot-name")
		fmt.Fprintln(os.Stderr, "        Robot name")
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		log.Fatal("ERROR: robot-name is required.")
	} else if flag.NArg() > 1 {
		flag.Usage()
		log.Fatalf("ERROR: too many positional arguments (%d), expected 1.", flag.NArg())
	} else if errs := validation.ValidateClusterName(flag.Arg(0), false); len(errs) > 0 {
		log.Fatalf("ERROR: invalid cluster name %q: %s", flag.Arg(0), strings.Join(errs, ", "))
	}

	*robotName = flag.Arg(0)

	mainNamespace = coretools.TenantMainNamespace(*tenant)
	robotConfigNamespace = coretools.RobotConfigNamespace(mainNamespace)

}

func newExponentialBackoff(initialInterval time.Duration, multiplier float64, retries uint64) backoff.BackOff {
	exponentialBackoff := backoff.ExponentialBackOff{
		InitialInterval: initialInterval,
		Multiplier:      multiplier,
		Clock:           backoff.SystemClock,
	}
	exponentialBackoff.Reset()
	return backoff.WithMaxRetries(&exponentialBackoff, retries)
}

// Since this might be the first interaction with the cluster, manually resolve the
// domain name with retries to give a better error in the case of failure.
func waitForDNS(domain string, retries uint64) error {
	log.Printf("DNS lookup for %q", domain)

	if err := backoff.RetryNotify(
		func() error {
			ips, err := net.LookupIP(domain)
			if err != nil {
				return err
			}

			// Check that the results contain an ipv4 addr. Initially, coredns may only
			// return ipv6 addresses in which case helm will fail.
			for _, ip := range ips {
				if ip.To4() != nil {
					return nil
				}
			}

			return errors.New("IP not found")
		},
		newExponentialBackoff(time.Second, 2, retries),
		func(_ error, _ time.Duration) {
			log.Printf("... Retry dns for %q", domain)
		},
	); err != nil {
		return fmt.Errorf("DNS lookup for %q failed: %v", domain, err)
	}

	return nil
}

// Tests a given cloud endpoint with a HEAD request a few times. This lets us wait for the service
// to be available or error with a better message
func waitForService(client *http.Client, url string, retries uint64) error {
	log.Printf("Service probe for %q", url)

	if err := backoff.RetryNotify(
		func() error {
			_, err := client.Head(url)
			return err
		},
		newExponentialBackoff(time.Second, 2, retries),
		func(_ error, _ time.Duration) {
			log.Printf("... Retry service for %q", url)
		},
	); err != nil {
		return fmt.Errorf("service probe for %q failed: %v", url, err)
	}

	return nil
}

// checkRobotName tests whether a Robot resource exists in the local cluster
// with a different name. It is not safe to rerun setup-robot with a different
// name as the chart-assignment-controller doesn't allow the clusterName field to change.
func checkRobotName(ctx context.Context, client dynamic.Interface) error {
	robots, err := client.Resource(robotGVR).Namespace(mainNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "list local robots")
	}
	for _, r := range robots.Items {
		if r.GetName() != *robotName {
			return fmt.Errorf(`this cluster was already set up with a different name. It is not safe to rename an existing cluster.
    - either, use the old name:
        setup_robot.sh %q [...]
    - or, reset the cluster before renaming it:
        sudo kubeadm reset
	setup_robot.sh %q [...]`, r.GetName(), *robotName)
		}
	}
	return nil
}

// storeInK8sSecret write new robot-id to kubernetes secret.
func storeInK8sSecret(ctx context.Context, clientset *kubernetes.Clientset, namespace string, r *robotauth.RobotAuth) error {
	authJson, err := json.Marshal(r)
	if err != nil {
		return err
	}

	return kubeutils.UpdateSecret(
		ctx,
		clientset,
		"robot-auth",
		namespace,
		corev1.SecretTypeOpaque,
		map[string][]byte{
			"json": authJson,
		})
}

func main() {
	parseFlags()
	ctx := context.Background()
	envToken := os.Getenv("ACCESS_TOKEN")
	if envToken == "" {
		log.Fatal("ACCESS_TOKEN environment variable is required.")
	}
	registry := os.Getenv("REGISTRY")
	if registry == "" {
		log.Fatal("REGISTRY environment variable is required.")
	}
	parsedLabels, err := parseKeyValues(*labels)
	if err != nil {
		log.Fatalf("Invalid labels %q: %s", *labels, err)
	}
	parsedAnnotations, err := parseKeyValues(*annotations)
	if err != nil {
		log.Fatalf("Invalid annotations %q: %s", *annotations, err)
	}

	// Set up the OAuth2 token source.
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: envToken})

	remoteServer := fmt.Sprintf("k8s.%s", *domain)

	// Wait until we can resolve the cluster domain.
	if err := waitForDNS(remoteServer, numDNSRetries); err != nil {
		log.Fatalf("Failed to resolve cloud cluster domain %s: %s. Please retry in 5 minutes.", remoteServer, err)
	}

	httpClient := oauth2.NewClient(context.Background(), tokenSource)

	if err := waitForService(httpClient, fmt.Sprintf("https://%s", remoteServer), numServiceRetries); err != nil {
		log.Fatalf("Failed to connect to the cloud cluster: %s. Please retry in 5 minutes.", err)
	}

	// Connect to the surrounding k8s cluster.
	localConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to load in-cluster config: ", err)
	}
	k8sLocalClientSet, err := kubernetes.NewForConfig(localConfig)
	if err != nil {
		log.Fatal("Failed to create kubernetes client set: ", err)
	}
	if _, err := k8sLocalClientSet.AppsV1().Deployments("default").Get(ctx, "app-rollout-controller", metav1.GetOptions{}); err == nil {
		// It's important to avoid deploying the cloud-robotics
		// metadata-server & cr-syncer in the main cluster. This might break the setup.
		log.Fatal("The local context contains a app-rollout-controller deployment. It is not safe to run robot setup on a Kyma cloud cluster.")
	}
	if err := ensureTenantMainNamespace(ctx, k8sLocalClientSet); err != nil {
		log.Fatal("Failed to ensure tenant main namespace: ", err)
	}

	k8sLocalDynamic, err := dynamic.NewForConfig(localConfig)
	if err != nil {
		log.Fatal("Failed to create dynamic client set: ", err)
	}
	if err := checkRobotName(ctx, k8sLocalDynamic); err != nil {
		log.Fatal("Error: ", err)
	}

	k8sCloudCfg := kubeutils.BuildCloudKubernetesConfig(tokenSource, remoteServer)

	// Authenticating robot-setup in upstream cluster using robot-setup service account API token
	k8sClientSet, err := kubernetes.NewForConfig(k8sCloudCfg)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}
	token, err := waitForServiceAccountToken(ctx, k8sClientSet, numServiceRetries)
	if err != nil {
		log.Fatalf("Getting service account token for robot in upstream cluster failed: %v", err)
	}

	robotSetupConfig, err := getRobotSetupConfigmap(ctx, k8sClientSet, numServiceRetries)
	if err != nil {
		log.Fatalf("Getting robot-setup configmap in upstream cluster failed: %v", err)
	}

	// Set up client for cloud k8s cluster (needed only to obtain list of robots).
	k8sDynamicClient, err := dynamic.NewForConfig(k8sCloudCfg)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}
	if err := createOrUpdateRobot(ctx, k8sDynamicClient, parsedLabels, parsedAnnotations, robotSetupConfig); err != nil {
		log.Fatalf("Failed to update robot CR %v: %v", *robotName, err)
	}

	// Set up robot authentication.
	auth := &robotauth.RobotAuth{
		RobotName:         *robotName,
		Domain:            *domain,
		UpstreamAuthToken: *token,
	}

	if err := storeInK8sSecret(ctx, k8sLocalClientSet, baseNamespace, auth); err != nil {
		log.Fatal(fmt.Errorf("failed to write auth secret: %v", err))
	}
	if err := syncDockerSecret(ctx, k8sClientSet, k8sLocalClientSet); err != nil {
		log.Fatal(err)
	}

	// ensure tls certs
	whCert, whKey, err := ensureWebhookCerts(ctx, k8sLocalClientSet, baseNamespace)
	if err != nil {
		log.Fatalf("Failed to create tls certs for webhook: %v.", err)
	}

	log.Println("Initializing Synk")
	output, err := exec.Command(synkPath, "init").CombinedOutput()
	if err != nil {
		log.Fatalf("Synk init failed: %v. Synk output:\n%s\n", err, output)
	}

	// Use "robot" as a suffix for consistency for Synk deployments.
	installChartOrDie(ctx, k8sLocalClientSet, *domain, registry, "base-robot", baseNamespace,
		"base-robot-0.1.0.tgz", whCert, whKey, robotSetupConfig)
	log.Println("Setup complete")
}

// create tls certs for the webhook if they don't exist or need an update
func ensureWebhookCerts(ctx context.Context, cs kubernetes.Interface, namespace string) (string, string, error) {
	sa, err := cs.CoreV1().Secrets(namespace).Get(ctx, "chart-assignment-controller-tls", metav1.GetOptions{})
	if err == nil && sa.Labels["cert-format"] == "v2" {
		// If we already have it and it has the right label, return the certs.
		// This is crucial, since mounted secrets are only updated once a minute.
		log.Print("Returning existing certificate.")
		return b64.URLEncoding.EncodeToString(sa.Data["tls.crt"]), b64.URLEncoding.EncodeToString(sa.Data["tls.key"]), nil
	}

	// Generate new certs
	// based on https://golang.org/src/crypto/tls/generate_cert.go

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to generate private key")
	}
	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to generate serial number")
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"chart-assignment-controller." + namespace + ".svc"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(100, 0, 0), // 100 years

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"chart-assignment-controller." + namespace + ".svc"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to create certificate")
	}

	var crt bytes.Buffer
	if err := pem.Encode(&crt, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", errors.Wrap(err, "Failed to write cert data")
	}

	var key bytes.Buffer
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", errors.Wrap(err, "Unable to marshal private key")
	}
	if err := pem.Encode(&key, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", errors.Wrap(err, "Failed to write key data")
	}

	return b64.URLEncoding.EncodeToString(crt.Bytes()), b64.URLEncoding.EncodeToString(key.Bytes()), nil
}

func helmValuesStringFromMap(varMap map[string]string) string {
	varList := []string{}
	for k, v := range varMap {
		varList = append(varList, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(varList, ",")
}

// installChartOrDie installs a chart using Synk.
func installChartOrDie(ctx context.Context, cs *kubernetes.Clientset, domain, registry, name, namespace, chartPath, whCert, whKey string, robotSetupConfig *corev1.ConfigMap) {
	// ensure namespace for chart exists
	if _, err := cs.CoreV1().Namespaces().Create(ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		},
		metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		log.Fatalf("Failed to create %s namespace: %v.", namespace, err)
	}

	vars := helmValuesStringFromMap(map[string]string{
		"domain":                             domain,
		"registry":                           registry,
		"docker_data_root":                   *dockerDataRoot,
		"pod_cidr":                           *podCIDR,
		"robot.name":                         *robotName,
		"webhook.tls.crt":                    whCert,
		"webhook.tls.key":                    whKey,
		"images.chart_assignment_controller": robotSetupConfig.Data["chart_assignment_controller_image"],
		"images.cr_syncer":                   robotSetupConfig.Data["cr_syncer_image"],
		"images.metadata_server":             robotSetupConfig.Data["metadata_server_image"],
		"tenant":                             robotSetupConfig.Data["tenant"],
		"tenant_domain":                      robotSetupConfig.Data["tenant_domain"],
		"tenant_main_namespace":              robotSetupConfig.Data["tenant_main_namespace"],
		// This is a bit cumbersome, but the whole array needs to bet set in order to replace two values
		"fluent-bit.extraContainers[0].name":         "logging-proxy",
		"fluent-bit.extraContainers[0].image":        registry + "/" + robotSetupConfig.Data["logging_proxy_image"],
		"fluent-bit.extraContainers[0].env[0].name":  "FLUENTD_HOST",
		"fluent-bit.extraContainers[0].env[0].value": "fluentd." + domain,
		"fluent-bit.extraContainers[0].env[1].name":  "LISTENING_ADDR",
		"fluent-bit.extraContainers[0].env[1].value": "127.0.0.1:8080",
		"fluent-bit.extraContainers[0].env[2].name":  "ENABLE_DEBUG",
		"fluent-bit.extraContainers[0].env[2].value": "false",
		"fluent-bit.extraContainers[0].env[3].name":  "TENANT_NAMESPACE",
		"fluent-bit.extraContainers[0].env[3].value": robotSetupConfig.Data["tenant_main_namespace"],
	})
	log.Printf("Installing %s chart using Synk from %s", name, chartPath)

	output, err := exec.Command(
		helmPath,
		"template",
		"--set-string", vars,
		"--name", name,
		"--namespace", namespace,
		filepath.Join(filesDir, chartPath),
	).CombinedOutput()
	if err != nil {
		log.Fatalf("Synk install of %s failed: %v\nHelm output:\n%s\n", name, err, output)
	}
	cmd := exec.Command(
		synkPath,
		"apply",
		name,
		"-n", namespace,
		"-f", "-",
	)
	// Helm writes the templated manifests and errors alike to stderr.
	// So we can just take the combined output as is.
	cmd.Stdin = bytes.NewReader(output)

	if output, err = cmd.CombinedOutput(); err != nil {
		log.Fatalf("Synk install of %s failed: %v\nSynk output:\n%s\n", name, err, output)
	}
}

// megeMaps returns `base` with `additions` added on top.
// I.e., if the same key is present in both maps, the one from `additions` wins.
func mergeMaps(base, additions map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range additions {
		result[k] = v
	}
	return result
}

func ensureTenantMainNamespace(ctx context.Context, client *kubernetes.Clientset) error {
	_, err := client.CoreV1().Namespaces().Get(ctx, mainNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		var namespace corev1.Namespace
		namespace.Name = mainNamespace
		_, err := client.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
		if err != nil {
			errors.Wrap(err, "create tenant main namespace")
		}
	} else if err != nil {
		errors.Wrap(err, "get tenant main namespace")
	}
	return nil
}

func createOrUpdateRobot(ctx context.Context, k8sDynamicClient dynamic.Interface, labels map[string]string, annotations map[string]string, robotSetupConfig *corev1.ConfigMap) error {
	labels["cloudrobotics.com/robot-name"] = *robotName
	host := os.Getenv("HOST_HOSTNAME")
	if host != "" && labels["cloudrobotics.com/master-host"] == "" {
		labels["cloudrobotics.com/master-host"] = host
	}
	crc_version := robotSetupConfig.Data["setup_robot_crc"]
	if crc_version != "" {
		annotations["cloudrobotics.com/crc-version"] = crc_version
	}

	robotClient := k8sDynamicClient.Resource(robotGVR).Namespace(mainNamespace)
	robot, err := robotClient.Get(ctx, *robotName, metav1.GetOptions{})
	if err != nil {
		if s, ok := err.(*apierrors.StatusError); ok && s.ErrStatus.Reason == metav1.StatusReasonNotFound {
			robot := &unstructured.Unstructured{}
			robot.SetKind("Robot")
			robot.SetAPIVersion("registry.cloudrobotics.com/v1alpha1")
			robot.SetName(*robotName)

			robot.SetLabels(labels)
			robot.SetAnnotations(annotations)
			robot.Object["spec"] = map[string]interface{}{
				"type": *robotType,
			}
			robot.Object["status"] = make(map[string]interface{})
			_, err := robotClient.Create(ctx, robot, metav1.CreateOptions{})
			return err
		} else {
			return fmt.Errorf("failed to get robot %v: %v", *robotName, err)
		}
	}

	// A robot with the same name already exists.
	robot.SetLabels(mergeMaps(robot.GetLabels(), labels))
	robot.SetAnnotations(mergeMaps(robot.GetAnnotations(), annotations))
	spec, ok := robot.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unmarshaling robot failed: spec is not a map")
	}
	spec["type"] = *robotType
	_, err = robotClient.Update(ctx, robot, metav1.UpdateOptions{})
	return err
}

// parseKeyValues splits a string on ',' and the entries on '=' to build a map.
func parseKeyValues(s string) (map[string]string, error) {
	lset := map[string]string{}

	if s == "" {
		return lset, nil
	}

	// To handle escaped commas, we replace them with a sentinel, then
	// restore them after splitting individual values.
	s = strings.ReplaceAll(s, "\\,", commaSentinel)
	for _, l := range strings.Split(s, ",") {
		l = strings.ReplaceAll(l, commaSentinel, ",")
		parts := strings.SplitN(l, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("not a key/value pair")
		}
		lset[parts[0]] = parts[1]
	}
	return lset, nil
}

func createServiceAccountSecret(ctx context.Context, client *kubernetes.Clientset) (*corev1.Secret, error) {
	secretName := fmt.Sprintf("robot-token-%s", *robotName)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   robotConfigNamespace,
			Annotations: map[string]string{"kubernetes.io/service-account.name": "robot-service"},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	secret, err := client.CoreV1().Secrets(robotConfigNamespace).Create(ctx, secret, metav1.CreateOptions{})
	return secret, err
}

func getServiceAccountSecret(ctx context.Context, client *kubernetes.Clientset) (*corev1.Secret, error) {
	secretName := fmt.Sprintf("robot-token-%s", *robotName)
	secret, err := client.CoreV1().Secrets(robotConfigNamespace).Get(ctx, secretName, metav1.GetOptions{})
	return secret, err
}

func syncDockerSecret(ctx context.Context, remoteClient, localClient *kubernetes.Clientset) error {
	secretRemote, err := remoteClient.CoreV1().Secrets(robotConfigNamespace).Get(ctx, coretools.ImagePullSecret, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting docker secret from remote cluster failed: %s", err)
	}

	nsList, err := localClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces in local cluster: %v", err)
	}

	for _, ns := range nsList.Items {

		secretLocal, err := localClient.CoreV1().Secrets(ns.Name).Get(ctx, coretools.ImagePullSecret, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			if ns.Name == baseNamespace {
				newSecret := &corev1.Secret{}
				newSecret.SetNamespace(ns.Name)
				newSecret.SetName(secretRemote.GetName())
				newSecret.Type = secretRemote.Type
				newSecret.Data = secretRemote.Data
				log.Printf("Creating docker pull secret in %s namespace", baseNamespace)
				if _, err := localClient.CoreV1().Secrets(ns.Name).Create(ctx, newSecret, metav1.CreateOptions{}); err != nil {
					return fmt.Errorf("creating docker secret in local cluster failed: %s", err)
				}
				sa := localClient.CoreV1().ServiceAccounts(ns.Name)
				patchData := []byte(`{"imagePullSecrets": [{"name": "` + coretools.ImagePullSecret + `"}]}`)
				return backoff.Retry(
					func() error {
						_, err := sa.Patch(ctx, baseNamespace, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
						if err != nil && !apierrors.IsNotFound(err) {
							return backoff.Permanent(fmt.Errorf("failed to apply %q: %v", patchData, err))
						}
						return err
					},
					backoff.NewConstantBackOff(time.Second),
				)
			}
			continue

		} else if err != nil {
			return fmt.Errorf("getting docker secret from namespace %s of local cluster failed: %s", ns.Name, err)
		}

		secretLocal.Data = secretRemote.Data
		secretLocal.Type = secretRemote.Type
		log.Printf("Updating docker pull secret in namespace %s", ns.Name)
		if _, err = localClient.CoreV1().Secrets(ns.Name).Update(ctx, secretLocal, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("updating docker secret in namespace %s of local cluster failed: %s", ns.Name, err)
		}
	}
	return nil
}

func getTokenFromSecret(secret *corev1.Secret) (*string, error) {
	tokenByte := secret.Data["token"]
	if len(tokenByte) == 0 {
		return nil, fmt.Errorf("no token in secret %s", secret.GetName())
	}
	token := string(tokenByte)
	return &token, nil
}

// Create ServiceAccount secret and wait until the access token was created
func waitForServiceAccountToken(ctx context.Context, client *kubernetes.Clientset, retries uint64) (*string, error) {
	log.Print("Wait for Service Account API token creation in Cloud Cluster")
	_, err := getServiceAccountSecret(ctx, client)
	if apierrors.IsNotFound(err) {
		log.Print("Creating new Service Account API token for robot")
		_, err := createServiceAccountSecret(ctx, client)
		if err != nil {
			return nil, err
		}
	}

	var token *string

	if err := backoff.RetryNotify(
		func() error {
			secret, err := getServiceAccountSecret(ctx, client)
			if err != nil {
				return err
			}
			token, err = getTokenFromSecret(secret)
			return err
		},
		newExponentialBackoff(time.Second, 2, retries),
		func(_ error, _ time.Duration) {
			log.Print("... Retry getting Service Account API token from Cloud Cluster")
		},
	); err != nil {
		return nil, fmt.Errorf("waiting for for Service Account API token creation in Cloud Cluster failed: %v", err)
	}

	return token, nil
}

func getRobotSetupConfigmap(ctx context.Context, client *kubernetes.Clientset, retries uint64) (*corev1.ConfigMap, error) {

	var robotSetup *corev1.ConfigMap

	if err := backoff.RetryNotify(
		func() error {
			var err error
			robotSetup, err = client.CoreV1().ConfigMaps(robotConfigNamespace).Get(ctx, coretools.RobotSetupConfigmap, metav1.GetOptions{})
			return err
		},
		newExponentialBackoff(time.Second, 2, retries),
		func(_ error, _ time.Duration) {
			log.Printf("... Retry getting %s confimap from Cloud Cluster", coretools.RobotSetupConfigmap)
		},
	); err != nil {
		return nil, fmt.Errorf("waiting for for getting confimap %s from Cloud Cluster failed: %v", coretools.RobotSetupConfigmap, err)
	}

	if robotSetup.Data["setup_robot_crc"] == "" ||
		robotSetup.Data["chart_assignment_controller_image"] == "" ||
		robotSetup.Data["cr_syncer_image"] == "" ||
		robotSetup.Data["metadata_server_image"] == "" ||
		robotSetup.Data["logging_proxy_image"] == "" ||
		robotSetup.Data["tenant"] == "" ||
		robotSetup.Data["tenant_main_namespace"] == "" {

		return nil, fmt.Errorf("incomplete ConfigMap %s/%s in cloud cluster. Please check configuration", robotConfigNamespace, coretools.RobotSetupConfigmap)
	}

	return robotSetup, nil
}
