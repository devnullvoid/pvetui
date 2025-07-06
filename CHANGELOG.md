# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Support for SOPS/age encrypted configuration files with automatic key lookup
- `.sops.yaml` for convenient encryption of config files with SOPS
- Log message when encrypted config is decrypted
### Fixed
- Refresh VNC session `LastUsed` timestamp on all WebSocket proxy traffic to prevent unexpected timeouts

## [0.8.0-rc1] - 2025-07-04

### Added
- **View Switching with Brackets**: Changed default view switching keys to `]` (forward) and `[` (reverse) for better reliability across terminals.
- **Configurable Key Bindings**: Added support for customizing all major actions via the `key_bindings` section in the config file.

### Fixed
- **Keybinding Reliability**: Overhauled the keybinding system to correctly handle modifier keys (`Ctrl`, `Alt`, `Shift`), fixing numerous issues with custom shortcuts.
- **Shell Connection Issues**: Fixed VM shell connections that were failing due to broken QEMU guest agent approach.
- **GitHub Workflow Fixes**: Added `submodules: recursive` to all GitHub Actions checkout steps to properly handle noVNC submodule during builds.
- **Windows ARM64 Support**: Added Windows ARM64 build target to both Makefile and GitHub release workflow.

## [0.7.1] - 2025-07-01

### Fixed
- **noVNC Files Embedding**: Fixed noVNC files to be properly embedded in compiled binary using Go's `//go:embed` directive instead of runtime filesystem access
- **Windows URL Truncation**: Fixed VNC URLs being truncated in Windows browser address bar by replacing `cmd /c start` with `rundll32 url.dll,FileProtocolHandler` to avoid command line length limitations

## [0.7.0] - 2025-06-30

### Added
- **VM/Container Migration**: Added comprehensive migration functionality
  - **Context Menu Integration**: Added "Migrate" option to VM context menu (accessible via 'M' key)
  - **Simplified Migration Dialog**: Streamlined dialog matching Proxmox UI design
    - Target node selection (shows only online nodes excluding current host)
    - Smart migration mode defaults: "restart" for LXC, "online/offline" for QEMU based on VM status
    - Clean confirmation dialog with migration summary
    - Removed complex advanced options in favor of sensible defaults
  - **Enhanced API Implementation**: Full migration API support with improved error handling
    - POST to `/nodes/{node}/{vmtype}/{vmid}/migrate` with detailed response logging
    - Support for both QEMU and LXC migration with type-specific parameters
    - Smart defaults: online migration for running VMs, offline for stopped VMs
    - LXC containers use "restart" migration parameter (restart=1) instead of online parameter
    - Fixed LXC migration API compatibility by removing unsupported migration_type parameter
    - Fixed LXC migration errors by using correct restart parameter for LXC containers
    - Comprehensive error feedback with detailed API response logging
    - Automatic validation of target node availability
  - **Improved User Experience**: Better feedback and error handling
    - Detailed error messages with migration context (VM name, target, mode)
    - API response logging for troubleshooting migration issues
    - Asynchronous operation with progress feedback
    - Automatic refresh after migration to show updated VM location and tasks
    - Migration dialog with minimum height for better visibility on smaller terminals
    - Consistent 2-second refresh delay matching other VM operations
    - Manual refresh (R key) now properly refreshes tasks in addition to nodes/VMs
    - Migration status visible in Tasks tab for monitoring progress
    - Help documentation updated to include migration information

### Fixed
- **Search Filter Persistence**: Fixed issue where search/filtered lists would reset to unfiltered state during auto-refresh and after guest agent data loading
  - Search filters now properly preserved across all refresh operations (manual, auto-refresh, and guest agent enrichment)
  - Fixed key mismatch between search state storage and retrieval (was using lowercase strings instead of proper page constants)
  - Initial data loading now respects existing search filters instead of always showing unfiltered data
  - VM enrichment callback now preserves active search filters when updating with guest agent data

## [0.6.0] - 2025-06-23

### Added
- Automated release script with full workflow automation
- Makefile integration for release commands
- **VM/Container Deletion**: Added delete option to VM/LXC context menu with confirmation
  - Delete option available for all VMs and containers regardless of state
  - Comprehensive confirmation dialog warns about irreversible data destruction
  - Uses DELETE method on `/nodes/{node}/{type}/{vmid}` endpoint as specified
  - **Smart Running VM Handling**: Detects running VMs and offers direct force deletion
  - **Simplified Approach**: Uses force deletion directly for running VMs (no stop-and-delete)
  - **Force Delete Options**: Supports force deletion with `force`, `destroy-unreferenced-disks`, and `purge` parameters
  - **Cache Invalidation**: Clears API cache after deletion to ensure VM is removed from list immediately
  - **Delayed Refresh**: Waits 3 seconds after deletion before refreshing to allow server processing
  - Proper error handling and success feedback with status messages
  - Automatic VM list refresh after successful deletion
  - Specialized delete operation handler that refreshes entire VM list instead of trying to refresh deleted VM
- **Enhanced VM Operations**: Improved all VM operations (start/stop/restart) with auto-refresh
  - **Cache Invalidation**: Clears API cache after each operation for fresh state data
  - **Delayed Refresh**: Waits 2 seconds after operations before refreshing VM data
  - **DRY Implementation**: Unified approach across all VM operations for consistency
  - **Targeted Refresh**: Uses VM-specific refresh to preserve selection and context
  - Immediate success feedback with automatic state updates
