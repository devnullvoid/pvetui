#!/bin/bash

# Package Manager Orchestration Script for proxmox-tui
# This script manages releases across multiple package managers

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
PKGNAME="proxmox-tui"
VERSION=$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev")
WORKSPACE_DIR=$(pwd)

# Package manager configurations
PACKAGE_MANAGERS=(
    "aur:Arch User Repository:scripts/aur-package.sh"
    "homebrew:Homebrew Tap:scripts/homebrew-tap.sh"
    "scoop:Scoop Bucket:scripts/scoop-bucket.sh"
)

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

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

show_help() {
    cat << EOF
Package Manager Orchestration Script for proxmox-tui

This script manages releases across multiple package managers including:
- AUR (Arch User Repository)
- Homebrew Tap
- Scoop Bucket
- DEB/RPM packages (via GoReleaser)

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    setup           Set up all package manager repositories
    update          Update all package managers to latest version
    release         Create a complete release across all platforms
    status          Show status of all package managers
    clean           Clean up all package manager repositories
    help            Show this help message

Options:
    -v, --version   Specify version (default: auto-detect)
    -p, --platform  Specific platform to operate on (aur|homebrew|scoop|all)
    -d, --dry-run   Preview changes without making them
    -h, --help      Show this help message

Examples:
    $0 setup                    # Set up all package managers
    $0 update -v 0.6.0         # Update all to specific version
    $0 release -v 0.6.0        # Create complete release
    $0 status                   # Show status of all managers
    $0 update -p aur -v 0.6.0  # Update only AUR package

EOF
}

check_prerequisites() {
    log_step "Checking prerequisites..."

    local missing_tools=()

    # Check for required tools
    for tool in git curl; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            missing_tools+=("$tool")
        fi
    done

    # Check for platform-specific tools
    if [[ "$1" == "aur" || "$1" == "all" ]]; then
        if ! command -v makepkg >/dev/null 2>&1; then
            log_warn "makepkg not found. AUR operations may fail."
        fi
    fi

    if [[ "$1" == "homebrew" || "$1" == "all" ]]; then
        if ! command -v brew >/dev/null 2>&1; then
            missing_tools+=("brew")
        fi
    fi

    if [[ "$1" == "scoop" || "$1" == "all" ]]; then
        if ! command -v scoop >/dev/null 2>&1; then
            missing_tools+=("scoop")
        fi
    fi

    if [ ${#missing_tools[@]} -gt 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_info "Please install missing tools and try again."
        exit 1
    fi

    log_info "Prerequisites check passed"
}

setup_package_managers() {
    local platforms=${1:-"all"}

    log_step "Setting up package managers..."

    for pkg_mgr in "${PACKAGE_MANAGERS[@]}"; do
        IFS=':' read -r key name script <<< "$pkg_mgr"

        if [[ "$platforms" == "all" || "$platforms" == "$key" ]]; then
            log_info "Setting up $name..."
            if [ -f "$script" ]; then
                bash "$script" setup
            else
                log_error "Script not found: $script"
            fi
        fi
    done

    log_info "Package manager setup complete"
}

update_package_managers() {
    local platforms=${1:-"all"}
    local version=${2:-$VERSION}

    log_step "Updating package managers to version $version..."

    for pkg_mgr in "${PACKAGE_MANAGERS[@]}"; do
        IFS=':' read -r key name script <<< "$pkg_mgr"

        if [[ "$platforms" == "all" || "$platforms" == "$key" ]]; then
            log_info "Updating $name..."
            if [ -f "$script" ]; then
                bash "$script" update -v "$version"
            else
                log_error "Script not found: $script"
            fi
        fi
    done

    log_info "Package manager updates complete"
}

create_release() {
    local version=${1:-$VERSION}
    local dry_run=${2:-false}

    log_step "Creating release v$version..."

    if [ "$dry_run" = true ]; then
        log_info "DRY RUN MODE - No changes will be made"
    fi

    # Check if we're on a tag
    if ! git describe --exact-match --tags HEAD 2>/dev/null >/dev/null; then
        log_error "Not on a git tag. Please checkout a release tag first."
        exit 1
    fi

    # Verify version matches tag
    local current_tag=$(git describe --exact-match --tags HEAD)
    local expected_tag="v$version"

    if [ "$current_tag" != "$expected_tag" ]; then
        log_error "Version mismatch: expected $expected_tag, got $current_tag"
        exit 1
    fi

    # Create GitHub release first (if not dry run)
    if [ "$dry_run" = false ]; then
        log_info "Creating GitHub release..."
        if command -v gh >/dev/null 2>&1; then
            gh release create "$expected_tag" --generate-notes
        else
            log_warn "GitHub CLI not found. Please create release manually."
        fi
    fi

    # Update all package managers
    update_package_managers "all" "$version"

    # Build packages with GoReleaser
    if [ "$dry_run" = false ]; then
        log_info "Building packages with GoReleaser..."
        if command -v goreleaser >/dev/null 2>&1; then
            goreleaser release --clean
        else
            log_warn "GoReleaser not found. Please install it to build packages."
        fi
    else
        log_info "DRY RUN: Would run: goreleaser release --clean"
    fi

    log_info "Release v$version creation complete"
}

show_status() {
    log_step "Package Manager Status"

    echo
    echo "┌─────────────────────────────────────────────────────────────────┐"
    echo "│                    Package Manager Status                       │"
    echo "├─────────────────────────────────────────────────────────────────┤"

    for pkg_mgr in "${PACKAGE_MANAGERS[@]}"; do
        IFS=':' read -r key name script <<< "$pkg_mgr"

        echo -n "│ $name: "

        if [ -f "$script" ]; then
            # Check if repository exists
            case $key in
                "aur")
                    if [ -d "proxmox-tui" ]; then
                        echo -e "${GREEN}✓ Repository exists${NC}"
                    else
                        echo -e "${YELLOW}⚠ Repository not set up${NC}"
                    fi
                    ;;
                "homebrew")
                    if [ -d "homebrew-proxmox-tui" ]; then
                        echo -e "${GREEN}✓ Repository exists${NC}"
                    else
                        echo -e "${YELLOW}⚠ Repository not set up${NC}"
                    fi
                    ;;
                "scoop")
                    if [ -d "scoop-proxmox-tui" ]; then
                        echo -e "${GREEN}✓ Repository exists${NC}"
                    else
                        echo -e "${YELLOW}⚠ Repository not set up${NC}"
                    fi
                    ;;
            esac
        else
            echo -e "${RED}✗ Script not found${NC}"
        fi
    done

    echo "├─────────────────────────────────────────────────────────────────┤"
    echo "│ Current Version: $VERSION"
    echo "│ Git Tag: $(git describe --tags --always 2>/dev/null || echo 'none')"
    echo "└─────────────────────────────────────────────────────────────────┘"
    echo
}

