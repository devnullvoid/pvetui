# Plugin Guide

This guide explains how to work with the pvetui plugin system, including enabling existing extensions and authoring new ones.

## Overview

- Plugins are discovered through static registration in `internal/ui/plugins/loader.go`.
- At startup pvetui loads the plugin identifiers listed under `plugins.enabled` in your configuration file.
- When `plugins.enabled` is omitted or empty, no optional functionality is activated.

## Enabling Built-in Plugins

The repository currently ships with the following built-in plugins:

- `community-scripts`: exposes the community script installer from the node context menu
- `demo-guest-list`: adds a demo action that lists running guests for the selected node
- `command-runner`: execute whitelisted commands on Proxmox hosts via SSH

```yaml
plugins:
  enabled:
    - "community-scripts"
    - "demo-guest-list"
    - "command-runner"
```

Restart pvetui after editing the configuration to apply the change. If an unknown plugin ID is listed, the application prints a warning similar to `⚠️ Unknown plugins requested: my-plugin` during startup.

## Command Runner Plugin

The `command-runner` plugin enables secure execution of whitelisted commands on Proxmox hosts via SSH. When enabled, it adds a **Run Command (SSH)** action (shortcut: `c`) to the node context menu that appears when a node is online.

### Features

- **Whitelisted commands**: Only pre-approved commands can be executed for security
- **Template support**: Commands can include parameters (e.g., `systemctl status {service}`)
- **Input validation**: Parameters are sanitized to prevent shell injection
- **Output display**: Command results shown in scrollable modal with timing info
- **Timeout protection**: Commands respect configurable timeout (default: 30s)
- **OS-aware guest commands**: The VM menu automatically switches between Linux shell and Windows PowerShell snippets based on the guest OS reported by Proxmox.
- **Size limits**: Output truncated if exceeds max size (default: 1MB)

### Configuration

The plugin uses SSH key-based authentication by default. Ensure SSH keys are configured in `~/.ssh/` for passwordless authentication to your Proxmox hosts.

SSH username is taken from the `ssh_user` field in your pvetui config, falling back to the Proxmox API username if not specified:

```yaml
ssh_user: root  # SSH username for command execution
```

### Default Whitelisted Commands

**For Proxmox hosts:**
- `uptime` - System uptime
- `df -h` - Disk space usage
- `free -h` - Memory usage
- `systemctl status {service}` - Service status (with parameter)
- `journalctl -n 50` - Last 50 journal entries

**For containers:**
- `ps aux` - Process list
- `df -h` - Disk usage
- `apt list --upgradable` - Available updates

**For VMs:**
- **Linux guests**: reuse the shell whitelist (e.g., `uptime`, `systemctl status {service}`, `journalctl -u {service} -n 50`).
- **Windows guests**: commands execute via PowerShell (`Get-ComputerInfo`, `Get-Service`, `Get-Volume`, `Get-NetAdapter`, etc.) with the proper guest agent invocation.

### Security Considerations

- Commands are validated against a strict whitelist before execution
- Parameters cannot contain shell metacharacters (`;`, `|`, `$`, `` ` ``, etc.)
- SSH uses `InsecureIgnoreHostKey` in the initial implementation (TODO: implement proper known_hosts verification)
- All output is size-limited to prevent memory exhaustion

### Usage

1. Enable the plugin in your config (see above)
2. Restart pvetui
3. Navigate to a node in the Nodes view
4. Press `c` (or select from context menu) to run a command
5. Choose from the list of whitelisted commands
6. If the command has parameters (e.g., `{service}`), fill in the form
7. View the output in the scrollable result modal

## Demo Guest List Plugin

The `demo-guest-list` plugin is intentionally small and serves as a reference implementation. When enabled it contributes a node context menu entry labelled **Show Running Guests (Demo)**. Selecting the action opens a modal listing the running guests on the chosen node, including their IDs, types, and discovered IP addresses when available.

## Writing a New Plugin

1. Implement the `components.Plugin` interface (see `internal/ui/components/plugins.go`):
   - `ID() string` must return a stable identifier used in configuration files.
   - `Name()` and `Description()` provide user-facing metadata.
   - `Initialize(ctx, app, registrar)` is called once at startup. Register UI contributions (for example node actions) through the provided `registrar`.
   - `Shutdown(ctx)` should release resources acquired during initialization.
2. Place the implementation in `internal/ui/plugins/<yourplugin>/` and expose a constructor (for example `func New() components.Plugin`).
3. Register the plugin in `internal/ui/plugins/loader.go` by adding an entry to the `registry` map.
4. Add unit tests in `internal/ui/plugins` that cover registration logic and any behaviour that can be exercised without the full TUI runtime.

Plugins may use the `components.App` helper methods passed to `Initialize` to access configuration, API clients, and UI primitives. Keep long-running work cancellable by respecting the provided `context.Context`.

## Testing Plugins

Run `go test ./internal/ui/plugins/...` to execute plugin-level unit tests. For end-to-end validation launch pvetui with a configuration that enables your plugin and verify the contributed UI pieces (such as context menu entries) appear and behave as expected.