- **Cluster Tasks Page**: New dedicated page for viewing recent cluster tasks
  - Access via Tab navigation or F3 key
  - Shows task history with timestamps, status, duration, and details
  - Automatic sorting by newest tasks first
  - Colored status indicators (green for OK, red for errors, yellow for running)
  - Friendly task type formatting (e.g., "VM Start" instead of "qmstart")
  - Auto-refresh integration when tasks page is active
  - Comprehensive task type support for VMs, containers, backups, and system operations
    - **VM Operations**: Start, Stop, Restart, Shutdown, Reset, Reboot, Create, Delete, Clone, Migrate, Restore, Template
    - **Container Operations**: PCT and LXC variants (Start, Stop, Create, Delete, etc.)
    - **System Operations**: APT Update/Upgrade, Service management, Image operations, File transfers
    - **Legacy LXC**: vzcreate, vzstart, vzstop, vzdestroy and other vz* operations
  - **Search Filtering**: Full search support with `/` key activation
    - Real-time filtering across task ID, node, type, status, user, and UPID
    - Search state preservation during auto-refresh operations
    - Integrated with existing search system used by Nodes and Guests pages

### Fixed
- **TUI Suspend/Resume Issue**: Fixed critical issue where users couldn't return to TUI after script installation or SSH sessions
  - Added `app.Sync()` calls after `app.Suspend()` to properly restore terminal state
  - Resolves the problem where "Press Enter to return to the TUI..." would not work
  - Applied fix to both script installation and SSH shell functionality
  - Based on known tview issue where terminal state doesn't restore properly after suspension
  - Users can now successfully return to the application after all suspend operations
- **Unified Logging System**: Fixed all packages to use unified log file instead of separate log files
  - Implemented global logger system that all packages (scripts, VNC services, etc.) now use
  - All components now log to the same `proxmox-tui.log` file in the configured cache directory
  - Eliminated multiple log files being created in current directory (scripts, VNC components)
  - Proper cache directory initialization ensures consistent logging location across all packages

### Enhanced
- **Press Enter to Return**: Re-implemented "Press Enter to return to TUI" functionality for both script installation and SSH sessions
  - Users can now see complete script output and error messages before returning to the application
  - Status messages show success (✅) or failure (❌) with clear feedback
  - Applied to all SSH session types: node shells, LXC containers, QEMU VMs, and guest agent shells
  - Maintains the working suspend/resume pattern while providing better user control
  - Allows users to troubleshoot issues or verify successful installations before continuing
- **Community Script Selector UI**: Converted from modal to full-page view for better usability
  - Provides more screen real estate for script browsing and selection
  - Improved responsive layout that adapts to terminal size
  - Better integration with the overall application navigation flow
- **Community Script Search**: Added search functionality to the script selector
  - Real-time search filtering as you type in the search input field
  - Searches across script names, descriptions, and types (container/VM)
  - Press `/` or `Tab` to activate search mode from the script list
  - Press `Escape` to clear search and return to full script list
  - Press `Enter` or `Tab` to move from search field back to script list
  - Maintains all existing navigation (hjkl, arrows, backspace to go back)
  - Filtered results update instantly and preserve selection behavior

### Improved
- Release process now fully automated from changelog to GitHub release

## [0.5.0] - 2025-06-22

### Added
- Guest data loading indicator on app startup
- Enhanced VM details panel with network interface and storage configuration
- Quit confirmation for active VNC sessions
- Auto-refresh functionality with 'A' hotkey toggle (10-second interval)
- Always-visible status indicators in footer (VNC sessions and auto-refresh status)
- Workflow testing integration in Makefile with targets for local CI testing
- Build tags for examples to prevent linting conflicts

### Fixed
- VM selection and search filter preservation during operations and refreshes
  - VM operations (start/stop/restart) now preserve selected VM position even when status changes
  - Search filters remain active after VM operations and manual refreshes
  - Startup enrichment process preserves user's VM selection if they navigate during loading
  - Selection tracking by VM ID and node instead of list position prevents losing selection when VMs move due to status sorting
- Auto-refresh cache bypass for real-time performance data updates
- Node list ordering consistency during auto-refresh operations
- Manual refresh (R hotkey) VM selection preservation using correct sorted slice
- Logger test panic with nil pointer dereference handling
- Config integration tests with proper environment variable isolation
- Boolean field merging logic in configuration file processing
- Container runtime prioritization (Podman first, Docker fallback)

### Improved
- Network interface display layout in VM details
- Storage configuration display layout
- Footer layout with right-aligned status indicators
- Consistent node list sorting (alphabetical by name)
- Test infrastructure with comprehensive fixes and improvements

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
- **Unified Logging System**: Fixed all packages to use unified log file instead of separate log files
  - Implemented global logger system that all packages (scripts, VNC services, etc.) now use
  - All components now log to the same `proxmox-tui.log` file in the configured cache directory
  - Eliminated multiple log files being created in current directory (scripts, VNC components)
  - Proper cache directory initialization ensures consistent logging location across all packages
  - Eliminates confusion from multiple log files and significantly improves debugging experience

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
  - `h` = left/go back, `j` = down, `k` = up, `
