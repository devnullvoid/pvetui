# Makefile for proxmox-tui

# Configuration
APP_NAME := proxmox-tui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REGISTRY ?=
IMAGE_NAME := $(APP_NAME)
FULL_IMAGE_NAME := $(if $(REGISTRY),$(REGISTRY)/$(IMAGE_NAME),$(IMAGE_NAME))

# Go configuration
GO_VERSION := 1.24.2
# Default to host platform, allow override via environment variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Colors
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m

.PHONY: help build test clean docker-build docker-run podman-build podman-run compose-up compose-down test-workflows test-workflow-lint test-workflow-test test-workflow-build test-workflow-integration workflow-list workflow-setup release release-github release-dry-run release-no-github release-dry-run-no-github release-build test-integration test-integration-real test-all test-coverage test-coverage-all demo screenshots

# Default target
help: ## Show this help message
	@printf "$(GREEN)Available targets:$(NC)\n"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Go targets
build: ## Build the application binary
	@printf "$(GREEN)Building $(APP_NAME)...$(NC)\n"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOAMD64=v1 go build -a -installsuffix cgo -tags netgo,osusergo \
		-ldflags="-X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) \
		-X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
		-o ./bin/$(APP_NAME) ./cmd/proxmox-tui

install: ## Build and install the application from source
	@printf "$(GREEN)Building and installing $(APP_NAME) from source...$(NC)\n"
	@printf "$(YELLOW)Installing to: $(shell go env GOPATH)/bin/$(APP_NAME)$(NC)\n"
	@mkdir -p $(shell go env GOPATH)/bin
	go build -ldflags="-X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) \
		-X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
		-o $(shell go env GOPATH)/bin/$(APP_NAME) ./cmd/proxmox-tui
	@printf "$(GREEN)✅ Installation complete!$(NC)\n"
	@printf "$(YELLOW)Make sure $(shell go env GOPATH)/bin is in your PATH$(NC)\n"

install-go: ## Install the application using go install from local source
	@printf "$(GREEN)Installing $(APP_NAME) using go install...$(NC)\n"
	@printf "$(YELLOW)Installing to: $(shell go env GOPATH)/bin/$(APP_NAME)$(NC)\n"
	go install -ldflags="-X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) \
		-X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
		./cmd/proxmox-tui
	@printf "$(GREEN)✅ Installation complete!$(NC)\n"
	@printf "$(YELLOW)Make sure $(shell go env GOPATH)/bin is in your PATH$(NC)\n"

uninstall: ## Uninstall the application
	@printf "$(GREEN)Uninstalling $(APP_NAME)...$(NC)\n"
	@rm -f $(shell go env GOPATH)/bin/$(APP_NAME)
	@printf "$(GREEN)✅ Uninstallation complete!$(NC)\n"

test: ## Run unit tests
	@printf "$(GREEN)Running unit tests...$(NC)\n"
	go test -v $(shell go list ./... | grep -v /examples | grep -v /test/integration)

test-unit: test ## Alias for test (unit tests only)

test-integration: ## Run integration tests
	@printf "$(GREEN)Running integration tests...$(NC)\n"
	go test -v ./test/integration/...

test-integration-real: ## Run integration tests against real Proxmox (requires PROXMOX_INTEGRATION_TEST=true)
	@printf "$(GREEN)Running integration tests against real Proxmox...$(NC)\n"
	@printf "$(YELLOW)Make sure PROXMOX_TEST_* environment variables are set$(NC)\n"
	PROXMOX_INTEGRATION_TEST=true go test -v ./test/integration/...

test-all: ## Run all tests (unit + integration)
	@printf "$(GREEN)Running all tests...$(NC)\n"
	go test -v $(shell go list ./... | grep -v /examples)

test-coverage: ## Run unit tests with coverage
	@printf "$(GREEN)Running unit tests with coverage...$(NC)\n"
	go test -v -coverprofile=coverage.out $(shell go list ./... | grep -v /examples | grep -v /test/integration)
	go tool cover -html=coverage.out -o coverage.html

test-coverage-all: ## Run all tests with coverage
	@printf "$(GREEN)Running all tests with coverage...$(NC)\n"
	go test -v -coverprofile=coverage.out $(shell go list ./... | grep -v /examples)
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
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) -X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-linux-amd64 ./cmd/proxmox-tui
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) -X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-linux-arm64 ./cmd/proxmox-tui
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) -X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-darwin-amd64 ./cmd/proxmox-tui
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) -X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-darwin-arm64 ./cmd/proxmox-tui
	GOOS=windows GOARCH=amd64 GOAMD64=v1 CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) -X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/proxmox-tui
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w -X github.com/devnullvoid/proxmox-tui/internal/version.version=$(VERSION) -X github.com/devnullvoid/proxmox-tui/internal/version.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/devnullvoid/proxmox-tui/internal/version.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o dist/$(APP_NAME)-windows-arm64.exe ./cmd/proxmox-tui

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
