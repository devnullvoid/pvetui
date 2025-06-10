# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **VI-like Navigation**: Added comprehensive hjkl key support throughout the interface
  - `h` = left/go back, `j` = down, `k` = up, `l` = right/select/enter
  - Works in all lists, details panels, and modals
  - Maintains full backward compatibility with arrow keys
- **Help Modal**: Added comprehensive help system accessible with `?` key
  - Detailed keybindings reference organized by category
  - Scrollable content with VI-like navigation (j/k keys)
  - Multiple exit options (?, Escape, q)
  - Professional styling with proper sizing and formatting
  - Includes troubleshooting tips and usage guidance

### Improved
- **Footer Simplification**: Streamlined footer from crowded keybindings to clean essentials
  - Changed from: `F1:Nodes F2:Guests /:Search S:Shell V:VNC C:Scripts M:Menu Tab:Next Tab hjkl:Navigate Q:Quit`
  - To: `Tab:Switch /:Search M:Menu ?:Help Q:Quit`
  - Much cleaner and less overwhelming user experience
- **Navigation Consistency**: All UI components now support both arrow keys and hjkl navigation
  - Node list, node details, VM list, VM details, script selector, context menu
  - Consistent behavior across all interface elements
- **Help System**: Comprehensive documentation accessible within the application
  - No need to reference external documentation for basic usage
  - Context-aware help with detailed explanations
  - Better onboarding experience for new users

### Technical Improvements
- Enhanced keyboard event handling with `tcell.KeyRune` for hjkl support
- Improved modal system using `tview.Pages` for better sizing control
- Better focus management and input capture throughout the interface
- Cleaner code organization with consistent event handling patterns

## [0.1.1] - 2025-06-09

### Fixed
- Fixed VM details panel scroll position to always start at the top
- Prevents auto-scroll to bottom when displaying QEMU guests with extensive filesystem and network data
- Ensures consistent user experience where basic VM information (ID, Name, Node, Type, Status) is always visible first
- Maintains manual scroll functionality for viewing detailed information

### Improved
- Enhanced user experience for VM details viewing, especially for QEMU guests with rich metadata

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

[Unreleased]: https://github.com/devnullvoid/proxmox-tui/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/devnullvoid/proxmox-tui/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/devnullvoid/proxmox-tui/releases/tag/v0.1.0