# Docker & Podman Support for Proxmox TUI

This document describes how to build and run the Proxmox TUI application using Docker or Podman containers.

## Quick Start

### Using Docker

1. **Set up environment:**
   ```bash
   make dev-setup
   # Edit .env with your Proxmox configuration
   ```

2. **Build and run:**
   ```bash
   make docker-build
   make docker-run
   ```

### Using Podman

1. **Set up environment:**
   ```bash
   make dev-setup
   # Edit .env with your Proxmox configuration
   ```

2. **Build and run:**
   ```bash
   make podman-build
   make podman-run
   ```

### Using Docker Compose

1. **Set up environment:**
   ```bash
   cp .env.example .env
   # Edit .env with your Proxmox configuration
   ```

2. **Start application (interactive):**
   ```bash
   docker-compose up
   ```

   **Note:** For TUI applications, do NOT use `docker-compose up -d` as it runs in detached mode which doesn't work with interactive terminal applications.

## Configuration

### Environment Variables

The application can be configured using environment variables. Copy `.env.example` to `.env` and configure:

```bash
# Required: Proxmox server details
PROXMOX_ADDR=https://your-proxmox-server:8006
PROXMOX_USER=root@pam
PROXMOX_PASSWORD=your-password
PROXMOX_REALM=pam

# Alternative: Use API tokens (recommended for production)
# PROXMOX_TOKEN_ID=your-token-id
# PROXMOX_TOKEN_SECRET=your-token-secret

# Optional: Application settings
PROXMOX_DEBUG=false
PROXMOX_CACHE_DIR=/app/cache
PROXMOX_API_PATH=/api2/json
PROXMOX_INSECURE=false
PROXMOX_SSH_USER=root
```

### Volume Mounts

The container uses the following volume mounts for persistence:

- `./cache:/app/cache` - Application cache data
- `./logs:/app/logs` - Application logs
- `./configs:/app/configs:ro` - Configuration files (read-only)

## Building Images

### Docker

```bash
# Build with default settings
make docker-build

# Build with custom version and registry
VERSION=v1.0.0 REGISTRY=myregistry.com make docker-build

# Build manually
./scripts/docker-build.sh
```

### Podman

```bash
# Build with default settings
make podman-build

# Build with custom version and registry
VERSION=v1.0.0 REGISTRY=myregistry.com make podman-build

# Build manually
./scripts/podman-build.sh
```

## Running Containers

### Docker

```bash
# Run with make
make docker-run

# Run manually
./scripts/docker-run.sh

# Run with custom arguments
./scripts/docker-run.sh --debug --config /app/configs/custom.yml
```

### Podman

```bash
# Run with make
make podman-run

# Run manually
./scripts/podman-run.sh

# Run with custom arguments
./scripts/podman-run.sh --debug --config /app/configs/custom.yml
```

### Docker Compose

```bash
# Start interactively (recommended for TUI apps)
docker-compose up

# Build and start
make compose-build

# Stop services
docker-compose down

# View logs (if running detached)
docker-compose logs -f
```

**Important:** Since this is a TUI application, always run `docker-compose up` without the `-d` flag to maintain interactivity.

## TUI Application Considerations

Since this is a Terminal User Interface (TUI) application, special considerations are needed:

### TTY and Interactive Mode

The container must be run with:
- `-t` (TTY): Allocates a pseudo-TTY
- `-i` (Interactive): Keeps STDIN open

This is automatically handled by the provided scripts.

### Terminal Size

The application will adapt to the terminal size of the host. Resize events are properly forwarded to the container.

### Keyboard Input

All keyboard input is forwarded to the containerized application, including special key combinations.

### VNC Feature

The VNC feature will work in containers by opening VNC consoles in the host's browser, since the container shares the host's network for outbound connections.

## Security

### Non-Root User

The container runs as a non-root user (`appuser:appgroup`) with UID/GID 1001 for security.