cleanup_package_managers() {
    local platforms=${1:-"all"}

    log_step "Cleaning up package manager repositories..."

    for pkg_mgr in "${PACKAGE_MANAGERS[@]}"; do
        IFS=':' read -r key name script <<< "$pkg_mgr"

        if [[ "$platforms" == "all" || "$platforms" == "$key" ]]; then
            log_info "Cleaning up $name..."

            case $key in
                "aur")
                    if [ -d "proxmox-tui" ]; then
                        rm -rf "proxmox-tui"
                        log_info "Removed AUR repository"
                    fi
                    ;;
                "homebrew")
                    if [ -d "homebrew-proxmox-tui" ]; then
                        rm -rf "homebrew-proxmox-tui"
                        log_info "Removed Homebrew tap"
                    fi
                    ;;
                "scoop")
                    if [ -d "scoop-proxmox-tui" ]; then
                        rm -rf "scoop-proxmox-tui"
                        log_info "Removed Scoop bucket"
                    fi
                    ;;
            esac
        fi
    done

    log_info "Cleanup complete"
}

# Parse command line arguments
COMMAND="help"
PLATFORMS="all"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        setup|update|release|status|clean)
            COMMAND="$1"
            shift
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -p|--platform)
            PLATFORMS="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        help|--help|-h)
            show_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Validate platforms
if [[ "$PLATFORMS" != "all" ]]; then
    local valid_platforms=("aur" "homebrew" "scoop")
    local valid=false

    for platform in "${valid_platforms[@]}"; do
        if [[ "$PLATFORMS" == "$platform" ]]; then
            valid=true
            break
        fi
    done

    if [ "$valid" = false ]; then
        log_error "Invalid platform: $PLATFORMS"
        log_error "Valid platforms: ${valid_platforms[*]} or 'all'"
        exit 1
    fi
fi

# Execute command
case $COMMAND in
    setup)
        check_prerequisites "$PLATFORMS"
        setup_package_managers "$PLATFORMS"
        ;;
    update)
        check_prerequisites "$PLATFORMS"
        update_package_managers "$PLATFORMS"
        ;;
    release)
        check_prerequisites "$PLATFORMS"
        create_release "$VERSION" "$DRY_RUN"
        ;;
    status)
        show_status
        ;;
    clean)
        cleanup_package_managers "$PLATFORMS"
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
