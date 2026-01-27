package commandrunner

import (
	"context"
	"fmt"
	"time"
)

// ExecutionResult represents the result of a command execution
type ExecutionResult struct {
	Command   string
	Output    string
	Error     error
	ExitCode  int
	Duration  time.Duration
	Truncated bool
}

// Executor handles command execution on various targets
type Executor struct {
	config    Config
	validator *Validator
	sshClient SSHClient
	apiClient ProxmoxAPIClient
}

// SSHClient interface for SSH command execution (abstraction for testing)
type SSHClient interface {
	ExecuteCommand(ctx context.Context, host, command string) (output string, err error)
	ExecuteContainerCommand(ctx context.Context, host string, containerID int, command string) (output string, err error)
}

// ProxmoxAPIClient interface for Proxmox API operations (abstraction for testing)
type ProxmoxAPIClient interface {
	ExecuteGuestAgentCommand(ctx context.Context, vm VM, command []string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
}

// VM represents a minimal VM structure needed for guest agent execution
type VM struct {
	ID           int
	Node         string
	Type         string
	Status       string
	AgentEnabled bool
	AgentRunning bool
	OSType       string
}

// NewExecutor creates a new command executor
func NewExecutor(config Config, sshClient SSHClient, apiClient ProxmoxAPIClient) *Executor {
	return &Executor{
		config:    config,
		validator: NewValidator(config),
		sshClient: sshClient,
		apiClient: apiClient,
	}
}

// SetSSHClient updates the SSH client used by the executor.
func (e *Executor) SetSSHClient(sshClient SSHClient) {
	e.sshClient = sshClient
}

// ExecuteHostCommand executes a command on a Proxmox host via SSH
func (e *Executor) ExecuteHostCommand(ctx context.Context, host, command string) ExecutionResult {
	start := time.Now()

	result := ExecutionResult{
		Command: command,
	}

	// Validate command against whitelist
	if err := e.validator.ValidateCommand(TargetHost, command); err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Execute command via SSH
	output, err := e.sshClient.ExecuteCommand(ctx, host, command)
	result.Duration = time.Since(start)

	if err != nil {
		result.Error = fmt.Errorf("execution failed: %w", err)
		return result
	}

	// Enforce output size limit
	if len(output) > e.config.MaxOutputSize {
		result.Output = output[:e.config.MaxOutputSize]
		result.Truncated = true
	} else {
		result.Output = output
	}

	return result
}

// ExecuteContainerCommand executes a command in an LXC container via SSH to the host.
// It uses 'pct exec' to run the command inside the container.
func (e *Executor) ExecuteContainerCommand(ctx context.Context, host string, containerID int, command string) ExecutionResult {
	start := time.Now()

	result := ExecutionResult{
		Command: command,
	}

	// Validate command against whitelist
	if err := e.validator.ValidateCommand(TargetContainer, command); err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Execute command via SSH + pct exec
	output, err := e.sshClient.ExecuteContainerCommand(ctx, host, containerID, command)
	result.Duration = time.Since(start)

	if err != nil {
		result.Error = fmt.Errorf("execution failed: %w", err)
		return result
	}

	// Enforce output size limit
	if len(output) > e.config.MaxOutputSize {
		result.Output = output[:e.config.MaxOutputSize]
		result.Truncated = true
	} else {
		result.Output = output
	}

	return result
}

// ExecuteVMCommand executes a command in a QEMU VM via guest agent
func (e *Executor) ExecuteVMCommand(ctx context.Context, vm VM, command string) ExecutionResult {
	start := time.Now()

	result := ExecutionResult{
		Command: command,
	}

	// Validate command against whitelist using VM context
	if err := e.validator.ValidateVMCommand(vm, command); err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Check API client is available
	if e.apiClient == nil {
		result.Error = fmt.Errorf("API client not configured")
		result.Duration = time.Since(start)
		return result
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Split command into array for guest agent, respecting guest OS
	cmdParts := e.buildGuestAgentCommand(vm, command)

	// Execute command via guest agent
	stdout, stderr, exitCode, err := e.apiClient.ExecuteGuestAgentCommand(ctx, vm, cmdParts, e.config.Timeout)
	result.Duration = time.Since(start)
	result.ExitCode = exitCode

	if err != nil {
		result.Error = fmt.Errorf("execution failed: %w", err)
		// Include stderr in output if available
		if stderr != "" {
			result.Output = stderr
		}
		return result
	}

	// Combine stdout and stderr
	output := stdout
	if stderr != "" {
		output = stdout + "\n--- stderr ---\n" + stderr
	}

	// Enforce output size limit
	if len(output) > e.config.MaxOutputSize {
		result.Output = output[:e.config.MaxOutputSize]
		result.Truncated = true
	} else {
		result.Output = output
	}

	return result
}

// ExecuteTemplatedCommand executes a command with parameters filled in
func (e *Executor) ExecuteTemplatedCommand(ctx context.Context, targetType TargetType, host string, templateCmd string, params map[string]string) ExecutionResult {
	start := time.Now()

	result := ExecutionResult{
		Command: templateCmd,
	}

	// Parse and fill template
	template := ParseTemplate(templateCmd)
	filledCmd, err := template.FillTemplate(params)
	if err != nil {
		result.Error = fmt.Errorf("template error: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Command = filledCmd

	// Execute based on target type
	switch targetType {
	case TargetHost:
		return e.ExecuteHostCommand(ctx, host, filledCmd)
	case TargetContainer:
		result.Error = fmt.Errorf("use ExecuteTemplatedContainerCommand for container targets")
		result.Duration = time.Since(start)
		return result
	case TargetVM:
		result.Error = fmt.Errorf("use ExecuteTemplatedVMCommand for VM targets")
		result.Duration = time.Since(start)
		return result
	default:
		result.Error = fmt.Errorf("unknown target type: %s", targetType)
		result.Duration = time.Since(start)
		return result
	}
}

// ExecuteTemplatedContainerCommand executes a templated command in an LXC container.
func (e *Executor) ExecuteTemplatedContainerCommand(ctx context.Context, host string, containerID int, templateCmd string, params map[string]string) ExecutionResult {
	start := time.Now()

	result := ExecutionResult{
		Command: templateCmd,
	}

	// Parse and fill template
	template := ParseTemplate(templateCmd)
	filledCmd, err := template.FillTemplate(params)
	if err != nil {
		result.Error = fmt.Errorf("template error: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Command = filledCmd

	// Execute container command
	return e.ExecuteContainerCommand(ctx, host, containerID, filledCmd)
}

// ExecuteTemplatedVMCommand executes a templated command in a QEMU VM via guest agent.
func (e *Executor) ExecuteTemplatedVMCommand(ctx context.Context, vm VM, templateCmd string, params map[string]string) ExecutionResult {
	start := time.Now()

	result := ExecutionResult{
		Command: templateCmd,
	}

	// Parse and fill template
	template := ParseTemplate(templateCmd)
	filledCmd, err := template.FillTemplate(params)
	if err != nil {
		result.Error = fmt.Errorf("template error: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Command = filledCmd

	// Execute VM command
	return e.ExecuteVMCommand(ctx, vm, filledCmd)
}

// GetAllowedCommands returns the whitelist for a target type
func (e *Executor) GetAllowedCommands(targetType TargetType) []string {
	return e.validator.GetAllowedCommands(targetType)
}

// GetAllowedVMCommands returns the whitelist for the provided VM context.
func (e *Executor) GetAllowedVMCommands(vm VM) []string {
	return e.validator.GetAllowedVMCommands(vm)
}

func (e *Executor) buildGuestAgentCommand(vm VM, command string) []string {
	switch detectOSFamily(vm.OSType) {
	case OSFamilyWindows:
		return []string{
			"powershell.exe",
			"-NoProfile",
			"-NonInteractive",
			"-ExecutionPolicy", "Bypass",
			"-Command", command,
		}
	default:
		return []string{"/bin/sh", "-c", command}
	}
}
