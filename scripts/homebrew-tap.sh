#!/bin/bash

# Homebrew Tap Management Script for proxmox-tui
# This script helps set up and maintain the Homebrew tap repository

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
PKGNAME="proxmox-tui"
TAP_REPO="homebrew-proxmox-tui"
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
Homebrew Tap Management Script for proxmox-tui

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    setup           Set up Homebrew tap repository locally
    update          Update formula with latest version
    test            Test formula locally
    install         Install formula locally
    help            Show this help message

Options:
    -v, --version   Specify version (default: auto-detect)
    -r, --repo      Tap repository name (default: $TAP_REPO)
    -h, --help      Show this help message

Examples:
    $0 setup                    # Set up Homebrew tap
    $0 update -v 0.6.0         # Update to specific version
    $0 test                    # Test formula locally
    $0 install                 # Install formula locally

EOF
}

setup_homebrew_tap() {
    log_info "Setting up Homebrew tap repository: $TAP_REPO"

    if [ -d "$TAP_REPO" ]; then
        log_warn "Directory $TAP_REPO already exists"
        read -p "Do you want to remove it and start fresh? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$TAP_REPO"
        else
            log_info "Using existing directory"
            return
        fi
    fi

    # Check if Homebrew is installed
    if ! command -v brew >/dev/null 2>&1; then
        log_error "Homebrew not found. Please install Homebrew first:"
        log_error "https://brew.sh/"
        exit 1
    fi

    # Clone tap repository
    log_info "Cloning Homebrew tap repository"
    git clone "https://github.com/devnullvoid/$TAP_REPO.git"

    cd "$TAP_REPO"

    # Create Formula directory if it doesn't exist
    mkdir -p Formula

    # Create formula if it doesn't exist
    if [ ! -f "Formula/proxmox-tui.rb" ]; then
        log_info "Creating Homebrew formula"
        cat > Formula/proxmox-tui.rb << 'EOF'
class ProxmoxTui < Formula
  desc "A terminal user interface (TUI) for Proxmox VE"
  homepage "https://github.com/devnullvoid/proxmox-tui"
  version "VERSION_PLACEHOLDER"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/devnullvoid/proxmox-tui/releases/download/v#{version}/proxmox-tui_#{version}_darwin_arm64.tar.gz"
      sha256 "SHA256_PLACEHOLDER_ARM64"
    else
      url "https://github.com/devnullvoid/proxmox-tui/releases/download/v#{version}/proxmox-tui_#{version}_darwin_amd64.tar.gz"
      sha256 "SHA256_PLACEHOLDER_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/devnullvoid/proxmox-tui/releases/download/v#{version}/proxmox-tui_#{version}_linux_arm64.tar.gz"
      sha256 "SHA256_PLACEHOLDER_LINUX_ARM64"
    else
      url "https://github.com/devnullvoid/proxmox-tui/releases/download/v#{version}/proxmox-tui_#{version}_linux_amd64.tar.gz"
      sha256 "SHA256_PLACEHOLDER_LINUX_AMD64"
    end
  end

  def install
    bin.install "proxmox-tui"
  end

  test do
    system "#{bin}/proxmox-tui", "--help"
  end

  def caveats
    <<~EOS
      proxmox-tui requires a Proxmox VE server to connect to.
      Run 'proxmox-tui --help' to see configuration options.
    EOS
  end
end
EOF
    fi

    log_info "Homebrew tap setup complete in $TAP_REPO/"
    log_info "Next steps:"
    log_info "1. cd $TAP_REPO"
    log_info "2. Update formula with latest version and checksums"
    log_info "3. Commit and push changes"
    log_info "4. Users can install with: brew install devnullvoid/proxmox-tui/proxmox-tui"
}

