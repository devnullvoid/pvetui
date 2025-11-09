package commandrunner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClientImpl implements SSH command execution using Go's crypto/ssh library
type SSHClientImpl struct {
	username string
	password string
	keyPath  string
	timeout  time.Duration
	port     int
}

// SSHClientConfig holds configuration for SSH connections
type SSHClientConfig struct {
	Username string
	Password string
	KeyPath  string
	Timeout  time.Duration
	Port     int
}

// NewSSHClient creates a new SSH client with the given configuration
func NewSSHClient(config SSHClientConfig) *SSHClientImpl {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Port == 0 {
		config.Port = 22
	}

	return &SSHClientImpl{
		username: config.Username,
		password: config.Password,
		keyPath:  config.KeyPath,
		timeout:  config.Timeout,
		port:     config.Port,
	}
}

// ExecuteCommand executes a command on a host via SSH
func (c *SSHClientImpl) ExecuteCommand(ctx context.Context, host, command string) (string, error) {
	// Build SSH client config
	config := &ssh.ClientConfig{
		User: c.username,
		// nolint:gosec // G106: InsecureIgnoreHostKey is acceptable for initial implementation
		// TODO: Implement proper host key verification (known_hosts)
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.timeout,
	}

	// Add authentication methods
	// Try key-based authentication first
	if signers, err := c.loadSSHKeys(); err == nil && len(signers) > 0 {
		config.Auth = append(config.Auth, ssh.PublicKeys(signers...))
	}

	// Fallback to password authentication if provided
	if c.password != "" {
		config.Auth = append(config.Auth, ssh.Password(c.password))
	}

	// If no authentication method available, return error
	if len(config.Auth) == 0 {
		return "", fmt.Errorf("no SSH authentication method available (no keys in ~/.ssh/ and no password configured)")
	}

	// Connect to the remote host
	addr := fmt.Sprintf("%s:%d", host, c.port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer func() {
		_ = client.Close() // Error on close is not critical
	}()

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() {
		_ = session.Close() // Error on close is not critical
	}()

	// Set up output buffers
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Execute command with context timeout
	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM) // Best effort signal
		_ = session.Close()             // Best effort cleanup
		return "", fmt.Errorf("command execution cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			// Include stderr in error if available
			if stderr.Len() > 0 {
				return stdout.String(), fmt.Errorf("command failed: %w (stderr: %s)", err, stderr.String())
			}
			return stdout.String(), fmt.Errorf("command failed: %w", err)
		}
	}

	// Return stdout output
	return stdout.String(), nil
}

// loadSSHKeys attempts to load SSH private keys from standard locations
func (c *SSHClientImpl) loadSSHKeys() ([]ssh.Signer, error) {
	var signers []ssh.Signer

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Standard SSH key locations to try
	keyPaths := []string{
		filepath.Join(homeDir, ".ssh", "id_rsa"),
		filepath.Join(homeDir, ".ssh", "id_ed25519"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa"),
	}

	// If specific key path is configured, try that first
	if c.keyPath != "" {
		keyPaths = append([]string{c.keyPath}, keyPaths...)
	}

	// Try each key path
	for _, keyPath := range keyPaths {
		// nolint:gosec // G304: Reading SSH keys from standard paths is expected behavior
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			continue // Skip if key doesn't exist
		}

		// Try to parse the key
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			// Key might be encrypted, skip for now
			// TODO: Support encrypted keys with ssh-agent or passphrase prompt
			continue
		}

		signers = append(signers, signer)
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("no valid SSH keys found in %v", keyPaths)
	}

	return signers, nil
}

// ExecuteContainerCommand executes a command in an LXC container via SSH to the host.
// It uses 'pct exec' to run the command inside the container.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - host: Proxmox node hostname or IP
//   - containerID: LXC container ID (VMID)
//   - command: Command to execute inside the container
//
// Returns the command output and any error encountered.
func (c *SSHClientImpl) ExecuteContainerCommand(ctx context.Context, host string, containerID int, command string) (string, error) {
	// Build SSH client config
	config := &ssh.ClientConfig{
		User: c.username,
		// nolint:gosec // G106: InsecureIgnoreHostKey is acceptable for initial implementation
		// TODO: Implement proper host key verification (known_hosts)
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.timeout,
	}

	// Add authentication methods
	// Try key-based authentication first
	if signers, err := c.loadSSHKeys(); err == nil && len(signers) > 0 {
		config.Auth = append(config.Auth, ssh.PublicKeys(signers...))
	}

	// Fallback to password authentication if provided
	if c.password != "" {
		config.Auth = append(config.Auth, ssh.Password(c.password))
	}

	// If no authentication method available, return error
	if len(config.Auth) == 0 {
		return "", fmt.Errorf("no SSH authentication method available (no keys in ~/.ssh/ and no password configured)")
	}

	// Connect to the remote host
	addr := fmt.Sprintf("%s:%d", host, c.port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer func() {
		_ = client.Close() // Error on close is not critical
	}()

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() {
		_ = session.Close() // Error on close is not critical
	}()

	// Set up output buffers
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Build the pct exec command
	// Note: Using 'pct exec' instead of 'pct enter' for non-interactive command execution
	pctCommand := fmt.Sprintf("sudo pct exec %d -- %s", containerID, command)

	// Execute command with context timeout
	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(pctCommand)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM) // Best effort signal
		_ = session.Close()             // Best effort cleanup
		return "", fmt.Errorf("command execution cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			// Include stderr in error if available
			if stderr.Len() > 0 {
				return stdout.String(), fmt.Errorf("command failed: %w (stderr: %s)", err, stderr.String())
			}
			return stdout.String(), fmt.Errorf("command failed: %w", err)
		}
	}

	// Return stdout output
	return stdout.String(), nil
}

// MockSSHClient is a mock implementation for testing
type MockSSHClient struct {
	ExecuteFunc          func(ctx context.Context, host, command string) (string, error)
	ExecuteContainerFunc func(ctx context.Context, host string, containerID int, command string) (string, error)
}

// ExecuteCommand calls the mock function
func (m *MockSSHClient) ExecuteCommand(ctx context.Context, host, command string) (string, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, host, command)
	}
	return "", fmt.Errorf("mock not configured")
}

// ExecuteContainerCommand calls the mock container function
func (m *MockSSHClient) ExecuteContainerCommand(ctx context.Context, host string, containerID int, command string) (string, error) {
	if m.ExecuteContainerFunc != nil {
		return m.ExecuteContainerFunc(ctx, host, containerID, command)
	}
	return "", fmt.Errorf("mock not configured")
}
