VERSION ?= latest
IMG_REGISTRY ?= quay.io/kubevirt
IMG_PLATFORMS ?= linux/amd64,linux/arm64,linux/s390x
IMG_CONTROLLER ?= ${IMG_REGISTRY}/virt-template-controller:${VERSION}
IMG_APISERVER ?= ${IMG_REGISTRY}/virt-template-apiserver:${VERSION}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Supported values are podman and docker.
CONTAINER_TOOL ?= podman

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: fmt vet vendor lint manifests generate check-uncommitted

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

CONTROLLER_GEN_PATHS ?= "{$(CURDIR)/api/...,$(CURDIR)/internal/controller/...,$(CURDIR)/internal/webhook/...}"
CONTROLLER_GEN_PATHS_APISERVER ?= "{$(CURDIR)/internal/apiserver/storage/...}"
CONTROLLER_GEN_APISERVER_RBAC ?= rbac:roleName=apiserver-role,fileName=role_apiserver.yaml

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(call go-tool,controller-gen,rbac:roleName=manager-role crd webhook paths=$(CONTROLLER_GEN_PATHS) output:crd:artifacts:config=$(CURDIR)/config/crd/bases)
	$(call go-tool,controller-gen,$(CONTROLLER_GEN_APISERVER_RBAC) paths=$(CONTROLLER_GEN_PATHS_APISERVER))
	@# These are created by controller-gen because the subresource objects needed to be marked as root objects,
	@# so all DeepCopy implementations are generated but we don't need them as CRDs.
	rm $(CURDIR)/config/crd/bases/subresources.template.kubevirt.io_*.yaml

.PHONY: generate
generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(call go-tool,controller-gen,object:headerFile="$(CURDIR)/hack/boilerplate.go.txt" paths=$(CONTROLLER_GEN_PATHS))
	./hack/generate.sh

.PHONY: fmt
fmt: ## Run gofumpt against code.
	$(call go-tool,gofumpt,-w -extra $(CURDIR))

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint linter and lint.sh script
	$(call go-tool,golangci-lint,run,$(CURDIR))
	./hack/lint.sh
	./hack/license-header-check.sh

.PHONY: vendor
vendor: ## Update vendored modules
	cd api && go mod tidy
	cd staging/src/kubevirt.io/virt-template-engine && go mod tidy
	cd staging/src/kubevirt.io/virt-template-client-go && go mod tidy
	go mod tidy
	go work sync
	go work vendor
	cd tools && GOWORK=off go mod tidy && GOWORK=off go mod vendor

.PHONY: check-uncommitted
check-uncommitted: ## Check for uncommitted changes.
	./hack/check-uncommitted.sh

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell GOWORK=off go -C $(CURDIR)/tools tool setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... ./staging/src/kubevirt.io/virt-template-engine/... | grep -v /tests) -coverprofile cover.out

.PHONY: functest
functest: manifests generate fmt vet ## Run the functional tests.
	go test -v -timeout 0 ./tests/... -ginkgo.v -ginkgo.randomize-all $(FUNCTEST_EXTRA_ARGS)

.PHONY: cluster-up
cluster-up: ## Start a kubevirtci cluster running a stable version of KubeVirt.
	./hack/kubevirtci.sh up
	KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) $(MAKE) deploy-cert-manager

.PHONY: cluster-down
cluster-down: ## Stop the kubevirtci cluster running a stable version of KubeVirt.
	./hack/kubevirtci.sh down

.PHONY: cluster-sync
cluster-sync: generate ## Install virt-template to the kubevirtci cluster running a stable version of KubeVirt.
	$(MAKE) container-build container-push IMG_REGISTRY=$$(./hack/kubevirtci.sh registry) IMG_PLATFORMS=linux/$(IMG_BUILD_ARCH) TLS_VERIFY=false
	KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) $(MAKE) undeploy uninstall install deploy IMG_REGISTRY=registry:5000 IGNORE_NOT_FOUND=true
	KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) hack/wait.sh

