# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Embedded noVNC Client**: Revolutionary VNC console access without requiring Proxmox web UI login
  - Self-contained noVNC client embedded directly in the application
  - Automatic authentication handling for both API token and password authentication
  - WebSocket reverse proxy bridges noVNC client to Proxmox VNC websocket endpoints
  - One-time password generation and automatic connection establishment
  - Local HTTP server hosts noVNC client on dynamically allocated ports
  - Supports QEMU VMs, LXC containers, and node shell sessions
  - No browser session management or manual authentication required
  - Enhanced security with automatic session cleanup and timeout handling
- **Authentication Handling**: Improved VNC authentication to work correctly with both QEMU VMs and LXC containers
- **VNC User Experience**: Removed outdated VNC warning modal since embedded client no longer requires Proxmox web UI login

### Fixed
- **Node Storage Display**: Fixed node details showing "0.00 GB" for storage values
  - Resolved inconsistent storage units between cluster and individual node data
  - Node storage values now consistently stored in GB (converted from bytes)
  - Storage percentages now display with correct used/total GB values
  - Maintains consistency with cluster resource processing

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
- **Script Selector Enhancements**: Major improvements to community scripts installation UI
  - Responsive modal layout that adapts to terminal size with min/max constraints
  - Animated loading indicator with smooth Unicode spinner during GitHub API calls
  - In-modal loading screen with clear progress feedback and time expectations
  - Cancel functionality during loading (Backspace, Escape, h key)
  - Professional loading states that prevent UI freeze perception
- **XDG Base Directory Compliance**: Full compliance with XDG Base Directory specification
  - Config files: `$XDG_CONFIG_HOME/proxmox-tui/config.yml` (defaults to `~/.config/proxmox-tui/config.yml`)
  - Cache directory: `$XDG_CACHE_HOME/proxmox-tui` (defaults to `~/.cache/proxmox-tui`)
  - Log files: Stored in cache directory as `proxmox-tui.log`
  - Automatic config loading from default XDG location if no `-config` flag specified
  - Maintains backward compatibility with existing configurations
- **Comprehensive Logging System**: Enhanced debug logging infrastructure
  - Centralized logger system shared across all application components
  - Detailed application lifecycle logging (startup, initialization, shutdown)
  - API operation tracking (authentication, requests, responses, errors)
  - UI component state changes and user interactions
  - Cache operations and performance metrics
  - All logs stored in XDG-compliant cache directory

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
- **Script Selector Modal**: Enhanced community scripts installation experience
  - Improved modal positioning to prevent clipping in small terminal windows
  - Dynamic height with flexible content scaling instead of fixed dimensions
  - Better visual feedback during 10-15 second GitHub script fetching operations
  - More user-friendly messaging ("may take a moment" vs specific time ranges)
  - Enhanced error handling and graceful fallbacks
- **Configuration Management**: Streamlined configuration and file organization
  - Automatic detection and loading of config files from standard XDG locations
  - Simplified Docker setup with unified cache/log directory structure
  - Better error messages and validation for configuration issues
  - Improved help text with XDG path information
- **Debug Experience**: Significantly enhanced debugging and troubleshooting capabilities
  - Comprehensive logging covers all application operations and state changes
  - Centralized logger prevents missing debug information from different components
  - Better error tracking and context for issue diagnosis
  - Performance monitoring through detailed cache and API operation logging

### Technical Improvements
- Enhanced keyboard event handling with `tcell.KeyRune` for hjkl support
- Improved modal system using `tview.Pages` for better sizing control
- Better focus management and input capture throughout the interface
- Cleaner code organization with consistent event handling patterns
- Advanced animation system with proper goroutine management and cleanup
- Responsive layout calculations with proportional sizing (6:1 ratios)
- Non-blocking UI updates with proper QueueUpdateDraw handling
- Deadlock prevention in concurrent loading operations
- **XDG Standards Implementation**: Complete XDG Base Directory specification compliance
  - Helper functions for XDG cache and config directory resolution
  - Environment variable support (`XDG_CACHE_HOME`, `XDG_CONFIG_HOME`)
  - Graceful fallbacks to standard locations (`~/.cache`, `~/.config`)
  - Unified file organization following Linux desktop standards
- **Logging Architecture Overhaul**: Centralized and comprehensive logging system
  - Shared logger instance across all application components
  - Cache-aware logger that follows XDG directory structure
  - Improved LoggerAdapter with proper dependency injection
  - Enhanced error handling and debug information capture
  - Performance monitoring and operation tracking throughout the application
  - **Logger Function Consolidation**: Eliminated confusing dual logger creation functions
    - Unified `NewInternalLogger(level, cacheDir)` function for all internal logging
    - Removed deprecated `NewInternalLoggerWithCacheDir` to prevent confusion
    - Consistent logger creation pattern across all packages
    - All cache-related logging now properly centralized to cache directory

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