# AGENT INSTRUCTIONS

**Last Updated:** January 2025 | **For:** pvetui - Proxmox TUI

## Table of Contents
- [Initial Setup](#initial-setup)
- [Development Workflow](#development-workflow)
- [Quick Reference](#quick-reference)
- [Code Quality Standards](#code-quality-standards)
- [Style Guidelines](#style-guidelines)
- [Documentation Requirements](#documentation-requirements)
- [Commit Standards](#commit-standards)
- [Security and Performance](#security-and-performance)
- [Tools and Environment](#tools-and-environment)
- [Testing Strategy](#testing-strategy)
- [Project Context](#project-context)
  - [Overview](#overview)
  - [Architecture](#architecture)
  - [Recent Code Quality Improvements](#recent-code-quality-improvements)
  - [Key Files](#key-files)
  - [Authentication](#authentication)
  - [Caching Strategy](#caching-strategy)
  - [Testing](#testing)
  - [Plugin Architecture](#plugin-architecture)
  - [Plugin Development Guidelines](#plugin-development-guidelines)
  - [Architectural Decision Log](#architectural-decision-log)
- [Common Pitfalls](#common-pitfalls)
- [Troubleshooting](#troubleshooting)

---

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

## Quick Reference

| Task | Command |
|------|---------|
| Build | `make build` |
| Run all checks | `make code-quality && make test` |
| Fast iteration | `make test-quick` |
| Integration tests | `PVETUI_INTEGRATION_TEST=true make test-integration` |
| Install hooks | `pre-commit install` |
| View logs | `tail -f ~/.cache/pvetui/pvetui.log` |
| Clean build | `make clean && make build` |
| Development setup | `make dev-setup` |

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

  **Example:**
  ```go
  // Good
  if err := client.StartVM(vmid); err != nil {
      return fmt.Errorf("failed to start VM %d: %w", vmid, err)
  }

  // Bad - no context
  if err := client.StartVM(vmid); err != nil {
      return err
  }
  ```

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

- Validate and sanitize all external inputs (especially VM names, IDs, and user commands).
- **Never log credentials**: Passwords, API tokens, or CSRF tokens should never appear in logs.
- Implement proper error handling and timeouts for external calls (default: 30s).
- Use secure defaults for authentication and configuration.
- **File permissions**: Config files with secrets must be 0o600, cache directories 0o700.
- Profile and benchmark performance-critical code paths.
- Always validate array indices and map keys before access.
- Use prepared statements or proper escaping for any dynamic command construction.

## Tools and Environment

- Go version is pinned in `.go-version` file for consistency.
- Use `golangci-lint` for comprehensive linting (config auto-migrated to v2 format).
- Environment variables can be configured via `.env` file (see `.env.example`).
- Consider using direnv for automatic environment loading (`.envrc.example` provided).
- Pre-commit hooks available for automated code quality checks.

## Testing Strategy

- **Unit tests**: Fast, isolated, mocked dependencies
- **Integration tests**: Real system interactions (separate from unit tests)
  - Set environment variable: `PVETUI_INTEGRATION_TEST=true`
  - Configure test Proxmox instance in `.env.test` (see `.env.example`)
  - Alternatively, use mock Proxmox server from `test/testutils/integration_helpers.go`
- Use `make test-quick` for fast feedback during development
- Ensure tests are deterministic and can run in parallel
- Table-driven tests are preferred for testing multiple scenarios
- Mock external dependencies using interfaces

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

### Recent Code Quality Improvements

**Note:** For changes older than 6 months, see [CHANGELOG.md](CHANGELOG.md).

Comprehensive code review resulted in these fixes (Oct 2025):

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

### Plugin Architecture

- Recently added pluggable architecture for UI extensions
- Plugins disabled by default, opt-in via `plugins.enabled` config
- Community Scripts extracted to plugin
- Namespaced cache support for plugin isolation

### Plugin Development Guidelines

When developing new plugins:

1. **Location**: Place plugins in `internal/plugins/<plugin-name>/`
2. **Interface**: Implement the plugin interface defined in `internal/plugins/interface.go`
3. **Caching**: Use namespaced cache: `cache.GetNamespaced("plugin:<plugin-name>")`
4. **Registration**: Register plugin in `internal/plugins/registry.go`
5. **Documentation**: Document plugin configuration in `README.md` and `.env.example`
6. **Testing**: Test plugin independently with mocked dependencies
7. **Graceful degradation**: Plugins must gracefully handle being disabled
8. **Configuration**: All plugin settings should have reasonable defaults
9. **Error handling**: Return errors rather than panicking; let the host handle display

### Architectural Decision Log

Key architectural decisions and rationale:

- **BadgerDB over BoltDB**: Chosen for better concurrent read performance and automatic garbage collection
- **tview over bubbletea**: More mature widget system for complex TUI layouts with better documentation
- **Clean Architecture enforcement**: Enables easier testing and future API changes without affecting business logic
- **Plugin opt-in model**: Disabled by default to maintain security and performance baselines; users explicitly enable
- **Interface-driven design**: All public APIs accept interfaces for maximum testability and flexibility
- **Namespaced plugin caching**: Prevents cache key collisions and allows per-plugin cache management

## Common Pitfalls

- BadgerDB requires proper cleanup channel to prevent goroutine leaks
- Test files should use 0o600 permissions for sensitive data
- All API methods should have timeouts to prevent indefinite hangs
- Lock file validation must check PID to avoid corruption from stale locks
- Never log sensitive information (passwords, tokens, API keys)
- Always defer Close() calls immediately after successful resource acquisition
- Use context.WithTimeout() for all external calls, not context.Background()

## Troubleshooting

### Stale Lock File Error

**Symptoms:** Application fails to start with "lock file already exists" error

**Root cause:** Lock file PID validation failing or previous unclean shutdown

**Fix:**
1. Check if pvetui is actually running: `ps aux | grep pvetui`
2. If not running, manually remove: `rm ~/.cache/pvetui/pvetui.lock`
3. If recurring, ensure `LockFile.Unlock()` is properly deferred in all code paths

### BadgerDB Goroutine Leak

**Symptoms:** Increasing goroutine count, memory growth over time

**Root cause:** Missing cleanup channel or improper shutdown sequence

**Fix:**
1. Ensure `badgerCache.Close()` is deferred in all initialization code paths
2. Check that cleanup channel is being listened to and properly closed
3. Use `defer` immediately after successful cache initialization

### Test Timeouts

**Symptoms:** Tests hang indefinitely or timeout after long wait

**Root cause:** Missing context deadline in API calls

**Fix:**
1. Always pass context with timeout (use `DefaultAPITimeout` constant)
2. Example: `ctx, cancel := context.WithTimeout(context.Background(), api.DefaultAPITimeout)`
3. Don't forget to `defer cancel()`

### Integration Tests Failing

**Symptoms:** Integration tests fail with connection errors

**Root cause:** Missing or incorrect Proxmox test environment setup

**Fix:**
1. Ensure `PVETUI_INTEGRATION_TEST=true` is set
2. Configure `.env.test` with valid Proxmox credentials
3. Alternatively, use the mock server: check `test/testutils/integration_helpers.go`

### Code Quality Check Failures

**Symptoms:** `make code-quality` reports linting errors

**Root cause:** Code doesn't meet golangci-lint standards

**Fix:**
1. Run `golangci-lint run` to see detailed errors
2. Many issues can be auto-fixed: `golangci-lint run --fix`
3. For persistent issues, check `.golangci.yml` configuration
4. Ensure your editor is using the project's Go version (see `.go-version`)

### Cache Permission Errors

**Symptoms:** Application crashes with permission denied on cache operations

**Root cause:** Incorrect file permissions on cache directory or files

**Fix:**
1. Cache directory should be 0o700: `chmod 700 ~/.cache/pvetui`
2. Sensitive config files should be 0o600: `chmod 600 ~/.config/pvetui/config.yaml`
3. Check that the application is creating files with correct permissions in the first place
