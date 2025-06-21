# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Quit Confirmation for Active VNC Sessions**: Added user-friendly quit confirmation when VNC sessions are active
  - Application now prompts before quitting when there are active VNC sessions
  - Shows session count and warns that sessions will be disconnected
  - Provides "Yes/No" confirmation dialog to prevent accidental disconnections
  - Immediate quit when no VNC sessions are active (preserves existing behavior)
  - Updated help documentation to reflect new quit behavior

## [0.4.0] - 2025-06-20

### Added
- **Concurrent VNC Sessions**: Support for multiple simultaneous VNC connections
  - Session management system allows connecting to multiple VMs and nodes simultaneously
  - Each VNC session runs on its own dedicated port with independent WebSocket proxy
  - Automatic session tracking with unique identifiers and metadata
  - Real-time session count display in footer (e.g., "VNC:3" for 3 active sessions)
  - Smart session reuse - connecting to the same target returns existing session
  - Automatic cleanup of inactive sessions after 24 hours of inactivity
  - Session lifecycle management with proper resource cleanup on application exit
  - Backward compatibility maintained with existing VNC functionality
- **noVNC Git Submodule Integration**: Replaced manual noVNC file copying with git submodules
  - noVNC client now managed as a git submodule from official repository (v1.6.0)
  - Easy updates to new noVNC versions with standard git commands
  - Improved maintainability and version tracking
  - Added comprehensive documentation for submodule management
  - Requires `git clone --recurse-submodules` for new installations
  - Migrated from embedded filesystem to direct filesystem serving for better flexibility

### Fixed
- **VNC Session Auto-Disconnect**: Removed automatic 30-minute session timeout
  - VNC sessions now remain active for 24 hours instead of 30 minutes
  - Cleanup process runs every 30 minutes instead of every 5 minutes for efficiency
  - Sessions are only cleaned up when truly inactive for extended periods
  - Prevents unexpected disconnections during long VNC sessions
- **VNC Session Count Update Delay**: Implemented real-time session count updates
  - Added callback system for immediate UI updates when sessions are created or removed
  - Session count now updates instantly when browser tabs are closed (within 5 seconds)
  - Reduced polling interval from 30 seconds to 5 seconds as backup mechanism
  - UI footer now reflects accurate session count without delays
  - Improved user experience with responsive session management
- **VNC Session Timeouts**: Fixed VNC connections disconnecting after 20-30 seconds
  - Increased WebSocket proxy timeout from 30 seconds to 30 minutes for long-lived VNC sessions
  - Removed HTTP server read/write timeouts that were terminating WebSocket connections
  - Added WebSocket ping/pong keepalive mechanism with 30-second intervals
  - Enhanced connection stability with proper deadline management and error handling
  - VNC sessions now remain active during periods of user inactivity
  - Improved logging for connection lifecycle and timeout debugging
- **VNC Session Management**: Enhanced session lifecycle and client disconnect detection
  - Added real-time client connection/disconnection tracking for accurate session state
  - Implemented immediate session cleanup when browser tabs are closed
  - Fixed session reuse issues where reconnecting after browser close would fail
  - Added session state management (Active, Connected, Disconnected, Closed)
  - Sessions now properly detect and handle client disconnections
  - Improved session reuse logic to prevent "connection is closed" errors
  - Added 5-second grace period for reconnections to prevent premature cleanup
  - **Fixed session count accuracy**: Disconnected sessions are now properly removed after grace period
  - **Fixed stale VNC ticket reuse**: Sessions are completely removed instead of reused with expired tickets
  - **Reduced excessive logging**: VNC session monitoring reduced from 2-second to 30-second intervals
  - **Improved logging efficiency**: Session count only logged when it changes, not continuously
- **Unified Logging System**: Fixed multiple log files being created in different locations
  - All VNC components now use shared logger instance instead of creating separate loggers
  - Eliminated duplicate log files (internal/cache/, test/integration/, root directory)
  - All logging now unified to single file in cache directory (e.g., ./cache/proxmox-tui.log)
  - VNC service, session manager, WebSocket proxy, and HTTP server all use shared logger
  - Improved logging architecture with proper dependency injection throughout VNC components
  - Better log organization and debugging experience with centralized logging

## [0.3.0] - 2025-06-20

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
  - Implementation inspired by community solution from [Proxmox Forum discussion](https://forum.proxmox.com/threads/proxmox-api-vncwebsocket.73184/page-2)
- **Authentication Handling**: Improved VNC authentication to work correctly with both QEMU VMs and LXC containers
- **VNC User Experience**: Removed outdated VNC warning modal since embedded client no longer requires Proxmox web UI login
- **Comprehensive VNC Logging**: Added detailed logging throughout VNC components for better debugging and monitoring
  - API call logging with request/response details and authentication methods
  - WebSocket proxy logging with connection status, message counts, and error tracking
  - HTTP server logging with port allocation, startup/shutdown, and file serving
  - Service-level logging with connection attempts, status checks, and browser launching
  - Proxy configuration logging with authentication token types and endpoint details
  - Message-level debugging for WebSocket communication (configurable verbosity)

### Fixed
- **Configuration Realm Handling**: Fixed critical bug where config file realm setting was ignored
  - Configuration files now properly merge the `realm` field from YAML config
  - API authentication now uses correct realm (e.g., 'pve' instead of defaulting to 'pam')
  - Resolves authentication failures when using non-PAM realms with API tokens
  - Ensures proper authentication for all Proxmox API operations
- **Node VNC Shell Access**: Resolved node VNC shell functionality by removing unsupported generate-password parameter
  - Node shells now properly authenticate using VNC ticket as password
  - Fixed API compatibility issues specific to node shell VNC endpoints
  - Improved error handling and user feedback for node VNC operations

## [0.2.0] - 2025-06-11

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


[Unreleased]: https://github.com/devnullvoid/proxmox-tui/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/devnullvoid/proxmox-tui/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/devnullvoid/proxmox-tui/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/devnullvoid/proxmox-tui/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/devnullvoid/proxmox-tui/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/devnullvoid/proxmox-tui/releases/tag/v0.1.0