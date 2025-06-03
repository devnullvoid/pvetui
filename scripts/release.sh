#!/bin/bash

# Release script for proxmox-tui
# Usage: ./scripts/release.sh <version>
# Example: ./scripts/release.sh v1.0.0

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if version is provided
if [ $# -eq 0 ]; then
    echo -e "${RED}Error: Version is required${NC}"
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

VERSION=$1

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    echo -e "${RED}Error: Invalid version format${NC}"
    echo "Version should follow semantic versioning: v1.0.0, v1.0.0-beta.1, etc."
    exit 1
fi

echo -e "${GREEN}Creating release $VERSION${NC}"

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo -e "${YELLOW}Warning: You're not on the main branch (current: $CURRENT_BRANCH)${NC}"
    read -p "Do you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# Check if working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo -e "${RED}Error: Working directory is not clean${NC}"
    echo "Please commit or stash your changes before creating a release."
    git status --short
    exit 1
fi

# Check if tag already exists
if git tag -l | grep -q "^$VERSION$"; then
    echo -e "${RED}Error: Tag $VERSION already exists${NC}"
    exit 1
fi

# Pull latest changes
echo -e "${YELLOW}Pulling latest changes...${NC}"
git pull origin main

# Run tests
echo -e "${YELLOW}Running tests...${NC}"
make test

# Run linting
echo -e "${YELLOW}Running linting...${NC}"
make lint

# Build release binaries locally to verify
echo -e "${YELLOW}Building release binaries...${NC}"
make release-build

# Show what will be released
echo -e "${GREEN}Release binaries created:${NC}"
ls -la dist/

# Confirm release
echo
echo -e "${YELLOW}Ready to create release $VERSION${NC}"
echo "This will:"
echo "  1. Create and push the tag $VERSION"
echo "  2. Trigger the GitHub Actions release workflow"
echo "  3. Create a GitHub release with binaries and Docker images"
echo
read -p "Do you want to proceed? (y/N): " -n 1 -r
echo

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled."
    # Clean up dist directory
    rm -rf dist/
    exit 0
fi

# Create and push tag
echo -e "${YELLOW}Creating and pushing tag...${NC}"
git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"

# Clean up local dist directory
rm -rf dist/

echo -e "${GREEN}✅ Release $VERSION has been created!${NC}"
echo
echo "The GitHub Actions workflow will now:"
echo "  • Run tests and build binaries"
echo "  • Create a GitHub release with changelog"
echo "  • Upload binaries and checksums"
echo "  • Build and push Docker images"
echo
echo "You can monitor the progress at:"
echo "  https://github.com/devnullvoid/proxmox-tui/actions"
echo
echo "The release will be available at:"
echo "  https://github.com/devnullvoid/proxmox-tui/releases/tag/$VERSION" 