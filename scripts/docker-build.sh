#!/bin/bash

# Docker build script for peevetui
set -e

# Configuration
IMAGE_NAME="peevetui"
VERSION=${VERSION:-latest}
REGISTRY=${REGISTRY:-""}

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

log_info "Building Docker image for peevetui..."

# Get current user's UID and GID
USER_ID=$(id -u)
GROUP_ID=$(id -g)
log_info "Using UID:GID ${USER_ID}:${GROUP_ID} to match host user"

# Build arguments
BUILD_ARGS="--build-arg USER_ID=${USER_ID} --build-arg GROUP_ID=${GROUP_ID}"
if [ -n "$HTTP_PROXY" ]; then
    BUILD_ARGS="$BUILD_ARGS --build-arg HTTP_PROXY=$HTTP_PROXY"
fi
if [ -n "$HTTPS_PROXY" ]; then
    BUILD_ARGS="$BUILD_ARGS --build-arg HTTPS_PROXY=$HTTPS_PROXY"
fi

# Determine full image name
FULL_IMAGE_NAME="$IMAGE_NAME:$VERSION"
if [ -n "$REGISTRY" ]; then
    FULL_IMAGE_NAME="$REGISTRY/$FULL_IMAGE_NAME"
fi

# Build the image
log_info "Building image: $FULL_IMAGE_NAME"
docker build \
    $BUILD_ARGS \
    --tag "$FULL_IMAGE_NAME" \
    --file Dockerfile \
    .

# Also tag as latest if version is not latest
if [ "$VERSION" != "latest" ]; then
    LATEST_TAG="$IMAGE_NAME:latest"
    if [ -n "$REGISTRY" ]; then
        LATEST_TAG="$REGISTRY/$LATEST_TAG"
    fi
    log_info "Tagging as latest: $LATEST_TAG"
    docker tag "$FULL_IMAGE_NAME" "$LATEST_TAG"
fi

log_info "Build completed successfully!"
log_info "Image: $FULL_IMAGE_NAME"

# Show image size
IMAGE_SIZE=$(docker images --format "table {{.Size}}" "$FULL_IMAGE_NAME" | tail -n 1)
log_info "Image size: $IMAGE_SIZE"

# Optional: Run basic tests
if [ "$RUN_TESTS" = "true" ]; then
    log_info "Running basic container test..."
    docker run --rm "$FULL_IMAGE_NAME" --help > /dev/null
    log_info "Container test passed!"
fi
