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

# Set cluster config of cloud Kyma K8S cluster

set -e

# Name of Kubernetes image pull secret
image_pull_secret="cloud-robotics-images"

# Directory of this script
dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Include common functions
source "${dir}/common.sh"

# Escapes the input "foo bar" -> "foo\ bar".
function escape {
  sed 's/[^a-zA-Z0-9,._+@%/-]/\\&/g' <<< "$@"
}

# Escapes the input twice "foo bar" -> "foo\\\ bar"
function double_escape {
  sed 's/[^a-zA-Z0-9,._+@%/-]/\\\\\\&/g' <<< "$@"
}

# Asks a yes/no question and returns the mapped input.
function ask_yn {
  local question="$1"
  local default="$2"

  echo
  echo -n "$question"
  if [[ "${default}" = "n" ]]; then
    echo -n " [yN] "
  else
    echo -n " [Yn] "
  fi

  while true; do
    read -n 1 input
    if [[ -z "${input}" ]]; then
      if [[ "${default}" = "n" ]]; then
        return 1
      else
        return 0
      fi
    fi
    echo
    if [[ "${input}" =~ y|Y ]]; then
      return 0
    elif [[ "${input}" =~ n|N ]]; then
      return 1
    fi
    echo -n "Please answer with 'y' or 'n'. "
  done
}

# Reads a variable from user input.
function read_variable {
  local target_var="$1"
  local question="$2"
  local default="$3"

  echo
  echo "${question}"
  if [[ -n "${default}" ]]; then
    echo -n "[ENTER] for \"${default}\": "
  fi
  read -er input

  if [[ -z "${input}" ]]; then
    # shellcheck disable=SC2046
    eval ${target_var}=$( escape ${default} )
  else
    # shellcheck disable=SC2046
    eval ${target_var}=$( escape ${input} )
  fi
}

# Outputs the variable to the user.
function print_variable {
  local description="$1"
  local value="$2"

  if [[ -n "${value}" ]]; then
    echo "${description}: ${value}"
  fi
}

function get_domain_ingress {
  domain="<no value>"
  k8s_gateway_tls="<no value>"
  until [[ "$domain" != "<no value>" ]] && [[ "$k8s_gateway_tls" != "<no value>" ]]; do
    IFS=/ read -r ns name <<< ${default_gateway}
    until kc get gateways.networking.istio.io -n ${ns} ${name} &>/dev/null; do
      read_variable default_gateway "Gateway ${default_gateway} not found. Please enter a new Istio Gateway you would like use in the form <namespace>/<name>" $default_gateway
      IFS=/ read -r ns name <<< ${default_gateway}
    done
    # Using first host of first server from default istio gateway as default domain eventually removing the subdomain wildcard
    complete_domain=$(kc get gateways.networking.istio.io -n ${ns} ${name} -o=go-template --template='{{index (index .spec.servers 0).hosts 0}}')
    domain=$(echo $complete_domain | sed 's/*.//')
    if [[ "$domain" != "$complete_domain" ]]; then
      # Case domain with wildcard - assuming that there is a valid TLS wildcard certificate too
      k8s_gateway_tls=$(kc get gateways.networking.istio.io -n ${ns} ${name} -o=go-template --template='{{(index .spec.servers 0).tls.credentialName }}')
    else
      # Case domain without wildcard - use an own TLS secret for k8s endpoint
      k8s_gateway_tls=k8s-endpoint-tls
      # remove the first subdomain
      domain=$(echo $domain | sed 's/^[^.]*.//')
    fi
    if [[ "$domain" == "<no value>" ]] || [[ "$k8s_gateway_tls" == "<no value>" ]]; then
      read_variable default_gateway "Domain or TLS secret of the Gateway not found. Please enter a new Istio Gateway you would like use in the form <namespace>/<name>" $default_gateway
    fi
  done
  # Using first IP from istio ingress gateway as default.
  ingress_ip=$(kc get svc -n istio-system istio-ingressgateway -o=go-template --template='{{(index .status.loadBalancer.ingress 0).ip}}')
  if [[ $ingress_ip == "<no value>" ]]; then
    if ! which dig &>/dev/null; then
      die "dig not found, please install it"
    fi
    ingress_host=$(kc get svc -n istio-system istio-ingressgateway -o=go-template --template='{{(index .status.loadBalancer.ingress 0).hostname}}')
    ingress_ip=$(dig ${ingress_host} +short)
  fi
}

function get_default_vars {
  ask_docker_yn=n
  if kc get configmap -n $namespace cloud-robotics-core-config &>/dev/null; then
    if ! ask_yn "There is an existing configuration. Do you want to overwrite it?" "n"; then
      exit 0
    fi
    get_cluster_config $namespace
    docker_registry=$registry
  else
    ask_docker_yn=y
    default_gateway="kyma-system/kyma-gateway"
    deploy_environment="GCP"
    docker_registry=""
    if cat ${dir}/../.REGISTRY &>/dev/null; then
      docker_registry=$(cat ${dir}/../.REGISTRY)
    fi
  fi

  if [[ "${cloud_logging}" == "true" ]]; then
    cloud_logging_yn=y
  else
    cloud_logging_yn=n
  fi

  if [[ "${stackdriver_logging}" == "true" ]]; then
    stackdriver_logging_yn=y
    ask_stackdriver_sa_yn=n
  else
    stackdriver_logging_yn=n
    ask_stackdriver_sa_yn=y
  fi
  stackdriver_sa="<tbd>"
  

  if [[ "${k8s_service_catalog}" == "true" ]]; then
    k8s_service_catalog_yn=y
  else
    k8s_service_catalog_yn=n
  fi

  docker_user="<tbd>"
  docker_password="<tbd>"
  docker_email="<tbd>"
}

