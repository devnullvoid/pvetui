package ssh

import (
	"context"
	"fmt"
	"os"

	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
)

// SSHClient wraps an SSH connection context
// TODO: implement methods to connect and execute commands
// SSHClient wraps SSH connection parameters
type SSHClient struct {
	Host     string
	User     string
	Password string
	executor CommandExecutor
}

// NewSSHClient establishes an SSH connection to host.
// NewSSHClient returns a new SSHClient instance. Authentication is handled by
// the underlying "ssh" command which may use keys or passwords.
// Option configures SSHClient behavior.
type Option func(*SSHClient)

// WithExecutor sets a custom command executor for the SSH client.
func WithExecutor(exec CommandExecutor) Option {
	return func(c *SSHClient) { c.executor = exec }
}

func NewSSHClient(host, user, password string, opts ...Option) (*SSHClient, error) {
	client := &SSHClient{Host: host, User: user, Password: password, executor: NewDefaultExecutor()}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

// Shell opens an interactive shell on the configured host using the local ssh
// command. It inherits the current process stdio streams.
func (c *SSHClient) Shell() error {
	if c == nil {
		return fmt.Errorf("ssh client is nil")
	}
	return ExecuteNodeShellWith(context.Background(), c.executor, c.User, c.Host)
}

// ExecuteNodeShell opens an interactive SSH session to a node
func ExecuteNodeShell(user, nodeIP string) error {
	return ExecuteNodeShellWith(context.Background(), NewDefaultExecutor(), user, nodeIP)
}

// ExecuteNodeShellWith allows providing a custom executor and context.
func ExecuteNodeShellWith(ctx context.Context, execer CommandExecutor, user, nodeIP string) error {
	sshCmd := execer.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", user, nodeIP))
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

// ExecuteLXCShell opens an interactive SSH session to a node and then
// uses 'pct enter' to enter the container
func ExecuteLXCShell(user, nodeIP string, vmID int) error {
	return ExecuteLXCShellWith(context.Background(), NewDefaultExecutor(), user, nodeIP, vmID)
}

// ExecuteLXCShellWith allows providing a custom executor and context.
func ExecuteLXCShellWith(ctx context.Context, execer CommandExecutor, user, nodeIP string, vmID int) error {
	sshArgs := []string{
		fmt.Sprintf("%s@%s", user, nodeIP),
		"-t",
		fmt.Sprintf("sudo pct enter %d", vmID),
	}

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
	utils.WaitForEnterToReturn(err, "LXC shell session completed successfully", "LXC shell session ended with error")

	if err != nil {
		return fmt.Errorf("failed to execute LXC shell command: %w", err)
	}
	return nil
}

// ExecuteQemuShell attempts to connect to a QEMU VM using SSH directly
func ExecuteQemuShell(user, vmIP string) error {
	return ExecuteQemuShellWith(context.Background(), NewDefaultExecutor(), user, vmIP)
}

// ExecuteQemuShellWith attempts to connect to a QEMU VM using a custom executor.
func ExecuteQemuShellWith(ctx context.Context, execer CommandExecutor, user, vmIP string) error {
	if vmIP == "" {
		return fmt.Errorf("no IP address available for VM")
	}

	sshCmd := execer.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", user, vmIP))
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
