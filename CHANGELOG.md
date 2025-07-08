# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2025-07-08

### Added
- **Configurable Key Bindings**: Added support for customizing all major actions via the `key_bindings` section in the config file.
- **View Switching with Brackets**: Changed default view switching keys to `]` (forward) and `[` (reverse) for better reliability across terminals.
- Support for SOPS/age encrypted configuration files with automatic key lookup
- `.sops.yaml` for convenient encryption of config files with SOPS
- Log message when encrypted config is decrypted
- **NixOS LXC Container Support**: Added automatic detection and proper shell access for NixOS containers
  - Detects NixOS containers based on `OSType` configuration ("nixos" or "nix")
  - Uses `pct exec` with environment setup for NixOS containers instead of standard `pct enter`
  - Automatically sources `/etc/set-environment` if present for proper NixOS environment initialization
  - Maintains backward compatibility with standard LXC containers
  - Enhanced user feedback showing "NixOS LXC container" vs "LXC container" during connection
  - Comprehensive test coverage for all container types
### Fixed
- **Keybinding Reliability**: Overhauled the keybinding system to correctly handle modifier keys (`Ctrl`, `Alt`, `Shift`), fixing numerous issues with custom shortcuts.
- **Shell Connection Issues**: Fixed VM shell connections that were failing due to broken QEMU guest agent approach.
- **GitHub Workflow Fixes**: Added `submodules: recursive` to all GitHub Actions checkout steps to properly handle noVNC submodule during builds.
- **Windows ARM64 Support**: Added Windows ARM64 build target to both Makefile and GitHub release workflow.
- **VM/Container Restart**: Fixed 500 error when restarting VMs and containers by using correct `/status/reboot` endpoint (both QEMU and LXC use this endpoint, not `/status/restart`)
- **CI Linting**: Fixed golangci-lint configuration compatibility issues by migrating to v2 format
- **Code Quality**: Fixed variable shadowing issues in app initialization and cache tests
- Refresh VNC session `LastUsed` timestamp on all WebSocket proxy traffic to prevent unexpected timeouts

### Improved
- **Code Quality Workflow**: Added `go vet` to CI pipeline and development workflow for enhanced static analysis
  - New `make vet` target for running Go's built-in static analyzer
  - New `make code-quality` target combining `go vet` and `golangci-lint` for comprehensive checks
  - CI now runs `go vet` before `golangci-lint` to catch additional issues early
