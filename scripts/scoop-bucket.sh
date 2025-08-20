#!/bin/bash

# Scoop Bucket Management Script for proxmox-tui
# This script helps set up and maintain the Scoop bucket repository

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
PKGNAME="proxmox-tui"
BUCKET_REPO="scoop-proxmox-tui"
GITHUB_REPO="devnullvoid/proxmox-tui"
VERSION=$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev")

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

show_help() {
    cat << EOF
Scoop Bucket Management Script for proxmox-tui

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    setup           Set up Scoop bucket repository locally
    update          Update manifest with latest version
    test            Test manifest locally
    install         Install package locally
    help            Show this help message

Options:
    -v, --version   Specify version (default: auto-detect)
    -r, --repo      Bucket repository name (default: $BUCKET_REPO)
    -h, --help      Show this help message

Examples:
    $0 setup                    # Set up Scoop bucket
    $0 update -v 0.6.0         # Update to specific version
    $0 test                    # Test manifest locally
    $0 install                 # Install package locally

EOF
}

setup_scoop_bucket() {
    log_info "Setting up Scoop bucket repository: $BUCKET_REPO"

    if [ -d "$BUCKET_REPO" ]; then
        log_warn "Directory $BUCKET_REPO already exists"
        read -p "Do you want to remove it and start fresh? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$BUCKET_REPO"
        else
            log_info "Using existing directory"
            return
        fi
    fi

    # Check if Scoop is installed
    if ! command -v scoop >/dev/null 2>&1; then
        log_error "Scoop not found. Please install Scoop first:"
        log_error "https://scoop.sh/"
        exit 1
    fi

    # Clone bucket repository
    log_info "Cloning Scoop bucket repository"
    git clone "https://github.com/devnullvoid/$BUCKET_REPO.git"

    cd "$BUCKET_REPO"

    # Create bucket directory if it doesn't exist
    mkdir -p bucket

    # Create manifest if it doesn't exist
    if [ ! -f "bucket/proxmox-tui.json" ]; then
        log_info "Creating Scoop manifest"
        cat > bucket/proxmox-tui.json << 'EOF'
{
    "version": "VERSION_PLACEHOLDER",
    "description": "A terminal user interface (TUI) for Proxmox VE",
    "homepage": "https://github.com/devnullvoid/proxmox-tui",
    "license": "MIT",
    "architecture": {
        "64bit": {
            "url": "https://github.com/devnullvoid/proxmox-tui/releases/download/vVERSION_PLACEHOLDER/proxmox-tui_VERSION_PLACEHOLDER_windows_amd64.zip",
            "hash": "SHA256_PLACEHOLDER_AMD64"
        },
        "32bit": {
            "url": "https://github.com/devnullvoid/proxmox-tui/releases/download/vVERSION_PLACEHOLDER/proxmox-tui_VERSION_PLACEHOLDER_windows_386.zip",
            "hash": "SHA256_PLACEHOLDER_386"
        },
        "arm64": {
            "url": "https://github.com/devnullvoid/proxmox-tui/releases/download/vVERSION_PLACEHOLDER/proxmox-tui_VERSION_PLACEHOLDER_windows_arm64.zip",
            "hash": "SHA256_PLACEHOLDER_ARM64"
        }
    },
    "bin": "proxmox-tui.exe",
    "checkver": {
        "url": "https://github.com/devnullvoid/proxmox-tui/releases/latest",
        "re": "v([\\d.]+)"
    },
    "autoupdate": {
        "architecture": {
            "64bit": {
                "url": "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_windows_amd64.zip",
                "hash": "sha256"
            },
            "32bit": {
                "url": "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_windows_386.zip",
                "hash": "sha256"
            },
            "arm64": {
                "url": "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_windows_arm64.zip",
                "hash": "sha256"
            }
        }
    },
    "notes": "proxmox-tui requires a Proxmox VE server to connect to. Run 'proxmox-tui --help' to see configuration options."
}
EOF
    fi

    log_info "Scoop bucket setup complete in $BUCKET_REPO/"
    log_info "Next steps:"
    log_info "1. cd $BUCKET_REPO"
    log_info "2. Update manifest with latest version and checksums"
    log_info "3. Commit and push changes"
    log_info "4. Users can install with: scoop bucket add devnullvoid/proxmox-tui && scoop install proxmox-tui"
}