.PHONY: cluster-functest
cluster-functest: ## Run the functional tests on the kubevirtci cluster running a stable version of KubeVirt.
	KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) go test -v -timeout 0 ./tests/... -ginkgo.v -ginkgo.randomize-all $(FUNCTEST_EXTRA_ARGS)

.PHONY: kubevirt-up
kubevirt-up: ## Start a kubevirtci cluster running a git version of KubeVirt.
	./hack/kubevirt.sh up
	KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) $(MAKE) deploy-cert-manager

.PHONY: kubevirt-down
kubevirt-down: ## Stop the kubevirtci cluster running a git version of KubeVirt.
	./hack/kubevirt.sh down

.PHONY: kubevirt-sync
kubevirt-sync: generate ## Install virt-template to the kubevirtci cluster running a git version of KubeVirt.
	$(MAKE) container-build container-push IMG_REGISTRY=$$(./hack/kubevirt.sh registry) IMG_PLATFORMS=linux/$(IMG_BUILD_ARCH) TLS_VERIFY=false
	KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) $(MAKE) undeploy uninstall install deploy IMG_REGISTRY=registry:5000 IGNORE_NOT_FOUND=true
	KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) hack/wait.sh

.PHONY: kubevirt-functest
kubevirt-functest: ## Run the functional tests on the kubevirtci cluster running a git version of KubeVirt.
	KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) go test -v -timeout 0 ./tests/... -ginkgo.v -ginkgo.randomize-all $(FUNCTEST_EXTRA_ARGS)

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: build-apiserver
build-apiserver: manifests generate fmt vet ## Build apiserver binary.
	go build -o bin/apiserver cmd/apiserver/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: run-apiserver
run-apiserver: manifests generate fmt vet ## Run an apiserver from your host.
	go run ./cmd/apiserver/main.go

.PHONY: container-build
container-build: container-build-controller container-build-apiserver ## Build container images.

.PHONY: container-build-controller
container-build-controller: ## Build container image with the controller.
	$(call container-build-with-tool,$(CONTAINER_TOOL),$(IMG_CONTROLLER),Dockerfile)

.PHONY: container-build-apiserver
container-build-apiserver: ## Build container image with the controller.
	./hack/save-version.sh
	$(call container-build-with-tool,$(CONTAINER_TOOL),$(IMG_APISERVER),apiserver.Dockerfile)

IMG_BUILD_ARCH := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
DOCKER_BUILDER ?= virt-template-docker-builder

# Generic function to build container images
# Usage: $(call container-build-with-tool,<tool>,<image>,<containerfile>)
define container-build-with-tool
	BUILD_ARCH=$(IMG_BUILD_ARCH) PLATFORMS=$(IMG_PLATFORMS) \
	IMG=$(2) CONTAINERFILE=$(3) \
	$(if $(filter docker,$(1)),DOCKER_BUILDER=$(DOCKER_BUILDER)) \
	./hack/container-build-$(1).sh
endef

ifndef TLS_VERIFY
  TLS_VERIFY = true
endif

.PHONY: container-push
container-push: ## Push container images.
ifeq ($(CONTAINER_TOOL),podman)
	podman manifest push --tls-verify=$(TLS_VERIFY) ${IMG_CONTROLLER} ${IMG_CONTROLLER}
	podman manifest push --tls-verify=$(TLS_VERIFY) ${IMG_APISERVER} ${IMG_APISERVER}
endif

.PHONY: build-installer
build-installer: manifests generate ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	$(call go-tool,kustomize,edit set image controller=${IMG_CONTROLLER},$(CURDIR)/config/manager)
	$(call go-tool,kustomize,edit set image apiserver=${IMG_APISERVER},$(CURDIR)/config/apiserver)
	$(call go-tool,kustomize,edit set annotation template.kubevirt.io/virt-template-version:${VERSION},$(CURDIR)/config/components/version)
	$(call go-tool,kustomize,build $(CURDIR)/config/default) > dist/install.yaml
	hack/strip-namespace.sh dist/install.yaml

