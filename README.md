<p align="center">
  <img src="assets/proxmox-tui-gopher-logo.png" alt="Proxmox TUI Logo" width="300">
</p>

<h1 align="center">Proxmox TUI</h1>
<p align="center">
  <strong>A powerful Terminal User Interface for Proxmox VE clusters</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#screenshots">Screenshots</a> •
  <a href="#installation">Installation</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#usage">Usage</a> •
  <a href="#vnc-console-access">VNC Console</a>
</p>

<!-- Badges -->
<p align="center">
  <img src="https://img.shields.io/github/v/release/devnullvoid/proxmox-tui" alt="GitHub release">
  <img src="https://img.shields.io/github/license/devnullvoid/proxmox-tui" alt="License">
  <img src="https://img.shields.io/github/go-mod/go-version/devnullvoid/proxmox-tui" alt="Go Version">
  <img src="https://img.shields.io/github/actions/workflow/status/devnullvoid/proxmox-tui/ci.yml?branch=master" alt="Build Status">
  <img src="https://img.shields.io/github/downloads/devnullvoid/proxmox-tui/total" alt="Total Downloads">
</p>

<!-- Demo GIF -->
<p align="center">
  <img src="assets/demo.gif" width="600" alt="Live demo of Proxmox TUI in action">
  <br>
  <em>Live demo of Proxmox TUI in action</em>
</p>

## 🚀 Features

- **Lightning Fast**: Intelligent caching for responsive performance
- **Complete Management**: VMs, containers, nodes, and cluster resources
- **Secure Authentication**: API tokens or password-based auth with automatic renewal
- **Integrated Shells**: SSH directly to nodes, VMs, and containers
- **VNC Console Access**: Embedded noVNC client with automatic authentication
- **Community Scripts**: Install Proxmox community scripts directly from the TUI
- **Modern Interface**: Vim-style navigation with customizable key bindings
- **Flexible Theming**: Automatic adaptation to terminal emulator color schemes

## 📸 Screenshots

<p align="center">
  <img src="assets/screenshot-nodes.png" alt="Node Management" width="800"><br>
  <em>Node Management - Real-time cluster monitoring and control</em>
</p>

<p align="center">
  <img src="assets/screenshot-guests.png" alt="Guest Management" width="800"><br>
  <em>Guest Management - VM and container operations</em>
</p>

## 📦 Installation

### Quick Start

**From Pre-compiled Binaries:**
1. Download from [Releases](https://github.com/devnullvoid/proxmox-tui/releases)
2. Extract and run: `./proxmox-tui`

**From Source:**
```bash
git clone --recurse-submodules https://github.com/devnullvoid/proxmox-tui.git
cd proxmox-tui
go build -o proxmox-tui ./cmd/proxmox-tui
./proxmox-tui
```

## ⚙️ Configuration

### First Run & Interactive Config Wizard
- On first run, the app will offer to create and edit a config file in a user-friendly TUI wizard
- Launch the wizard anytime with `--config-wizard`
- Edit, validate, and save your config (supports SOPS-encrypted files)
- Only one authentication method (password or token) is allowed
- All errors and confirmations are shown in clear, interactive modals

### Manual Configuration
```yaml
# Connection
addr: "https://your-proxmox-host:8006"
insecure: false

# Authentication (choose one)
user: "your-user"
realm: "pam"

# Method 1: Password auth
password: "your-password"
# OR
token_id: "your-token-id"      # Method 2: API token (recommended)
token_secret: "your-secret"

# Optional
ssh_user: "your-ssh-user"
debug: false
```

### API Token Setup (Recommended)
1. In Proxmox web interface: **Datacenter → Permissions → API Tokens**
2. Click **Add** → Set user (e.g., `root@pam`) → Enter token ID
3. Copy the generated **Token ID** and **Secret** to your config

### Encrypted Configuration
Supports [SOPS](https://github.com/getsops/sops) encrypted config files. Point to an encrypted YAML file with `-config` and it will decrypt automatically.

## 🖥️ Usage

```bash
# Auto-detects config at ~/.config/proxmox-tui/config.yml
./proxmox-tui

# Or specify custom config
./proxmox-tui -config /path/to/config.yml
```

### Key Bindings

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `h j k l` | Navigate | `Alt+1/2/3` | Switch views |
| `Enter` | Select | `[ ]` | Previous/Next view |
| `s` | SSH Shell | `v` | VNC Console |
| `m` | Context Menu | `g` | Global Menu |
| `/` | Search | `a` | Auto-refresh |
| `?` | Help | `q` | Quit |

*Customize via `key_bindings` section in config*

## Theming

Proxmox TUI supports semantic theming. By default, it adapts to your terminal's color scheme, but you can override any color in your config file.

- To use your terminal's palette, set `use_terminal_colors: true` (default).
- To use custom colors, set `use_terminal_colors: false` and specify overrides in the `colors` map.
- You can use hex codes, ANSI color names, or the special value `default`.
- You can also select a built-in theme with `theme.name`. Available built-in themes:
  - `default`, `dracula`, `catppuccin-mocha`, `gruvbox`, `nord`, `rose-pine`, `tokyonight`, `solarized`, `kanagawa`, `everforest`
- You can override any color in a built-in theme by specifying it in `colors`.

Example:
```yaml
theme:
  name: dracula
  colors:
    error: "red" # ANSI red
    background: "#282a36" # Hex color
```

See [docs/THEMING.md](docs/THEMING.md) for the full list of color keys, advanced options, and troubleshooting.

## 🖥️ VNC Console Access

Built-in noVNC client provides seamless console access:
- **Zero Configuration**: Works out of the box
- **Automatic Authentication**: No separate login required
- **Universal Support**: VMs, containers, and node shells
- **Secure Proxy**: Local WebSocket proxy handles connections

**Note**: Node VNC shells require password authentication (Proxmox limitation).

## 🔧 Requirements

- Access to Proxmox VE cluster
- SSH access for shell functionality
- Go 1.20+ (for building from source)

## 💡 Tips

- **Use SSH keys for authentication**: For best security and convenience, set up SSH key-based authentication with your Proxmox hosts. Avoid password-based SSH logins.
- **Passwordless pct access**: Add a sudoers rule on your Proxmox hosts to allow your user to run `pct enter` and `pct exec` without being prompted for a password. Example sudoers line:
  ```
  youruser ALL=(ALL) NOPASSWD: /usr/sbin/pct enter *, /usr/sbin/pct exec *
  ```

## 🐳 Docker Usage

```bash
git clone --recurse-submodules https://github.com/devnullvoid/proxmox-tui.git
cd proxmox-tui
cp .env.example .env  # Edit with your Proxmox details
docker compose run --rm proxmox-tui
```

See [DOCKER.md](./DOCKER.md) for advanced usage.

## 🤝 Contributing

Contributions welcome! Check the [issues page](https://github.com/devnull-cr/proxmox-tui/issues).

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.
