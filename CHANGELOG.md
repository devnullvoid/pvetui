# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-06-04

### Added
- Initial release of Proxmox TUI
- Comprehensive cluster management interface
- Real-time node and guest monitoring
- Smart guest management with visual status indicators
- BadgerDB-powered caching system for improved performance
- Flexible authentication (username/password and API tokens)
- Interactive SSH shell integration for nodes, VMs, and containers
- VNC console access through browser integration
- Community scripts support for Proxmox Community Scripts repository
- Cross-platform support (Linux, macOS, Windows)
- Keyboard navigation and shortcuts
- Search and filtering capabilities
- Responsive terminal interface built with tview

### Features
- **Node Management**: View detailed node information, status, and resource usage
- **Guest Management**: Monitor VMs and containers with real-time metrics
- **Authentication**: Support for both password and API token authentication
- **Caching**: Intelligent local caching for improved performance
- **Shell Access**: Direct SSH access to nodes and guests
- **VNC Console**: Browser-based VNC access for VMs and nodes
- **Community Scripts**: Install and manage community scripts
- **Cross-Platform**: Native binaries for Linux, macOS, and Windows

### Technical Details
- Built with Go 1.24+ for optimal performance
- Uses tview for the terminal user interface
- BadgerDB for local caching
- Supports both Proxmox username/password and API token authentication
- Cross-platform SSH client integration
- Automatic VNC proxy handling through Proxmox API

[Unreleased]: https://github.com/devnullvoid/proxmox-tui/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/devnullvoid/proxmox-tui/releases/tag/v0.1.0