.PHONY: build-installer-openshift
build-installer-openshift: manifests generate ## Generate a consolidated YAML with CRDs and deployment for OpenShift.
	mkdir -p dist
	$(call go-tool,kustomize,edit set image controller=${IMG_CONTROLLER},$(CURDIR)/config/manager)
	$(call go-tool,kustomize,edit set image apiserver=${IMG_APISERVER},$(CURDIR)/config/apiserver)
	$(call go-tool,kustomize,edit set annotation template.kubevirt.io/virt-template-version:${VERSION},$(CURDIR)/config/components/version)
	$(call go-tool,kustomize,build $(CURDIR)/config/openshift) > dist/install-openshift.yaml
	hack/strip-namespace.sh dist/install-openshift.yaml

.PHONY: build-installer-virt-operator
build-installer-virt-operator: manifests generate ## Generate a consolidated YAML with CRDs and deployment for virt-operator.
	mkdir -p dist
	$(call go-tool,kustomize,edit set image controller=${IMG_CONTROLLER},$(CURDIR)/config/manager)
	$(call go-tool,kustomize,edit set image apiserver=${IMG_APISERVER},$(CURDIR)/config/apiserver)
	$(call go-tool,kustomize,edit set annotation template.kubevirt.io/virt-template-version:${VERSION},$(CURDIR)/config/components/version)
	$(call go-tool,kustomize,build $(CURDIR)/config/virt-operator) > dist/install-virt-operator.yaml
	hack/strip-namespace.sh dist/install-virt-operator.yaml

##@ Deployment

ifndef IGNORE_NOT_FOUND
  IGNORE_NOT_FOUND = false
endif

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster.
	$(call go-tool,kustomize,build $(CURDIR)/config/crd) | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(call go-tool,kustomize,build $(CURDIR)/config/crd) | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

.PHONY: deploy
deploy: manifests ## Deploy controller and apiserver to the K8s cluster.
	$(call go-tool,kustomize,edit set image controller=${IMG_CONTROLLER},$(CURDIR)/config/manager)
	$(call go-tool,kustomize,edit set image apiserver=${IMG_APISERVER},$(CURDIR)/config/apiserver)
	$(call go-tool,kustomize,build $(CURDIR)/config/default) | $(KUBECTL) apply -f -

.PHONY: deploy-openshift
deploy-openshift: manifests ## Deploy controller and apiserver to the OpenShift cluster.
	$(call go-tool,kustomize,edit set image controller=${IMG_CONTROLLER},$(CURDIR)/config/manager)
	$(call go-tool,kustomize,edit set image apiserver=${IMG_APISERVER},$(CURDIR)/config/apiserver)
	$(call go-tool,kustomize,build $(CURDIR)/config/openshift) | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller and apiserver from the K8s cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(call go-tool,kustomize,build $(CURDIR)/config/default) | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

.PHONY: undeploy-openshift
undeploy-openshift: ## Undeploy controller and apiserver from the OpenShift cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(call go-tool,kustomize,build $(CURDIR)/config/openshift) | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

CERT_MANAGER_VERSION ?= v1.20.0
.PHONE: deploy-cert-manager
deploy-cert-manager:
	$(KUBECTL) apply -f "https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml"
	$(call go-tool,cmctl,check api --wait=5m)

##@ Dependencies

KUBECTL ?= kubectl

## Location to install envtest k8s binaries
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

.PHONY: setup-envtest
setup-envtest: ## Install the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(call go-tool,setup-envtest,use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path) || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

# go-tool runs a build tool declared in tools/go.mod via Go's tool cache.
# Tool versions are pinned in tools/go.mod; Go's build cache avoids redundant rebuilds.
# $1 = tool name, $2 = tool arguments, $3 = working directory (default: project root)
define go-tool
sh -c 'cd $(or $(3),$(CURDIR)) && $$(GOWORK=off go -C $(CURDIR)/tools tool -n $(1)) $(2)'
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
