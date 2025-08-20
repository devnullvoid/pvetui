#!/bin/bash

# AUR Package Management Script for proxmox-tui
# This script helps set up and maintain the AUR package repository

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
PKGNAME="proxmox-tui"
AUR_REPO="proxmox-tui"
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
AUR Package Management Script for proxmox-tui

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    setup           Set up AUR repository locally
    update          Update package version and checksums
    build           Build package locally
    install         Install package locally
    clean           Clean build artifacts
    help            Show this help message

Options:
    -v, --version   Specify version (default: auto-detect)
    -r, --repo      AUR repository name (default: $AUR_REPO)
    -h, --help      Show this help message

Examples:
    $0 setup                    # Set up AUR repository
    $0 update -v 0.6.0         # Update to specific version
    $0 build                    # Build package locally
    $0 install                  # Install package locally

EOF
}

setup_aur_repo() {
    log_info "Setting up AUR repository: $AUR_REPO"

    if [ -d "$AUR_REPO" ]; then
        log_warn "Directory $AUR_REPO already exists"
        read -p "Do you want to remove it and start fresh? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$AUR_REPO"
        else
            log_info "Using existing directory"
            return
        fi
    fi

    # Clone AUR repository
    if command -v yay >/dev/null 2>&1; then
        log_info "Using yay to clone AUR repository"
        yay -G "$AUR_REPO"
    elif command -v git >/dev/null 2>&1; then
        log_info "Using git to clone AUR repository"
        git clone "https://aur.archlinux.org/$AUR_REPO.git"
    else
        log_error "Neither yay nor git found. Please install one of them."
        exit 1
    fi

    cd "$AUR_REPO"

    # Create PKGBUILD if it doesn't exist
    if [ ! -f "PKGBUILD" ]; then
        log_info "Creating PKGBUILD"
        cat > PKGBUILD << 'EOF'
# Maintainer: devnullvoid <noreply@github.com>
pkgname=proxmox-tui
pkgver=VERSION_PLACEHOLDER
pkgrel=1
pkgdesc="A terminal user interface (TUI) for Proxmox VE"
arch=('x86_64' 'aarch64')
url="https://github.com/devnullvoid/proxmox-tui"
license=('MIT')
depends=('glibc')
optdepends=('kitty: Better terminal support' 'alacritty: Better terminal support')
provides=('proxmox-tui')
conflicts=('proxmox-tui-git')
source=("$pkgname-$pkgver.tar.gz::https://github.com/devnullvoid/proxmox-tui/archive/v$pkgver.tar.gz")
sha256sums=('SKIP')

build() {
    cd "$pkgname-$pkgver"
    make build
}

package() {
    cd "$pkgname-$pkgver"
    install -Dm755 bin/proxmox-tui "$pkgdir/usr/bin/proxmox-tui"
    install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
    install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
}
EOF
    fi

    log_info "AUR repository setup complete in $AUR_REPO/"
    log_info "Next steps:"
    log_info "1. cd $AUR_REPO"
    log_info "2. Update PKGBUILD with latest version"
    log_info "3. Run: makepkg -si"
}

update_package() {
    local version=${1:-$VERSION}

    if [ ! -d "$AUR_REPO" ]; then
        log_error "AUR repository not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$AUR_REPO"

    log_info "Updating package to version $version"

    # Update PKGBUILD
    sed -i "s/pkgver=.*/pkgver=$version/" PKGBUILD
    sed -i "s/pkgrel=.*/pkgrel=1/" PKGBUILD

    # Download source and get checksum
    log_info "Downloading source and calculating checksum..."
    if command -v updpkgsums >/dev/null 2>&1; then
        updpkgsums
    else
        log_warn "updpkgsums not found. Please install pacman-contrib and run: updpkgsums"
    fi

    log_info "Package updated to version $version"
    log_info "Review changes and run: makepkg -si"
}

build_package() {
    if [ ! -d "$AUR_REPO" ]; then
        log_error "AUR repository not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$AUR_REPO"

    log_info "Building package..."

    if command -v makepkg >/dev/null 2>&1; then
        makepkg -f
    else
        log_error "makepkg not found. Please install base-devel package group."
        exit 1
    fi

    log_info "Package built successfully"
}

install_package() {
    if [ ! -d "$AUR_REPO" ]; then
        log_error "AUR repository not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$AUR_REPO"

    log_info "Installing package..."

    if command -v makepkg >/dev/null 2>&1; then
        makepkg -si
    else
        log_error "makepkg not found. Please install base-devel package group."
        exit 1
    fi
}

clean_build() {
    if [ ! -d "$AUR_REPO" ]; then
        log_error "AUR repository not found. Run '$0 setup' first."
        exit 1
    fi

    cd "$AUR_REPO"

    log_info "Cleaning build artifacts..."

    if command -v makepkg >/dev/null 2>&1; then
        makepkg -c
    else
        log_error "makepkg not found. Please install base-devel package group."
        exit 1
    fi

    log_info "Build artifacts cleaned"
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
        build)
            COMMAND="build"
            shift
            ;;
        install)
            COMMAND="install"
            shift
            ;;
        clean)
            COMMAND="clean"
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
            AUR_REPO="$2"
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
        setup_aur_repo
        ;;
    update)
        update_package
        ;;
    build)
        build_package
        ;;
    install)
        install_package
        ;;
    clean)
        clean_build
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
