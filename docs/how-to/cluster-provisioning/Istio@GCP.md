## Set env for your  your GCP project

```
GCP_PROJECT=<my GCP project>
```


## Create GKE Cluster assuming you have set your favorite compute/zone as default in gcloud CLI

```
gcloud beta container --project "${GCP_PROJECT}" clusters create "cloud-robotics" --cluster-version "latest" --release-channel "regular" --machine-type "e2-standard-4" --scopes "https://www.googleapis.com/auth/devstorage.read_only","https://www.googleapis.com/auth/logging.write","https://www.googleapis.com/auth/monitoring","https://www.googleapis.com/auth/servicecontrol","https://www.googleapis.com/auth/service.management.readonly","https://www.googleapis.com/auth/trace.append" --num-nodes "2" --enable-autoscaling --min-nodes "2" --max-nodes "10" --enable-autoupgrade --enable-autorepair --max-surge-upgrade 1 --max-unavailable-upgrade 0 --enable-shielded-nodes
```

```
gcloud beta container --project "${GCP_PROJECT}" clusters create "cloud-robotics" --no-enable-basic-auth --cluster-version "latest" --release-channel "regular" --machine-type "e2-standard-4" --image-type "COS_CONTAINERD" --disk-type "pd-standard" --disk-size "100" --metadata disable-legacy-endpoints=true --scopes "https://www.googleapis.com/auth/devstorage.read_only","https://www.googleapis.com/auth/logging.write","https://www.googleapis.com/auth/monitoring","https://www.googleapis.com/auth/servicecontrol","https://www.googleapis.com/auth/service.management.readonly","https://www.googleapis.com/auth/trace.append" --max-pods-per-node "110" --num-nodes "1" --logging=SYSTEM,WORKLOAD --monitoring=SYSTEM --enable-ip-alias --no-enable-intra-node-visibility --enable-autoscaling --min-nodes "1" --max-nodes "3" --no-enable-master-authorized-networks --addons HorizontalPodAutoscaling,HttpLoadBalancing,GcePersistentDiskCsiDriver --enable-autoupgrade --enable-autorepair --max-surge-upgrade 1 --max-unavailable-upgrade 0 --enable-shielded-nodes
```


## Install Istio with default profile
https://istio.io/latest/docs/setup/install/istioctl/#install-istio-using-the-default-profile

Istio sidecar-injection and mTLS are enabled by default on every namespace. It is closest to our configuration in Kyma, but it should work with different configurations too.

**Not working with Istio 1.12 & 1.12.1 because of a bug in istioctl**
https://github.com/istio/istio/pull/36260

```
istioctl install -y -f - <<EOF
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  profile: default
  meshConfig:
    defaultConfig:
      holdApplicationUntilProxyStarts: true
  components:
    ingressGateways:
    - name: istio-ingressgateway
      enabled: true
  values:
    sidecarInjectorWebhook:
      enableNamespacesByDefault: true
EOF
```


## Enable mTLS everywhere
https://istio.io/latest/docs/tasks/security/authentication/mtls-migration/#lock-down-mutual-tls-for-the-entire-mesh

```
kubectl apply -n istio-system -f - <<EOF
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: "default"
spec:
  mtls:
    mode: STRICT
EOF
```


## Get IP of loadbalancer service

```
INGRESS_IP=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```


## Create endpoint definitions for the cloud-robotics cluster

```
cat <<EOF > cloud-robotics-www-endpoint.yaml
swagger: "2.0"
info:
  version: "1.0.0"
  title: Cloud Robotics www endpoint
  description: Cloud Robotics www endpoint with bare minimum Swagger spec
host: www.endpoints.${GCP_PROJECT}.cloud.goog
x-google-endpoints:
    - name: "www.endpoints.${GCP_PROJECT}.cloud.goog"
      target: "${INGRESS_IP}"
paths:
  /:
    get:
      operationId: getAll
      responses:
        "200":
          description:  OK
EOF
cat <<EOF > cloud-robotics-k8s-endpoint.yaml
swagger: "2.0"
info:
  version: "1.0.0"
  title: Cloud Robotics k8s endpoint
  description: Cloud Robotics k8s endpoint with bare minimum Swagger spec
host: k8s.endpoints.${GCP_PROJECT}.cloud.goog
x-google-endpoints:
    - name: "k8s.endpoints.${GCP_PROJECT}.cloud.goog"
      target: "${INGRESS_IP}"
paths:
  /:
    get:
      operationId: getAll
      responses:
        "200":
          description:  OK
EOF
cat <<EOF > cloud-robotics-setup-robot-endpoint.yaml
swagger: "2.0"
info:
  version: "1.0.0"
  title: Cloud Robotics setup-robot endpoint
  description: Cloud Robotics setup-robot endpoint with bare minimum Swagger spec
host: setup-robot.endpoints.${GCP_PROJECT}.cloud.goog
x-google-endpoints:
    - name: "setup-robot.endpoints.${GCP_PROJECT}.cloud.goog"
      target: "${INGRESS_IP}"
paths:
  /:
    get:
      operationId: getAll
      responses:
        "200":
          description:  OK
EOF
cat <<EOF > cloud-robotics-default-prom-endpoint.yaml
swagger: "2.0"
info:
  version: "1.0.0"
  title: Cloud Robotics prometheus endpoint
  description: Cloud Robotics prometheus endpoint with bare minimum Swagger spec
host: default-prom.endpoints.${GCP_PROJECT}.cloud.goog
x-google-endpoints:
    - name: "default-prom.endpoints.${GCP_PROJECT}.cloud.goog"
      target: "${INGRESS_IP}"
paths:
  /:
    get:
      operationId: getAll
      responses:
        "200":
          description:  OK
EOF
cat <<EOF > cloud-robotics-fluentd-endpoint.yaml
swagger: "2.0"
info:
  version: "1.0.0"
  title: Cloud Robotics fluentd endpoint
  description: Cloud Robotics prometheus endpoint with bare minimum Swagger spec
host: fluentd.endpoints.${GCP_PROJECT}.cloud.goog
x-google-endpoints:
    - name: "fluentd.endpoints.${GCP_PROJECT}.cloud.goog"
      target: "${INGRESS_IP}"
paths:
  /:
    get:
      operationId: getAll
      responses:
        "200":
          description:  OK
EOF
```


