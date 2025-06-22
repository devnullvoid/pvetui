#!/bin/bash

# Script to create a GitHub release with changelog content
# Usage: ./scripts/create-release.sh v0.5.0

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.5.0"
    exit 1
fi

VERSION="$1"
VERSION_NO_V="${VERSION#v}"

# Check if version exists in changelog
if ! grep -q "## \[$VERSION_NO_V\]" CHANGELOG.md; then
    echo "Error: Version $VERSION_NO_V not found in CHANGELOG.md"
    echo "Make sure to update the changelog first:"
    echo "1. Move items from [Unreleased] to [$VERSION_NO_V] - $(date +%Y-%m-%d)"
    echo "2. Add new empty [Unreleased] section"
    exit 1
fi

# Extract changelog content for this version
echo "Extracting changelog content for $VERSION_NO_V..."

# Create temporary file with release notes
TEMP_FILE=$(mktemp)
trap 'rm -f "$TEMP_FILE"' EXIT

# Extract content between version headers
sed -n "/## \[$VERSION_NO_V\]/,/^## \[/p" CHANGELOG.md | sed '/^## \[/d' | tail -n +2 > "$TEMP_FILE"

# Add installation instructions
cat >> "$TEMP_FILE" << 'EOF'

## 📦 Downloads

Choose the appropriate binary for your platform:

- **Linux AMD64**: `proxmox-tui-linux-amd64.tar.gz`
- **Linux ARM64**: `proxmox-tui-linux-arm64.tar.gz`
- **macOS Intel**: `proxmox-tui-darwin-amd64.tar.gz`
- **macOS Apple Silicon**: `proxmox-tui-darwin-arm64.tar.gz`
- **Windows**: `proxmox-tui-windows-amd64.zip`

## 🔐 Verification

Verify your download with the provided `checksums.txt` file:
```bash
shasum -a 256 -c checksums.txt
```

## 📋 Installation

1. Download the appropriate archive for your platform
2. Extract the binary: `tar -xzf proxmox-tui-*.tar.gz` (or unzip for Windows)
3. Make executable (Unix): `chmod +x proxmox-tui-*`
4. Run: `./proxmox-tui-* --help`
EOF

echo "Creating GitHub release $VERSION..."

# Create the release with changelog content
gh release create "$VERSION" \
    --title "Release $VERSION" \
    --notes-file "$TEMP_FILE" \
    --draft

echo "✅ Draft release created successfully!"
echo "   Review at: https://github.com/$(gh repo view --json owner,name -q '.owner.login + "/" + .name')/releases"
echo "   Publish when ready with: gh release edit $VERSION --draft=false" 