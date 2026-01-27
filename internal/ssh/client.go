// Package ssh provides SSH client functionality for connecting to Proxmox nodes and containers.
//
// This package includes specialized support for different container types, including
// automatic detection and handling of NixOS containers which require special environment
// setup commands.
package ssh

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

func sshLogger() interfaces.Logger {
	return logger.GetGlobalLogger()
}

// SSHClient wraps SSH connection parameters and provides methods for establishing
// SSH connections to Proxmox nodes and containers.
//
// The client supports various authentication methods through the underlying SSH
// command and can be configured with custom command executors for testing.
type SSHClient struct {
	Host     string
	User     string
	Password string
	JumpHost config.SSHJumpHost
	executor CommandExecutor
}

// Option configures SSHClient behavior during initialization.
type Option func(*SSHClient)

// WithExecutor sets a custom command executor for the SSH client.
// This is primarily used for testing to inject mock executors.
func WithExecutor(exec CommandExecutor) Option {
	return func(c *SSHClient) { c.executor = exec }
}

// WithJumpHost sets the SSH jump host configuration.
func WithJumpHost(jumpHost config.SSHJumpHost) Option {
	return func(c *SSHClient) { c.JumpHost = jumpHost }
}

// NewSSHClient creates a new SSHClient instance with the specified connection parameters.
//
// Authentication is handled by the underlying "ssh" command which may use SSH keys,
// SSH agent, or other configured authentication methods. The password parameter is
// currently not used but reserved for future functionality.
//
// Example usage:
//
//	client, err := NewSSHClient("192.168.1.100", "root", "")
//	if err != nil {
//		return err
//	}
//	err = client.Shell()
//
// Parameters:
//   - host: The target host IP address or hostname
//   - user: The SSH username for authentication
//   - password: Reserved for future use (currently unused)
//   - opts: Optional configuration functions
//
// Returns a configured SSHClient instance or an error if initialization fails.
func NewSSHClient(host, user, password string, opts ...Option) (*SSHClient, error) {
	client := &SSHClient{Host: host, User: user, Password: password, executor: NewDefaultExecutor()}
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Shell opens an interactive SSH shell session on the configured host.
//
// This method uses the local ssh command and inherits the current process's
// stdio streams, providing a fully interactive terminal experience.
//
// Returns an error if the SSH connection fails or if the client is nil.
func (c *SSHClient) Shell() error {
	if c == nil {
		return fmt.Errorf("ssh client is nil")
	}

	return ExecuteNodeShellWith(context.Background(), c.executor, c.User, c.Host, c.JumpHost)
}

// ExecuteNodeShell opens an interactive SSH session to a Proxmox node.
//
// This is a convenience function that uses the default executor and context.
// For more control over execution, use ExecuteNodeShellWith.
//
// Parameters:
//   - user: SSH username for authentication
//   - nodeIP: IP address or hostname of the target node
//   - jumphost: Optional SSH jump host configuration
//
// Returns an error if the SSH connection fails.
func ExecuteNodeShell(user, nodeIP string, jumphost config.SSHJumpHost) error {
	return ExecuteNodeShellWith(context.Background(), NewDefaultExecutor(), user, nodeIP, jumphost)
}

// ExecuteNodeShellWith opens an interactive SSH session to a Proxmox node with custom execution context.
//
// This function provides full control over the execution context and command executor,
// making it suitable for testing and advanced use cases.
//
// The function automatically sets TERM=xterm-256color for better terminal compatibility
// with modern terminal emulators and displays completion status after the session ends.
//
// Parameters:
//   - ctx: Context for controlling execution lifetime and cancellation
//   - execer: Command executor interface for running SSH commands
//   - user: SSH username for authentication
//   - nodeIP: IP address or hostname of the target node
//   - jumphost: Optional SSH jump host configuration
//
// Returns an error if the SSH connection fails.
func ExecuteNodeShellWith(ctx context.Context, execer CommandExecutor, user, nodeIP string, jumphost config.SSHJumpHost) error {
	sshLogger().Debug("SSH node shell: user=%s host=%s jumphost=%+v", user, nodeIP, jumphost)

	args := buildSSHArgs(user, nodeIP, jumphost)
	sshCmd := execer.CommandContext(ctx, "ssh", args...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Set environment variables for better terminal compatibility
	// Override TERM to xterm-256color for better compatibility with remote systems
	// This fixes issues with terminals like Kitty (xterm-kitty) that aren't recognized on all systems
	sshCmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Execute command using the current process environment and stdin/stdout
	err := sshCmd.Run()

	// Show completion status and wait for user input before returning
	utils.WaitForEnterToReturn(err, "SSH session completed successfully", "SSH session ended with error")

	if err != nil {
		return fmt.Errorf("failed to execute SSH command: %w", err)
	}

	return nil
}

// ExecuteLXCShell opens an interactive session to an LXC container using 'pct enter'.
//
// This is a convenience function that uses the default executor and context.
// For containers that require special handling (like NixOS), use ExecuteLXCShellWithVM.
//
// Parameters:
//   - user: SSH username for authentication to the Proxmox node
//   - nodeIP: IP address or hostname of the Proxmox node hosting the container
//   - vmID: Container ID number
//   - jumphost: Optional SSH jump host configuration
//
// Returns an error if the connection fails.
func ExecuteLXCShell(user, nodeIP string, vmID int, jumphost config.SSHJumpHost) error {
	return ExecuteLXCShellWith(context.Background(), NewDefaultExecutor(), user, nodeIP, vmID, nil, jumphost)
}

// ExecuteLXCShellWithVM opens an interactive session to an LXC container with automatic OS detection.
//
// This function automatically detects NixOS containers and uses the appropriate command
// to properly initialize the container environment. For NixOS containers, it uses
// 'pct exec' with environment setup, while other containers use the standard 'pct enter'.
//
// NixOS containers are identified by OSType values of "nixos" or "nix" in the VM configuration.
//
// Parameters:
//   - user: SSH username for authentication to the Proxmox node
//   - nodeIP: IP address or hostname of the Proxmox node hosting the container
//   - vm: VM/container information including OS type for detection
//   - jumphost: Optional SSH jump host configuration
//
// Returns an error if the connection fails.
func ExecuteLXCShellWithVM(user, nodeIP string, vm *api.VM, jumphost config.SSHJumpHost) error {
	return ExecuteLXCShellWith(context.Background(), NewDefaultExecutor(), user, nodeIP, vm.ID, vm, jumphost)
}

// ExecuteLXCShellWith opens an interactive session to an LXC container with full control options.
//
// This function provides comprehensive control over LXC container access with automatic
// OS detection and appropriate command selection:
//
// For NixOS containers (OSType "nixos" or "nix"):
//   - Uses 'pct exec' with environment initialization
//   - Sources /etc/set-environment if present
//   - Executes bash with proper environment setup
//
// For other containers:
//   - Uses standard 'pct enter' command
//   - Provides direct container access
//
// The function automatically sets TERM=xterm-256color for better terminal compatibility
// and displays appropriate completion messages based on the container type.
//
// Example usage:
//
//	// Standard container
//	err := ExecuteLXCShellWith(ctx, executor, "root", "192.168.1.100", 101, nil, config.SSHJumpHost{})
//
//	// NixOS container with auto-detection
//	nixosVM := &api.VM{ID: 102, OSType: "nixos"}
//	err := ExecuteLXCShellWith(ctx, executor, "root", "192.168.1.100", 102, nixosVM, config.SSHJumpHost{})
//
// Parameters:
//   - ctx: Context for controlling execution lifetime and cancellation
//   - execer: Command executor interface for running SSH commands
//   - user: SSH username for authentication to the Proxmox node
//   - nodeIP: IP address or hostname of the Proxmox node hosting the container
//   - vmID: Container ID number
//   - vm: Optional VM information for OS detection (nil for standard behavior)
//   - jumphost: Optional SSH jump host configuration
//
// Returns an error if the connection fails.
func ExecuteLXCShellWith(ctx context.Context, execer CommandExecutor, user, nodeIP string, vmID int, vm *api.VM, jumphost config.SSHJumpHost) error {
	var sshArgs []string

	var sessionType string

	// Check if this is a NixOS container
	isNixOS := vm != nil && (vm.OSType == "nixos" || vm.OSType == "nix")

	buildPct := func(cmd string) string {
		if strings.EqualFold(user, "root") {
			return cmd
		}
		// Default to sudo for non-root users; PVE images often lack sudo, but
		// root doesn't need it so this keeps non-root behavior unchanged.
		return "sudo " + cmd
	}

	if isNixOS {
		// Use the NixOS-specific command for containers
		pctExec := buildPct(fmt.Sprintf("pct exec %d -- /bin/sh -c 'if [ -f /etc/set-environment ]; then . /etc/set-environment; fi; exec bash'", vmID))
		sshArgs = buildSSHArgsBase(user, nodeIP, jumphost)
		sshArgs = append(sshArgs, "-t", pctExec)
		sessionType = "NixOS LXC"
	} else {
		// Use the standard pct enter command
		pctEnter := buildPct(fmt.Sprintf("pct enter %d", vmID))
		sshArgs = buildSSHArgsBase(user, nodeIP, jumphost)
		sshArgs = append(sshArgs, "-t", pctEnter)
		sessionType = "LXC"
	}

	sshLogger().Debug("SSH LXC shell (%s): user=%s host=%s jumphost=%+v cmd=%s", sessionType, user, nodeIP, jumphost, sshArgs)

	sshCmd := execer.CommandContext(ctx, "ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Set environment variables for better terminal compatibility
	// Override TERM to xterm-256color for better compatibility with remote systems
	// This fixes issues with terminals like Kitty (xterm-kitty) that aren't recognized on all systems
	sshCmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Execute command using the current process environment and stdin/stdout
	err := sshCmd.Run()

	// Show completion status and wait for user input before returning
	utils.WaitForEnterToReturn(err, fmt.Sprintf("%s shell session completed successfully", sessionType), fmt.Sprintf("%s shell session ended with error", sessionType))

	if err != nil {
		return fmt.Errorf("failed to execute %s shell command: %w", sessionType, err)
	}

	return nil
}

// ExecuteQemuShell attempts to connect to a QEMU VM using SSH directly.
//
// This function connects directly to the VM's IP address rather than going through
// the Proxmox node. The VM must have network connectivity and SSH service running.
//
// This is a convenience function that uses the default executor and context.
//
// Parameters:
//   - user: SSH username for authentication to the VM
//   - vmIP: IP address of the target VM
//   - jumphost: Optional SSH jump host configuration
//
// Returns an error if the VM IP is empty or if the SSH connection fails.
func ExecuteQemuShell(user, vmIP string, jumphost config.SSHJumpHost) error {
	return ExecuteQemuShellWith(context.Background(), NewDefaultExecutor(), user, vmIP, jumphost)
}

// ExecuteQemuShellWith attempts to connect to a QEMU VM using SSH with custom execution context.
//
// This function provides full control over the execution context and command executor
// for connecting directly to QEMU VMs via SSH.
//
// The function automatically sets TERM=xterm-256color for better terminal compatibility
// and displays completion status after the session ends.
//
// Parameters:
//   - ctx: Context for controlling execution lifetime and cancellation
//   - execer: Command executor interface for running SSH commands
//   - user: SSH username for authentication to the VM
//   - vmIP: IP address of the target VM
//   - jumphost: Optional SSH jump host configuration
//
// Returns an error if the VM IP is empty or if the SSH connection fails.
func ExecuteQemuShellWith(ctx context.Context, execer CommandExecutor, user, vmIP string, jumphost config.SSHJumpHost) error {
	if vmIP == "" {
		return fmt.Errorf("no IP address available for VM")
	}

	sshLogger().Debug("SSH QEMU shell: user=%s host=%s jumphost=%+v", user, vmIP, jumphost)

	args := buildSSHArgs(user, vmIP, jumphost)
	sshCmd := execer.CommandContext(ctx, "ssh", args...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Set environment variables for better terminal compatibility
	// Override TERM to xterm-256color for better compatibility with remote systems
	// This fixes issues with terminals like Kitty (xterm-kitty) that aren't recognized on all systems
	sshCmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Execute command using the current process environment and stdin/stdout
	err := sshCmd.Run()

	// Show completion status and wait for user input before returning
	utils.WaitForEnterToReturn(err, "VM SSH session completed successfully", "VM SSH session ended with error")

	if err != nil {
		return fmt.Errorf("failed to connect to VM via SSH: %w", err)
	}

	return nil
}

func buildSSHArgs(user, host string, jumphost config.SSHJumpHost) []string {
	return buildSSHArgsBase(user, host, jumphost)
}

func buildSSHArgsBase(user, host string, jumphost config.SSHJumpHost) []string {
	var args []string
	target := fmt.Sprintf("%s@%s", user, host)

	if jumphost.Addr != "" {
		if jumphost.Keyfile != "" {
			proxyCmd := "ssh -W %h:%p"
			proxyCmd += fmt.Sprintf(" -i %s", shellQuote(jumphost.Keyfile))
			if jumphost.User != "" {
				proxyCmd += fmt.Sprintf(" -l %s", shellQuote(jumphost.User))
			}
			proxyCmd += fmt.Sprintf(" %s", shellQuote(jumphost.Addr))

			args = append(args, "-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd))
		} else {
			jumpSpec := jumphost.Addr
			if jumphost.User != "" {
				jumpSpec = fmt.Sprintf("%s@%s", jumphost.User, jumphost.Addr)
			}
			args = append(args, "-J", jumpSpec)
		}
	}

	args = append(args, target)
	return args
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
