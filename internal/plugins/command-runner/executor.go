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
}

// SSHClient interface for SSH command execution (abstraction for testing)
type SSHClient interface {
	ExecuteCommand(ctx context.Context, host, command string) (output string, err error)
	ExecuteContainerCommand(ctx context.Context, host string, containerID int, command string) (output string, err error)
}

// NewExecutor creates a new command executor
func NewExecutor(config Config, sshClient SSHClient) *Executor {
	return &Executor{
		config:    config,
		validator: NewValidator(config),
		sshClient: sshClient,
	}
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
		// TODO: Implement VM execution in Phase 3
		result.Error = fmt.Errorf("VM execution not yet implemented")
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

// GetAllowedCommands returns the whitelist for a target type
func (e *Executor) GetAllowedCommands(targetType TargetType) []string {
	return e.validator.GetAllowedCommands(targetType)
}
