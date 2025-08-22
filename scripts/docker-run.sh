#!/bin/bash

# Docker run script for pvetui
set -e

# Configuration
IMAGE_NAME="pvetui:latest"
CONTAINER_NAME="pvetui"

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

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
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
# Note: logs are now stored in cache directory (XDG-compliant)
log_info "Creating cache directory..."
mkdir -p cache

# Stop and remove existing container if it exists
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_info "Stopping and removing existing container..."
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
fi

log_info "Starting pvetui container..."

# Run the container with proper TTY settings for TUI
# Note: logs are now stored in cache directory (XDG-compliant)
docker run \
    --name "$CONTAINER_NAME" \
    --rm \
    -it \
    $ENV_FILE \
    -v "$(pwd)/cache:/app/cache" \
    -v "$(pwd)/configs:/app/configs:ro" \
    "$IMAGE_NAME" \
    "$@"

log_info "Container stopped."
