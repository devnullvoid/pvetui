# AGENT INSTRUCTIONS

The following conventions must be followed for any changes in this repository.

## Initial Setup
1. Run development setup: `make dev-setup` (installs required tools and validates environment).
2. The embedded noVNC client is managed as a git subtree; pull upstream changes when necessary with `make update-novnc`.
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

## Project-Specific Notes
- Manage plugins from the global menu (`showManagePluginsDialog` in `internal/ui/components/plugins_manager.go`). Changes persist to the main config via `SaveConfigToFile` and will trigger SOPS re-encryption if the original file was encrypted—avoid duplicating this logic elsewhere.
- Plugin metadata (ID, name, description) is exposed through `plugins.AvailableMetadata()`, which returns instances sorted by name. Use this helper instead of instantiating plugins manually when building management UIs.
- UI save operations that touch the config expect a restart warning when plugin states change; keep user messaging consistent with `header.ShowSuccess`.
- Shared agent notes live in the Basic Memory project named `pvetui` at `.notes/`; record new discoveries there while exploring the codebase.
