# Makefile for pvetui

# Configuration
APP_NAME := pvetui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REGISTRY ?=
IMAGE_NAME := $(APP_NAME)
FULL_IMAGE_NAME := $(if $(REGISTRY),$(REGISTRY)/$(IMAGE_NAME),$(IMAGE_NAME))

# Go configuration
GO_VERSION := 1.24.2
# Default to host platform, allow override via environment variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
# Optional full rebuild (set REBUILD=1 to force -a)
REBUILD ?=
# Optional trimpath for local builds (set TRIMPATH=0 to disable)
TRIMPATH ?= 1
GO_BUILD_FLAGS := -installsuffix cgo \
	$(if $(filter 1 true yes,$(REBUILD)),-a,) \
	$(if $(filter 1 true yes,$(TRIMPATH)),-trimpath,)

# Package lists (computed once)
PKGS_ALL := $(shell go list ./...)
PKGS_NO_EXAMPLES := $(filter-out %/examples,$(PKGS_ALL))
PKGS_UNIT := $(filter-out %/test/integration,$(PKGS_NO_EXAMPLES))

# Colors
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m

.PHONY: help build build-fast test test-quick clean docker-build docker-run podman-build podman-run compose-up compose-down test-workflows test-workflow-lint test-workflow-test test-workflow-build test-workflow-integration workflow-list workflow-setup release release-github release-dry-run release-no-github release-dry-run-no-github release-build test-integration test-integration-real test-all test-coverage test-coverage-all demo screenshots update-novnc gen-openapi openapi-serve openapi-serve-start openapi-serve-stop test-mock e2e-mock e2e-mock-update

# Default target
help: ## Show this help message
	@printf "$(GREEN)Available targets:$(NC)\n"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

gen-openapi: ## Generate OpenAPI spec from upstream apidoc.js
	@printf "$(GREEN)Generating OpenAPI spec from apidoc.js...$(NC)\n"
	go run ./cmd/pve-openapi-gen -out docs/api/pve-openapi.yaml -version $(VERSION)

openapi-serve-start: ## Start Redoc viewer in background (pid in /tmp/pve-openapi-serve.pid)
	@printf "$(GREEN)Starting OpenAPI server (background)...$(NC)\n"
	@npx --yes http-server docs/api -p 8080 >/tmp/pve-openapi-serve.log 2>&1 & \
		echo $$! > /tmp/pve-openapi-serve.pid && \
		printf "Viewer running at http://localhost:8080 (pid %s)\n" "`cat /tmp/pve-openapi-serve.pid`"

openapi-serve: ## Serve Redoc viewer in foreground (Ctrl+C to stop)
	@printf "$(GREEN)Serving OpenAPI spec with Redoc (foreground)...$(NC)\n"
	npx --yes http-server docs/api -p 8080

openapi-serve-stop: ## Stop Redoc viewer started by openapi-serve
	@if [ -f /tmp/pve-openapi-serve.pid ]; then \
		pid=$$(cat /tmp/pve-openapi-serve.pid); \
		printf "Stopping openapi server (pid %s)...\n" $$pid; \
		kill $$pid 2>/dev/null || true; \
		rm -f /tmp/pve-openapi-serve.pid; \
	else \
		printf "No openapi server pid file found.\n"; \
	fi

# Go targets
build: ## Build the application binary
	@printf "$(GREEN)Building $(APP_NAME)...$(NC)\n"
	# Use pure-Go build; only use GOAMD64=v1 and extra tags when targeting Windows/amd64
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GO_BUILD_FLAGS) \
		-ldflags="-X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) \
		-X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
		-o ./bin/$(APP_NAME) ./cmd/pvetui

build-fast: ## Fast local build (skip version ldflags)
	@printf "$(GREEN)Building $(APP_NAME) (fast)...$(NC)\n"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GO_BUILD_FLAGS) \
		-o ./bin/$(APP_NAME) ./cmd/pvetui

install: ## Build and install the application from source
	@printf "$(GREEN)Building and installing $(APP_NAME) from source...$(NC)\n"
	@printf "$(YELLOW)Installing to: $(shell go env GOPATH)/bin/$(APP_NAME)$(NC)\n"
	@mkdir -p $(shell go env GOPATH)/bin
	go build -ldflags="-X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) \
		-X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
		-o $(shell go env GOPATH)/bin/$(APP_NAME) ./cmd/pvetui
	@printf "$(GREEN)✅ Installation complete!$(NC)\n"
	@printf "$(YELLOW)Make sure $(shell go env GOPATH)/bin is in your PATH$(NC)\n"

