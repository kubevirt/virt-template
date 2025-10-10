IMG_TAG ?= latest
IMG_REGISTRY ?= quay.io/kubevirt
IMG_PLATFORMS ?= linux/amd64,linux/arm64,linux/s390x
IMG_CONTROLLER ?= ${IMG_REGISTRY}/virt-template-controller:${IMG_TAG}
IMG_APISERVER ?= ${IMG_REGISTRY}/virt-template-apiserver:${IMG_TAG}

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

CONTROLLER_GEN_PATHS ?= "{./api/...,./internal/controller/...,./internal/webhook/...}"
CONTROLLER_GEN_PATHS_APISERVER ?= "{./internal/apiserver/storage/...}"

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths=$(CONTROLLER_GEN_PATHS) output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=apiserver-role,fileName=role_apiserver.yaml paths=$(CONTROLLER_GEN_PATHS_APISERVER)
	@# These are created by controller-gen because the subresource objects needed to be marked as root objects,
	@# so all DeepCopy implementations are generated but we don't need them as CRDs.
	rm config/crd/bases/subresources.template.kubevirt.io_*.yaml

.PHONY: generate
generate: controller-gen openapi-gen client-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths=$(CONTROLLER_GEN_PATHS)
	./hack/generate.sh

.PHONY: fmt
fmt: gofumpt ## Run gofumpt against code.
	$(GOFUMPT) -w -extra .

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter and lint.sh script
	$(GOLANGCI_LINT) run
	./hack/lint.sh
	./hack/license-header-check.sh

.PHONY: vendor
vendor: ## Update vendored modules
	cd api && go mod tidy
	cd staging/src/kubevirt.io/virt-template/client-go && go mod tidy
	go mod tidy
	go work sync
	go work vendor

.PHONY: check-uncommitted
check-uncommitted: ## Check for uncommitted changes.
	./hack/check-uncommitted.sh

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /tests) -coverprofile cover.out

.PHONY: functest
functest: manifests generate fmt vet ## Run the functional tests.
	go test -v -timeout 0 ./tests/... -ginkgo.v -ginkgo.randomize-all $(FUNCTEST_EXTRA_ARGS)

.PHONY: cluster-up
cluster-up: cmctl ## Start a kubevirtci cluster running a stable version of KubeVirt.
	hack/kubevirtci.sh up
	KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) $(MAKE) deploy-cert-manager

.PHONY: cluster-down
cluster-down: ## Stop the kubevirtci cluster running a stable version of KubeVirt.
	hack/kubevirtci.sh down

.PHONY: cluster-sync
cluster-sync: generate ## Install virt-template to the kubevirtci cluster running a stable version of KubeVirt.
	$(MAKE) container-build container-push IMG_REGISTRY=$$(./hack/kubevirtci.sh registry) IMG_PLATFORMS=linux/$(IMG_BUILD_ARCH) TLS_VERIFY=false
	KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) $(MAKE) undeploy uninstall install deploy IMG_REGISTRY=registry:5000 IGNORE_NOT_FOUND=true
	KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) hack/wait.sh

.PHONY: cluster-functest
cluster-functest: ## Run the functional tests on the kubevirtci cluster running a stable version of KubeVirt.
	cd tests && KUBECONFIG=$$(./hack/kubevirtci.sh kubeconfig) go test -v -timeout 0 ./tests/... -ginkgo.v -ginkgo.randomize-all $(FUNCTEST_EXTRA_ARGS)

.PHONY: kubevirt-up
kubevirt-up: cmctl ## Start a kubevirtci cluster running a git version of KubeVirt.
	hack/kubevirt.sh up
	KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) $(MAKE) deploy-cert-manager

.PHONY: kubevirt-down
kubevirt-down: ## Stop the kubevirtci cluster running a git version of KubeVirt.
	hack/kubevirt.sh down

.PHONY: kubevirt-sync
kubevirt-sync: generate ## Install virt-template to the kubevirtci cluster running a git version of KubeVirt.
	$(MAKE) container-build container-push IMG_REGISTRY=$$(./hack/kubevirt.sh registry) IMG_PLATFORMS=linux/$(IMG_BUILD_ARCH) TLS_VERIFY=false
	KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) $(MAKE) undeploy uninstall install deploy IMG_REGISTRY=registry:5000 IGNORE_NOT_FOUND=true
	KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) hack/wait.sh

