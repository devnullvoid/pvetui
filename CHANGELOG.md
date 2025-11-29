# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.13] - 2025-11-29

### Added

- **Enhanced Guest Search**: Guest search now includes IP addresses and tags in addition to name, ID, type, status, and node name for more comprehensive filtering

### Fixed

- **noVNC Assets with go install**: Fixed missing noVNC vendor files (pako compression library) when installing via `go install` by renaming `vendor/` to `lib/` (Go's embed package excludes directories named 'vendor' by design). The `prune_novnc.sh` script now automatically handles this transformation after updating the noVNC subtree from upstream.
- **Selection Visibility on Windows**: Added reverse video attribute to selected items for better visibility on Windows Terminal with black backgrounds (Vintage, Campbell, IBM 5153 color schemes). Selected nodes/VMs now use inverted colors that work regardless of theme or terminal settings.

## [1.0.12] - 2025-11-24

### Added

- **Command Runner Plugin**: Standardized the Linux host/container/guest command sets and added richer troubleshooting helpers (process sorters, `ip route/link show`, resolver dumps, etc.) plus expanded Windows networking/DNS commands so you can capture CPU, memory, and connectivity data from the same menu.
- **VM SSH User Override**: New `vm_ssh_user` config/flag/env option lets you specify a different SSH username for QEMU VM shells while keeping `ssh_user` for node/LXC access (falls back automatically when omitted).

### Changed

- **Command Runner Plugin**: After closing the command output modal you now land back in the command list, making it much faster to run multiple commands back-to-back.

### Fixed

- **VM Migration Polling**: Fixed migration operations to properly wait for Proxmox task completion via UPID before attempting to poll the target node, eliminating "Configuration file does not exist" errors during active migrations
- **Task Completion Detection**: Improved task completion detection to use the `EndTime` field instead of status string matching, ensuring all task failures (including "migration problems" and other non-standard error messages) are properly caught
- **Post-Operation Refresh Blocking**: Fixed "Cannot refresh data while there are pending operations in progress" message appearing after successful VM operations by clearing pending state immediately after operation completion for all VM lifecycle actions (start, stop, shutdown, restart, reset, delete, and migrate)

## [1.0.11] - 2025-11-20

### Changed

- Guest Insights plugin now uses the full main panel dimensions so its table matches other plugin experiences.
- Tasks page table now expands to the full page width so its columns no longer appear cramped on larger terminals.

### Fixed

- Startup auto-encryption now runs only when plain-text secrets are actually detected, eliminating the repeating “Encrypted sensitive fields” banner and unnecessary config rewrites.
- Non-SOPS config saves no longer duplicate the active connection at the bottom of `config.yml`; sensitive-field encryption now keeps values inside their profile only.

## [1.0.10] - 2025-11-14

### Added

- Plaintext `password`/`token_secret` values in non-SOPS configs are now auto-encrypted on startup, so sensitive fields never linger in cleartext on disk once you've successfully connected.

### Changed

- Replaced the demo-oriented guest list plugin with a "Guest Insights" experience featuring a sortable/filterable table, jump-to-guest navigation, and on-demand metric refreshes so node actions are actually useful during day-to-day ops.
- Cluster status view now recognizes single-node installs, showing a sane 1/1 node count and a "Quorate: N/A" message instead of a scary "No" status.

## [1.0.9] - 2025-11-12

### Added

- **Command Runner Plugin - QEMU VM Support**: Execute whitelisted commands on QEMU VMs via guest agent
  - Commands execute via `/nodes/{node}/qemu/{vmid}/agent/exec` and `/agent/exec-status` endpoints
  - Support for templated commands with parameters (e.g., `systemctl status {service}`)
  - 'C' keyboard shortcut on VMs with guest agent enabled and running
  - Expanded VM command whitelist: `uptime`, `df -h`, `free -h`, `systemctl status`, `journalctl`, `ps aux`, `ip addr show`
  - Polling logic to wait for command completion with proper timeout handling
  - API client adapter to bridge plugin VM struct with full API client types
  - Commands wrapped in `["/bin/sh", "-c", "command"]` for shell feature support

- **Command Runner Plugin - OS-Aware VM Commands**: Detect QEMU guest operating systems and show Linux shell or Windows PowerShell command lists automatically.

- **Command Runner Plugin - Expanded Linux/LXC Utilities**: Added `journalctl -n 50`, `systemctl list-units --type=service --state=running`, `systemctl list-unit-files --state=enabled`, `who`, and `last -n 20` to the default Linux VM and LXC whitelists for faster troubleshooting.

### Fixed

- **Guest Agent Response Parsing**: Fixed critical bug where Proxmox returns `exited` field as integer (0/1) but code attempted to parse as boolean, causing infinite polling loop and "Invalid parameter 'pid'" errors on second poll
- **Version Detection**: `go install` builds now report the correct semantic version by using `debug.ReadBuildInfo()` to extract module metadata instead of hard-coding "vdev".

## [1.0.8] - 2025-11-04

### Changed

- Switched noVNC integration from a git submodule to a git subtree rooted at internal/vnc/novnc, ensuring full compatibility with `go install` and other Go tooling.
- All noVNC assets are now tracked directly in the repository. The update process is now documented in the README, and updating to new versions uses `git subtree pull`.

## [1.0.7] - 2025-10-21

### Added

- Pluggable feature architecture for UI contributions with runtime registration and lifecycle management.
- Community Scripts functionality extracted into the `community-scripts` plugin; enable it via the `plugins.enabled` setting.
- Demo "guest list" plugin that adds a node action presenting running guests in a modal.
- LRU (Least Recently Used) cache eviction with configurable size limits to prevent unbounded memory growth.
- Configurable API retry count via `DefaultRetryCount` constant for easier tuning.
- Manage Plugins dialog in the global menu to toggle plugins, persist configuration changes, and flag the required restart.

### Changed

- Plugins are now disabled by default; update configuration to opt into optional features such as community scripts.
- Configuration files now honour the `plugins.enabled` list instead of falling back to legacy defaults.
- Cache implementation now uses `json.RawMessage` to eliminate double JSON marshaling/unmarshaling overhead.
- FileCache now implements LRU eviction with doubly-linked list for efficient cache management.
- Manage Plugins dialog list now supports Vim-style `j`/`k` navigation keys for faster keyboard control.

### Fixed

- Allow post-operation refreshes to run by clearing VM pending state before triggering automatic data reloads after lifecycle actions.
- Removed potential password exposure from authentication debug logs.
- Fixed race condition in `AuthManager.GetValidToken()` method with improved locking pattern.
- Added HTTP request timeouts to all API methods (30-second default) to prevent indefinite hangs.
- BadgerDB goroutine leak fixed with proper cleanup channel for background garbage collection.
- Badger cache close routine is now idempotent to avoid `close of closed channel` panics during integration tests.
- Lock file handling vulnerability fixed with proper PID validation to prevent cache corruption.
- File permissions in test files changed from 0o644 to 0o600 for better security.

## [1.0.6] - 2025-09-13

### Added

- **VM Action Protection System**: Comprehensive protection mechanism to prevent VM actions while operations are pending
  - **Context Menu Protection**: Lifecycle actions (start, stop, restart, delete, migrate) are hidden when VMs have pending operations
  - **Keyboard Shortcut Protection**: Shell, VNC, and context menu shortcuts are blocked for VMs with pending operations
  - **Visual Indicators**: Pending VMs show dimmed status with special indicators
  - **Menu Title Updates**: Context menu titles show current pending operation status (e.g., "Guest Actions (Starting)")
  - **Snapshot Protection**: Create, delete, and rollback snapshot operations are blocked while VMs have pending operations
  - **Configuration Protection**: VM config editing and storage resizing are blocked during pending operations
  - **Migration Protection**: Migration dialog is blocked for VMs with pending operations
  - **Refresh Protection**: Individual VM refresh and global refresh are blocked while operations are pending
  - **Auto-Refresh Protection**: Auto-refresh cannot be enabled while there are pending operations
  - **Helper Functions**: Added `CanVMPerformActions()` and `GetVMPendingOperation()` for easier pending state checking
  - **Thread-Safe Operations**: All pending state operations use proper mutex protection for concurrent access

### Fixed

- **VM Pending State Timing**: Fixed visual glitch where deleted VMs would briefly return to "normal" state before being removed
  - **Delete Operations**: VMs now stay in pending state until refresh completes and they're removed from the UI
  - **Migration Operations**: VMs stay in pending state until refresh shows them in their new location
  - **Consistent Behavior**: All operations now maintain pending state until refresh operations complete
  - **Better User Experience**: Users can see VMs remain in pending state until operations truly complete

### Dependencies

- **Core Dependencies**: Updated key dependencies to latest versions
  - **github.com/stretchr/testify**: bumped from 1.10.0 to 1.11.1
  - **github.com/rivo/tview**: bumped to 0.42.0
  - **github.com/spf13/cobra**: bumped from 1.9.1 to 1.10.1
  - **golang.org/x/term**: bumped from 0.34.0 to 0.35.0
  - **github.com/gdamore/tcell/v2**: bumped from 2.8.1 to 2.9.0
- **Build Dependencies**: Updated build and CI dependencies
  - **golang**: bumped from 1.24.5-alpine to 1.25.1-alpine
  - **actions/setup-go**: bumped from 5 to 6
  - **actions/checkout**: bumped from 4 to 5

### Refactored

- **VM Migration Code Organization**: Moved migration-specific functions to dedicated file
  - **New File**: `vm_migration.go` created to house all VM migration functionality
  - **Moved Functions**: `showMigrationDialog()` and `performMigrationOperation()` relocated from `dialogs.go`
  - **Clean Separation**: Migration logic now properly separated from general dialog functions
  - **Better Maintainability**: Migration features can now be developed and maintained independently

## [1.0.5] - 2025-08-24

### MAJOR BREAKING CHANGE

- **Project Renamed to pvetui**
  - Rename was necessary in order to remain compliant with Proxmox trademark
  - Old paths referencing `proxmox-tui` must be renamed to `pvetui`. For example:

    ```
    mv ~/.config/proxmox-tui ~/.config/pvetui
    ```

  - **Additional migration steps:**
    - Update any shell aliases or scripts referencing the old binary name
    - Update any systemd service files or cron jobs
    - Update any documentation or bookmarks referencing the old project name
    - **Environment Variables:** Change prefix from `PROXMOX_` to `PVETUI_` (e.g., `PROXMOX_HOST` → `PVETUI_HOST`)
  - **Impact:** This change affects configuration paths, binary names, environment variables, and all project references

### Added

- **Multi-platform Package Distribution**: Added comprehensive support for distributing pvetui through multiple package managers
- **32-bit builds**: Add official 32-bit binaries by request in [#25](https://github.com/devnullvoid/pvetui/issues/25)
  - Linux: `linux/386`
  - Windows: `windows/386`
  - Included in GoReleaser config and local Makefile release target

  - **AUR Support**: Complete Arch User Repository integration with automated PKGBUILD generation and management
  - **Homebrew Tap**: macOS and Linux distribution via Homebrew with automated formula updates
  - **Scoop Bucket**: Windows distribution via Scoop with automated manifest management
  - **DEB/RPM Packages**: Traditional Linux package formats automatically generated by GoReleaser
  - **Docker Images**: Multi-platform container images published to GitHub Container Registry
  - **Orchestration Scripts**: Unified management of all package managers through a single command interface
  - **Automated Updates**: Scripts to automatically update package definitions with new versions and checksums
  - **Local Testing**: Built-in testing and validation for all package formats before publishing
  - **Makefile Integration**: New targets for package manager operations (setup, update, status, clean)
  - **Comprehensive Documentation**: Complete guide covering setup, maintenance, and troubleshooting for all platforms

### Documentation

- **README Updates**: Fixed CLI argument conventions and improved documentation
  - Updated all CLI examples to use correct double-dash format (`--config` instead of `-config`)
  - Added comprehensive CLI reference table with all available flags and short versions
  - Fixed broken anchor links in navigation for better user experience
  - Replaced problematic emojis with compatible ones for consistent anchor generation
  - Updated project title to 'TUI for Proxmox Virtual Environment' for trademark compliance
  - Added trademark disclaimer to clarify non-affiliation with Proxmox Server Solutions GmbH
  - Enhanced demo section with both GIF (GitHub compatible) and WebM (high quality) options
  - Fixed CLI examples throughout all documentation files for consistency

## [1.0.4] - 2025-08-19

### Added

- **Guest name editing**: Added ability to change QEMU VM names and LXC container hostnames from the config page
  - QEMU VMs: Edit the "name" field which updates the VM display name
  - LXC containers: Edit the "hostname" field which updates the container hostname
  - Real-time title updates show the new name as you type
  - Changes are saved to Proxmox and reflected immediately in the UI
  - Input validation prevents invalid hostname characters (underscores, spaces, special chars)
  - Validates hostname format (no leading/trailing hyphens, proper length limits)
  - Fixed UI refresh issue with professional polling approach that verifies API changes before refreshing
  - Added loading indicators during API propagation delay for better user experience
  - Enhanced header component with ShowWarning method for better user feedback
  - Refactored polling functionality into dedicated function for improved maintainability
  - Fixed race condition by polling both config and cluster resources endpoints to ensure complete propagation
- **Cross-platform config and cache paths**: Added native support for Windows config/cache directories
  - Windows: Config in `%APPDATA%/pvetui`, Cache in `%LOCALAPPDATA%/pvetui`
  - macOS: Uses XDG-style paths (`~/.config/pvetui`, `~/.cache/pvetui`) for consistency with other TUI applications
  - Linux: Maintains existing XDG support (`~/.config/pvetui`, `~/.cache/pvetui`)
  - Maintains backward compatibility with existing XDG functions
  - Environment variables still override platform defaults when set

### Breaking Changes

- **Windows users only**: Existing config files in XDG-style paths need to be moved to new platform-specific locations
  - **Windows**: Move from `~/.config/pvetui/` to `%APPDATA%/pvetui/`
  - **macOS/Linux**: No changes required - existing paths continue to work
  - The application will automatically use the new paths on first run after this update

### Fixed

- Community Scripts: returning from installation no longer blanks the screen. The selector now closes before refresh and a brief post-resume delay ensures stable UI restore.
- Data Refresh: new containers/VMs created by community scripts are shown immediately without restarting. After install we trigger a hard refresh (cache cleared) and the manual refresh rebuilds the guest list from fresh cluster data.
- Header: eliminated brief spinner flash that could reappear after success/error messages.
- Manual Refresh stability: VM list now rebuilt strictly from cluster resources; filtering preserved across consecutive refreshes; removed VM details flicker and selection jump by stabilizing selection and suppressing programmatic callbacks during list rebuild.
- VM delete selection fallback: after deleting a VM, selection now moves to the first remaining VM and the details panel updates accordingly; clears details when the list becomes empty.
- Manual Refresh optimization: refactored complex refresh logic into separate functions for better maintainability; reduced UI update calls and improved incremental node enrichment; fixed regression where VM list would become empty after refresh due to enriched nodes not preserving VM data from original cluster resources.

## [1.0.3] - 2025-08-09

### Added

- **Guest power actions:** Added Shutdown (graceful), Stop (force), and Reset (hard, QEMU-only) alongside Restart/Start in the guest context menu, with clear confirmations and shortcuts.

### Fixed

- **Windows: Saving profiles could fail with "The system cannot find the path specified"**
  - Ensure config directory creation uses OS-agnostic path handling when saving from the config/profile wizards and menu actions.
  - Fixes saving when adding/editing profiles and when setting default profile on Windows.
- **Windows: Locally built binaries sometimes failed to start**
  - Align local Windows builds with release artifacts by disabling CGO and using baseline CPU target.
  - Scoped compatibility flags to Windows/amd64 only to avoid affecting other platforms.
- **UI: Reduce noisy page removal errors in logs**
  - Remove pages only when present to avoid benign "Failed to remove … page" errors.

## [1.0.2] - 2025-08-07

### Fixed

- **Profile Switching: Separate Active vs Default profile**
  - Introduced a non-persisted runtime `ActiveProfile` distinct from the persisted `default_profile` in config.
  - Switching profiles in the UI now updates only the active profile; the default indicator and config file are not overwritten.
  - UI header shows the active profile, while the profiles menu star correctly marks the persisted default.
  - Validation prefers the active profile when set, falling back to the default profile.
- **VNC Profile Switching**: Fixed VNC sessions not updating when switching connection profiles
  - VNC service now properly closes all existing sessions when switching profiles
  - Ensures new VNC connections use the updated client connection
  - Prevents VNC sessions from trying to connect to old servers after profile changes
  - Maintains session management integrity across profile switches
- **VNC Browser Opening on Linux**: Fixed VNC connection issues when xdg-open is not available
  - Added detection for missing xdg-open command before attempting to open browser
  - Shows helpful error dialog with shortened VNC URL when browser cannot be opened automatically
  - Implements URL forwarding system: shortened URLs (e.g., `http://localhost:45167/vnc-forward`) automatically redirect to full VNC sessions
  - Uses scrollable text area for long VNC URLs to prevent UI overflow and improve readability
  - Improved dialog positioning and width to properly display long URLs without truncation
  - Enhanced button focus and keyboard handling (Enter/Escape) for proper dialog dismissal
  - Clarifies that the VNC server is still running and ready for connection
  - Prevents confusing situation where VNC server starts but browser doesn't open
  - Provides clear instructions for manual connection with the VNC URL
  - Especially important for WSL and minimal Linux distributions that don't include xdg-open
- **Config Wizard Theme Integration**: Fixed config wizard to use the same theme colors as the main application
  - Config wizard now applies custom theme configuration before setting tview styles
  - Ensures consistent visual appearance between main app and config wizard
  - Fixed input field colors to match the default theme (black instead of blue)
  - Applied to both standalone config wizard and embedded profile wizard
- **Config Wizard Loading Issues**: Fixed config wizard to properly load existing configuration files
  - Fixed config wizard to load from default locations (`~/.config/pvetui/config.yml`)
  - Added profile resolution and application logic to config wizard flow
  - Ensured both `--config-wizard` flag and `config-wizard` subcommand work consistently
  - Fixed issue where config wizard wouldn't load existing profiles when no config file specified
- **Profile Wizard Validation**: Fixed profile wizard to properly recognize filled authentication fields
  - Fixed profile wizard to create profile entries in memory for new profiles
  - Ensured form fields and validation logic work with the same data structure
  - Fixed validation to properly detect when password or token authentication is provided
  - Resolved issue where profile wizard wouldn't recognize filled authentication information
- **Profile Deletion Deadlock**: Fixed deadlock when deleting connection profiles
  - Removed nested `QueueUpdateDraw` calls that caused deadlocks
  - Fixed profile deletion modal to close properly after operation completion
  - Used direct UI updates instead of queued updates to prevent deadlocks
  - Ensured proper focus restoration after profile deletion operations
- **Shell Connection Deadlock**: Fixed deadlock when opening shell to VM without IP address
  - Added `showMessageSafe` function that doesn't use `QueueUpdateDraw` to avoid deadlocks
  - Updated shell functions to use `CreateErrorDialog` for errors and `showMessageSafe` for info messages
  - Fixed issue where screen would flash without showing error message to user
  - Provide clear error message explaining why connection failed and how to fix it
  - Follow same pattern as VNC functions to ensure consistency and prevent deadlocks
- **noVNC Extra Keys Display**: Fixed broken 'extra keys' image display in embedded noVNC client
  - Updated noVNC submodule from v1.6.0 to v1.6.0-11-g4cb5aa4 (11 commits ahead)
  - Includes upstream fix for extra keys image display bug
  - Resolves issue where extra keys button images would not display correctly
- **Guest List Search Selection Mismatch**: Fixed issue where selected item's details didn't match the selected item when searching/filtering
  - Fixed programmatic selection not triggering VM/node changed callbacks
  - Ensures details panel always shows correct information for selected item
  - Applied to search filtering, selection restoration, and VM operations
  - Resolves issue where details panel would show stale information after filtering
- **showMessage Deadlock Prevention**: Updated showMessage calls to use showMessageSafe to prevent deadlocks
  - Fixed showMessage calls in button handlers, event callbacks, and goroutines
  - Applied to VM config forms, snapshot operations, script selector, and connection profiles
  - Prevents UI deadlocks when showing error messages from async operations
  - Ensures consistent user experience without blocking the interface

## [1.0.1] - 2025-08-06

### Added

- **Cobra CLI Framework**: Migrated from Go's standard flag package to cobra for enhanced CLI experience
  - Much better help text formatting with proper descriptions and organization
  - Environment variable support with automatic binding to `PROXMOX_*` variables
  - Subcommand architecture for future extensibility (config-wizard subcommand)
  - Professional CLI interface with improved error handling and validation
  - Maintains 100% backward compatibility with existing functionality
- **Task List Refresh**: Automatically refresh tasks list when VM operations complete
  - Ensures tasks created by VM operations are immediately visible
  - Provides better visibility into operation progress and completion
  - Applied to start/stop/restart operations
  - Delete and migration operations already refresh tasks via manualRefresh()

### Fixed

- **Data Refresh Issues**: Improved data refresh after volume resize and snapshot rollback operations
  - Volume resize now shows updated volume size immediately and displays the resize task
  - Snapshot rollback now shows updated VM status and displays the rollback task
  - Especially important for LXC containers that get shut down after rollback
  - Extracted reusable `refreshVMDataAndTasks` function for consistent behavior
  - Added 2-second delay to allow Proxmox API to update config data before refresh
  - Prevents UI lockup by using non-blocking goroutine for delay
- **VM Selection Preservation**: Fixed selection jumping during pending operations
  - Preserve selected VM by ID and node instead of index position
  - Fixes issue where selected guest would change during pending status
  - Ensures consistent user experience during long-running operations
  - Applied to VM operations (start/stop/restart) and migration operations
- **Makefile Cross-Platform Build**: Fixed hardcoded GOOS/GOARCH in build target ([#19](https://github.com/devnullvoid/pvetui/issues/19))
  - Now builds for host platform by default instead of forcing Linux/amd64
  - Allows environment variable override for cross-compilation
  - Enables native development on macOS, Windows, and other platforms
  - Thanks to @unclesp1d3r for the detailed report and solution
- **Go Install Documentation**: Fixed incorrect installation instructions and improved macOS guidance ([#20](https://github.com/devnullvoid/pvetui/issues/20))
  - Removed non-functional `go install @latest` command (git submodule limitation)
  - Renamed misleading `install-remote` Makefile target to `install-go` for clarity
  - Added macOS Gatekeeper warning in README with direct link to troubleshooting guide
  - Updated troubleshooting documentation with correct installation methods
  - Thanks to @unclesp1d3r for reporting macOS Gatekeeper issues
- **First-Run Configuration Issues**: Fixed app failing to launch without config file ([#21](https://github.com/devnullvoid/pvetui/issues/21))
  - Fixed bootstrap flow to handle config wizard before profile resolution
  - Fixed profile resolution to not assume 'default' profile when no profiles exist
  - Improved onboarding flow with clear user guidance after config creation
  - Ensured --config-wizard flag and config-wizard subcommand work without existing config
  - Maintained full SOPS functionality for encrypted configuration support
  - Thanks to @BenRachmiel for the detailed bug report and reproduction steps

## [1.0.0] - 2025-08-03

### Added

- **Snapshot Management**: Added comprehensive snapshot management for VMs and containers
  - Full CRUD operations: create, delete, and rollback snapshots
  - Proper API integration with Proxmox snapshot endpoints
  - QEMU vs LXC support with VM state handling (QEMU only)
  - Theme-consistent UI with proper keyboard navigation
  - Escape key support for all dialogs and forms
  - Proper handling of 'current' state display as 'NOW'
  - Comprehensive error handling and user feedback
- **Connection Profile Management**: Added comprehensive profile management system for multiple Proxmox connections
  - Profile switching, editing, and persistence with validation
  - Automatic migration from legacy single-connection config
- **Global Menu**: Added comprehensive global menu with intuitive letter-based shortcuts
- **Quit Confirmation**: Added consistent quit confirmation dialog for both hotkey and menu with VNC session awareness
- **Flexible Theming System**: Added comprehensive theming support with automatic terminal emulator adaptation
  - **Semantic Color Constants**: Centralized color management with semantic meaning across themes
  - **Terminal Theme Adaptation**: Automatic adaptation to popular terminal themes (Dracula, Nord, Solarized, etc.)
  - **Configuration Options**: Theme settings in config file with `use_terminal_colors` and `color_scheme` options
  - **Documentation**: Comprehensive theming guide with setup instructions for popular terminal emulators
  - **Zero Configuration**: Works out of the box with most terminal emulators while maintaining semantic consistency
- **Display Node Storage Pools**: Added node storage pools to node details panel
- **Added comprehensive custom theming support**:
  - Users can override all semantic UI colors via the config file (`theme.colors`).
  - Supports hex codes, ANSI color names, and the special value `default`.
  - All themeable color keys are documented in docs/THEMING.md.
  - `use_terminal_colors` config option controls whether to use terminal palette or custom colors.
  - See docs/THEMING.md for full details and configuration examples.
- **Built-in themes**: default, dracula, catppuccin-mocha, gruvbox, nord, rose-pine, tokyonight, solarized, kanagawa, everforest. Users can select a built-in theme with theme.name in the config and override any color.
- **Interactive Config Wizard and Editor**:
  - Added a full-screen, interactive TUI wizard for creating and editing the main config file.
  - Automatically launches on first run if no config is found, or can be invoked at any time with `--config-wizard`.
  - Pre-fills fields from existing config, validates input, and provides clear error/success feedback.
  - Supports both password and token authentication, and SOPS/age-encrypted configs with opt-in re-encryption.
  - All onboarding and config editing flows now use consistent, user-friendly modals and prompts.

### Changed

- **Major Code Refactoring**: Comprehensive refactoring to improve code organization and maintainability
  - Split large UI component files into focused, single-responsibility modules
  - Improved separation of concerns across all UI components
  - Enhanced maintainability and testability while preserving all functionality
- **Logger Architecture Improvements**: Enhanced logging system with better error handling and circular import resolution
- **Configuration Package Split**: Modularized configuration management with separate profile and file operation modules
- **UI Simplification**: Removed redundant Global menu hotkey from footer display since Esc opens the global menu

### Fixed

- Node details panel and API now support displaying multiple storage pools per node, instead of only one. All storage pools are shown with usage stats and theming.
- **FormButton Theming**: Fixed FormButton styling to use proper theme colors instead of hardcoded tview.Styles colors, ensuring consistent appearance with other UI elements
- **FormButton Refactoring**: Refactored FormButton to embed a real tview.Button, providing proper button styling, behavior, and theme consistency
- **FormButton Sizing**: Fixed FormButton to properly size and center the button instead of taking up the entire width
- **FormButton Alignment**: Added configurable positioning options (center, left, right, custom) for FormButton alignment within forms
- **GolangCI-Lint Configuration**: Updated golangci-lint configuration to be compatible with newer versions, fixed GitHub Actions CI failures, and implemented conservative linting with zero errors while maintaining essential code quality checks

## [0.9.0] - 2025-07-16

### Changed

- **Major Refactor:** Split context menu, VM operations, and refresh logic into separate files for improved maintainability and DRYness.
- Improved form and modal UX, including better keyboard navigation and consistent input handling.
- Async feedback and pending state for all VM operations, including migration, with robust UI refresh and error handling.
- Robust selection restoration for both node and VM lists after refresh (fixes selection jump issues).
- Fixed linter/code-quality issues and removed duplicate or unused code.
- Updated and consolidated helpers for refresh and selection logic.
- All changes maintain code quality and pass all tests.

### Added

- **Guest configuration editor:** Edit CPU, memory, and description for both QEMU and LXC guests.
- **Storage volume resize:** Resize disks from the config editor, with robust filtering for resizable volumes only.
- **Interactive First-Run Setup**: Added user-friendly configuration wizard for new users
  - Automatically detects when configuration is missing or incomplete
  - Prompts users to create a default configuration file in the XDG config directory
  - Embeds the configuration template directly in the binary for offline setup
  - Provides clear, friendly messaging with proper spacing and visual indicators
  - Supports both `.yml` and `.yaml` file extensions for configuration discovery
  - Eliminates the need for users to manually create configuration files or read documentation first
- **Startup Connectivity Verification**: Added comprehensive startup sequence with real-time feedback
  - Tests network connectivity and authentication before loading the main interface
  - Clear console progress messages showing each startup step (config loading, client initialization, connection testing, authentication verification)
  - Intelligent error categorization with specific suggestions for different failure types
  - Prevents users from waiting at "Loading..." screens when configuration issues exist
  - Helpful error messages pointing users to the exact config file and suggesting fixes for connection or authentication problems
- Added a reusable custom FormItem (FormButton) for use in forms.

### Fixed

- **VNC Connectivity**: Fixed issue where VNC failed to connect when using SSH port forwarding (e.g., in VS Code). The noVNC client now uses a relative URL, allowing it to connect correctly through forwarded ports.
- Fixed: Auto-refresh countdown and periodic refresh now work correctly after a manual refresh or config edit. Enabling auto-refresh after a manual refresh no longer leaves the UI stuck in 'Refreshing...' state.
- Cleaned up auto-refresh logic: startAutoRefresh only starts ticker/goroutines if not already running, and toggleAutoRefresh only calls stopAutoRefresh when disabling.
- Fixed: Node details (kernel version, CPU model, load average, version) are now preserved after a manual refresh, matching auto-refresh behavior. Previously, these fields would disappear after manual refresh.

## [0.8.1] - 2025-07-10

### Added

- **Docker Image**: Added `openssh-client` to support the shell feature.

### Fixed

- **Configuration**: The application now automatically discovers and loads the default configuration file (`config.yml` or `config.yaml`) from the XDG config directory (`~/.config/pvetui/`) without requiring the `--config` flag.
- **Search**: Pressing `ESC` in the search bar now clears the filter text in addition to closing the bar, providing a more intuitive, VIM-like experience.

### Improved

- **Docker**: The Docker instructions have been completely revamped for clarity and correctness, now recommending `docker compose run --rm pvetui` for an improved user experience.
- Robust selection restoration for both VM and Node lists after per-item and global refreshes. Selection is now always restored by name, not index, fixing issues with selection jumping to the top after refreshes.

## [v0.8.0]

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
  - All components now log to the same `pvetui.log` file in the configured cache directory
  - Eliminated multiple log files being created in current directory (scripts, VNC components)
  - Proper cache directory initialization ensures consistent logging location across all packages

### Improved

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
- Backspace now closes the script details page in the script selector (same as Escape) for faster navigation.
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
  - All components now log to the same `pvetui.log` file in the configured cache directory
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

### Added

- **VI-like Navigation**: Added comprehensive hjkl key support throughout the interface
  - `h` = left/go back, `j` = down, `k` = up, `l` = right

### Fixed

- **Node Storage Display**: Fixed node details showing "0.00 GB" for storage values
  - Resolved inconsistent storage units between cluster and individual node data
  - Node storage values now consistently stored in GB (converted from bytes)
  - Storage percentages now display with correct used/total GB values
  - Maintains consistency with cluster resource processing

## [0.1.0] - Unreleased

- Internal alpha version with basic functionality.