install-go: ## Install the application using go install from local source
	@printf "$(GREEN)Installing $(APP_NAME) using go install...$(NC)\n"
	@printf "$(YELLOW)Installing to: $(shell go env GOPATH)/bin/$(APP_NAME)$(NC)\n"
	go install -ldflags="-X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) \
		-X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
		./cmd/pvetui
	@printf "$(GREEN)✅ Installation complete!$(NC)\n"
	@printf "$(YELLOW)Make sure $(shell go env GOPATH)/bin is in your PATH$(NC)\n"

uninstall: ## Uninstall the application
	@printf "$(GREEN)Uninstalling $(APP_NAME)...$(NC)\n"
	@rm -f $(shell go env GOPATH)/bin/$(APP_NAME)
	@printf "$(GREEN)✅ Uninstallation complete!$(NC)\n"

test: ## Run unit tests
	@printf "$(GREEN)Running unit tests...$(NC)\n"
	go test -v $(PKGS_UNIT)

test-unit: test ## Alias for test (unit tests only)

test-integration: ## Run integration tests
	@printf "$(GREEN)Running integration tests...$(NC)\n"
	go test -v ./test/integration/...

test-mock: ## Run integration tests against the stateful mock API (no real Proxmox needed)
	@printf "$(GREEN)Running integration tests against pve-mock-api...$(NC)\n"
	go test -v ./test/integration/...

e2e-mock: ## Record and diff VHS text output against golden (uses mock API)
	@scripts/e2e/run-mock-e2e.sh

e2e-mock-update: ## Update golden file for VHS E2E run (uses mock API)
	@UPDATE=1 scripts/e2e/run-mock-e2e.sh

test-integration-real: ## Run integration tests against real Proxmox (requires PVETUI_INTEGRATION_TEST=true)
	@printf "$(GREEN)Running integration tests against real Proxmox...$(NC)\n"
	@printf "$(YELLOW)Make sure PVETUI_TEST_* environment variables are set$(NC)\n"
	PVETUI_INTEGRATION_TEST=true go test -v ./test/integration/...

test-all: ## Run all tests (unit + integration)
	@printf "$(GREEN)Running all tests...$(NC)\n"
	go test -v $(PKGS_NO_EXAMPLES)

test-quick: ## Run unit tests (no race/coverage)
	@printf "$(GREEN)Running quick unit tests...$(NC)\n"
	go test -v $(PKGS_UNIT)

test-coverage: ## Run unit tests with coverage
	@printf "$(GREEN)Running unit tests with coverage...$(NC)\n"
	go test -v -coverprofile=coverage.out $(PKGS_UNIT)
	go tool cover -html=coverage.out -o coverage.html

test-coverage-all: ## Run all tests with coverage
	@printf "$(GREEN)Running all tests with coverage...$(NC)\n"
	go test -v -coverprofile=coverage.out $(PKGS_NO_EXAMPLES)
	go tool cover -html=coverage.out -o coverage.html

# Workflow testing targets
test-workflows: ## Run all GitHub Actions workflows locally using act
	@printf "$(GREEN)Running all GitHub Actions workflows locally...$(NC)\n"
	@chmod +x scripts/test-workflows.sh
	./scripts/test-workflows.sh ci all

test-workflow-lint: ## Run lint workflow locally
	@printf "$(GREEN)Running lint workflow locally...$(NC)\n"
	@chmod +x scripts/test-workflows.sh
	./scripts/test-workflows.sh ci lint

test-workflow-test: ## Run test workflow locally
	@printf "$(GREEN)Running test workflow locally...$(NC)\n"
	@chmod +x scripts/test-workflows.sh
	./scripts/test-workflows.sh ci test

test-workflow-build: ## Run build workflow locally
	@printf "$(GREEN)Running build workflow locally...$(NC)\n"
	@chmod +x scripts/test-workflows.sh
	./scripts/test-workflows.sh ci build

test-workflow-integration: ## Run integration test workflow locally
	@printf "$(GREEN)Running integration test workflow locally...$(NC)\n"
	@chmod +x scripts/test-workflows.sh
	./scripts/test-workflows.sh ci integration

workflow-list: ## List available GitHub Actions workflows
	@printf "$(GREEN)Available GitHub Actions workflows:$(NC)\n"
	@chmod +x scripts/test-workflows.sh
	./scripts/test-workflows.sh list

workflow-setup: ## Set up local workflow testing environment
	@printf "$(GREEN)Setting up local workflow testing environment...$(NC)\n"
	@chmod +x scripts/test-workflows.sh
	./scripts/test-workflows.sh setup

clean: ## Clean build artifacts
	@printf "$(GREEN)Cleaning...$(NC)\n"
	rm -f $(APP_NAME)
	rm -f coverage.out coverage.html
	docker rmi $(FULL_IMAGE_NAME):$(VERSION) $(FULL_IMAGE_NAME):latest 2>/dev/null || true

