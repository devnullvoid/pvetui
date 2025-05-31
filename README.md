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
  <a href="#authentication">Authentication</a> •
  <a href="#usage">Usage</a>
</p>

## 🚀 Overview

Proxmox TUI provides a sleek, efficient way to manage your Proxmox Virtual Environment directly from your terminal. Built with Go, it offers a modern terminal interface that combines the speed of CLI with the intuitive navigation of a GUI, featuring intelligent caching and flexible authentication options.

Say goodbye to slow web interfaces and hello to lightning-fast cluster management right where you work - in the terminal.

## ✨ Features

Proxmox TUI transforms your terminal into a powerful Proxmox management console with:

### Comprehensive Cluster Management

Experience your entire Proxmox ecosystem at a glance with real-time cluster status, resource usage metrics, and node health indicators. The intuitive interface provides immediate visual feedback on your infrastructure's state.

### Smart Guest Management

Manage your VMs and containers with an intelligent display that prioritizes running guests at the top with clear visual indicators. Stopped guests appear with dimmed text, creating an intuitive visual hierarchy that helps you focus on what matters.

### Advanced Caching System

Enjoy responsive performance thanks to the BadgerDB-powered local caching system that intelligently stores and refreshes API responses. This significantly improves speed for repeated operations while ensuring data accuracy.

### Flexible Authentication

Choose between traditional username/password authentication with automatic ticket renewal or more secure API token authentication with no expiration. The system automatically detects and uses your preferred method.

### Interactive Shell Integration

Open SSH shells directly to Proxmox nodes, QEMU VMs, and LXC containers without leaving the interface, streamlining your workflow and eliminating context switching.

### Community Scripts Support

Install and manage scripts from the Proxmox Community Scripts repository directly to your nodes, extending functionality with community-contributed tools.

## 📸 Screenshots

<p align="center">
  <img src="assets/proxmox-tui-nodes.png" alt="Proxmox TUI Node View" width="800"><br>
  <em>Node Management View - Detailed node information and status</em>
</p>

<p align="center">
  <img src="assets/proxmox-tui-guests.png" alt="Proxmox TUI Guest View" width="800"><br>
  <em>Guest Management View - Real-time monitoring of VMs and containers</em>
</p>

## 🔧 Requirements

- Go (version 1.20 or later recommended)
- Access to a Proxmox VE cluster
- SSH access to nodes/guests (for shell functionality)

## 📦 Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/devnullvoid/proxmox-tui.git
cd proxmox-tui

# Run the application directly
go run ./cmd/proxmox-tui -config ./configs/config.yml

# Build the application
go build -o proxmox-tui ./cmd/proxmox-tui

# Run the application
./proxmox-tui -config ./configs/config.yml
```

### Pre-compiled Binaries (Coming Soon)

Pre-compiled binaries for various platforms will be available on the Releases page.

## ⚙️ Configuration

Proxmox TUI offers flexible configuration through YAML files, environment variables, and command-line flags. Configuration follows this precedence order (highest to lowest):

1. Command-line flags
2. Configuration file
3. Environment variables

### Configuration File

Create a `config.yml` file with your Proxmox connection details:

```yaml
# Basic connection settings
addr: "https://your-proxmox-host:8006"
api_path: "/api2/json"
insecure: false  # Set to true to skip TLS verification (not recommended for production)

# Authentication (choose one method)
user: "your-api-user"
realm: "pam"

# Method 1: Password authentication
password: "your-password"

# Method 2: API Token authentication (recommended)
token_id: "your-token-id"
token_secret: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

# Additional settings
ssh_user: "your-ssh-user"
debug: false
cache_dir: "~/.proxmox-tui/cache"
```

## 🔐 Authentication

Proxmox TUI supports two authentication methods:

### Username/Password Authentication

Uses Proxmox's ticket-based authentication with automatic renewal every 2 hours. Simple to set up but requires storing your password.

### API Token Authentication (Recommended)

Uses Proxmox API tokens for enhanced security with these benefits:
- No expiration (unless manually revoked)
- Granular permission control
- Better for automation and long-running sessions
- More secure than password-based authentication

#### Creating API Tokens in Proxmox

1. Log into your Proxmox web interface
2. Navigate to **Datacenter → Permissions → API Tokens**
3. Click **Add** to create a new token
4. Set the **User** (e.g., `root@pam`)
5. Enter a **Token ID** (e.g., `proxmox-tui`)
6. Decide whether to check **Privilege Separation** (unchecked gives the token the same permissions as the user)
7. Click **Create**
8. **Important**: Copy both the **Token ID** and **Secret** as the secret will only be shown once

## 🖥️ Usage

Run Proxmox TUI with your configuration file:

```bash
./proxmox-tui -config /path/to/your/config.yml
```

### Keyboard Navigation

- **F1**: View Nodes
- **F2**: View Guests
- **/**: Search/Filter
- **S**: Open Shell
- **C**: View Community Scripts
- **M**: Open Menu
- **Tab/Next Tab**: Switch between tabs
- **Q**: Quit

## 🤝 Contributing

Contributions are welcome! Feel free to submit issues or pull requests.

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.
