# AGENT INSTRUCTIONS

The following conventions must be followed for any changes in this repository.

## Initial Setup

1. Ensure git submodules are initialized: `git submodule update --init --recursive`.
2. Run development setup: `make dev-setup` (installs required tools and validates environment).
3. For enhanced development experience (optional):
   - Install direnv: `sudo pacman -S direnv` or equivalent
   - Copy `.envrc.example` to `.envrc` and configure as needed
   - Install pre-commit hooks: `pre-commit install`

## Development Workflow

1. Confirm the application builds with `make build`.
2. Run comprehensive code quality checks with `make code-quality` (includes `go vet` and `golangci-lint`).
3. Execute all tests with `make test`.
4. For integration tests: `make test-integration` (requires Proxmox environment).
5. Keep the working tree clean before finishing.

## Code Quality Standards

- All code must pass `make code-quality` without errors (includes go vet and golangci-lint).
- Maintain test coverage; add tests for new functionality.
- Use table-driven tests where appropriate.
- Mock external dependencies in unit tests.

## Style Guidelines

- Follow idiomatic Go and clean architecture practices.
- Apply Clean Architecture: handlers → services → repositories → domain models.
- Prefer small, focused interfaces and dependency injection via constructors.
- Use interface-driven development; public functions should accept interfaces, not concrete types.
- Document all exported identifiers with comprehensive GoDoc comments including:
  - Package-level documentation explaining purpose and usage patterns
  - Function documentation with parameter descriptions and examples
  - Type documentation with use cases and thread safety considerations
- Handle errors explicitly; wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Use context propagation for request-scoped values, deadlines, and cancellations.

## Documentation Requirements

- Update `CHANGELOG.md` under **[Unreleased]** section with user-visible changes.
- Add GoDoc examples for complex public APIs.
- Update relevant documentation files when changing behavior.

## Commit Standards

- Write concise commit messages (imperative mood, present tense).
- Use simple, descriptive messages stating what was done.
- Include relevant emojis when appropriate.
- Ask about committing after successfully implementing features.

## Security and Performance

- Validate and sanitize all external inputs.
- Implement proper error handling and timeouts for external calls.
- Use secure defaults for authentication and configuration.
- Profile and benchmark performance-critical code paths.

## Tools and Environment

- Go version is pinned in `.go-version` file for consistency.
- Use `golangci-lint` for comprehensive linting (config auto-migrated to v2 format).
- Environment variables can be configured via `.env` file (see `.env.example`).
- Consider using direnv for automatic environment loading (`.envrc.example` provided).
- Pre-commit hooks available for automated code quality checks.

## Testing Strategy

- Unit tests: Fast, isolated, mocked dependencies
- Integration tests: Real system interactions (separate from unit tests)
- Use `make test-quick` for fast feedback during development
- Ensure tests are deterministic and can run in parallel

---

## Project Context

### Overview

**pvetui** is a Terminal User Interface (TUI) for Proxmox Virtual Environment, written in Go. It provides a fast, keyboard-driven interface for managing VMs, containers, nodes, and clusters without requiring the web UI.

### Architecture

#### Key Packages

- **`pkg/api/`** - Proxmox API client with authentication, caching, and HTTP communication
  - `client.go` - Main API client with methods for all Proxmox operations
  - `auth.go` - Authentication manager supporting both password and API token auth
  - `http.go` - HTTP client with retry logic and timeout handling
  - Constants: `DefaultAPITimeout = 30s`, `DefaultRetryCount = 3`

- **`internal/cache/`** - Caching layer with BadgerDB and in-memory implementations
  - `badger_cache.go` - Persistent cache using BadgerDB with proper goroutine cleanup
  - `cache.go` - In-memory FileCache with LRU eviction (using `container/list`)
  - CacheItem stores data as `json.RawMessage` to avoid double marshaling
  - Supports configurable size limits for memory management

- **`internal/ui/`** - TUI components using tview library
  - Main interface with tabbed navigation (Nodes, Guests, Tasks)
  - Context menus, dialogs, forms, and detail panels
  - VNC integration with embedded noVNC client

- **`internal/logger/`** - Unified logging system
  - All components log to single file in cache directory
  - Debug/Info/Warn/Error levels

#### Design Patterns

- **Clean Architecture**: Dependency injection via constructors, interfaces over concrete types
- **Adapter Pattern**: Config and logger adapters bridge internal and pkg interfaces
- **Thread Safety**: All cache operations protected with RWMutex, auth manager uses mutex for token access
- **LRU Eviction**: FileCache uses doubly-linked list for efficient cache management

### Recent Code Quality Improvements (Oct 2025)

Comprehensive code review resulted in these fixes:

1. **Security**: Removed password logging, fixed race condition in auth token retrieval, improved file permissions
2. **Performance**: Eliminated double JSON marshaling, implemented LRU cache with size limits
3. **Reliability**: Added HTTP timeouts, fixed BadgerDB goroutine leak, configurable retry count
4. **Lock File Handling**: Proper PID validation prevents stale lock file issues

### Key Files

- **`cmd/pvetui/main.go`** - Entry point, CLI setup with Cobra
- **`internal/config/config.go`** - Configuration management with SOPS/age encryption support
- **`pkg/api/interfaces/interfaces.go`** - Core interfaces for Logger, Cache, Config
- **`test/testutils/integration_helpers.go`** - Integration test utilities with mock Proxmox server

### Authentication

- Supports both password-based (tickets/CSRF) and API token authentication
- AuthManager handles token caching, expiration, and automatic refresh
- Thread-safe token access with proper locking patterns

### Caching Strategy

- BadgerDB for persistent cache (background GC with proper cleanup)
- FileCache for in-memory with optional persistence
- LRU eviction when cache exceeds maxSize (0 = unlimited)
- Namespaced caches for plugins (separate storage per plugin)

### Testing

- Unit tests with mocked dependencies
- Integration tests require Proxmox environment (controlled by `PVETUI_INTEGRATION_TEST=true`)
- Mock Proxmox server in test utilities for offline testing
- Pre-commit hooks ensure code quality (go vet, golangci-lint, formatting)

### Common Pitfalls

- BadgerDB requires proper cleanup channel to prevent goroutine leaks
- Test files should use 0o600 permissions for sensitive data
- All API methods should have timeouts to prevent indefinite hangs
- Lock file validation must check PID to avoid corruption from stale locks

### Plugin Architecture

- Recently added pluggable architecture for UI extensions
- Plugins disabled by default, opt-in via `plugins.enabled` config
- Community Scripts extracted to plugin
- Namespaced cache support for plugin isolation