update_manifest() {
    local version=${1:-$VERSION}

    if [ ! -d "$BUCKET_REPO" ]; then
        log_error "Scoop bucket not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$BUCKET_REPO"

    log_info "Updating manifest to version $version"

    # Update version in manifest
    sed -i.bak "s/\"version\": \"VERSION_PLACEHOLDER\"/\"version\": \"$version\"/g" bucket/proxmox-tui.json
    sed -i.bak "s/VERSION_PLACEHOLDER/$version/g" bucket/proxmox-tui.json

    # Download and calculate checksums for all platforms
    log_info "Downloading binaries and calculating checksums..."

    # Windows AMD64
    if curl -fsSL -o /tmp/proxmox-tui-windows-amd64.zip "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_windows_amd64.zip" 2>/dev/null; then
        SHA256_AMD64=$(shasum -a 256 /tmp/proxmox-tui-windows-amd64.zip | cut -d' ' -f1)
        sed -i.bak "s/SHA256_PLACEHOLDER_AMD64/$SHA256_AMD64/" bucket/proxmox-tui.json
        log_info "Updated Windows AMD64 checksum: $SHA256_AMD64"
    fi

    # Windows 386 (32-bit)
    if curl -fsSL -o /tmp/proxmox-tui-windows-386.zip "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_windows_386.zip" 2>/dev/null; then
        SHA256_386=$(shasum -a 256 /tmp/proxmox-tui-windows-386.zip | cut -d' ' -f1)
        sed -i.bak "s/SHA256_PLACEHOLDER_386/$SHA256_386/" bucket/proxmox-tui.json
        log_info "Updated Windows 386 checksum: $SHA256_386"
    fi

    # Windows ARM64
    if curl -fsSL -o /tmp/proxmox-tui-windows-arm64.zip "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_windows_arm64.zip" 2>/dev/null; then
        SHA256_ARM64=$(shasum -a 256 /tmp/proxmox-tui-windows-arm64.zip | cut -d' ' -f1)
        sed -i.bak "s/SHA256_PLACEHOLDER_ARM64/$SHA256_ARM64/" bucket/proxmox-tui.json
        log_info "Updated Windows ARM64 checksum: $SHA256_ARM64"
    fi

    # Clean up temporary files
    rm -f /tmp/proxmox-tui-*.zip

    # Remove backup files
    rm -f bucket/proxmox-tui.json.bak

    log_info "Manifest updated to version $version"
    log_info "Review changes and commit them"
}

test_manifest() {
    if [ ! -d "$BUCKET_REPO" ]; then
        log_error "Scoop bucket not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$BUCKET_REPO"

    log_info "Testing manifest locally..."

    # Add local bucket
    scoop bucket add . devnullvoid/proxmox-tui

    # Test manifest
    scoop install proxmox-tui

    log_info "Manifest test completed"
}

install_package() {
    if [ ! -d "$BUCKET_REPO" ]; then
        log_error "Scoop bucket not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$BUCKET_REPO"

    log_info "Installing package locally..."

    # Add local bucket
    scoop bucket add . devnullvoid/proxmox-tui

    # Install package
    scoop install proxmox-tui

    log_info "Package installed successfully"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        setup)
            COMMAND="setup"
            shift
            ;;
        update)
            COMMAND="update"
            shift
            ;;
        test)
            COMMAND="test"
            shift
            ;;
        install)
            COMMAND="install"
            shift
            ;;
        help|--help|-h)
            show_help
            exit 0
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -r|--repo)
            BUCKET_REPO="$2"
            shift 2
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Default command
COMMAND=${COMMAND:-"help"}

# Execute command
case $COMMAND in
    setup)
        setup_scoop_bucket
        ;;
    update)
        update_manifest
        ;;
    test)
        test_manifest
        ;;
    install)
        install_package
        ;;
    help)
        show_help
        ;;
    *)
        log_error "Unknown command: $COMMAND"
        show_help
        exit 1
        ;;
esac
