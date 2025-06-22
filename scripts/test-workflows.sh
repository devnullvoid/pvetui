#!/bin/bash

# Script to test GitHub Actions workflows locally using act
# Automatically detects and works with Docker or Podman
# Usage: ./scripts/test-workflows.sh [workflow] [job]
# Examples:
#   ./scripts/test-workflows.sh                    # Interactive menu
#   ./scripts/test-workflows.sh ci                 # Run all CI jobs
#   ./scripts/test-workflows.sh ci lint            # Run specific CI job
#   ./scripts/test-workflows.sh release            # Test release workflow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if act is installed
if ! command -v act &> /dev/null; then
    print_error "act is not installed. Please install it first:"
    echo "  - GitHub: https://github.com/nektos/act"
    echo "  - Arch: sudo pacman -S act"
    echo "  - Homebrew: brew install act"
    echo "  - Also requires Docker or Podman to be installed"
    exit 1
fi

# Setup container runtime for act
setup_container_runtime() {
    print_info "Detecting container runtime..."
    
    # Check for Podman first (some systems alias docker to podman)
    if command -v podman &> /dev/null; then
        print_info "Setting up Podman..."
        
        # Check if podman socket is running
        if ! systemctl --user is-active --quiet podman.socket; then
            print_info "Starting Podman socket..."
            if ! systemctl --user start podman.socket; then
                print_error "Failed to start Podman socket"
                exit 1
            fi
        fi
        
        # Set DOCKER_HOST for act to use Podman
        export DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock
        
        if [ -S "/run/user/$(id -u)/podman/podman.sock" ]; then
            print_success "Podman socket is ready at $DOCKER_HOST"
            return 0
        else
            print_error "Podman socket not found at /run/user/$(id -u)/podman/podman.sock"
            print_error "Make sure Podman is installed and socket is enabled:"
            echo "  systemctl --user enable --now podman.socket"
            exit 1
        fi
    fi
    
    # Check for Docker as fallback
    if command -v docker &> /dev/null && docker info &> /dev/null; then
        print_success "Docker detected and running"
        # Docker is available and running, act will use it by default
        # Make sure DOCKER_HOST is not set to avoid conflicts
        unset DOCKER_HOST
        return 0
    fi
    
    # Neither Docker nor Podman available
    print_error "Neither Docker nor Podman is available"
    echo "Please install one of the following:"
    echo "  - Docker: https://docs.docker.com/get-docker/"
    echo "  - Podman: https://podman.io/getting-started/installation"
    exit 1
}

# List available workflows and jobs
list_workflows() {
    print_info "Available workflows and jobs:"
    echo
    act --list
    echo
}

# Test CI workflow
test_ci() {
    local job=$1
    
    print_info "Testing CI workflow..."
    
    if [ -n "$job" ]; then
        print_info "Running specific CI job: $job"
        act -j "$job" pull_request
    else
        print_info "Running all CI jobs"
        act pull_request
    fi
}

# Test release workflow (requires creating a temporary tag)
test_release() {
    print_info "Testing release workflow..."
    
    # Check if there are uncommitted changes
    if ! git diff --quiet || ! git diff --cached --quiet; then
        print_warning "You have uncommitted changes. Consider committing them first."
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
    # Create a temporary test tag
    local test_tag="v0.0.0-test-$(date +%s)"
    print_info "Creating temporary test tag: $test_tag"
    git tag "$test_tag"
    
    # Function to cleanup tag
    cleanup_tag() {
        print_info "Cleaning up test tag: $test_tag"
        git tag -d "$test_tag" 2>/dev/null || true
    }
    
    # Set trap to cleanup tag on exit
    trap cleanup_tag EXIT
    
    print_info "Running release workflow with test tag..."
    print_warning "This will test the workflow but won't actually create a GitHub release"
    
    # Run the release workflow
    if act -j release push; then
        print_success "Release workflow test completed successfully!"
    else
        print_error "Release workflow test failed"
        exit 1
    fi
}

# Interactive menu
show_menu() {
    echo
    print_info "GitHub Actions Workflow Tester"
    echo "================================"
    echo
    echo "1) List all workflows and jobs"
    echo "2) Test CI workflow (all jobs)"
    echo "3) Test CI - Lint job only"
    echo "4) Test CI - Test job only"
    echo "5) Test CI - Build job only"
    echo "6) Test Release workflow"
    echo "7) Custom act command"
    echo "0) Exit"
    echo
    read -p "Select option (0-7): " choice
    
    case $choice in
        1) list_workflows ;;
        2) test_ci ;;
        3) test_ci "lint" ;;
        4) test_ci "test" ;;
        5) test_ci "build" ;;
        6) test_release ;;
        7) 
            read -p "Enter act command (without 'act'): " custom_cmd
            print_info "Running: act $custom_cmd"
            act $custom_cmd
            ;;
        0) exit 0 ;;
        *) 
            print_error "Invalid option"
            show_menu
            ;;
    esac
}

# Main script logic
main() {
    # Setup container runtime
    setup_container_runtime
    
    # Parse command line arguments
    if [ $# -eq 0 ]; then
        # No arguments - show interactive menu
        while true; do
            show_menu
            echo
            read -p "Press Enter to continue or Ctrl+C to exit..."
            echo
        done
    elif [ "$1" = "list" ]; then
        list_workflows
    elif [ "$1" = "ci" ]; then
        test_ci "$2"
    elif [ "$1" = "release" ]; then
        test_release
    else
        print_error "Unknown workflow: $1"
        echo
        echo "Usage: $0 [workflow] [job]"
        echo "  workflow: ci, release, list"
        echo "  job: lint, test, build (for CI workflow)"
        echo
        echo "Examples:"
        echo "  $0                    # Interactive menu"
        echo "  $0 list              # List workflows"
        echo "  $0 ci                # Test all CI jobs"
        echo "  $0 ci lint           # Test CI lint job"
        echo "  $0 release           # Test release workflow"
        exit 1
    fi
}

# Run main function
main "$@" 