function read_configuration {
  echo "#######################################"
  echo "Please enter cluster configuration"
  echo "#######################################"
  echo

  get_domain_ingress

  read_variable domain "Enter the domain name of your cluster. Using first host of first server from Istio Gateway ${default_gateway} as default removing the subdomain wildcard" $domain

  read_variable ingress_ip "Enter ingress IP of your cluster. Using first IP from istio ingress gateway as default." $ingress_ip
 
  read_variable deploy_environment "Please enter the deploy environment of your Kyma cluster. Valid options are AWS/Azure/GCP" $deploy_environment

  until [[ "$deploy_environment" == "AWS" ]] || [[ "$deploy_environment" == "Azure" ]] || [[ "$deploy_environment" == "GCP" ]]; do
    echo "Deploy environment ${deploy_environment} is invalid"
    read_variable deploy_environment "Please enter the deploy environment of your Kyma cluster. Valid options are AWS/Azure/GCP" $deploy_environment
  done

  if ask_yn "Should SAP Cloud Logging be enabled?" "${cloud_logging_yn}"; then
    cloud_logging=true
  else
    cloud_logging=false
  fi

  if ask_yn "Should Stackdriver logging be enabled?" "${stackdriver_logging_yn}"; then
    stackdriver_logging=true
    if ask_yn "Would you like to provide a new stackdriver service account?" "${ask_stackdriver_sa_yn}"; then
      ask_stackdriver_sa_yn=y
    fi
  else
    stackdriver_logging=false
    ask_stackdriver_sa_yn=n
  fi

  if [[ "$ask_stackdriver_sa_yn" == "y" ]]; then
    read_variable stackdriver_sa "Please enter your stackdriver service account json key (the whole input must fit into one line)" $stackdriver_sa
  fi 
  

  if ask_yn "Should K8S service catalog should be enabled? (for user authentication with SAP BTP Services)" "${k8s_service_catalog_yn}"; then
    k8s_service_catalog=true
  else
    k8s_service_catalog=false
  fi

  read_variable docker_registry "Please enter the docker registry url for this cluster" $docker_registry

  if ask_yn "Would you like to provide a new docker user?" "${ask_docker_yn}"; then
    ask_docker_yn=y
  fi

  if [[ "$ask_docker_yn" == "y" ]]; then
    read_variable docker_user "Please enter the user for the docker registry" $docker_user

    read_variable docker_password "Please enter the password for the docker registry" $docker_password

    read_variable docker_email "Please enter the email address of the docker user" $docker_email
  fi 

  echo
  echo "#######################################"
  echo "This is your configuration"
  echo "#######################################"
  print_variable "Istio Gateway" $default_gateway
  print_variable "TLS secret for K8S API Gateway" $k8s_gateway_tls
  print_variable "Cluster domain" $domain
  print_variable "Ingress IP" $ingress_ip
  print_variable "Deploy environment" $deploy_environment
  print_variable "Use SAP Cloud Logging" $cloud_logging
  print_variable "Use Stackdriver Logging" $stackdriver_logging
  print_variable "Use K8S service catalog" $k8s_service_catalog
  print_variable "Docker registry" $docker_registry
  if [ "${docker_user}" != "<tbd>" ]; then 
    print_variable "Docker user" $docker_user
    print_variable "Docker password" "***"
    print_variable "Docker user email" $docker_email
  fi
}

function save_configuration {
  values=$(cat <<EOF
    --set-string domain=${domain}
    --set-string ingress_ip=${ingress_ip}
    --set-string deploy_environment=${deploy_environment}
    --set-string registry=${docker_registry}
    --set-string cloud_logging=${cloud_logging}
    --set-string stackdriver_logging=${stackdriver_logging}
    --set-string default_gateway=${default_gateway}
    --set-string k8s_gateway_tls=${k8s_gateway_tls}
    --set-string k8s_service_catalog=${k8s_service_catalog}
EOF
)

  if $helm upgrade --install --atomic --create-namespace --namespace $namespace $values setup-cloud ${dir}/../charts/setup-cloud; then
    echo "Successfully saved your cluster config"
    echo
    if ! cat ${dir}/../.REGISTRY &>/dev/null; then
      echo "Writing registry URL to .REGISTRY"
      echo $docker_registry > "${dir}/../.REGISTRY"
    fi
  else
    die "Saving cluster config failed"
  fi
}

function create_image_pull_secret {
  echo "Create image pull secret"
  kc create secret -n $namespace docker-registry $image_pull_secret \
    --docker-server=${docker_registry} \
    --docker-username=${docker_user} \
    --docker-password="${docker_password}" \
    --docker-email=${docker_email} \
    --dry-run=client -o yaml | kc apply -f -
  echo
  echo "Add image pull secret to default serviceaccount"
  kc patch serviceaccount -n $namespace default -p '{"imagePullSecrets": [{"name": "'${image_pull_secret}'"}]}'
  echo
}

function create_stackdriver_sa {
  echo "create stackdriver service account"
  kc create secret -n $namespace generic stackdriver-service-account \
    --from-literal=service-account-file.json="${stackdriver_sa}" \
    --dry-run=client -o yaml | kc apply -f -
}

# main
if [[ "$#" -lt 2 ]] ; then
  die "Usage: $0 <path to kubeconfig> <namespace>"
fi

# setup
configure_tooling "$1"
namespace=$2

get_default_vars

read_configuration

if ! ask_yn "Would you like to save this configuration?"; then
  exit 0
fi
echo

save_configuration

if [[ "${docker_user}" != "<tbd>" ]]; then
  create_image_pull_secret
fi

if [[ "${stackdriver_sa}" != "<tbd>" ]]; then
  create_stackdriver_sa
fi