## Deploy GCP endpoints
```
gcloud endpoints services deploy --project "${GCP_PROJECT}" cloud-robotics-www-endpoint.yaml
gcloud endpoints services deploy --project "${GCP_PROJECT}" cloud-robotics-k8s-endpoint.yaml
gcloud endpoints services deploy --project "${GCP_PROJECT}" cloud-robotics-setup-robot-endpoint.yaml
gcloud endpoints services deploy --project "${GCP_PROJECT}" cloud-robotics-default-prom-endpoint.yaml
gcloud endpoints services deploy --project "${GCP_PROJECT}" cloud-robotics-fluentd-endpoint.yaml
```

## Create Istio Gateway
SSL wont work until we finish the cloud-robotics deployment. We could use its cert-manager deployment to get letsencrypt certificates

```
kubectl apply -n istio-system -f - <<EOF
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: istio-gateway
spec:
  selector:
    app: istio-ingressgateway
    istio: ingressgateway
  servers:
  - hosts:
    - "www.endpoints.${GCP_PROJECT}.cloud.goog"
    - "setup-robot.endpoints.${GCP_PROJECT}.cloud.goog"
    - "default-prom.endpoints.${GCP_PROJECT}.cloud.goog"
    - "fluentd.endpoints.${GCP_PROJECT}.cloud.goog"
    port:
      name: https
      number: 443
      protocol: HTTPS
    tls:
      cipherSuites:
      - ECDHE-RSA-CHACHA20-POLY1305
      - ECDHE-RSA-AES256-GCM-SHA384
      - ECDHE-RSA-AES256-SHA
      - ECDHE-RSA-AES128-GCM-SHA256
      - ECDHE-RSA-AES128-SHA
      credentialName: www-endpoint-tls
      minProtocolVersion: TLSV1_2
      mode: SIMPLE
  - hosts:
    - "www.endpoints.${GCP_PROJECT}.cloud.goog"
    - "setup-robot.endpoints.${GCP_PROJECT}.cloud.goog"
    - "default-prom.endpoints.${GCP_PROJECT}.cloud.goog"
    - "fluentd.endpoints.${GCP_PROJECT}.cloud.goog"
    port:
      name: http
      number: 80
      protocol: HTTP
    tls:
      httpsRedirect: true
EOF
```


## Deploy cloud-robotics

[Deploying Cloud Robotics from sources](../deploy-from-sources.md)


## Create ingressclass for cert-manager
```
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: istio
spec:
  controller: istio.io/ingress-controller
EOF
```

## Set email for cert-manager cluster issuer
```
EMAIL=<your email>
```

## Create cert-manager cluster issuer
```
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    email: ${EMAIL}
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-secret
    solvers:
    - http01:
        ingress:
          class: istio
EOF
```

## Create TLS certificates
```
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: www-endpoint-tls
  namespace: istio-system
spec:
  secretName: www-endpoint-tls
  commonName: www.endpoints.${GCP_PROJECT}.cloud.goog
  dnsNames:
    - www.endpoints.${GCP_PROJECT}.cloud.goog
    - setup-robot.endpoints.${GCP_PROJECT}.cloud.goog
    - default-prom.endpoints.${GCP_PROJECT}.cloud.goog
    - fluentd.endpoints.${GCP_PROJECT}.cloud.goog
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: k8s-endpoint-tls
  namespace: istio-system
spec:
  secretName: k8s-endpoint-tls
  commonName: k8s.endpoints.${GCP_PROJECT}.cloud.goog
  dnsNames:
    - k8s.endpoints.${GCP_PROJECT}.cloud.goog
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
EOF
```

## Check if certificates are ready.
```
kubectl get certificates.cert-manager.io -n istio-system
```

## Create prometheus CRDs

This is optional in case you would like to use the provided kube-prometheus-stack app. Prometheus CRDs are not created automatically to avoid conflicts with potentially existing prometheus installations.

```
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-alertmanagerconfigs.yaml
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-alertmanagers.yaml
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-podmonitors.yaml
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-probes.yaml
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-prometheuses.yaml
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-prometheusrules.yaml
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-servicemonitors.yaml
kubectl create -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-30.0.1/charts/kube-prometheus-stack/crds/crd-thanosrulers.yaml
```


Now you can continue to add robots.