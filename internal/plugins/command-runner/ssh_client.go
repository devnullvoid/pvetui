package commandrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHClientImpl implements SSH command execution using Go's crypto/ssh library
type SSHClientImpl struct {
	username  string
	authValue string
	keyPath   string
	timeout   time.Duration
	port      int
	jumpHost  JumpHostConfig
}

func crSSHLogger() interfaces.Logger {
	return logger.GetGlobalLogger()
}

// SSHClientConfig holds configuration for SSH connections
type SSHClientConfig struct {
	Username  string
	AuthValue string
	KeyPath   string
	Timeout   time.Duration
	Port      int
	JumpHost  JumpHostConfig
}

// JumpHostConfig holds SSH jump host configuration.
type JumpHostConfig struct {
	Addr    string
	User    string
	KeyPath string
	Port    int
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
		username:  config.Username,
		authValue: config.AuthValue,
		keyPath:   config.KeyPath,
		timeout:   config.Timeout,
		port:      config.Port,
		jumpHost:  config.JumpHost,
	}
}

// ExecuteCommand executes a command on a host via SSH
func (c *SSHClientImpl) ExecuteCommand(ctx context.Context, host, command string) (string, error) {
	crSSHLogger().Debug("SSH exec (host): user=%s host=%s cmd=%s", c.username, host, command)

	client, cleanup, err := c.dialHost(host)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = client.Close() // Error on close is not critical
	}()
	if cleanup != nil {
		defer cleanup()
	}

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

func (c *SSHClientImpl) buildClientConfig(user, authValue, keyPath string) (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User: user,
		// nolint:gosec // G106: InsecureIgnoreHostKey is acceptable for initial implementation
		// TODO: Implement proper host key verification (known_hosts)
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.timeout,
	}

	if signers, err := c.loadSSHKeys(keyPath); err == nil && len(signers) > 0 {
		config.Auth = append(config.Auth, ssh.PublicKeys(signers...))
	}

	if authValue != "" {
		config.Auth = append(config.Auth, ssh.Password(authValue))
	}

	if len(config.Auth) == 0 {
		return nil, fmt.Errorf("no SSH authentication method available (no SSH agent, no keys found, and no password configured)")
	}

	return config, nil
}

func (c *SSHClientImpl) dialHost(host string) (*ssh.Client, func(), error) {
	targetConfig, err := c.buildClientConfig(c.username, c.authValue, c.keyPath)
	if err != nil {
		return nil, nil, err
	}

	addr := fmt.Sprintf("%s:%d", host, c.port)

	if c.jumpHost.Addr == "" {
		crSSHLogger().Debug("SSH exec: direct dial host=%s port=%d", host, c.port)
		client, err := ssh.Dial("tcp", addr, targetConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
		}
		return client, nil, nil
	}

	jumpUser := c.jumpHost.User
	if jumpUser == "" {
		jumpUser = c.username
	}

	jumpConfig, err := c.buildClientConfig(jumpUser, "", c.jumpHost.KeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure jump host: %w", err)
	}

	jumpPort := c.jumpHost.Port
	if jumpPort == 0 {
		jumpPort = c.port
	}
	jumpAddr := fmt.Sprintf("%s:%d", c.jumpHost.Addr, jumpPort)
	crSSHLogger().Debug("SSH exec: dial via jump host=%s user=%s target=%s", jumpAddr, jumpUser, addr)
	jumpClient, err := ssh.Dial("tcp", jumpAddr, jumpConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to jump host %s: %w", jumpAddr, err)
	}

	conn, err := jumpClient.Dial("tcp", addr)
	if err != nil {
		_ = jumpClient.Close()
		return nil, nil, fmt.Errorf("failed to connect to %s via jump host: %w", addr, err)
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, addr, targetConfig)
	if err != nil {
		_ = jumpClient.Close()
		return nil, nil, fmt.Errorf("failed to establish tunneled SSH connection to %s: %w", addr, err)
	}

	cleanup := func() {
		_ = jumpClient.Close()
	}

	return ssh.NewClient(clientConn, chans, reqs), cleanup, nil
}