update_formula() {
    local version=${1:-$VERSION}

    if [ ! -d "$TAP_REPO" ]; then
        log_error "Homebrew tap not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$TAP_REPO"

    log_info "Updating formula to version $version"

    # Update version in formula
    sed -i.bak "s/version \"VERSION_PLACEHOLDER\"/version \"$version\"/" Formula/proxmox-tui.rb

    # Download and calculate checksums for all platforms
    log_info "Downloading binaries and calculating checksums..."

    # macOS ARM64
    if curl -fsSL -o /tmp/proxmox-tui-darwin-arm64.tar.gz "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_darwin_arm64.tar.gz" 2>/dev/null; then
        SHA256_ARM64=$(shasum -a 256 /tmp/proxmox-tui-darwin-arm64.tar.gz | cut -d' ' -f1)
        sed -i.bak "s/SHA256_PLACEHOLDER_ARM64/$SHA256_ARM64/" Formula/proxmox-tui.rb
        log_info "Updated macOS ARM64 checksum: $SHA256_ARM64"
    fi

    # macOS AMD64
    if curl -fsSL -o /tmp/proxmox-tui-darwin-amd64.tar.gz "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_darwin_amd64.tar.gz" 2>/dev/null; then
        SHA256_AMD64=$(shasum -a 256 /tmp/proxmox-tui-darwin-amd64.tar.gz | cut -d' ' -f1)
        sed -i.bak "s/SHA256_PLACEHOLDER_AMD64/$SHA256_AMD64/" Formula/proxmox-tui.rb
        log_info "Updated macOS AMD64 checksum: $SHA256_AMD64"
    fi

    # Linux ARM64
    if curl -fsSL -o /tmp/proxmox-tui-linux-arm64.tar.gz "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_linux_arm64.tar.gz" 2>/dev/null; then
        SHA256_LINUX_ARM64=$(shasum -a 256 /tmp/proxmox-tui-linux-arm64.tar.gz | cut -d' ' -f1)
        sed -i.bak "s/SHA256_PLACEHOLDER_LINUX_ARM64/$SHA256_LINUX_ARM64/" Formula/proxmox-tui.rb
        log_info "Updated Linux ARM64 checksum: $SHA256_LINUX_ARM64"
    fi

    # Linux AMD64
    if curl -fsSL -o /tmp/proxmox-tui-linux-amd64.tar.gz "https://github.com/devnullvoid/proxmox-tui/releases/download/v$version/proxmox-tui_$version_linux_amd64.tar.gz" 2>/dev/null; then
        SHA256_LINUX_AMD64=$(shasum -a 256 /tmp/proxmox-tui-linux-amd64.tar.gz | cut -d' ' -f1)
        sed -i.bak "s/SHA256_PLACEHOLDER_LINUX_AMD64/$SHA256_LINUX_AMD64/" Formula/proxmox-tui.rb
        log_info "Updated Linux AMD64 checksum: $SHA256_LINUX_AMD64"
    fi

    # Clean up temporary files
    rm -f /tmp/proxmox-tui-*.tar.gz

    # Remove backup files
    rm -f Formula/proxmox-tui.rb.bak

    log_info "Formula updated to version $version"
    log_info "Review changes and commit them"
}

test_formula() {
    if [ ! -d "$TAP_REPO" ]; then
        log_error "Homebrew tap not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$TAP_REPO"

    log_info "Testing formula locally..."

    # Add local tap
    brew tap --full .

    # Test formula
    brew audit --strict Formula/proxmox-tui.rb
    brew style Formula/proxmox-tui.rb

    log_info "Formula validation completed"
}

install_formula() {
    if [ ! -d "$TAP_REPO" ]; then
        log_error "Homebrew tap not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$TAP_REPO"

    log_info "Installing formula locally..."

    # Add local tap
    brew tap --full .

    # Install formula
    brew install Formula/proxmox-tui.rb

    log_info "Formula installed successfully"
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
            TAP_REPO="$2"
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
        setup_homebrew_tap
        ;;
    update)
        update_formula
        ;;
    test)
        test_formula
        ;;
    install)
        install_formula
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
