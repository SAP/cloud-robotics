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

# Deploy core services in a K8S cluster

set -e

# Kubernetes namespace used for deployment of core services
core_namespace="default"
robot_config_namespace="robot-config"

# Directory of this script
dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Include common functions
source "${dir}/common.sh"

function construct_image_string {
    local registry=$1
    local image_name=$2
    local version=$3
    echo ${registry}"/"${image_name}":"${version}
}

function get_core_container_digests {
  local effective_version
  if [[ "$registry" == "$default_docker_registry" ]] ; then
    effective_version="latest"
    crc_version="crc-bin/crc-bin+latest"
  else
    local version
    version=$(cat ${dir}/../VERSION)
    local sha
    if [[ -d $dir/../.git ]]; then
      sha=$(git rev-parse --short HEAD)
    else
      die "no git dir SHA is unknown"
    fi
    effective_version=${version}-${sha}

    crc_version="crc-${version}/crc-${version}+${sha}"
  fi
  echo "Using version '"${effective_version}"' of core images"
  local image
  # app-auth-proxy
  image=$(construct_image_string ${registry} app-auth-proxy ${effective_version})
  echo "Getting digest for '${image}'"
  app_auth_proxy_digest=$(get_container_digest ${image})
  # app-rollout-controller
  image=$(construct_image_string ${registry} app-rollout-controller ${effective_version})
  echo "Getting digest for '${image}'"
  app_rollout_controller_digest=$(get_container_digest ${image})
  # chart-assignment-controller
  image=$(construct_image_string ${registry} chart-assignment-controller ${effective_version})
  echo "Getting digest for '${image}'"
  chart_assignment_controller_digest=$(get_container_digest ${image})
  # http-relay-client
  image=$(construct_image_string ${registry} http-relay-client ${effective_version})
  echo "Getting digest for '${image}'"
  http_relay_client_digest=$(get_container_digest ${image})
  # http-relay-server
  image=$(construct_image_string ${registry} http-relay-server ${effective_version})
  echo "Getting digest for '${image}'"
  http_relay_server_digest=$(get_container_digest ${image})
  # setup-robot
  image=$(construct_image_string ${registry} setup-robot ${effective_version})
  echo "Getting digest for '${image}'"
  setup_robot_digest=$(get_container_digest ${image})
  # cr-syncer
  image=$(construct_image_string ${registry} cr-syncer ${effective_version})
  echo "Getting digest for '${image}'"
  cr_syncer_digest=$(get_container_digest ${image})
  # metadata-server
  image=$(construct_image_string ${registry} metadata-server ${effective_version})
  echo "Getting digest for '${image}'"
  metadata_server_digest=$(get_container_digest ${image})
  # logging-proxy
  image=$(construct_image_string ${registry} logging-proxy ${effective_version})
  echo "Getting digest for '${image}'"
  logging_proxy_digest=$(get_container_digest ${image})
  # tenant-controller
  image=$(construct_image_string ${registry} tenant-controller ${effective_version})
  echo "Getting digest for '${image}'"
  tenant_controller_digest=$(get_container_digest ${image})
}

function get_container_digest {
  local image=$1
  local image_untagged=$(echo $image | sed 's/\(.*\):.*/\1/')
  if ! docker pull $image >/dev/null; then
    die "Error pulling image "${image}
  fi
  local digest=$(docker image inspect $image -f '{{range $r:=.RepoDigests}}{{println $r}}{{end}}' | grep $image_untagged | sed 's/.*\///')
  echo $digest
}

