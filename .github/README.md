# GitHub Actions Workflows

This directory contains GitHub Actions workflows for automated CI/CD processes.

## Workflows

### 1. CI Workflow (`.github/workflows/ci.yml`)

**Triggers:**
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop` branches

**Jobs:**
- **Test**: Runs unit tests with coverage reporting
- **Lint**: Runs golangci-lint for code quality checks
- **Build**: Builds the application and creates multi-platform binaries
- **Security**: Runs Gosec security scanner

**Features:**
- Go module caching for faster builds
- Test coverage reporting to Codecov
- Multi-platform binary artifacts
- Security vulnerability scanning

### 2. Release Workflow (`.github/workflows/release.yml`)

**Triggers:**
- Push of tags matching `v*` (e.g., `v1.0.0`, `v2.1.3`)

**Jobs:**
- **Release**: Creates GitHub releases with binaries and Docker images
- **Docker Release**: Builds and pushes multi-arch Docker images

**Features:**
- Automatic changelog generation from git commits
- Multi-platform binary builds (Linux, macOS, Windows)
- SHA256 checksums for all binaries
- Compressed archives (tar.gz for Unix, zip for Windows)
- Docker images pushed to GitHub Container Registry
- Semantic versioning support
- Pre-release detection (versions with `-` are marked as pre-release)

### 3. Docker Workflow (`.github/workflows/docker.yml`)

**Triggers:**
- Push to `main` branch (excluding documentation changes)
- Manual workflow dispatch

**Features:**
- Multi-architecture Docker builds (AMD64, ARM64)
- Automatic tagging with branch name and commit SHA
- Docker layer caching for faster builds
- Push to GitHub Container Registry

### 4. Dependabot Configuration (`.github/dependabot.yml`)

**Features:**
- Weekly dependency updates for Go modules
- Weekly updates for GitHub Actions
- Weekly updates for Docker base images
- Automatic PR creation with proper labeling
- Configurable reviewers and assignees

## Usage

### Creating a Release

1. **Tag your release:**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **The release workflow will automatically:**
   - Run tests to ensure quality
   - Build binaries for all supported platforms
   - Generate checksums
   - Create compressed archives
   - Generate release notes from git history
   - Create a GitHub release with all assets
   - Build and push Docker images

### Supported Platforms

The release workflow builds binaries for:
- **Linux**: AMD64, ARM64
- **macOS**: Intel (AMD64), Apple Silicon (ARM64)
- **Windows**: AMD64

### Docker Images

Docker images are available at:
- `ghcr.io/devnullvoid/proxmox-tui:latest` (latest main branch)
- `ghcr.io/devnullvoid/proxmox-tui:v1.0.0` (specific version)
- `ghcr.io/devnullvoid/proxmox-tui:v1` (major version)

### Environment Variables

The workflows use the following environment variables:
- `GITHUB_TOKEN`: Automatically provided by GitHub Actions
- `CODECOV_TOKEN`: Optional, for enhanced Codecov integration

### Secrets

No additional secrets are required. The workflows use the built-in `GITHUB_TOKEN` for:
- Creating releases
- Pushing to GitHub Container Registry
- Uploading artifacts

## Development

### Running Locally

You can run the same checks locally:

```bash
# Run tests
make test

# Run linting
make lint

# Build release binaries
make release-build

# Build Docker image
make docker-build
```

### Linting Configuration

The project uses golangci-lint with configuration in `.golangci.yml`. The CI workflow runs the same linters to ensure consistency.

### Adding New Workflows

When adding new workflows:
1. Place them in `.github/workflows/`
2. Use descriptive names and comments
3. Follow the existing patterns for caching and error handling
4. Test workflows on feature branches before merging

## Troubleshooting

### Common Issues

1. **Go version mismatch**: Ensure the Go version in workflows matches `go.mod`
2. **Docker build failures**: Check Dockerfile and ensure all dependencies are available
3. **Release failures**: Verify tag format matches `v*` pattern
4. **Permission errors**: Ensure repository has proper permissions for packages

### Debugging

- Check the Actions tab in your GitHub repository
- Review workflow logs for detailed error messages
- Use `workflow_dispatch` triggers for manual testing
- Test Docker builds locally before pushing 