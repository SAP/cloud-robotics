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

FROM golang:1.17 AS proto_base
LABEL stage=intermediate
WORKDIR /
# Install protoc compiler
RUN apt -qq update && apt -qq install -y unzip && \
    mkdir /protoc && \
    curl -LO "https://github.com/protocolbuffers/protobuf/releases/download/v3.17.3/protoc-3.17.3-linux-x86_64.zip" && \
    unzip -q "protoc-3.17.3-linux-x86_64.zip" -d /protoc
# Install protoc Go plugin
RUN go install "google.golang.org/protobuf/cmd/protoc-gen-go@v1.26"


# Generates Go code from .proto files
FROM proto_base AS proto_generator
LABEL stage=intermediate
# Copy entire repository to image
COPY . /code
WORKDIR /code/src/proto
RUN bash ./proto-generate.sh
WORKDIR /code/src/go
RUN bash ./crd-generate.sh


# Installs helm by unpacking .tar.gz provided in /third_party of this project.
FROM golang:1.17 AS helm_base
LABEL stage=intermediate
# Copy entire repository to image
COPY . /code
# Installs helm to file /helm_bin/linux-amd64/helm
RUN mkdir /helm_bin && tar -xf /code/third_party/helm/helm-v2.17.0-linux-amd64.tar.gz -C /helm_bin


# Build go code into binaries
FROM helm_base AS builder
LABEL stage=intermediate
ARG SKIP_TESTS=false
WORKDIR /code
# Run all unit tests unless SKIP_TESTS is true
RUN if [ "$SKIP_TESTS" = "false" ] ; then \
      echo "commencing tests..." && \
      go test ./src/go/pkg/... ./src/go/cmd/... ; \
    elif [ "$SKIP_TESTS" = "true" ] ; then \
      echo "unit tests skipped." ; \
    else \
      echo "SKIP_TESTS must be either 'true' or 'false'. Your input: SKIP_TESTS='$SKIP_TESTS'." && \
      exit 95 ; \
    fi
# Build go executables into binaries
RUN mkdir /build && GOBIN=/build \
    GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -mod vendor -a ./...
# Package helm charts for setup-robot
RUN mkdir /charts && /helm_bin/linux-amd64/helm init --client-only --stable-repo-url https://k8s-at-home.com/charts && \
    /helm_bin/linux-amd64/helm package -u ./charts/base-robot --destination /charts


# Executable container bases
# --------------------------

FROM alpine:3.14.2 AS ssl_runner
# Install SSL ca certificates
RUN apk add --no-cache ca-certificates
# Create user to be used in executable containers
# Add a non-root user matching the nonroot user from the main container
RUN addgroup -g 65532 -S nonroot && adduser -u 65532 -S nonroot -G nonroot
# Set the uid as an integer for compatibility with runAsNonRoot in Kubernetes
USER 65532

FROM alpine:3.14.2 AS ssl_iptables_root_runner
# Install SSL ca certificates
RUN apk add --no-cache ca-certificates iptables


# Executables
# -----------------

FROM ssl_runner AS app-auth-proxy
WORKDIR /
COPY --from=builder /build/app-auth-proxy /app-auth-proxy
EXPOSE 8000
ENTRYPOINT [ "./app-auth-proxy" ]

FROM ssl_runner AS app-rollout-controller
WORKDIR /
COPY --from=builder /build/app-rollout-controller /app-rollout-controller
ENTRYPOINT [ "./app-rollout-controller" ]

FROM ssl_runner AS chart-assignment-controller
WORKDIR /
# Helm used by init container
COPY --from=builder /helm_bin/linux-amd64/helm /helm
COPY --from=builder /build/chart-assignment-controller /chart-assignment-controller
ENTRYPOINT [ "./chart-assignment-controller" ]

FROM ssl_runner AS cr-syncer
WORKDIR /
COPY --from=builder /build/cr-syncer /cr-syncer
ENTRYPOINT [ "./cr-syncer" ]

FROM ssl_runner AS crd-generator
WORKDIR /
COPY --from=builder /build/crd-generator /crd-generator
ENTRYPOINT [ "./crd-generator" ]

FROM ssl_runner AS http-relay-client
WORKDIR /
COPY --from=builder /build/http-relay-client /http-relay-client
ENTRYPOINT [ "./http-relay-client" ]

FROM ssl_runner AS http-relay-server
WORKDIR /
COPY --from=builder /build/http-relay-server /http-relay-server
ENTRYPOINT [ "./http-relay-server" ]

FROM ssl_runner AS logging-proxy
WORKDIR /
COPY --from=builder /build/logging-proxy /logging-proxy
ENTRYPOINT [ "./logging-proxy" ]

FROM ssl_iptables_root_runner AS metadata-server
WORKDIR /
COPY --from=builder /build/metadata-server /metadata-server
ENTRYPOINT [ "./metadata-server" ]

FROM ssl_runner AS setup-robot
WORKDIR /
# Helm used for templating charts
COPY --from=builder /helm_bin/linux-amd64/helm /setup-robot-files/helm
COPY --from=builder /build/synk /setup-robot-files/synk
COPY --from=builder /build/setup-robot /setup-robot
COPY --from=builder /charts/*.tgz /setup-robot-files/
ENTRYPOINT [ "./setup-robot" ]

FROM ssl_runner AS synk
WORKDIR /
COPY --from=builder /build/synk /synk
ENTRYPOINT [ "./synk" ]

FROM ssl_runner AS tenant-controller
WORKDIR /
COPY --from=builder /build/tenant-controller /tenant-controller
ENTRYPOINT [ "./tenant-controller" ]