# Docker targets
docker-build: ## Build Docker image
	@printf "$(GREEN)Building Docker image...$(NC)\n"
	@chmod +x scripts/docker-build.sh
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) ./scripts/docker-build.sh

docker-run: ## Run application in Docker container
	@printf "$(GREEN)Running Docker container...$(NC)\n"
	@chmod +x scripts/docker-run.sh
	./scripts/docker-run.sh

docker-test: ## Test Docker image
	@printf "$(GREEN)Testing Docker image...$(NC)\n"
	docker run --rm $(FULL_IMAGE_NAME):$(VERSION) --help

# Podman targets
podman-build: ## Build Podman image
	@printf "$(GREEN)Building Podman image...$(NC)\n"
	@chmod +x scripts/podman-build.sh
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) ./scripts/podman-build.sh

podman-run: ## Run application in Podman container
	@printf "$(GREEN)Running Podman container...$(NC)\n"
	@chmod +x scripts/podman-run.sh
	./scripts/podman-run.sh

podman-test: ## Test Podman image
	@printf "$(GREEN)Testing Podman image...$(NC)\n"
	podman run --rm $(FULL_IMAGE_NAME):$(VERSION) --help

# Docker Compose targets
compose-up: ## Start TUI application with docker-compose (interactive)
	@printf "$(GREEN)Starting TUI application with docker-compose...$(NC)\n"
	@printf "$(YELLOW)Note: This will run interactively. Use Ctrl+C to stop.$(NC)\n"
	docker-compose up

compose-build: ## Build and start with docker-compose
	@printf "$(GREEN)Building and starting with docker-compose...$(NC)\n"
	docker-compose up --build

compose-down: ## Stop services with docker-compose
	@printf "$(GREEN)Stopping services with docker-compose...$(NC)\n"
	docker-compose down

compose-logs: ## Show docker-compose logs
	docker-compose logs -f

# Development targets
dev-setup: ## Set up development environment
	@printf "$(GREEN)Setting up development environment...$(NC)\n"
	@if [ ! -f .env ]; then \
		printf "$(YELLOW)Creating .env from .env.example...$(NC)\n"; \
		cp .env.example .env; \
		printf "$(RED)Please edit .env with your Proxmox configuration$(NC)\n"; \
	fi
	@mkdir -p cache logs
	@printf "$(GREEN)Checking development tools...$(NC)\n"
	@command -v golangci-lint >/dev/null 2>&1 || { \
		printf "$(YELLOW)Installing golangci-lint...$(NC)\n"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin; \
	}
	@command -v act >/dev/null 2>&1 || { \
		printf "$(YELLOW)act not found. Install with: curl -s https://raw.githubusercontent.com/nektos/act/master/install.sh | bash$(NC)\n"; \
	}
	@printf "$(GREEN)Development environment ready!$(NC)\n"
	@printf "$(YELLOW)Optional enhancements:$(NC)\n"
	@printf "  • direnv: cp .envrc.example .envrc && direnv allow\n"
	@printf "  • pre-commit: pip install pre-commit && pre-commit install\n"

vet: ## Run go vet
	@printf "$(GREEN)Running go vet...$(NC)\n"
	go vet ./...

lint: ## Run linters
	@printf "$(GREEN)Running linters...$(NC)\n"
	golangci-lint run

code-quality: vet lint ## Run all code quality checks (go vet + linters)

format: ## Format code
	@printf "$(GREEN)Formatting code...$(NC)\n"
	go fmt ./...
	goimports -w .

# Release targets
release: ## Create a new release (usage: make release VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make release VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@chmod +x scripts/create-release.sh
	./scripts/create-release.sh $(VERSION)

# Package Manager targets
package-managers-setup: ## Set up all package manager repositories
	@printf "$(GREEN)Setting up package managers...$(NC)\n"
	@chmod +x scripts/package-managers.sh
	./scripts/package-managers.sh setup

package-managers-update: ## Update all package managers to latest version (usage: make package-managers-update VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make package-managers-update VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)Updating package managers...$(NC)\n"
	@chmod +x scripts/package-managers.sh
	./scripts/package-managers.sh update -v $(VERSION)

package-managers-release: ## Create complete release across all platforms (usage: make package-managers-release VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make package-managers-release VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)Creating complete release...$(NC)\n"
	@chmod +x scripts/package-managers.sh
	./scripts/package-managers.sh release -v $(VERSION)

package-managers-status: ## Show status of all package managers
	@printf "$(GREEN)Checking package manager status...$(NC)\n"
	@chmod +x scripts/package-managers.sh
	./scripts/package-managers.sh status

