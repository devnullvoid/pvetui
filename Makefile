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
	@echo "$(GREEN)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Go targets
build: ## Build the application binary
	@echo "$(GREEN)Building $(APP_NAME)...$(NC)"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -a -installsuffix cgo -o $(APP_NAME) ./cmd/proxmox-tui

test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning...$(NC)"
	rm -f $(APP_NAME)
	rm -f coverage.out coverage.html
	docker rmi $(FULL_IMAGE_NAME):$(VERSION) $(FULL_IMAGE_NAME):latest 2>/dev/null || true

# Docker targets
docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(NC)"
	@chmod +x scripts/docker-build.sh
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) ./scripts/docker-build.sh

docker-run: ## Run application in Docker container
	@echo "$(GREEN)Running Docker container...$(NC)"
	@chmod +x scripts/docker-run.sh
	./scripts/docker-run.sh

docker-test: ## Test Docker image
	@echo "$(GREEN)Testing Docker image...$(NC)"
	docker run --rm $(FULL_IMAGE_NAME):$(VERSION) --help

# Podman targets
podman-build: ## Build Podman image
	@echo "$(GREEN)Building Podman image...$(NC)"
	@chmod +x scripts/podman-build.sh
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) ./scripts/podman-build.sh

podman-run: ## Run application in Podman container
	@echo "$(GREEN)Running Podman container...$(NC)"
	@chmod +x scripts/podman-run.sh
	./scripts/podman-run.sh

podman-test: ## Test Podman image
	@echo "$(GREEN)Testing Podman image...$(NC)"
	podman run --rm $(FULL_IMAGE_NAME):$(VERSION) --help

# Docker Compose targets
compose-up: ## Start services with docker-compose
	@echo "$(GREEN)Starting services with docker-compose...$(NC)"
	docker-compose up -d

compose-down: ## Stop services with docker-compose
	@echo "$(GREEN)Stopping services with docker-compose...$(NC)"
	docker-compose down

compose-logs: ## Show docker-compose logs
	docker-compose logs -f

# Development targets
dev-setup: ## Set up development environment
	@echo "$(GREEN)Setting up development environment...$(NC)"
	@if [ ! -f .env ]; then \
		echo "$(YELLOW)Creating .env from .env.example...$(NC)"; \
		cp .env.example .env; \
		echo "$(RED)Please edit .env with your Proxmox configuration$(NC)"; \
	fi
	@mkdir -p cache logs

lint: ## Run linters
	@echo "$(GREEN)Running linters...$(NC)"
	golangci-lint run

format: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	go fmt ./...
	goimports -w .

# Release targets
release-build: ## Build release binaries for multiple platforms
	@echo "$(GREEN)Building release binaries...$(NC)"
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/$(APP_NAME)-linux-amd64 ./cmd/proxmox-tui
	GOOS=linux GOARCH=arm64 go build -o dist/$(APP_NAME)-linux-arm64 ./cmd/proxmox-tui
	GOOS=darwin GOARCH=amd64 go build -o dist/$(APP_NAME)-darwin-amd64 ./cmd/proxmox-tui
	GOOS=darwin GOARCH=arm64 go build -o dist/$(APP_NAME)-darwin-arm64 ./cmd/proxmox-tui
	GOOS=windows GOARCH=amd64 go build -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/proxmox-tui

# Utility targets
version: ## Show version information
	@echo "App: $(APP_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(GO_VERSION)"
	@echo "Image: $(FULL_IMAGE_NAME):$(VERSION)"

.DEFAULT_GOAL := help 