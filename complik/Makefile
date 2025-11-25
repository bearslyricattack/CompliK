# CompliK Monorepo Makefile
# Unified build system for CompliK, block-controller, and procscan

IMG ?= ghcr.io/labring/sealos-complik-service:latest
TARGETARCH ?= $(shell go env GOARCH)
GOOS=linux
CGO_ENABLED=0
GOARCH=$(shell go env GOARCH)
GO_BUILD_FLAGS=-trimpath -ldflags "-s -w"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build-all

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build All Projects

.PHONY: build-all
build-all: build-complik build-block-controller build-procscan ## Build all three projects

.PHONY: clean-all
clean-all: clean-complik clean-block-controller clean-procscan ## Clean all build artifacts

.PHONY: test-all
test-all: test-complik test-block-controller test-procscan ## Run tests for all projects

##@ CompliK Project

.PHONY: clean-complik
clean-complik: ## Clean CompliK build artifacts
	@echo "Cleaning CompliK..."
	@cd complik && rm -rf bin/manager bin/service-complik-*

.PHONY: build-complik
build-complik: ## Build CompliK binary
	@echo "Building CompliK..."
	@cd complik && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o bin/manager cmd/complik/main.go
	@echo "✓ CompliK built successfully: complik/bin/manager"

.PHONY: test-complik
test-complik: ## Run CompliK tests
	@echo "Running CompliK tests..."
	@cd complik && go test -v ./...

.PHONY: docker-build-complik
docker-build-complik: build-complik ## Build CompliK Docker image
	@cd complik && docker build -t $(IMG) .

.PHONY: docker-push-complik
docker-push-complik: ## Push CompliK Docker image
	@docker push $(IMG)

##@ Block Controller

.PHONY: clean-block-controller
clean-block-controller: ## Clean block-controller build artifacts
	@echo "Cleaning block-controller..."
	@cd block-controller && rm -rf bin/manager bin/controller-gen bin/kustomize bin/setup-envtest

.PHONY: build-block-controller
build-block-controller: ## Build block-controller binary
	@echo "Building block-controller..."
	@cd block-controller && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o bin/manager cmd/main.go
	@echo "✓ block-controller built successfully: block-controller/bin/manager"

.PHONY: test-block-controller
test-block-controller: ## Run block-controller tests
	@echo "Running block-controller tests..."
	@cd block-controller && go test -v ./...

.PHONY: docker-build-block-controller
docker-build-block-controller: build-block-controller ## Build block-controller Docker image
	@cd block-controller && docker build -t ghcr.io/bearslyricattack/block-controller:latest .

##@ ProcScan

.PHONY: clean-procscan
clean-procscan: ## Clean procscan build artifacts
	@echo "Cleaning procscan..."
	@cd procscan && rm -rf bin/procscan bin/manager

.PHONY: build-procscan
build-procscan: ## Build procscan binary
	@echo "Building procscan..."
	@cd procscan && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o bin/procscan cmd/procscan/main.go
	@echo "✓ procscan built successfully: procscan/bin/procscan"

.PHONY: test-procscan
test-procscan: ## Run procscan tests
	@echo "Running procscan tests..."
	@cd procscan && go test -v ./...

.PHONY: docker-build-procscan
docker-build-procscan: build-procscan ## Build procscan Docker image
	@cd procscan && docker build -t ghcr.io/bearslyricattack/procscan:latest .

##@ Development

.PHONY: tidy-all
tidy-all: ## Run go mod tidy for all projects
	@echo "Running go mod tidy for CompliK..."
	@cd complik && go mod tidy
	@echo "Running go mod tidy for block-controller..."
	@cd block-controller && go mod tidy
	@echo "Running go mod tidy for procscan..."
	@cd procscan && go mod tidy
	@echo "✓ All modules tidied"

.PHONY: fmt-all
fmt-all: ## Format all Go code
	@echo "Formatting code..."
	@cd complik && go fmt ./...
	@cd block-controller && go fmt ./...
	@cd procscan && go fmt ./...
	@echo "✓ Code formatted"

.PHONY: vet-all
vet-all: ## Run go vet for all projects
	@echo "Running go vet..."
	@cd complik && go vet ./...
	@cd block-controller && go vet ./...
	@cd procscan && go vet ./...
	@echo "✓ Vet completed"

##@ Deployment

.PHONY: deploy-all
deploy-all: ## Deploy all projects to Kubernetes
	@echo "Deploying CompliK..."
	@cd complik && kubectl apply -f deploy/
	@echo "Deploying block-controller..."
	@cd block-controller && kubectl apply -f deploy/
	@echo "Deploying procscan..."
	@cd procscan && kubectl apply -f deploy/
	@echo "✓ All projects deployed"