package-managers-clean: ## Clean up all package manager repositories
	@printf "$(GREEN)Cleaning up package managers...$(NC)\n"
	@chmod +x scripts/package-managers.sh
	./scripts/package-managers.sh clean

# Individual package manager targets
aur-setup: ## Set up AUR package repository
	@printf "$(GREEN)Setting up AUR package...$(NC)\n"
	@chmod +x scripts/aur-package.sh
	./scripts/aur-package.sh setup

aur-update: ## Update AUR package (usage: make aur-update VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make aur-update VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)Updating AUR package...$(NC)\n"
	@chmod +x scripts/aur-package.sh
	./scripts/aur-package.sh update -v $(VERSION)

homebrew-setup: ## Set up Homebrew tap
	@printf "$(GREEN)Setting up Homebrew tap...$(NC)\n"
	@chmod +x scripts/homebrew-tap.sh
	./scripts/homebrew-tap.sh setup

homebrew-update: ## Update Homebrew tap (usage: make homebrew-update VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make homebrew-update VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)Updating Homebrew tap...$(NC)\n"
	@chmod +x scripts/homebrew-tap.sh
	./scripts/homebrew-tap.sh update -v $(VERSION)

scoop-setup: ## Set up Scoop bucket
	@printf "$(GREEN)Setting up Scoop bucket...$(NC)\n"
	@chmod +x scripts/scoop-bucket.sh
	./scripts/scoop-bucket.sh setup

scoop-update: ## Update Scoop bucket (usage: make scoop-update VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)Updating Scoop bucket...$(NC)\n"
	@chmod +x scripts/scoop-bucket.sh
	./scripts/scoop-bucket.sh update -v $(VERSION)

release-github: ## Create release with GitHub CLI (usage: make release-github VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make release-github VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@chmod +x scripts/create-release.sh
	./scripts/create-release.sh $(VERSION) --github

release-dry-run: ## Preview release changes (usage: make release-dry-run VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make release-dry-run VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@chmod +x scripts/create-release.sh
	./scripts/create-release.sh $(VERSION) --dry-run

release-no-github: ## Create release without GitHub integration (usage: make release-no-github VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make release-no-github VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@chmod +x scripts/create-release.sh
	./scripts/create-release.sh $(VERSION) --no-github

release-dry-run-no-github: ## Preview release changes without GitHub (usage: make release-dry-run-no-github VERSION=v0.6.0)
	@if [ -z "$(VERSION)" ]; then \
		printf "$(RED)Error: VERSION is required$(NC)\n"; \
		printf "Usage: make release-dry-run-no-github VERSION=v0.6.0\n"; \
		exit 1; \
	fi
	@chmod +x scripts/create-release.sh
	./scripts/create-release.sh $(VERSION) --dry-run --no-github

release-build: ## Build release binaries for multiple platforms
	@printf "$(GREEN)Building release binaries...$(NC)\n"
	@mkdir -p dist
	GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-linux-386 ./cmd/pvetui
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-linux-amd64 ./cmd/pvetui
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-linux-arm64 ./cmd/pvetui
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-darwin-amd64 ./cmd/pvetui
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-darwin-arm64 ./cmd/pvetui
	GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-windows-386.exe ./cmd/pvetui
	GOOS=windows GOARCH=amd64 GOAMD64=v1 CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/pvetui
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-windows-arm64.exe ./cmd/pvetui

# Convenience target for local Windows compat build
build-windows-compat: ## Build Windows amd64 with compat flags
	@printf "$(GREEN)Building Windows amd64 (compat)...$(NC)\n"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOAMD64=v1 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/pvetui/internal/version.version=$(VERSION) -X github.com/devnullvoid/pvetui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/pvetui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/pvetui

# Utility targets
version: ## Show version information
	@printf "App: $(APP_NAME)\n"
	@printf "Version: $(VERSION)\n"
	@printf "Go Version: $(GO_VERSION)\n"
	@printf "Image: $(FULL_IMAGE_NAME):$(VERSION)\n"

demo: ## Run the VHS demo tape
	@printf "$(GREEN)Running VHS demo...$(NC)\n"
	vhs ./docs/demo.tape

screenshots: ## Run the VHS screenshots tape
	@printf "$(GREEN)Running VHS screenshots...$(NC)\n"
	vhs ./docs/screenshots.tape

.DEFAULT_GOAL := help

# Update novnc subtree and prune after update
update-novnc: ## Update embedded novnc client from upstream and prune unnecessary files
	git subtree pull --prefix=internal/vnc/novnc https://github.com/novnc/noVNC.git master --squash
	./scripts/prune_novnc.sh
	git add internal/vnc/novnc
	@printf "$(GREEN)novnc subtree updated and pruned.$(NC)\n"
