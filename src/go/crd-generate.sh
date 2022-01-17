#!/usr/bin/env bash
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

# This script can be run just like the regular dep tool. It copies the Go
# code to a shadow repo against dep can operate as usual and copies the
# resulting Gopkg.toml and Gopkg.lock files to this directory.
# It then stages the changed dependenies in the bazel WORKSPACE for manual cleanup.

set -e

# K8S release for api, apimachinery and code-generator
K8S_RELEASE="release-1.22"

CURRENT_DIR=$(pwd)
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# We create the shadow repo outside the module that it does not mess up with the module
export GOPATH="${DIR}/../../../.gopath"
SHADOW_REPO="${GOPATH}/src/github.com/SAP/cloud-robotics/src/go"

# Ensure shadow repo is created and clean
rm -rf ${SHADOW_REPO}
mkdir -p ${SHADOW_REPO}
cd ${SHADOW_REPO}

# Ensure go modules are activated
export GO111MODULE=on

# Create module for shadow repository
go mod init
go mod tidy

# Install generator and its dependencies
go get k8s.io/api/...@$K8S_RELEASE
go get k8s.io/apimachinery/...@$K8S_RELEASE
go get k8s.io/code-generator/...@$K8S_RELEASE

export PATH="$PATH:$GOPATH/bin"

rm -rf "${DIR}/pkg/client"
cp -rp ${DIR}/* ${SHADOW_REPO}

function finalize {
  cp -rp ${SHADOW_REPO}/pkg/client ${DIR}/pkg/client
  cp -rp ${SHADOW_REPO}/pkg/apis/. ${DIR}/pkg/apis
  cd ${CURRENT_DIR}

  rm -rf "${GOPATH}/src"
  echo "Please cleanup /bin and /pkg sub directories in '${GOPATH}' manually"
}

trap finalize EXIT
cd ${SHADOW_REPO}

REPO=github.com/SAP/cloud-robotics/src/go

cat > "${SHADOW_REPO}/HEADER" <<EOF
// Copyright $(date +%Y) The Cloud Robotics Authors
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
EOF

dirs=""
groupversions=""

for d in ${SHADOW_REPO}/pkg/apis/*/*; do
  version=$(basename $d)
  group=$(basename "$(dirname $d)")
  echo "generating for ${group}/${version}"

  groupversions="${groupversions},${group}/${version}"
  dirs="${dirs},${REPO}/pkg/apis/${group}/${version}"
done

dirs="${dirs:1}"
groupversions="${groupversions:1}"

deepcopy-gen \
  --go-header-file   "${SHADOW_REPO}/HEADER" \
  --input-dirs       "${dirs}" \
  --bounding-dirs    "${REPO}/pkg/apis" \
  --output-file-base zz_generated.deepcopy

client-gen \
  --go-header-file "${SHADOW_REPO}/HEADER" \
  --clientset-name "versioned" \
  --input-base     "${REPO}/pkg/apis" \
  --input          "${groupversions}" \
  --output-package "${REPO}/pkg/client" \

lister-gen \
  --go-header-file "${SHADOW_REPO}/HEADER" \
  --input-dirs     "${dirs}" \
  --output-package "${REPO}/pkg/client/listers"

informer-gen \
  --go-header-file   "${SHADOW_REPO}/HEADER" \
  --single-directory \
  --listers-package  "${REPO}/pkg/client/listers" \
  --input-dirs       "${dirs}" \
  --output-package   "${REPO}/pkg/client/informers" \
  --versioned-clientset-package "${REPO}/pkg/client/versioned"
