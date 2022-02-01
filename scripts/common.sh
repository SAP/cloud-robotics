#!/usr/bin/env bash
#
# Copyright 2021 The Cloud Robotics Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Common functions for scripts

# Name of config map used to store cluster config. Must be the same used in /charts/setup-cloud/templates/cloud-robotics-core-config.yaml
core_config_map="cloud-robotics-core-config"

# Public cloud robotics docker registry
default_docker_registry="ghcr.io/sap/cloud-robotics"

# Name of Kubernetes image pull secret
image_pull_secret="cloud-robotics-images"

function die {
  echo
  echo "$1" >&2
  exit 1
}

# Directory of this script
scripts_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Current platform
platform=$(uname -s | tr '[:upper:]' '[:lower:]')

# Check if tools are available
if ! which kubectl &>/dev/null; then
  die "kubectl not found, please install it"
fi
if ! which helm &>/dev/null; then
  die "helm not found, please install it"
fi
if ! which docker &>/dev/null; then
  die "docker not found, please install it"
fi
if ! which git &>/dev/null; then
  die "git not found, please install it"
fi

function configure_tooling {
  kube_config="$1"

  if [ "$kube_config" == "" ]; then
    die "empty kubeconfig is not allowed"
  fi

  helm_command="$(which helm)"
  synk_command="${scripts_dir}/../bin/synk_${platform}_amd64"

  helm="${helm_command} --kubeconfig ${kube_config}"
  synk="${synk_command} --kubeconfig ${kube_config}"
}

function kc {
  kubectl --kubeconfig="${kube_config}" "$@"
}

function get_cluster_config {
  local ns=$1
  if ! kc get configmap -n $ns $core_config_map &>/dev/null; then
    die "No cluster config found. Please ensure that cluster is online and './deploy.sh set_config ...' ran successfully"
  fi

  echo "Getting config from cluster"
  deploy_environment=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "deploy_environment"}}')
  domain=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "domain"}}')
  ingress_ip=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "ingress_ip"}}')
  registry=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "registry"}}')
  public_registry=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "public_registry"}}')
  cloud_logging=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "cloud_logging"}}')
  stackdriver_logging=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "stackdriver_logging"}}')
  default_gateway=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "default_gateway"}}')
  k8s_gateway_tls=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "k8s_gateway_tls"}}')
  k8s_service_catalog=$(kc get configmap -n $ns $core_config_map -o=go-template --template='{{index .data "k8s_service_catalog"}}')
  echo "Cluster config received"

  if ! kc get secret -n $ns cluster-authority &>/dev/null; then
    die "Cluster certificate authority not found. Please run ,/deploy.sh set_config again"
  fi
  
  echo "Getting certificate authority from cluster"
  ca_crt=$(kc get secret -n $ns cluster-authority -o=go-template --template='{{index .data "tls.crt"}}')
  ca_key=$(kc get secret -n $ns cluster-authority -o=go-template --template='{{index .data "tls.key"}}')
  echo "Cluster certificate authority received"

}
