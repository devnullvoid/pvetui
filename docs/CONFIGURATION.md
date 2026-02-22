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
    vm_ssh_user: "vm-login-user"   # Optional: overrides ssh_user for QEMU VMs
    ssh_jump_host:                 # Optional: route SSH through a bastion host
      addr: "jump.example.com"
      user: "jumpuser"
      keyfile: "/path/to/jump.key"
      port: 2222
    groups: # Optional: Add profile to one or more groups
      - home-lab
      - all-servers

  work:
    addr: "https://work-proxmox:8006"
    user: "workuser"
    token_id: "worktoken"
    token_secret: "worksecret"
    realm: "pam"
    insecure: false
    ssh_user: "workuser"
    vm_ssh_user: "work-vm-user"
    ssh_jump_host:
      addr: "work-jump.example.com"
      port: 2222
    groups:
      - all-servers

default_profile: "all-servers" # Can be a profile name or a group name
debug: false
show_icons: true               # Optional: toggle icons/emojis in the UI
cache_dir: "/custom/cache/path"  # Optional: overrides platform defaults
age_dir: "/custom/age/path"      # Optional: overrides where age keys are stored

# Key bindings customization
key_bindings:
  switch_view: "]"
  switch_view_reverse: "["
  nodes_page: "Alt+1"
  guests_page: "Alt+2"
  tasks_page: "Alt+3"
  tasks_toggle_queue: "t"
  task_stop_cancel: "x"
  menu: "m"
  global_menu: "Ctrl+g"  # Optional additional key; set "" to disable extra key (Esc always opens global menu)
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

`vm_ssh_user` lets you specify a different login for QEMU VM shells. If omitted, pvetui falls back to `ssh_user`, so you only need to set it when your VM accounts differ from the Proxmox host user. `ssh_jump_host` is optional and lets you route SSH connections through a bastion host when your Proxmox nodes or VMs are not directly reachable.

Guest tags can be edited directly in the VM/LXC **Edit Configuration** form.
Use a semicolon-separated format such as `prod;monitoring;db`.

## Profile Management

The built-in profile manager allows you to:
- **Switch between profiles** (e.g., home, work, development)
- **Add new profiles** with different Proxmox connections
- **Edit existing profiles** with validation
- **Delete profiles** with confirmation
- **Set default profile or group** for automatic startup

Access the profile manager through the global menu (`Esc`) or context menus.

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
    vm_ssh_user: "root"
    ssh_jump_host:
      addr: "jump.example.com"
      port: 2222
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
| `tasks_toggle_queue` | `t` | Toggle active queue panel visibility in Tasks page |
| `task_stop_cancel` | `x` | Stop running task / cancel queued task in active queue |
| `menu` | `m` | Open context menu |
| `global_menu` | `Ctrl+g` | Additional key to open global menu (Esc always works). Set `""` to disable this extra shortcut. |
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
  tasks_toggle_queue: "t"
  task_stop_cancel: "x"
  menu: "Space"
  global_menu: "Ctrl+g"
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
- Built-in plugin identifiers: `community-scripts`, `guest-insights` (legacy alias: `demo-guest-list`).
- See [PLUGINS.md](PLUGINS.md) for implementation details and authoring guidance.

```yaml
plugins:
  enabled:
    - "community-scripts"  # Opt-in to the community script installer plugin
    - "guest-insights"     # Optional Guest Insights plugin (legacy alias: demo-guest-list)
```

## Advanced Options

### Icon Toggle

Enable or disable icons/emojis in the UI:

```yaml
show_icons: false
```

Equivalent environment variable: `PVETUI_SHOW_ICONS=false`
Equivalent CLI flag: `--show-icons=false`

### Encrypted Configuration

Supports [SOPS](https://github.com/getsops/sops) encrypted config files. Point to an encrypted YAML file with `--config` and it will decrypt automatically.

### Cache Directory

Customize the cache directory location:

```yaml
cache_dir: "/custom/cache/path"  # Optional: overrides platform defaults
```

Leading `~` is expanded to your home directory in config values, flags, and `PVETUI_CACHE_DIR`.

### Age Key Directory

pvetui stores age identity and recipient files alongside the config by default. You can override
the directory when sharing a config across multiple machines:

```yaml
age_dir: "/custom/age/path"  # Optional: overrides where age keys are stored
```

You can also set this via `PVETUI_AGE_DIR` or the `--age-dir` flag.
If the directory does not already contain `.age-identity` and `.age-recipient`,
pvetui will generate new keys, and any existing encrypted values will fail to decrypt.
Leading `~` is expanded to your home directory in config values, flags, and `PVETUI_AGE_DIR`.

### Debug Mode

Enable debug logging:

> **Note**: logs can be found in the cache directory

```yaml
debug: true
```

### Node Details Data

The Node Details panel includes:
- Disk SMART/health data (including disk type, model, size, and health state)
- System package update notifications (up to 5 updates listed with version details, plus remaining count)

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

### SSH Jump Host

Route SSH connections through a bastion host:

```yaml
profiles:
  default:
    ssh_user: "your-ssh-user"
    vm_ssh_user: "vm-login-user"
    ssh_jump_host:
      addr: "jump.example.com"
      user: "jumpuser"
      keyfile: "/path/to/jump.key"
      port: 2222
```

You can also configure these via environment variables:
`PVETUI_SSH_JUMPHOST_ADDR`, `PVETUI_SSH_JUMPHOST_USER`, `PVETUI_SSH_JUMPHOST_KEYFILE`, and `PVETUI_SSH_JUMPHOST_PORT`.
```

## Configuration File Locations

pvetui looks for configuration files in the following order:

1. File specified with `--config` flag
2. Platform-appropriate config directory:
   - **Windows**: `%APPDATA%/pvetui/config.yml`
   - **macOS**: `~/.config/pvetui/config.yml` (or `$XDG_CONFIG_HOME/pvetui/config.yml`)
   - **Linux**: `~/.config/pvetui/config.yml` (or `$XDG_CONFIG_HOME/pvetui/config.yml`)
3. `./config.yml` (current directory)

> **Windows compatibility note**: pvetui now defaults to `%APPDATA%/pvetui/config.yml`, but it will still detect legacy configs at `~/.config/pvetui/config.yml` (or `$XDG_CONFIG_HOME/pvetui/config.yml`) as a fallback.

Cache directories follow platform defaults:
- **Windows**: `%LOCALAPPDATA%/pvetui` (with legacy `~/.cache/pvetui` / `$XDG_CACHE_HOME/pvetui` fallback if present)
- **macOS**: `~/.cache/pvetui` (or `$XDG_CACHE_HOME/pvetui`)
- **Linux**: `~/.cache/pvetui` (or `$XDG_CACHE_HOME/pvetui`)

## First Run & Interactive Config Wizard

- On first run, the app will offer to create and edit a config file in a user-friendly TUI wizard
- Launch the wizard anytime with `--config-wizard`
- Create and manage multiple connection profiles with validation
- Edit, validate, and save your config (supports SOPS-encrypted files)
- All errors and confirmations are shown in clear, interactive modals
