# Proxmox TUI

![Proxmox TUI Screenshot](https://i.imgur.com/your-screenshot.png) <!-- Replace with actual screenshot URL -->

A Terminal User Interface (TUI) for managing Proxmox VE clusters with modern authentication and intelligent caching.

## Overview

Proxmox TUI provides a convenient and fast way to interact with your Proxmox Virtual Environment (PVE) directly from the terminal. It offers a user-friendly experience for common Proxmox management tasks without needing to leave your command-line workflow, featuring custom authentication, intelligent caching, and a responsive interface.

## Features

*   **Cluster Overview:** View the status of your Proxmox cluster, including version, node status, and resource usage (CPU, Memory).
*   **Node Listing & Details:** List all nodes in the cluster and view detailed information for each node, such as CPU usage, memory usage, storage, uptime, kernel version, and IP address.
*   **Guest (VM & LXC) Management:** 
    *   Smart VM/Container listing with running guests prioritized at the top
    *   Visual status indicators (🟢 for running, 🔴 for stopped)
    *   Stopped guests displayed with dimmed text for better visual hierarchy
    *   Real-time resource monitoring and detailed guest information
    *   Immediate display of basic resource data with background enrichment for advanced details
*   **Flexible Authentication:**
    *   Traditional username/password authentication with automatic ticket renewal
    *   API Token authentication for enhanced security and no expiration
    *   Automatic authentication method detection based on configuration
*   **Interactive Shell:** Open an SSH shell directly to Proxmox nodes, QEMU and LXC guests.
*   **Community Scripts Integration:** Install scripts from the [Proxmox Community Scripts](https://github.com/community-scripts/ProxmoxVE) repository directly to your nodes.
*   **Search/Filter:** Quickly find nodes or guests with real-time filtering.
*   **Advanced Caching System:** 
    *   BadgerDB-powered local caching for API responses and GitHub data
    *   Intelligent cache invalidation and background refresh
    *   Configurable cache directory and disable options
    *   Significant performance improvements for repeated operations
*   **Keyboard Navigation:** Efficiently navigate the interface using keyboard shortcuts.
*   **Debug Logging:** Comprehensive debug output for troubleshooting and development.

## Installation

### Prerequisites

*   Go (version 1.20 or later recommended)
*   Access to a Proxmox VE cluster
*   SSH access to nodes/guests (for shell functionality)

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

Proxmox TUI supports flexible configuration via YAML files, environment variables, and command-line flags, with multiple authentication methods.

**Order of Precedence:**

1.  **Command-line flags:** Highest precedence. These override any other settings.
2.  **Configuration file:** Values from the config file override environment variables.
3.  **Environment variables:** Lowest precedence, used as defaults.

### Authentication Methods

Proxmox TUI supports two authentication methods:

#### 1. Username/Password Authentication (Traditional)
Uses Proxmox's ticket-based authentication with automatic renewal every 2 hours.

#### 2. API Token Authentication (Recommended)
Uses Proxmox API tokens for enhanced security:
- No expiration (unless manually revoked)
- Granular permission control
- Better for automation and long-running sessions
- More secure than password-based authentication

### Configuration File

You can specify a configuration file using the `-config` command-line flag:
```bash
./proxmox-tui -config /path/to/your/config.yml
```
If no `-config` flag is provided, the application will rely on environment variables and command-line flags.

**Example `config.yml` with Password Authentication:**

```yaml
addr: "https://your-proxmox-host-or-ip:8006" # Full Proxmox API URL
user: "your-api-user"                     # Proxmox username (without @realm)
password: "your-password"                 # Your Proxmox user's password
realm: "pam"                              # Authentication realm (default: "pam")
api_path: "/api2/json"                    # Proxmox API path (default: "/api2/json")
insecure: false                           # Skip TLS verification (not recommended for production)
ssh_user: "your-ssh-user"                 # Default SSH username for node/guest connections
debug: false                              # Enable debug logging
cache_dir: "~/.proxmox-tui/cache"         # Cache directory (default: ~/.proxmox-tui/cache)
```

**Example `config.yml` with API Token Authentication:**

```yaml
addr: "https://your-proxmox-host-or-ip:8006" # Full Proxmox API URL
user: "your-api-user"                     # Proxmox username (without @realm)
token_id: "your-token-id"                 # API Token ID (e.g., "mytoken")
token_secret: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" # API Token Secret
realm: "pam"                              # Authentication realm (default: "pam")
api_path: "/api2/json"                    # Proxmox API path (default: "/api2/json")
insecure: false                           # Skip TLS verification (not recommended for production)
ssh_user: "your-ssh-user"                 # Default SSH username for node/guest connections
debug: false                              # Enable debug logging
cache_dir: "~/.proxmox-tui/cache"         # Cache directory (default: ~/.proxmox-tui/cache)
```

**Configuration File Options:**

*   `addr`: (Required) The full URL of your Proxmox VE API endpoint (e.g., `https://pve.example.com:8006`).
*   `user`: (Required) Your Proxmox VE username (without the `@realm`).
*   **Authentication (choose one):**
    *   `password`: The password for username/password authentication.
    *   `token_id` + `token_secret`: API token credentials for token-based authentication.
*   `realm`: (Optional, default: `pam`) The Proxmox authentication realm (e.g., `pam`, `pve`, your LDAP realm).
*   `api_path`: (Optional, default: `/api2/json`) The API path for Proxmox.
*   `insecure`: (Optional, default: `false`) Set to `true` to disable TLS certificate verification. Useful for self-signed certificates in development but should be avoided in production.
*   `ssh_user`: (Optional) The default username for SSH connections to nodes and guests.
*   `debug`: (Optional, default: `false`) Set to `true` to enable verbose debug logging.
*   `cache_dir`: (Optional) Custom directory path for storing cache files. Defaults to `~/.proxmox-tui/cache`.

### Creating API Tokens in Proxmox

To use API token authentication:

1. Log into your Proxmox web interface
2. Go to **Datacenter** → **Permissions** → **API Tokens**
3. Click **Add** to create a new token
4. Set the **User** (e.g., `root@pam`)
5. Set the **Token ID** (e.g., `mytoken`)
6. Optionally set an **Expire** date (leave empty for no expiration)
7. Uncheck **Privilege Separation** if you want the token to have the same permissions as the user
8. Click **Add** and copy the generated **Secret** (you won't be able to see it again)

Use the **Token ID** and **Secret** in your configuration file.

### Environment Variables

You can configure the application using the following environment variables:

*   `PROXMOX_ADDR`: Proxmox API URL (e.g., `https://pve.example.com:8006`).
*   `PROXMOX_USER`: Proxmox username.
*   `PROXMOX_PASSWORD`: Proxmox password (for password auth).
*   `PROXMOX_TOKEN_ID`: API Token ID (for token auth).
*   `PROXMOX_TOKEN_SECRET`: API Token Secret (for token auth).
*   `PROXMOX_REALM`: Proxmox authentication realm (default: `pam`).
*   `PROXMOX_API_PATH`: Proxmox API path (default: `/api2/json`).
*   `PROXMOX_INSECURE`: Set to `true` to skip TLS verification.
*   `PROXMOX_SSH_USER`: Default SSH username.
*   `PROXMOX_DEBUG`: Set to `true` to enable debug logging.
*   `PROXMOX_CACHE_DIR`: Custom directory for cache files.

### Command-Line Flags

The following command-line flags are available and will override settings from the config file and environment variables:

*   `-addr <url>`: Proxmox API URL.
*   `-user <username>`: Proxmox username.
*   `-password <password>`: Proxmox password.
*   `-token-id <id>`: API Token ID.
*   `-token-secret <secret>`: API Token Secret.
*   `-realm <realm>`: Proxmox authentication realm.
*   `-api-path <path>`: Proxmox API path.
*   `-insecure`: Skip TLS verification (boolean flag).
*   `-ssh-user <username>`: Default SSH username.
*   `-debug`: Enable debug logging (boolean flag).
*   `-config <path>`: Path to the YAML configuration file.
*   `-cache-dir <path>`: Directory for caching data (defaults to ~/.proxmox-tui/cache).
*   `-no-cache`: Disable caching entirely (boolean flag).

**Security Note:** It is recommended to create a dedicated API user with appropriate permissions on your Proxmox VE server rather than using the `root` user. For production environments, prefer API token authentication over password-based authentication for enhanced security.

## Usage

Launch the application from your terminal:

```bash
./proxmox-tui -config config.yml
```

### Interface Overview

The application features a clean, organized interface:

*   **Running VMs/Containers** appear at the top of the guest list with green indicators (🟢▲)
*   **Stopped VMs/Containers** appear below with red indicators (🔴▼) and dimmed text
*   **Real-time resource data** displays immediately upon selection
*   **Background enrichment** provides additional details like guest agent info and network interfaces

**Default Keybindings:**

*   **F1 / `1`**: Switch to Nodes Tab
*   **F2 / `2`**: Switch to Guests Tab
*   **Tab**: Cycle through interactive elements / Switch to next main panel
*   **Shift+Tab**: Cycle backward through interactive elements / Switch to previous main panel
*   **Arrow Keys (Up/Down)**: Navigate lists (Nodes, Guests)
*   **Arrow Keys (Left/Right)**: Switch between list and details panels
*   **Enter**: Select item in a list / Confirm action
*   **/**: Activate search/filter input
*   **S**: Open SSH shell for the selected Node/Guest
*   **C**: Access Community Scripts installer for the selected Node/Guest
*   **M**: Open context menu for the selected Node/Guest
*   **Q / Ctrl+C**: Quit the application

*(Check the application footer for the most up-to-date keybindings)*

### Community Scripts Feature

Proxmox TUI integrates with the [Proxmox Community Scripts](https://github.com/community-scripts/ProxmoxVE) repository, allowing you to browse and install helpful scripts directly to your Proxmox nodes. This feature provides:

* Container templates for quickly setting up LXC containers
* VM installation scripts
* Utility scripts for Proxmox management
* System tools and improvements

**Requirements:**
1. SSH access to your Proxmox nodes with the configured `ssh_user`
2. The user having appropriate permissions on the node to execute scripts

**To use this feature:**
1. Select a node or guest
2. Press `C` or open the context menu (press `M`) and select "Install Community Script"
3. Browse the available script categories and select the script you wish to install
4. View the script details and confirm the installation

The script will be downloaded directly from the Community Scripts repository and executed on the selected node.

### Caching System

Proxmox TUI features an intelligent caching system powered by BadgerDB:

*   **API Response Caching:** Reduces load on your Proxmox server and improves response times
*   **GitHub Data Caching:** Community Scripts metadata is cached locally for faster browsing
*   **Background Refresh:** Cache is updated in the background while serving cached data
*   **Configurable:** Cache can be disabled with `-no-cache` flag or custom directory with `-cache-dir`

**Cache Management:**
- Cache files are stored in `~/.proxmox-tui/cache` by default
- Cache automatically expires and refreshes based on data type
- Use `-no-cache` flag to disable caching entirely
- Use `-cache-dir` to specify a custom cache location

## Logging

Proxmox TUI features a comprehensive logging system designed for TUI applications:

### Log Files

All application logs are written to files to avoid interfering with the terminal user interface:

- **Default Location:** `logs/proxmox-tui.log` (in the current working directory)
- **Fallback Location:** `./proxmox-tui.log` (if `logs/` directory cannot be created)
- **Format:** `[YYYY-MM-DD HH:MM:SS] [LEVEL] message`

### Log Levels

The application supports multiple log levels:

- **INFO:** General application information and important events
- **DEBUG:** Detailed debugging information (enabled with `debug: true` in config or `-debug` flag)
- **ERROR:** Error messages and exceptions

### Configuration

Enable debug logging in your configuration:

```yaml
debug: true  # Enable detailed debug logging
```

Or use the command-line flag:

```bash
./proxmox-tui -config config.yml -debug
```

### Log Content

Debug logs include detailed information about:
- API authentication and requests
- Cache operations (hits, misses, storage)
- SSH connection attempts
- Community scripts operations
- UI filtering and search operations
- Background data refresh operations

### Log Management

- Log files are appended to (not overwritten) on each application run
- No automatic log rotation is performed - manage log files manually if needed
- Logs are written in real-time as operations occur
- File logging gracefully falls back to stdout if file writing fails

## Troubleshooting

### Debug Mode

Enable debug logging to troubleshoot issues:

```bash
./proxmox-tui -config config.yml -debug
```

This will output detailed information about:
- Authentication attempts and token refresh
- API calls and responses
- Cache operations
- SSH connection attempts

### Common Issues

**Authentication Failures:**
- Verify your credentials are correct
- Check that the user has appropriate permissions
- For API tokens, ensure the token hasn't been revoked
- Verify the realm is correct (usually `pam` for local users)

**Connection Issues:**
- Verify the Proxmox server URL is correct and accessible
- Check if you need to use `-insecure` flag for self-signed certificates
- Ensure the API path is correct (usually `/api2/json`)

**SSH Issues:**
- Verify SSH access to nodes/guests with the configured user
- Check that the SSH user has appropriate permissions
- Ensure SSH keys are properly configured if using key-based authentication

## Contributing

Contributions are welcome! If you'd like to contribute, please:

1.  Fork the repository.
2.  Create a new branch (`git checkout -b feature/your-feature-name`).
3.  Make your changes.
4.  Commit your changes (`git commit -am 'Add some feature'`).
5.  Push to the branch (`git push origin feature/your-feature-name`).
6.  Create a new Pull Request.

Please ensure your code adheres to the existing style and that Go modules are tidy (`go mod tidy`).

### Development

For development, you can run the application directly:

```bash
go run . -config config.yml -debug
```

The application follows Go best practices and clean architecture principles with:
- Modular design with clear separation of concerns
- Interface-driven development for testability
- Comprehensive error handling and logging
- Thread-safe operations for concurrent access

## License

This project is licensed under the MIT License - see the `LICENSE` file for details.

---
