# Makefile for proxmox-tui

# Configuration
APP_NAME := proxmox-tui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REGISTRY ?= 
IMAGE_NAME := $(APP_NAME)
FULL_IMAGE_NAME := $(if $(REGISTRY),$(REGISTRY)/$(IMAGE_NAME),$(IMAGE_NAME))

# Go configuration
GO_VERSION := 1.24.2
GOOS := linux
GOARCH := amd64

# Colors
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m

.PHONY: help build test clean docker-build docker-run podman-build podman-run compose-up compose-down

# Default target
help: ## Show this help message
	@printf "$(GREEN)Available targets:$(NC)\n"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Go targets
build: ## Build the application binary
	@printf "$(GREEN)Building $(APP_NAME)...$(NC)\n"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -a -installsuffix cgo -o $(APP_NAME) ./cmd/proxmox-tui

test: ## Run tests
	@printf "$(GREEN)Running tests...$(NC)\n"
	go test -v ./...

test-coverage: ## Run tests with coverage
	@printf "$(GREEN)Running tests with coverage...$(NC)\n"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

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

lint: ## Run linters
	@printf "$(GREEN)Running linters...$(NC)\n"
	golangci-lint run

format: ## Format code
	@printf "$(GREEN)Formatting code...$(NC)\n"
	go fmt ./...
	goimports -w .

# Release targets
release-build: ## Build release binaries for multiple platforms
	@printf "$(GREEN)Building release binaries...$(NC)\n"
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/$(APP_NAME)-linux-amd64 ./cmd/proxmox-tui
	GOOS=linux GOARCH=arm64 go build -o dist/$(APP_NAME)-linux-arm64 ./cmd/proxmox-tui
	GOOS=darwin GOARCH=amd64 go build -o dist/$(APP_NAME)-darwin-amd64 ./cmd/proxmox-tui
	GOOS=darwin GOARCH=arm64 go build -o dist/$(APP_NAME)-darwin-arm64 ./cmd/proxmox-tui
	GOOS=windows GOARCH=amd64 go build -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/proxmox-tui

# Utility targets
version: ## Show version information
	@printf "App: $(APP_NAME)\n"
	@printf "Version: $(VERSION)\n"
	@printf "Go Version: $(GO_VERSION)\n"
	@printf "Image: $(FULL_IMAGE_NAME):$(VERSION)\n"

.DEFAULT_GOAL := help 