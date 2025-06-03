#!/bin/bash

# Podman run script for proxmox-tui
set -e

# Configuration
IMAGE_NAME="proxmox-tui:latest"
CONTAINER_NAME="proxmox-tui"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Podman is available
if ! command -v podman &> /dev/null; then
    log_error "Podman is not installed or not in PATH"
    exit 1
fi

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Change to project root
cd "$PROJECT_ROOT"

# Check if .env file exists
if [ -f ".env" ]; then
    log_info "Loading environment from .env file"
    ENV_FILE="--env-file .env"
else
    log_warn ".env file not found. Using .env.example as reference"
    log_warn "Copy .env.example to .env and configure your Proxmox settings"
    ENV_FILE=""
fi

# Create necessary directories
log_info "Creating cache and logs directories..."
mkdir -p cache logs

# Stop and remove existing container if it exists
if podman ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_info "Stopping and removing existing container..."
    podman stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    podman rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
fi

log_info "Starting proxmox-tui container with Podman..."

# Run the container with proper TTY settings for TUI
# Note: Podman runs rootless by default, which is great for security
podman run \
    --name "$CONTAINER_NAME" \
    --rm \
    -it \
    $ENV_FILE \
    -v "$(pwd)/cache:/app/cache:Z" \
    -v "$(pwd)/logs:/app/logs:Z" \
    -v "$(pwd)/configs:/app/configs:ro,Z" \
    "$IMAGE_NAME" \
    "$@"

log_info "Container stopped." 