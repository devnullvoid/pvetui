# Configuration Guide

This document provides comprehensive information about configuring pvetui, including all available options and examples.

## Table of Contents

- [Configuration Format](#configuration-format)
- [Profile Management](#profile-management)
- [Authentication Methods](#authentication-methods)
- [Key Bindings](#key-bindings)
- [Theming](#theming)
- [Plugins](#plugins)
- [Advanced Options](#advanced-options)

## Configuration Format

pvetui uses a modern multi-profile configuration format that supports multiple Proxmox connections:

```yaml
profiles:
  default:
    addr: "https://your-proxmox-host:8006"
    user: "your-user"
    realm: "pam"
    # Choose one authentication method:
    password: "your-password"           # Method 1: Password auth
    # OR
    token_id: "your-token-id"          # Method 2: API token (recommended)
    token_secret: "your-secret"
    insecure: false
    ssh_user: "your-ssh-user"

  work:
    addr: "https://work-proxmox:8006"
    user: "workuser"
    token_id: "worktoken"
    token_secret: "worksecret"
    realm: "pam"
    insecure: false
    ssh_user: "workuser"

default_profile: "default"
debug: false
cache_dir: "/custom/cache/path"  # Optional: overrides platform defaults

# Key bindings customization
key_bindings:
  switch_view: "]"
  switch_view_reverse: "["
  nodes_page: "Alt+1"
  guests_page: "Alt+2"
  tasks_page: "Alt+3"
  menu: "m"
  global_menu: "g"
  shell: "s"
  vnc: "v"
  refresh: "Ctrl+r"
  auto_refresh: "a"
  search: "/"
  help: "?"
  quit: "q"

# Theme configuration
theme:
  name: "default"  # Built-in theme name
  colors:
    primary: "white"
    secondary: "gray"
    error: "red"
    # ... other color overrides

# Plugin configuration
plugins:
  enabled:
    - "community-scripts"  # Enable the default community scripts plugin
```

## Profile Management

The built-in profile manager allows you to:
- **Switch between profiles** (e.g., home, work, development)
- **Add new profiles** with different Proxmox connections
- **Edit existing profiles** with validation
- **Delete profiles** with confirmation
- **Set default profile** for automatic connection

Access the profile manager through the global menu (`g` key) or context menus.

## Authentication Methods

### API Token Authentication (Recommended)

1. In Proxmox web interface: **Datacenter → Permissions → API Tokens**
2. Click **Add** → Set user (e.g., `root`) → Enter token ID
3. Copy the generated **Token ID** and **Secret** to your config

> Important: Proxmox shows the Token ID as `user@realm!tokenid` (e.g., `root@pam!mytoken`). Split this into fields when configuring:

```yaml
profiles:
  default:
    addr: "https://your-proxmox-host:8006"
    user: "root"          # from user@realm!tokenid → user
    realm: "pam"          # from user@realm!tokenid → realm
    token_id: "mytoken"   # from user@realm!tokenid → tokenid
    token_secret: "YOUR_SECRET"
    insecure: false
    ssh_user: "root"
```

### Password Authentication

```yaml
profiles:
  default:
    addr: "https://your-proxmox-host:8006"
    user: "root"
    password: "your-password"
    realm: "pam"
    insecure: false
    ssh_user: "root"
```

**Note**: Only one authentication method (password or token) per profile is allowed.

## Key Bindings

pvetui supports fully customizable key bindings through the `key_bindings` section in your configuration file.

### Default Key Bindings

| Action | Default Key | Description |
|--------|-------------|-------------|
| `switch_view` | `]` | Switch to next view |
| `switch_view_reverse` | `[` | Switch to previous view |
| `nodes_page` | `Alt+1` | Jump to Nodes page |
| `guests_page` | `Alt+2` | Jump to Guests page |
| `tasks_page` | `Alt+3` | Jump to Tasks page |
| `menu` | `m` | Open context menu |
| `global_menu` | `g` | Open global menu |
| `shell` | `s` | Open SSH shell |
| `vnc` | `v` | Open VNC console |
| `refresh` | `Ctrl+r` | Manual refresh |
| `auto_refresh` | `a` | Toggle auto-refresh |
| `search` | `/` | Activate search |
| `help` | `?` | Toggle help modal |
| `quit` | `q` | Quit application |

### Customizing Key Bindings

```yaml
key_bindings:
  switch_view: "Ctrl+n"
  switch_view_reverse: "Ctrl+p"
  nodes_page: "F1"
  guests_page: "F2"
  tasks_page: "F3"
  menu: "Space"
  global_menu: "g"
  shell: "s"
  vnc: "v"
  refresh: "Ctrl+r"
  auto_refresh: "a"
  search: "/"
  help: "?"
  quit: "q"
```

**Note**: On macOS, you can use `Opt` instead of `Alt` for modifier keys (e.g., `Opt+1` instead of `Alt+1`).

### Supported Key Formats

- **Single characters**: `m`, `s`, `v`, `q`
- **Function keys**: `F1`, `F2`, etc.
- **Modifier combinations**: `Ctrl+r`, `Alt+1` (or `Opt+1` on macOS), `Ctrl+Shift+a`
- **Special keys**: `Enter`, `Escape`, `Tab`, `Backspace`

### Reserved Keys

The following keys cannot be reassigned as they are used for core navigation:
- `h`, `j`, `k`, `l` (Vim-style navigation)
- Arrow keys
- `Tab`, `Enter`, `Escape`, `Backspace`
- System combinations like `Ctrl+C`, `Ctrl+D`, `Ctrl+Z`

## Theming

pvetui supports semantic theming with automatic adaptation to your terminal's color scheme.

### Built-in Themes

Available built-in themes:
- `default` (default)
- `dracula`
- `catppuccin-mocha`
- `gruvbox`
- `nord`
- `rose-pine`
- `tokyonight`
- `solarized`
- `kanagawa`
- `everforest`

### Theme Configuration

```yaml
theme:
  name: "dracula"  # Built-in theme name
  colors:
    error: "red"  # ANSI red
    background: "#282a36"  # Hex color
    primary: "white"  # Override theme color
```

### Color Options

You can override any color in a built-in theme by specifying it in the `colors` map:

- **primary**: Main text color
- **secondary**: Secondary text color
- **tertiary**: Tertiary text color
- **success**: Success indicators
- **warning**: Warning indicators
- **error**: Error indicators
- **info**: Information indicators
- **background**: Background color
- **border**: Border color
- **selection**: Selection highlight
- **header**: Header background
- **headertext**: Header text
- **footer**: Footer background
- **footertext**: Footer text
- **title**: Title text
- **contrast**: Contrast elements
- **morecontrast**: High contrast elements
- **inverse**: Inverse text
- **statusrunning**: Running status
- **statusstopped**: Stopped status
- **statuspending**: Pending status
- **statuserror**: Error status
- **usagelow**: Low usage indicators
- **usagemedium**: Medium usage indicators
- **usagehigh**: High usage indicators
- **usagecritical**: Critical usage indicators

See [THEMING.md](THEMING.md) for detailed theming information and troubleshooting.

## Plugins

pvetui loads optional features through a plugin system. Plugins contribute UI actions and services without bloating the core binary.

- The `plugins.enabled` list controls which plugins are activated at startup.
- When omitted or left empty, no plugins are loaded. Enable functionality explicitly to opt in.
- Set `plugins.enabled: []` to keep all optional features disabled (e.g., in hardened environments).
- Built-in plugin identifiers: `community-scripts`, `demo-guest-list`.
- See [PLUGINS.md](PLUGINS.md) for implementation details and authoring guidance.

```yaml
plugins:
  enabled:
    - "community-scripts"  # Opt-in to the community script installer plugin
    - "demo-guest-list"    # Optional demo plugin that lists running guests on a node
```

## Advanced Options

### Encrypted Configuration

Supports [SOPS](https://github.com/getsops/sops) encrypted config files. Point to an encrypted YAML file with `--config` and it will decrypt automatically.

### Cache Directory

Customize the cache directory location:

```yaml
cache_dir: "/custom/cache/path"  # Optional: overrides platform defaults
```

### Debug Mode

Enable debug logging:

> **Note**: logs can be found in the cache directory

```yaml
debug: true
```

### Insecure Connections

Allow insecure HTTPS connections (for self-signed certificates):

```yaml
profiles:
  default:
    addr: "https://your-proxmox-host:8006"
    user: "your-user"
    token_id: "your-token-id"
    token_secret: "your-secret"
    realm: "pam"
    insecure: true  # Allow self-signed certificates
    ssh_user: "your-ssh-user"
```

## Configuration File Locations

pvetui looks for configuration files in the following order:

1. File specified with `--config` flag
2. Platform-appropriate config directory:
   - **Windows**: `%APPDATA%/pvetui/config.yml`
- **macOS**: `~/.config/pvetui/config.yml` (or `$XDG_CONFIG_HOME/pvetui/config.yml`)
- **Linux**: `~/.config/pvetui/config.yml` (or `$XDG_CONFIG_HOME/pvetui/config.yml`)
3. `./config.yml` (current directory)

> **Important**: If you're upgrading from a previous version on Windows and have existing config files in `~/.config/pvetui/`, you'll need to move them to the new platform-specific location (`%APPDATA%/pvetui/`). macOS and Linux users can continue using their existing config files without any changes.

## First Run & Interactive Config Wizard

- On first run, the app will offer to create and edit a config file in a user-friendly TUI wizard
- Launch the wizard anytime with `--config-wizard`
- Create and manage multiple connection profiles with validation
- Edit, validate, and save your config (supports SOPS-encrypted files)
- All errors and confirmations are shown in clear, interactive modals
