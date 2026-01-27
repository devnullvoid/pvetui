# Release Process

This document outlines how to create releases with changelog content included in GitHub releases.

## Prerequisites

- [GitHub CLI](https://cli.github.com/) installed and authenticated
- Write access to the repository
- Updated `CHANGELOG.md` with the new version
- Release announcement secrets configured in GitHub:
  - `MASTODON_SERVER`, `MASTODON_CLIENT_ID`, `MASTODON_CLIENT_SECRET`, `MASTODON_ACCESS_TOKEN`
  - `BLUESKY_USERNAME`, `BLUESKY_APP_PASSWORD`

## Testing Workflows Locally

Before creating releases, you can test the GitHub Actions workflows locally using [act](https://github.com/nektos/act):

### Prerequisites
- [act](https://github.com/nektos/act) installed
- Either Docker or Podman installed and running
- The script automatically detects which container runtime you have

### Quick Testing
```bash
# Interactive menu for testing workflows
./scripts/test-workflows.sh

# Test CI workflow
./scripts/test-workflows.sh ci

# Test specific CI jobs
./scripts/test-workflows.sh ci lint
./scripts/test-workflows.sh ci test

# Test release workflow (creates temporary tag)
./scripts/test-workflows.sh release

# List available workflows and jobs
./scripts/test-workflows.sh list
```

The script automatically handles Podman configuration and provides colored output for easy debugging.

## Release Process

### 1. Update the Changelog

Before creating a release, update `CHANGELOG.md`:

1. Move items from `[Unreleased]` section to a new version section
2. Use format: `## [X.Y.Z] - YYYY-MM-DD`
3. Add a new empty `[Unreleased]` section at the top

Example:
```markdown
## [Unreleased]

## [0.5.0] - 2025-01-20

### Added
- Auto-refresh functionality with 'A' hotkey toggle
- Guest data loading indicator on app startup

### Fixed
- VM selection preservation during operations
```

### 2. Create Release (Option A: Manual Script)

Use the provided script for manual releases:

```bash
# Create a draft release with changelog content
./scripts/create-release.sh v0.5.0

# Review the draft release in GitHub
# Publish when ready:
gh release edit v0.5.0 --draft=false
```

### 3. Create Release (Option B: Git Tag)

Push a git tag to trigger automated release:

```bash
# Create and push tag
git tag v0.5.0
git push origin v0.5.0

# The GitHub Actions workflow will:
# - Build binaries for all platforms
# - Extract changelog content for the version
# - Create GitHub release with changelog as description
# - Upload binaries and checksums
```

## Release Checklist

- [ ] Update `CHANGELOG.md` with new version
- [ ] Commit changelog changes
- [ ] Create release (script or tag)
- [ ] Verify release notes include changelog content
- [ ] Test download links work
- [ ] Announce release if needed

## Troubleshooting

### Changelog Not Found
If you get "Version not found in CHANGELOG.md":
- Ensure the version format matches exactly: `## [X.Y.Z] - YYYY-MM-DD`
- Check there are no extra spaces or formatting issues

### Empty Release Notes
If release notes are empty:
- Verify the changelog section has content between version headers
- Check the sed command works: `sed -n "/## \[0.4.0\]/,/^## \[/p" CHANGELOG.md | sed '/^## \[/d' | tail -n +2`

### GitHub CLI Issues
If `gh` commands fail:
- Authenticate: `gh auth login`
- Check repository access: `gh repo view`