.PHONY: kubevirt-functest
kubevirt-functest: ## Run the functional tests on the kubevirtci cluster running a git version of KubeVirt.
	cd tests && KUBECONFIG=$$(./hack/kubevirt.sh kubeconfig) go test -v -timeout 0 ./tests/... -ginkgo.v -ginkgo.randomize-all $(FUNCTEST_EXTRA_ARGS)

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: build-apiserver
build-apiserver: manifests generate fmt vet ## Build apiserver binary.
	go build -o bin/apiserver cmd/apiserver/main.go

.PHONY: build-virttemplatectl
build-virttemplatectl: manifests generate fmt vet ## Build virttemplatectl binary.
	CGO_ENABLED=0 go build -o bin/virttemplatectl cmd/virttemplatectl/main.go

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
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_CONTROLLER}
	cd config/apiserver && $(KUSTOMIZE) edit set image apiserver=${IMG_APISERVER}
	$(KUSTOMIZE) build config/default > dist/install.yaml

.PHONY: build-installer-openshift
build-installer-openshift: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment for OpenShift.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_CONTROLLER}
	cd config/apiserver && $(KUSTOMIZE) edit set image apiserver=${IMG_APISERVER}
	$(KUSTOMIZE) build config/openshift > dist/install-openshift.yaml

##@ Deployment

ifndef IGNORE_NOT_FOUND
  IGNORE_NOT_FOUND = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller and apiserver to the K8s cluster.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_CONTROLLER}
	cd config/apiserver && $(KUSTOMIZE) edit set image apiserver=${IMG_APISERVER}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: deploy-openshift
deploy-openshift: manifests kustomize ## Deploy controller and apiserver to the OpenShift cluster.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_CONTROLLER}
	cd config/apiserver && $(KUSTOMIZE) edit set image apiserver=${IMG_APISERVER}
	$(KUSTOMIZE) build config/openshift | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller and apiserver from the K8s cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

.PHONY: undeploy-openshift
undeploy-openshift: kustomize ## Undeploy controller and apiserver from the OpenShift cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/openshift | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

CERT_MANAGER_VERSION ?= v1.18.2
.PHONE: deploy-cert-manager
deploy-cert-manager: cmctl
	$(KUBECTL) apply -f "https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml"
	$(CMCTL) check api --wait=5m

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
OPENAPI_GEN ?= $(LOCALBIN)/openapi-gen
CLIENT_GEN ?= $(LOCALBIN)/client-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOFUMPT ?= $(LOCALBIN)/gofumpt
CMCTL ?= $(LOCALBIN)/cmctl

## Tool Versions
KUSTOMIZE_VERSION ?= v5.6.0
CONTROLLER_TOOLS_VERSION ?= v0.18.0
KUBE_OPENAPI_VERSION ?= v0.0.0-20250905195725-d35305924705
CODE_GENERATOR_VERSION ?= v0.34.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
GOLANGCI_LINT_VERSION ?= v2.4.0
GOFUMPT_VERSION ?= v0.8.0
CMCTL_VERSION ?= v2.3.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: openapi-gen
openapi-gen: $(OPENAPI_GEN) ## Download openapi-gen locally if necessary.
$(OPENAPI_GEN): $(LOCALBIN)
	$(call go-install-tool,$(OPENAPI_GEN),k8s.io/kube-openapi/cmd/openapi-gen,$(KUBE_OPENAPI_VERSION))

.PHONY: client-gen
client-gen: $(CLIENT_GEN) ## Download client-gen locally if necessary.
$(CLIENT_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen,$(CODE_GENERATOR_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: gofumpt
gofumpt: $(GOFUMPT) ## Download gofumpt locally if necessary.
$(GOFUMPT): $(LOCALBIN)
	$(call go-install-tool,$(GOFUMPT),mvdan.cc/gofumpt,$(GOFUMPT_VERSION))

.PHONY: cmctl
cmctl: $(CMCTL) ## Download cmctl locally if necessary.
$(CMCTL): $(LOCALBIN)
	$(call go-install-tool,$(CMCTL),github.com/cert-manager/cmctl/v2,$(CMCTL_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $$(realpath $(1)-$(3)) $(1)
endef