### SELinux Support (Podman)

When using Podman on SELinux-enabled systems, volume mounts include the `:Z` flag for proper labeling.

### Network Isolation

The container doesn't expose any ports by default. Network access is only needed for outbound connections to the Proxmox server.

## Troubleshooting

### Common Issues

1. **Permission Denied on Cache/Logs:**
   ```bash
   # Fix permissions
   sudo chown -R 1001:1001 cache logs
   ```

2. **TLS Certificate Issues:**
   ```bash
   # Add to .env for testing (not recommended for production)
   PROXMOX_INSECURE=true
   ```

3. **Container Won't Start:**
   ```bash
   # Check logs
   docker logs proxmox-tui
   # or
   podman logs proxmox-tui
   ```

4. **Environment Variable Issues:**
   Make sure you're using the correct variable names from `config.go`:
   - `PROXMOX_USER` (not `PROXMOX_USERNAME`)
   - `PROXMOX_DEBUG` (not `DEBUG`)
   - `PROXMOX_CACHE_DIR` (not `CACHE_DIR`)

### Debug Mode

Enable debug mode by setting `PROXMOX_DEBUG=true` in your `.env` file:

```bash
PROXMOX_DEBUG=true
```

This will provide verbose logging to help diagnose issues.

### Health Checks

The Docker image includes a health check that verifies the application process is running:

```bash
# Check container health
docker ps
# Look for "healthy" status
```

## Development

### Building for Development

```bash
# Build and test
make build
make test

# Build container and run tests
make docker-build
make docker-test
```

### Multi-Platform Builds

```bash
# Build for multiple platforms
make release-build

# This creates binaries in dist/ for:
# - Linux (amd64, arm64)
# - macOS (amd64, arm64)  
# - Windows (amd64)
```

### Custom Dockerfile

The provided Dockerfile uses multi-stage builds for optimal image size:

1. **Builder stage**: Uses `golang:1.24.2-alpine` to compile the application
2. **Runtime stage**: Uses `alpine:latest` with only the compiled binary

## Available Make Targets

Run `make help` to see all available targets:

```bash
make help
```

Key targets:
- `docker-build` - Build Docker image
- `docker-run` - Run Docker container
- `podman-build` - Build Podman image
- `podman-run` - Run Podman container
- `compose-up` - Start with docker-compose (interactive)
- `compose-build` - Build and start with docker-compose
- `dev-setup` - Set up development environment
- `clean` - Clean build artifacts

## Production Deployment

### Using API Tokens (Recommended)

Instead of passwords, use Proxmox API tokens:

1. Create an API token in Proxmox:
   - Go to Datacenter → Permissions → API Tokens
   - Create a new token with appropriate privileges

2. Configure in `.env`:
   ```bash
   PROXMOX_TOKEN_ID=your-token-id
   PROXMOX_TOKEN_SECRET=your-token-secret
   # Remove or comment out PROXMOX_PASSWORD
   ```

### Resource Limits

The docker-compose.yml includes resource limits:

```yaml
deploy:
  resources:
    limits:
      memory: 256M
      cpus: '0.5'
    reservations:
      memory: 64M
      cpus: '0.1'
```

Adjust these based on your needs.

### Persistent Storage

Ensure cache and logs directories are properly backed up if needed:

```bash
# Create backup
tar -czf proxmox-tui-data-$(date +%Y%m%d).tar.gz cache logs

# Restore backup
tar -xzf proxmox-tui-data-YYYYMMDD.tar.gz
```

## Interactive vs Detached Mode

**Important for TUI Applications:**

- ✅ **Use:** `docker-compose up` (interactive)
- ✅ **Use:** `make docker-run` or `make podman-run`
- ❌ **Don't use:** `docker-compose up -d` (detached mode breaks TUI)

The application requires an interactive terminal to function properly. Detached mode will cause the application to exit immediately or become unresponsive. 