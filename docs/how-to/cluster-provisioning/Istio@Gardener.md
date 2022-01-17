# Istio@Gardener

## Get Gardener cluster

## Set env for "shoot domain" of your cluster
You find it on "OVERVIEW" tab in your cluster details of your Gardener Dashboard
```
DOMAIN=<your gardener domain>
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
      k8s:
        serviceAnnotations:
          cert.gardener.cloud/purpose: managed
          cert.gardener.cloud/secretname: wildcard-tls
          dns.gardener.cloud/class: garden
          dns.gardener.cloud/dnsnames: "*.${DOMAIN}"
          dns.gardener.cloud/ttl: "120"
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

## Create Istio Gateway

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
    - "*.${DOMAIN}"
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
      credentialName: wildcard-tls
      minProtocolVersion: TLSV1_2
      mode: SIMPLE
  - hosts:
    - "*.${DOMAIN}"
    port:
      name: http
      number: 80
      protocol: HTTP
    tls:
      httpsRedirect: true
EOF
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


## Deploy cloud-robotics

[Deploying Cloud Robotics from sources](../deploy-from-sources.md)
