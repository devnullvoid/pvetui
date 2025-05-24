# Proxmox TUI

![Proxmox TUI Screenshot](https://i.imgur.com/your-screenshot.png) <!-- Replace with actual screenshot URL -->

A Terminal User Interface (TUI) for managing Proxmox VE clusters.

## Overview

Proxmox TUI provides a convenient and fast way to interact with your Proxmox Virtual Environment (PVE) directly from the terminal. It aims to offer a user-friendly experience for common Proxmox management tasks without needing to leave your command-line workflow.

## Features

*   **Cluster Overview:** View the status of your Proxmox cluster, including version, node status, and resource usage (CPU, Memory).
*   **Node Listing & Details:** List all nodes in the cluster and view detailed information for each node, such as CPU usage, memory usage, storage, uptime, kernel version, and IP address.
*   **Guest (VM & LXC) Listing & Details:** List all VMs and LXC containers across the cluster. View detailed information for selected guests, including status, resource usage, and configuration.
*   **Interactive Shell:** Open an SSH shell directly to Proxmox nodes, QEMU and LXC guests.
*   **Search/Filter:** Quickly find nodes or guests.
*   **Keyboard Navigation:** Efficiently navigate the interface using keyboard shortcuts.

## Installation

### Prerequisites

*   Go (version 1.20 or later recommended)
*   Access to a Proxmox VE cluster

### From Source

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/proxmox-tui.git
    cd proxmox-tui
    ```
    *(Replace `your-username` with your actual GitHub username)*

2.  **Build the application:**
    ```bash
    go build -o proxmox-tui .
    ```

3.  **Run the application:**
    ```bash
    ./proxmox-tui -config config.yml
    ```

### Pre-compiled Binaries (Planned)

*(Coming soon - check the Releases page for pre-compiled binaries for your platform.)*

## Configuration

Proxmox TUI can be configured via a YAML file, environment variables, and command-line flags.

**Order of Precedence:**

1.  **Command-line flags:** Highest precedence. These override any other settings.
2.  **Configuration file:** Values from the config file override environment variables.
3.  **Environment variables:** Lowest precedence, used as defaults.

### Configuration File

You can specify a configuration file using the `-config` command-line flag:
```bash
./proxmox-tui -config /path/to/your/config.yml
```
If no `-config` flag is provided, the application will rely on environment variables and command-line flags.

**Example `config.yml`:**

```yaml
addr: "https://your-proxmox-host-or-ip:8006" # Full Proxmox API URL, e.g., "https://pve.example.com:8006"
user: "your-api-user"                     # Proxmox username, e.g., "root" or "api-user"
password: "your-password"                 # Your Proxmox user's password
realm: "pam"                              # Proxmox authentication realm, e.g., "pam", "pve", "ldap" (default: "pam")
api_path: "/api2/json"                    # Proxmox API path (default: "/api2/json")
insecure: false                           # Set to true to skip TLS certificate verification (not recommended for production)
ssh_user: "your-ssh-user"                 # Optional: Default SSH username for connecting to nodes/guests (overrides PROXMOX_SSH_USER)
debug: false                              # Set to true to enable debug logging
```

**Configuration File Options:**

*   `addr`: (Required) The full URL of your Proxmox VE API endpoint (e.g., `https://pve.example.com:8006`).
*   `user`: (Required) Your Proxmox VE username (without the `@realm`).
*   `password`: (Required) The password for the specified user.
*   `realm`: (Optional, default: `pam`) The Proxmox authentication realm (e.g., `pam`, `pve`, your LDAP realm).
*   `api_path`: (Optional, default: `/api2/json`) The API path for Proxmox.
*   `insecure`: (Optional, default: `false`) Set to `true` to disable TLS certificate verification. This is useful for self-signed certificates in development environments but should be avoided in production.
*   `ssh_user`: (Optional) The default username to use for SSH connections to nodes and guests. If not set, the system will try `PROXMOX_SSH_USER` environment variable or may prompt if an SSH client requires it.
*   `debug`: (Optional, default: `false`) Set to `true` to enable verbose debug logging to standard error.

### Environment Variables

You can configure the application using the following environment variables:

*   `PROXMOX_ADDR`: Proxmox API URL (e.g., `https://pve.example.com:8006`).
*   `PROXMOX_USER`: Proxmox username.
*   `PROXMOX_PASSWORD`: Proxmox password.
*   `PROXMOX_REALM`: Proxmox authentication realm (default: `pam`).
*   `PROXMOX_API_PATH`: Proxmox API path (default: `/api2/json`).
*   `PROXMOX_INSECURE`: Set to `true` to skip TLS verification.
*   `PROXMOX_SSH_USER`: Default SSH username.
*   `PROXMOX_DEBUG`: Set to `true` to enable debug logging.

### Command-Line Flags

The following command-line flags are available and will override settings from the config file and environment variables:

*   `-addr <url>`: Proxmox API URL.
*   `-user <username>`: Proxmox username.
*   `-password <password>`: Proxmox password.
*   `-realm <realm>`: Proxmox authentication realm.
*   `-api-path <path>`: Proxmox API path.
*   `-insecure`: Skip TLS verification (boolean flag, presence means true).
*   `-ssh-user <username>`: Default SSH username.
*   `-debug`: Enable debug logging (boolean flag, presence means true).
*   `-config <path>`: Path to the YAML configuration file.

**Note on Credentials:** It is recommended to create a dedicated API user with appropriate permissions on your Proxmox VE server for enhanced security, rather than using the `root` user. Avoid hardcoding credentials directly in scripts or command history where possible; use a configuration file with appropriate permissions or environment variables.

## Usage

Launch the application from your terminal:

```bash
./proxmox-tui
```

**Default Keybindings:**

*   **F1 / `1`**: Switch to Nodes Tab
*   **F2 / `2`**: Switch to Guests Tab
*   **Tab**: Cycle through interactive elements / Switch to next main panel
*   **Shift+Tab**: Cycle backward through interactive elements / Switch to previous main panel
*   **Arrow Keys (Up/Down)**: Navigate lists (Nodes, Guests)
*   **Enter**: Select item in a list / Confirm action
*   **/**: Activate search/filter input
*   **S**: Open SSH shell for the selected Node
*   **Q / Ctrl+C**: Quit the application

*(These may evolve, check the application footer for the most up-to-date keybindings)*

## Contributing

Contributions are welcome! If you'd like to contribute, please:

1.  Fork the repository.
2.  Create a new branch (`git checkout -b feature/your-feature-name`).
3.  Make your changes.
4.  Commit your changes (`git commit -am 'Add some feature'`).
5.  Push to the branch (`git push origin feature/your-feature-name`).
6.  Create a new Pull Request.

Please ensure your code adheres to the existing style and that Go modules are tidy (`go mod tidy`).

## License

This project is licensed under the MIT License - see the `LICENSE` file for details (You'll need to create this file if you choose this license).

---
