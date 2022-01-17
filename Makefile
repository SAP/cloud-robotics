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

# Cloud Robotics Makefile

REGISTRY                               := $(shell cat .REGISTRY 2>/dev/null)
PUSH_LATEST_TAG                        := true
VERSION                                := $(shell cat VERSION)
EFFECTIVE_VERSION                      := $(VERSION)-$(shell git rev-parse --short HEAD)
OS                                     := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH                                   := $(shell uname -m)


APP_AUTH_PROXY_IMAGE_REPOSITORY              := $(REGISTRY)/app-auth-proxy
APP_ROLLOUT_CONTROLLER_IMAGE_REPOSITORY      := $(REGISTRY)/app-rollout-controller
CHART_ASSIGNMENT_CONTROLLER_IMAGE_REPOSITORY := $(REGISTRY)/chart-assignment-controller
CR_SYNCER_IMAGE_REPOSITORY                   := $(REGISTRY)/cr-syncer
HTTP_RELAY_CLIENT_IMAGE_REPOSITORY           := $(REGISTRY)/http-relay-client
HTTP_RELAY_SERVER_IMAGE_REPOSITORY           := $(REGISTRY)/http-relay-server
LOGGING_PROXY_IMAGE_REPOSITORY				 := $(REGISTRY)/logging-proxy
METADATA_SERVER_IMAGE_REPOSITORY             := $(REGISTRY)/metadata-server
SETUP_ROBOT_IMAGE_REPOSITORY                 := $(REGISTRY)/setup-robot
TENANT_CONTROLLER_IMAGE_REPOSITORY           := $(REGISTRY)/tenant-controller



#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################
.PHONY: docker-images
docker-images:
ifeq ("$(REGISTRY)", "")
	@echo "Please set your docker registry in .REGISTRY file first."; false;
endif
	@echo "Building docker images with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(APP_AUTH_PROXY_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)              -t $(APP_AUTH_PROXY_IMAGE_REPOSITORY):latest              -f Dockerfile --target app-auth-proxy .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(APP_ROLLOUT_CONTROLLER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)      -t $(APP_ROLLOUT_CONTROLLER_IMAGE_REPOSITORY):latest      -f Dockerfile --target app-rollout-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(CHART_ASSIGNMENT_CONTROLLER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(CHART_ASSIGNMENT_CONTROLLER_IMAGE_REPOSITORY):latest -f Dockerfile --target chart-assignment-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(CR_SYNCER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                   -t $(CR_SYNCER_IMAGE_REPOSITORY):latest                   -f Dockerfile --target cr-syncer .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(HTTP_RELAY_CLIENT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)           -t $(HTTP_RELAY_CLIENT_IMAGE_REPOSITORY):latest           -f Dockerfile --target http-relay-client .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(HTTP_RELAY_SERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)           -t $(HTTP_RELAY_SERVER_IMAGE_REPOSITORY):latest           -f Dockerfile --target http-relay-server .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(LOGGING_PROXY_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)               -t $(LOGGING_PROXY_IMAGE_REPOSITORY):latest               -f Dockerfile --target logging-proxy .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(METADATA_SERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)             -t $(METADATA_SERVER_IMAGE_REPOSITORY):latest             -f Dockerfile --target metadata-server .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SETUP_ROBOT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(SETUP_ROBOT_IMAGE_REPOSITORY):latest                 -f Dockerfile --target setup-robot .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(TENANT_CONTROLLER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)           -t $(TENANT_CONTROLLER_IMAGE_REPOSITORY):latest           -f Dockerfile --target tenant-controller .

.PHONY: docker-push
docker-push:
ifeq ("$(REGISTRY)", "")
	@echo "Please set your docker registry in .REGISTRY file first."; false;
endif
	@if ! docker images $(APP_AUTH_PROXY_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(APP_AUTH_PROXY_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(APP_ROLLOUT_CONTROLLER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(APP_ROLLOUT_CONTROLLER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CHART_ASSIGNMENT_CONTROLLER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(CHART_ASSIGNMENT_CONTROLLER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CR_SYNCER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(CR_SYNCER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(HTTP_RELAY_CLIENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(HTTP_RELAY_CLIENT_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(HTTP_RELAY_SERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(HTTP_RELAY_SERVER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(LOGGING_PROXY_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(LOGGING_PROXY_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(METADATA_SERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(METADATA_SERVER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SETUP_ROBOT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SETUP_ROBOT_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(TENANT_CONTROLLER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(TENANT_CONTROLLER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@docker push $(APP_AUTH_PROXY_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(APP_ROLLOUT_CONTROLLER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(CHART_ASSIGNMENT_CONTROLLER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(CR_SYNCER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(HTTP_RELAY_CLIENT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(HTTP_RELAY_SERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(LOGGING_PROXY_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(METADATA_SERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(SETUP_ROBOT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@docker push $(TENANT_CONTROLLER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
ifeq ("$(PUSH_LATEST_TAG)", "true")
	@docker push $(APP_AUTH_PROXY_IMAGE_REPOSITORY):latest
	@docker push $(APP_ROLLOUT_CONTROLLER_IMAGE_REPOSITORY):latest
	@docker push $(CHART_ASSIGNMENT_CONTROLLER_IMAGE_REPOSITORY):latest
	@docker push $(CR_SYNCER_IMAGE_REPOSITORY):latest
	@docker push $(HTTP_RELAY_CLIENT_IMAGE_REPOSITORY):latest
	@docker push $(HTTP_RELAY_SERVER_IMAGE_REPOSITORY):latest
	@docker push $(LOGGING_PROXY_IMAGE_REPOSITORY):latest
	@docker push $(METADATA_SERVER_IMAGE_REPOSITORY):latest
	@docker push $(SETUP_ROBOT_IMAGE_REPOSITORY):latest
	@docker push $(TENANT_CONTROLLER_IMAGE_REPOSITORY):latest
endif

.PHONY: set-deployment-config
set-deployment-config:
	@bash ./scripts/deploy.sh set_config $(kubeconfig)

.PHONY: create-deployment
create-deployment:
	@bash ./scripts/deploy.sh create $(kubeconfig)

.PHONY: update-deployment
update-deployment:
	@bash ./scripts/deploy.sh update $(kubeconfig)

.PHONY: build-synk
build-synk:
	@bash ./scripts/build-synk.sh

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod tidy
	@GO111MODULE=on go mod vendor

.PHONY: generate-go-code
generate-go-code:
	@bash ./scripts/generate-go-code.sh

.PHONY: update-helm-charts
update-helm-charts:
	@helm dependency update charts/base-cloud
	@helm dependency update charts/base-robot
	@bash ./charts/platform-apps/package-subcharts.sh
