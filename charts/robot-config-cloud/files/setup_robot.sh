#!/bin/bash
#
# Copyright 2019 The Cloud Robotics Authors
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

# This script is a convenience wrapper for starting the setup-robot container, i.e., for doing
# "kubectl run ... --image=...setup-robot...".

set -e
set -o pipefail

DOMAIN=""

function kc {
  kubectl --context="${KUBE_CONTEXT}" "$@"
}

function faketty {
  # Run command inside a TTY.
  script -qfec "$(printf "%q " "$@")" /dev/null
}

# Extract the cloud cluster domain and tenant from the command-line args. It is required to identify the reference for the
# setup-robot image. This is challenging as --domain and --tenant is an option, so we have to do some
# rudimentary CLI parameter parsing.
for i in $(seq 1 $#) ; do
  if [[ "${!i}" == "--domain" ]] ; then
    j=$((i+1))
    DOMAIN=${!j}
  fi
  if [[ "${!i}" == "--tenant" ]] ; then
    j=$((i+1))
    TENANT=${!j}
  fi
done

if [[ -z "$DOMAIN" ]] ; then
  echo "ERROR: --domain <cloud-cluster-domain> is required" >&2
  exit 1
fi

if [[ -z "$TENANT" ]] ; then
  echo "ERROR: --tenant <cloud-cluster-tenant> is required" >&2
  exit 1
fi

# Add domain and tenant command line args from default value if not set explicitly
if [[ ! "$*" =~ "--domain"  ]] ; then
  set "$@" --domain "$DOMAIN"
fi
if [[ ! "$*" =~ "--tenant"  ]] ; then
  set "$@" --tenant "$TENANT"
fi

if [[ -z "${KUBE_CONTEXT}" ]] ; then
  KUBE_CONTEXT=kubernetes-admin@kubernetes
fi

if [[ -n "$ACCESS_TOKEN_FILE" ]]; then
  ACCESS_TOKEN=$(cat ${ACCESS_TOKEN_FILE})
fi

ROBOT_CONFIG_NS=t-${TENANT}-robot-config
if [[ "$TENANT" == "default" ]]; then
  ROBOT_CONFIG_NS=robot-config
fi

if [[ -z "$ACCESS_TOKEN" ]]; then
  echo "Enter API token for cloud-robotics-setup-robot service account."
  echo "You could run the following kubectl on your local machine towards your cloud cluster:"
  echo "  kubectl get secrets -n ${ROBOT_CONFIG_NS} \ "
  echo "    \$(kubectl get serviceaccounts -n ${ROBOT_CONFIG_NS} robot-service-setup -o=go-template --template='{{(index .secrets 0).name}}') \ "
  echo "    -o=go-template --template='{{index .data \"token\"}}' | base64 -d"
  echo "Enter API token:"
  read ACCESS_TOKEN
fi

if [[ -z "${HOST_HOSTNAME}" ]] ; then
  HOST_HOSTNAME=$(hostname)
fi

# Digest setup-robot image.
IMAGE_REFERENCE=$(curl -fsSL -H "Authorization: Bearer ${ACCESS_TOKEN}" \
"https://k8s.${DOMAIN}/api/v1/namespaces/${ROBOT_CONFIG_NS}/configmaps/robot-setup" | \
python3 -c "import sys, json; print(json.load(sys.stdin)['data']['setup_robot_image'])") || \
  IMAGE_REFERENCE=""

if [[ -z "$IMAGE_REFERENCE" ]] ; then
  echo "ERROR: failed to get setup_robot_image from cloud cluster" >&2
  exit 1
fi

# Registry of cloud robotics images
REGISTRY=$(curl -fsSL -H "Authorization: Bearer ${ACCESS_TOKEN}" \
"https://k8s.${DOMAIN}/api/v1/namespaces/${ROBOT_CONFIG_NS}/configmaps/robot-setup" | \
python3 -c "import sys, json; print(json.load(sys.stdin)['data']['registry'])") || \
  REGISTRY=""

if [[ -z "$REGISTRY" ]] ; then
  echo "ERROR: failed to get container registry from cloud cluster" >&2
  exit 1
fi

echo "Getting image pull secret from remote cluster"
DOCKER_CONFIG_JSON=$(curl -fsSL -H "Authorization: Bearer ${ACCESS_TOKEN}" \
"https://k8s.${DOMAIN}/api/v1/namespaces/${ROBOT_CONFIG_NS}/secrets/cloud-robotics-images" | \
python3 -c "import sys, json; print(json.load(sys.stdin)['data'])")

echo "Applying image pull secret to local cluster"
cat <<EOF | kc apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: setup-robot-docker
type: kubernetes.io/dockerconfigjson
data:
  ${DOCKER_CONFIG_JSON}
EOF

# Wait for creation of the default service account.
# https://github.com/kubernetes/kubernetes/issues/66689
i=0
until kc get serviceaccount default &>/dev/null; do
  sleep 1
  i=$((i + 1))
  if ((i >= 60)) ; then
    # Try again, without suppressing stderr this time.
    if ! kc get serviceaccount default >/dev/null; then
      echo "ERROR: 'kubectl get serviceaccount default' failed" >&2
      exit 1
    fi
  fi
done

echo "Running setup-robot"

# Remove previous instance, in case installation was canceled
kc delete pod setup-robot 2> /dev/null || true
faketty kubectl --context "${KUBE_CONTEXT}" run setup-robot --restart=Never -it --rm \
  --image="${REGISTRY}/${IMAGE_REFERENCE}" \
  --overrides='{ "spec": { "imagePullSecrets": [{"name": "setup-robot-docker"}] } }' \
  --env="ACCESS_TOKEN=${ACCESS_TOKEN}" \
  --env="REGISTRY=${REGISTRY}" \
  --env="HOST_HOSTNAME=${HOST_HOSTNAME}" \
  -- "$@"


echo "Deleting image pull secret from local cluster"

kc delete secrets setup-robot-docker