// loadSSHKeys returns SSH signers from all available sources in priority order:
//  1. SSH agent (if SSH_AUTH_SOCK is set) — supports encrypted and hardware keys
//  2. Explicit keyPath if provided
//  3. Standard key paths (~/.ssh/id_ed25519, id_rsa, id_ecdsa) as fallback
func (c *SSHClientImpl) loadSSHKeys(keyPath string) ([]ssh.Signer, error) {
	var signers []ssh.Signer

	// 1. SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		// nolint:gosec // G704: sock is sourced from SSH_AUTH_SOCK env var, not user input
		if conn, err := net.Dial("unix", sock); err == nil {
			agentSigners, err := agent.NewClient(conn).Signers()
			if err == nil && len(agentSigners) > 0 {
				crSSHLogger().Debug("SSH auth: loaded %d signer(s) from SSH agent", len(agentSigners))
				signers = append(signers, agentSigners...)
			}
		}
	}

	// 2. Explicit keyfile or standard paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		if len(signers) > 0 {
			return signers, nil
		}
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	var keyPaths []string
	if keyPath != "" {
		keyPaths = []string{keyPath}
	} else {
		keyPaths = []string{
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		}
	}

	for _, candidate := range keyPaths {
		// nolint:gosec // G304: Reading SSH keys from configured/standard paths is expected behavior
		keyBytes, err := os.ReadFile(candidate)
		if err != nil {
			continue // key doesn't exist at this path
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			crSSHLogger().Debug("SSH auth: skipping key %s (encrypted or unsupported format)", candidate)
			continue // encrypted key — agent should cover this case
		}
		crSSHLogger().Debug("SSH auth: loaded key from %s", candidate)
		signers = append(signers, signer)
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("no SSH authentication available: SSH_AUTH_SOCK not set and no readable keys found in %v", keyPaths)
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
	client, cleanup, err := c.dialHost(host)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = client.Close() // Error on close is not critical
	}()
	if cleanup != nil {
		defer cleanup()
	}

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
	basePctCommand := fmt.Sprintf("pct exec %d -- %s", containerID, command)
	pctCommand := basePctCommand
	if !strings.EqualFold(c.username, "root") {
		pctCommand = fmt.Sprintf("sudo %s", basePctCommand)
	}
	crSSHLogger().Debug("SSH exec (container): user=%s host=%s vmid=%d cmd=%s", c.username, host, containerID, pctCommand)

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

// ExecuteContainerCommandDetailed executes a command inside an LXC container via
// SSH to the Proxmox node and returns stdout, stderr, and the exit code separately.
// cmdParts is an argument list (e.g. []string{"/bin/sh", "-c", "uptime"}) that is
// shell-quoted and joined into the pct exec call.
func (c *SSHClientImpl) ExecuteContainerCommandDetailed(ctx context.Context, host string, containerID int, cmdParts []string) (stdout, stderr string, exitCode int, err error) {
	client, cleanup, dialErr := c.dialHost(host)
	if dialErr != nil {
		return "", "", 0, dialErr
	}
	defer func() { _ = client.Close() }()
	if cleanup != nil {
		defer cleanup()
	}

	session, sessionErr := client.NewSession()
	if sessionErr != nil {
		return "", "", 0, fmt.Errorf("failed to create session: %w", sessionErr)
	}
	defer func() { _ = session.Close() }()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	// Build: pct exec <ctid> -- <cmd> [args...], shell-quoting each argument.
	quoted := make([]string, len(cmdParts))
	for i, p := range cmdParts {
		quoted[i] = shellQuoteArg(p)
	}
	pctCmd := fmt.Sprintf("pct exec %d -- %s", containerID, strings.Join(quoted, " "))
	if !strings.EqualFold(c.username, "root") {
		pctCmd = "sudo " + pctCmd
	}
	crSSHLogger().Debug("SSH exec (container detailed): user=%s host=%s vmid=%d cmd=%s", c.username, host, containerID, pctCmd)

	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(pctCmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
		return stdoutBuf.String(), stderrBuf.String(), 0, fmt.Errorf("command execution cancelled: %w", ctx.Err())
	case runErr := <-errChan:
		stdoutStr := stdoutBuf.String()
		stderrStr := stderrBuf.String()
		if runErr != nil {
			var exitErr *ssh.ExitError
			if errors.As(runErr, &exitErr) {
				return stdoutStr, stderrStr, exitErr.ExitStatus(), nil
			}
			return stdoutStr, stderrStr, 1, fmt.Errorf("command failed: %w", runErr)
		}
		return stdoutStr, stderrStr, 0, nil
	}
}

// shellQuoteArg wraps s in single quotes, escaping any embedded single quotes.
func shellQuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