function helm_charts {

  # Check if SAP Cloud Logging is enabled
  if [[ "${cloud_logging}" == "true" ]]; then
    i=0
    until kc get secrets -n $core_namespace cloud-logging-core &>/dev/null; do
      instance_state=$(kc get serviceinstance.servicecatalog.k8s.io -n $core_namespace cloud-logging-core -o=go-template --template='{{.status.lastConditionState}}')
      if [[ "${instance_state}" == "Provisioning" ]]; then
        if [[ $(($i % 12)) == 0 ]]; then
          echo "Waiting for provisioning of SAP Cloud Logging. This may take while...(It is safe to cancel and try again later too)"
        fi
        i=$((i + 1))
      elif [[ "${instance_state}" == "ProvisionCallFailed" ]]; then
        echo "Error during provisioning of SAP Cloud Logging."
        kc get serviceinstance.servicecatalog.k8s.io -n $core_namespace cloud-logging-core -o yaml
        kc get servicebinding.servicecatalog.k8s.io -n $core_namespace cloud-logging-core -o yaml
        die "Provisioning of SAP Cloud Logging failed. If you cannot fix it, please deactivate SAP Cloud Logging in configuration to continue."
      elif [[ "${instance_state}" == "Ready" ]]; then
        echo "Provisioning of SAP Cloud Logging completed. The process should continue soon."
      else
        kc get serviceinstance.servicecatalog.k8s.io -n $core_namespace cloud-logging-core -o yaml
        kc get servicebinding.servicecatalog.k8s.io -n $core_namespace cloud-logging-core -o yaml
        die "Unexpected state during provisioning of SAP Cloud Logging"        
      fi
      sleep 5
    done
  
    cloudLoggingFluentdEndpoint=$(kc get secrets -n $core_namespace cloud-logging-core -o=go-template --template='{{index .data "Fluentd-endpoint"}}' | base64 -d )
    cloudLoggingFluentdUser=$(kc get secrets -n $core_namespace cloud-logging-core -o=go-template --template='{{index .data "Fluentd-username"}}' | base64 -d )
    cloudLoggingFluentdPassword=$(kc get secrets -n $core_namespace cloud-logging-core -o=go-template --template='{{index .data "Fluentd-password"}}' | base64 -d )

  fi

  # Create robot_config_namespace namespace if not exists yet
  if ! kc get namespace $robot_config_namespace &>/dev/null; then
    kc create namespace $robot_config_namespace
  fi

  # Test if Gardener dnsentry CRD is available in the cluster
  if kc get crds dnsentries.dns.gardener.cloud &>/dev/null; then
    echo "Gardener detected - Enabling tenant specific Istio Gateways"
    tenant_specific_gateways="true"
  else
    echo "Gardener not detected - Disabling tenant specific Istio Gateways"
    tenant_specific_gateways="false"
  fi

  if [[ "$k8s_service_catalog" == "true" ]]; then
    # Test if K8S service catalog is available in the cluster
    if kc get crds servicebindingusages.servicecatalog.kyma-project.io &>/dev/null && kc get crds serviceinstances.servicecatalog.k8s.io &>/dev/null; then
      echo "Enabling K8S service catalog"
      k8s_service_catalog="true"
    else
      echo "K8S service catalog CRDs not found - continuing with disabled catalog"
      k8s_service_catalog="false"
    fi
  fi

  $synk init
  echo "synk init done"

  # Prepare helm values
  values=$(cat <<EOF
    --set-string domain=${domain}
    --set-string ingress_ip=${ingress_ip}
    --set-string deploy_environment=${deploy_environment}
    --set-string registry=${registry}
    --set-string public_registry=${public_registry}
    --set-string certificate_authority.key=${ca_key}
    --set-string certificate_authority.crt=${ca_crt}
    --set-string setup_robot_crc=${crc_version}
    --set-string default_gateway=${default_gateway}
    --set-string k8s_gateway_tls=${k8s_gateway_tls}
    --set-string tenant_specific_gateways=${tenant_specific_gateways}
    --set-string k8s_service_catalog=${k8s_service_catalog}
    --set-string cloud_logging=${cloud_logging}
    --set-string stackdriver_logging=${stackdriver_logging}
    --set-string cloudLoggingFluentdEndpoint=${cloudLoggingFluentdEndpoint}
    --set-string cloudLoggingFluentdUser=${cloudLoggingFluentdUser}
    --set-string cloudLoggingFluentdPassword=${cloudLoggingFluentdPassword}
    --set-string images.setup_robot_image=${setup_robot_digest}
    --set-string images.app_rollout_controller=${app_rollout_controller_digest}
    --set-string images.http_relay_client=${http_relay_client_digest}
    --set-string images.http_relay_server=${http_relay_server_digest}
    --set-string images.app_auth_proxy=${app_auth_proxy_digest}
    --set-string images.chart_assignment_controller=${chart_assignment_controller_digest}
    --set-string images.cr_syncer=${cr_syncer_digest}
    --set-string images.metadata_server=${metadata_server_digest}
    --set-string images.logging_proxy=${logging_proxy_digest}
    --set-string images.tenant_controller=${tenant_controller_digest}
EOF
)

  # CRDs are installed into default namespace
  echo "installing base-cloud to ${kube_config}..."
  $helm lint ${dir}/../charts/base-cloud && \
  $helm template --namespace $core_namespace $values base-cloud ${dir}/../charts/base-cloud \
    | $synk apply base-cloud -n $core_namespace -f - \
    || die "Synk failed for base-cloud"

  if [[ "${cloud_logging}" == "true" ]]; then
    kc rollout restart deployment/fluentd
  fi
  if [[ "${stackdriver_logging}" == "true" ]]; then
    kc rollout restart deployment/fluentd-gcp
  fi

  echo "installing robot-config to ${kube_config}..."
  $helm lint ${dir}/../charts/robot-config-cloud && \
  $helm template --namespace $robot_config_namespace $values robot-config-cloud ${dir}/../charts/robot-config-cloud \
    | $synk apply robot-config-cloud -n $robot_config_namespace -f - \
    || die "Synk failed for robot-config"

  # Platform apps are App CRDs, thus always install them into default namespace
  echo "installing platform-apps to ${kube_config}..."
  $helm lint ${dir}/../charts/platform-apps && \
  $helm template --namespace default $values platform-apps ${dir}/../charts/platform-apps \
    | $synk apply platform-apps -n default -f - \
    || die "Synk failed for platform-apps"
}

function create_default_tenant {
  # Create default tenant if there is no tenant
  if ! kc get tenants -o=go-template --template='{{ (index .items 0) }}' &>/dev/null; then
    echo "Creating default tenant"
    i=0
    until [[ $(kc get pods -l app=tenant-controller -o=go-template --template='{{ (index .items 0).status.phase }}') == "Running" ]]; do
      if [[ $(($i % 12)) == 0 ]]; then
        echo "Waiting until tenant-controller is running. This may take while..."
      fi
      i=$((i + 1))
      sleep 5
    done
    cat <<EOF | kc apply -f -
apiVersion: config.cloudrobotics.com/v1alpha1
kind: Tenant
metadata:
  name: default
EOF
  fi
}

# commands
function set_config {
  configure_tooling $1
  ${dir}/set-cloud-cluster-config.sh $kube_config $core_namespace
}

function create {
  configure_tooling $1
  get_cluster_config $core_namespace
  get_core_container_digests
  helm_charts
  create_default_tenant
}

# Alias for create.
function update {
  create $1
}

# main
if [[ "$#" -lt 1 ]] || [[ ! "$1" =~ ^(set_config|create|delete|update)$ ]]; then
  die "Usage: $0 {set_config|create|delete|update} <path to kubeconfig>"
fi

echo "#######################################"
echo "Starting. Cluster must be online"
echo "#######################################"
echo

# call arguments verbatim:
"$@"