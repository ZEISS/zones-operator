.DEFAULT_GOAL := build

VERSION 				?= latest

GO 							?= go
GO_TOOL 				?= $(GO) tool
GO_HELM_UPDATE 	?= $(GO_RUN_TOOLS) github.com/zeiss/pkg/cmd/helm/update
GO_KIND 				?= $(GO_TOOL) sigs.k8s.io/kind/cmd/kind
GO_KUSTOMIZE 		?= $(GO_TOOL) sigs.k8s.io/kustomize/kustomize/v5
GO_LINT 				?= $(GO_TOOL) github.com/golangci/golangci-lint/v2/cmd/golangci-lint
GO_MOD 					?= $(shell ${GO} list -m)
GO_RELEASER 		?= $(GO_TOOL) github.com/goreleaser/goreleaser/v2
GO_TEST 				?= $(GO_TOOL) gotest.tools/gotestsum --format pkgname

# Variables
REPO 					  ?= $(GITHUB_REPO)
TOKEN 					?= $(GITHUB_TOKEN)
CLUSTER_NAME		?= kind-charts-cluster
CLUSTER_CONFIG	?= cluster.yaml
BASE_DIR				?= $(CURDIR)
PWD 						?= $(shell pwd)
IMAGE_TAG_BASE 	?= ghcr.io/zeiss/zones-operator/operator
IMG 						?= $(IMAGE_TAG_BASE):$(VERSION)

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: build
build: ## Build the binary file.
	$(GO_RELEASER) build --snapshot --clean

.PHONY: snapshot
snapshot: ## Create a snapshot release
	$(GO_RELEASER) release --clean --snapshot

.PHONY: release
release: ## Create a release
	$(GO_RELEASER) release --clean

.PHONY: up
up: ## Run the operator locally.
	$(GO_RUN_TOOLS) github.com/zeiss/pkg/cmd/runproc -f ${PWD}/Procfile -l ${PWD}/Procfile.local

.PHONY: start
start: ## Run the operator locally with hot reloading.
	$(GO_AIR) -c .air.toml

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(GO_KUSTOMIZE) build manifests/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(GO_KUSTOMIZE) build manifests/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(GO_KUSTOMIZE) edit set image controller=${IMG}
	$(GO_KUSTOMIZE) build manifests/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(GO_KUSTOMIZE) build manifests/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: setup
setup: ## Setup the development environment.
	$(PWD)/scripts/setup.sh

.PHONY: generate
generate: ## Generate code.
	$(GO) generate ./...
	$(GO_KUSTOMIZE) build manifests/crd > $(BASE_DIR)/helm/crds/crds.yaml
	@echo "✅ Successfully generated CRDs."

.PHONY: helm/update
helm/update: ## Update helm dependencies.
	$(GO_HELM_UPDATE) --file helm/charts/Chart.yaml --version ${RELEASE_VERSION}

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GO_TOOL) mvdan.cc/gofumpt -w .

.PHONY: vet
vet: ## Run go vet against code.
	$(GO) vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	mkdir -p .test/reports
	$(GO_TEST) --junitfile .test/reports/unit-test.xml -- -race ./... -count=1 -short -cover -coverprofile .test/reports/unit-test-coverage.out

.PHONY: lint
lint: ## Run lint.
	$(GO_LINT) run --timeout 10m -c .golangci.yml

.PHONY: cluster-create
cluster-create: ## Create a local Kubernetes cluster using kind.
	$(GO_KIND) create cluster --config $(CLUSTER_CONFIG)
	@echo "✅ Kind cluster created successfully."

.PHONY: cluster-delete
cluster-delete: ## Destroy the local Kubernetes cluster using kind.
	$(GO_KIND) delete cluster --name $(CLUSTER_NAME)
	@echo "✅ Kind cluster destroyed successfully."

.PHONY: clean
clean: ## Remove previous build.
	rm -rf .test .dist
	find . -type f -name '*.gen.go' -exec rm {} +
	git checkout go.mod

.PHONY: help
help: ## Display this help screen.
	@grep -E '^[a-z.A-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# codegen
include hack/inc.codegen.mk
