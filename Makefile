SHELL ?= /bin/bash

.DEFAULT_GOAL := build

################################################################################
# Version details                                                              #
################################################################################

# This will reliably return the short SHA1 of HEAD or, if the working directory
# is dirty, will return that + "-dirty"
GIT_VERSION = $(shell git describe --always --abbrev=7 --dirty --match=NeVeRmAtCh)

################################################################################
# Containerized development environment-- or lack thereof                      #
################################################################################

ifneq ($(SKIP_DOCKER),true)
	PROJECT_ROOT := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
	GO_DEV_IMAGE := brigadecore/go-tools:v0.4.0

	GO_DOCKER_CMD := docker run \
		-it \
		--rm \
		-e SKIP_DOCKER=true \
		-e GOCACHE=/workspaces/brigade-slack-gateway/.gocache \
		-v $(PROJECT_ROOT):/workspaces/brigade-slack-gateway \
		-w /workspaces/brigade-slack-gateway \
		$(GO_DEV_IMAGE)

	HELM_IMAGE := brigadecore/helm-tools:v0.4.0

	HELM_DOCKER_CMD := docker run \
	  -it \
		--rm \
		-e SKIP_DOCKER=true \
		-e HELM_PASSWORD=$${HELM_PASSWORD} \
		-v $(PROJECT_ROOT):/workspaces/brigade-slack-gateway \
		-w /workspaces/brigade-slack-gateway \
		$(HELM_IMAGE)
endif

################################################################################
# Docker images and charts we build and publish                                #
################################################################################

ifdef DOCKER_REGISTRY
	DOCKER_REGISTRY := $(DOCKER_REGISTRY)/
endif

ifdef DOCKER_ORG
	DOCKER_ORG := $(DOCKER_ORG)/
endif

DOCKER_IMAGE_PREFIX := $(DOCKER_REGISTRY)$(DOCKER_ORG)brigade-slack-gateway-

ifdef HELM_REGISTRY
	HELM_REGISTRY := $(HELM_REGISTRY)/
endif

ifdef HELM_ORG
	HELM_ORG := $(HELM_ORG)/
endif

HELM_CHART_PREFIX := $(HELM_REGISTRY)$(HELM_ORG)

ifdef VERSION
	MUTABLE_DOCKER_TAG := latest
else
	VERSION            := $(GIT_VERSION)
	MUTABLE_DOCKER_TAG := edge
endif

IMMUTABLE_DOCKER_TAG := $(VERSION)

################################################################################
# Tests                                                                        #
################################################################################

.PHONY: lint
lint:
	$(GO_DOCKER_CMD) golangci-lint run --config golangci.yaml

.PHONY: test-unit
test-unit:
	$(GO_DOCKER_CMD) go test \
		-v \
		-timeout=60s \
		-race \
		-coverprofile=coverage.txt \
		-covermode=atomic \
		./...

.PHONY: lint-chart
lint-chart:
	$(HELM_DOCKER_CMD) sh -c ' \
		cd charts/brigade-slack-gateway && \
		helm dep up && \
		helm lint . \
	'

################################################################################
# Upload Code Coverage Reports                                                 #
################################################################################

.PHONY: upload-code-coverage
upload-code-coverage:
	$(GO_DOCKER_CMD) codecov

################################################################################
# Build / Publish                                                              #
################################################################################

.PHONY: build
build: build-images

.PHONY: build-images
build-images: build-receiver build-monitor

.PHONY: build-%
build-%:
	docker buildx build \
		-f $*/Dockerfile \
		-t $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) \
		-t $(DOCKER_IMAGE_PREFIX)$*:$(MUTABLE_DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(GIT_VERSION) \
		--platform linux/amd64,linux/arm64 \
		.

################################################################################
# Publish                                                                      #
################################################################################

.PHONY: publish
publish: push-images publish-chart

.PHONY: push-images
push-images: push-receiver push-monitor

.PHONY: push-%
push-%:
	docker login $(DOCKER_REGISTRY) -u $(DOCKER_USERNAME) -p $${DOCKER_PASSWORD}
	docker buildx build \
		-f $*/Dockerfile \
		-t $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) \
		-t $(DOCKER_IMAGE_PREFIX)$*:$(MUTABLE_DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(GIT_VERSION) \
		--platform linux/amd64,linux/arm64 \
		--push \
		.

.PHONY: publish-chart
publish-chart:
	$(HELM_DOCKER_CMD) sh	-c ' \
		helm registry login $(HELM_REGISTRY) -u $(HELM_USERNAME) -p $${HELM_PASSWORD} && \
		cd charts/brigade-slack-gateway && \
		helm dep up && \
		helm package . --version $(VERSION) --app-version $(VERSION) && \
		helm push brigade-slack-gateway-$(VERSION).tgz oci://$(HELM_REGISTRY)$(HELM_ORG) \
	'

################################################################################
# Targets to facilitate hacking on this gateway.                               #
################################################################################

.PHONY: hack-build-%
hack-build-%:
	docker build \
		-f $*/Dockerfile \
		-t $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) \
		-t $(DOCKER_IMAGE_PREFIX)$*:$(MUTABLE_DOCKER_TAG) \
		--build-arg VERSION='$(VERSION)' \
		--build-arg COMMIT='$(GIT_VERSION)' \
		.

.PHONY: hack-push-images
hack-push-images: hack-push-receiver hack-push-monitor

.PHONY: hack-push-%
hack-push-%: hack-build-%
	docker push $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG)
	docker push $(DOCKER_IMAGE_PREFIX)$*:$(MUTABLE_DOCKER_TAG)

IMAGE_PULL_POLICY ?= Always

.PHONY: hack-deploy
hack-deploy:
ifndef BRIGADE_API_TOKEN
	@echo "BRIGADE_API_TOKEN must be defined" && false
endif
	helm dep up charts/brigade-slack-gateway && \
	helm upgrade brigade-slack-gateway charts/brigade-slack-gateway \
		--install \
		--namespace brigade-slack-gateway \
		--create-namespace \
		--set receiver.image.repository=$(DOCKER_IMAGE_PREFIX)receiver \
		--set receiver.image.tag=$(IMMUTABLE_DOCKER_TAG) \
		--set receiver.image.pullPolicy=$(IMAGE_PULL_POLICY) \
		--set monitor.image.repository=$(DOCKER_IMAGE_PREFIX)monitor \
		--set monitor.image.tag=$(IMMUTABLE_DOCKER_TAG) \
		--set monitor.image.pullPolicy=$(IMAGE_PULL_POLICY) \
		--set brigade.apiToken=$(BRIGADE_API_TOKEN)

.PHONY: hack
hack: hack-push-images hack-deploy
