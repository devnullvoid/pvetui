package commandrunner

import (
	"context"
	"fmt"
)

// Plugin implements the command runner plugin
type Plugin struct {
	config    Config
	executor  *Executor
	uiManager *UIManager
}

// NewPlugin creates a new command runner plugin
func NewPlugin(config Config, sshClient SSHClient, apiClient ProxmoxAPIClient, app UIApp) (*Plugin, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	executor := NewExecutor(config, sshClient, apiClient)
	uiManager := NewUIManager(app, executor)

	return &Plugin{
		config:    config,
		executor:  executor,
		uiManager: uiManager,
	}, nil
}

// Name returns the plugin name
func (p *Plugin) Name() string {
	return "command-runner"
}

// Enabled returns whether the plugin is enabled
func (p *Plugin) Enabled() bool {
	return p.config.Enabled
}

// SetSSHClient updates the SSH client used by the plugin executor.
func (p *Plugin) SetSSHClient(sshClient SSHClient) {
	if p == nil || p.executor == nil {
		return
	}

	p.executor.SetSSHClient(sshClient)
}

// ShowHostCommandMenu displays the command menu for a host
func (p *Plugin) ShowHostCommandMenu(host string, onClose func()) {
	p.uiManager.ShowCommandMenu(TargetHost, host, onClose)
}

// ShowContainerCommandMenu displays the command menu for a container
func (p *Plugin) ShowContainerCommandMenu(node string, vmid int, onClose func()) {
	target := fmt.Sprintf("%s/%d", node, vmid)
	p.uiManager.ShowCommandMenu(TargetContainer, target, onClose)
}

// ShowVMCommandMenu displays the command menu for a VM
func (p *Plugin) ShowVMCommandMenu(vm VM, onClose func()) {
	p.uiManager.ShowVMCommandMenu(vm, onClose)
}

// ExecuteHostCommand executes a command on a host and returns the result
func (p *Plugin) ExecuteHostCommand(ctx context.Context, host, command string) ExecutionResult {
	return p.executor.ExecuteHostCommand(ctx, host, command)
}

// GetAllowedHostCommands returns the list of allowed host commands
func (p *Plugin) GetAllowedHostCommands() []string {
	return p.executor.GetAllowedCommands(TargetHost)
}
