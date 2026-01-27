package commandrunner

import (
	"context"
	"fmt"
	"time"

	"github.com/devnullvoid/pvetui/internal/config"
	commandrunner "github.com/devnullvoid/pvetui/internal/plugins/command-runner"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// PluginID identifies the command runner plugin for configuration toggles.
const PluginID = "command-runner"

// Plugin provides command execution capabilities on Proxmox hosts.
type Plugin struct {
	app     *components.App
	runner  *commandrunner.Plugin
	timeout time.Duration
}

// New creates a fresh plugin instance.
func New() *Plugin {
	return &Plugin{}
}

// ID returns the stable identifier for configuration wiring.
func (p *Plugin) ID() string {
	return PluginID
}

// Name returns a human-friendly plugin name.
func (p *Plugin) Name() string {
	return "Command Runner"
}

// Description summarises the plugin's behaviour.
func (p *Plugin) Description() string {
	return "Execute whitelisted commands on Proxmox hosts via SSH."
}

// Initialize registers the plugin's node-level action.
func (p *Plugin) Initialize(ctx context.Context, app *components.App, registrar components.PluginRegistrar) error {
	p.app = app

	// Get configuration from app
	config := commandrunner.DefaultConfig()
	// TODO: Load config from app.Config.Plugins.CommandRunner when config structure is added
	config.Enabled = true // For now, enable if plugin is loaded
	p.timeout = config.Timeout

	// Create SSH client
	appConfig := app.Config()
	sshClient := buildSSHClient(appConfig, nil, config.Timeout)

	// Create API client adapter for guest agent commands
	apiClient := commandrunner.NewAPIClientAdapter(app.Client())

	// Create command runner
	// Pass the full app which implements the UIApp interface
	runner, err := commandrunner.NewPlugin(config, sshClient, apiClient, app)
	if err != nil {
		return fmt.Errorf("failed to initialize command runner: %w", err)
	}

	p.runner = runner

	// Register node action
	registrar.RegisterNodeAction(components.NodeAction{
		ID:       "commandrunner.run_command",
		Label:    "Run Command",
		Shortcut: 'C', // 'C' for command
		IsAvailable: func(node *api.Node) bool {
			return node != nil && node.Online
		},
		Handler: p.handleRunCommand,
	})

	// Register guest action for containers
	registrar.RegisterGuestAction(components.GuestAction{
		ID:       "commandrunner.run_container_command",
		Label:    "Run Command",
		Shortcut: 'C', // 'C' for command (same as host)
		IsAvailable: func(node *api.Node, guest *api.VM) bool {
			// Only available for LXC containers on online nodes
			return node != nil && node.Online && guest != nil && guest.Type == "lxc"
		},
		Handler: p.handleRunContainerCommand,
	})

	// Register guest action for QEMU VMs with guest agent
	registrar.RegisterGuestAction(components.GuestAction{
		ID:       "commandrunner.run_vm_command",
		Label:    "Run Command",
		Shortcut: 'C', // 'C' for command (same as host and container)
		IsAvailable: func(node *api.Node, guest *api.VM) bool {
			// Only available for QEMU VMs with guest agent enabled and running
			return node != nil && node.Online && guest != nil &&
				guest.Type == "qemu" && guest.AgentEnabled && guest.AgentRunning
		},
		Handler: p.handleRunVMCommand,
	})

	return nil
}

// Shutdown releases resources associated with the plugin.
func (p *Plugin) Shutdown(ctx context.Context) error {
	p.app = nil
	p.runner = nil

	return nil
}

// ModalPageNames returns the list of modal page names this plugin registers.
func (p *Plugin) ModalPageNames() []string {
	return []string{
		"commandMenu",
		"parameterForm",
		"commandResult",
		"commandError",
		"executingCommand",
	}
}

// handleRunCommand shows the command menu for the selected node.
func (p *Plugin) handleRunCommand(ctx context.Context, app *components.App, node *api.Node) error {
	if app == nil {
		return fmt.Errorf("application context unavailable")
	}

	if node == nil {
		return fmt.Errorf("no node selected")
	}

	if p.runner == nil || !p.runner.Enabled() {
		return fmt.Errorf("command runner not initialized")
	}

	p.updateSSHClient(node)

	// Prefer IP for SSH to avoid DNS/host lookups; fall back to node name.
	targetHost := node.IP
	if targetHost == "" {
		targetHost = node.Name
	}

	// Show command menu
	p.runner.ShowHostCommandMenu(targetHost, nil)

	return nil
}

// handleRunContainerCommand shows the command menu for the selected container.
func (p *Plugin) handleRunContainerCommand(ctx context.Context, app *components.App, node *api.Node, guest *api.VM) error {
	if app == nil {
		return fmt.Errorf("application context unavailable")
	}

	if node == nil {
		return fmt.Errorf("no node selected")
	}

	if guest == nil {
		return fmt.Errorf("no guest selected")
	}

	if p.runner == nil || !p.runner.Enabled() {
		return fmt.Errorf("command runner not initialized")
	}

	p.updateSSHClient(node)

	// Prefer IP for SSH to avoid DNS/host lookups; fall back to node name.
	targetHost := node.IP
	if targetHost == "" {
		targetHost = node.Name
	}

	// Show container command menu
	p.runner.ShowContainerCommandMenu(targetHost, guest.ID, nil)

	return nil
}

// handleRunVMCommand shows the command menu for the selected QEMU VM.
func (p *Plugin) handleRunVMCommand(ctx context.Context, app *components.App, node *api.Node, guest *api.VM) error {
	if app == nil {
		return fmt.Errorf("application context unavailable")
	}

	if node == nil {
		return fmt.Errorf("no node selected")
	}

	if guest == nil {
		return fmt.Errorf("no guest selected")
	}

	if p.runner == nil || !p.runner.Enabled() {
		return fmt.Errorf("command runner not initialized")
	}

	p.updateSSHClient(node)

	vmCtx := commandrunner.VM{
		ID:           guest.ID,
		Node:         node.Name,
		Type:         guest.Type,
		Status:       guest.Status,
		AgentEnabled: guest.AgentEnabled,
		AgentRunning: guest.AgentRunning,
		OSType:       guest.OSType,
	}

	// Show VM command menu
	p.runner.ShowVMCommandMenu(vmCtx, nil)

	return nil
}

func (p *Plugin) updateSSHClient(node *api.Node) {
	if p == nil || p.runner == nil {
		return
	}

	appConfig := p.app.Config()
	if appConfig == nil {
		return
	}

	sshClient := buildSSHClient(appConfig, node, p.timeout)
	p.runner.SetSSHClient(sshClient)
}

func buildSSHClient(appConfig *config.Config, node *api.Node, timeout time.Duration) *commandrunner.SSHClientImpl {
	sshUser := resolveSSHUser(appConfig, node)
	if sshUser == "" {
		sshUser = appConfig.GetUser()
	}

	jumpHost := resolveJumpHost(appConfig, node)

	return commandrunner.NewSSHClient(commandrunner.SSHClientConfig{
		Username: sshUser,
		Timeout:  timeout,
		Port:     22,
		JumpHost: commandrunner.JumpHostConfig{
			Addr:    jumpHost.Addr,
			User:    jumpHost.User,
			KeyPath: jumpHost.Keyfile,
			Port:    jumpHost.Port,
		},
	})
}

func resolveSSHUser(appConfig *config.Config, node *api.Node) string {
	if appConfig == nil {
		return ""
	}

	if node != nil && node.SourceProfile != "" {
		if profile, ok := appConfig.Profiles[node.SourceProfile]; ok && profile.SSHUser != "" {
			return profile.SSHUser
		}
	}

	if appConfig.ActiveProfile != "" {
		if profile, ok := appConfig.Profiles[appConfig.ActiveProfile]; ok && profile.SSHUser != "" {
			return profile.SSHUser
		}
	}

	return appConfig.SSHUser
}

func resolveJumpHost(appConfig *config.Config, node *api.Node) config.SSHJumpHost {
	if appConfig == nil {
		return config.SSHJumpHost{}
	}

	if node != nil && node.SourceProfile != "" {
		if profile, ok := appConfig.Profiles[node.SourceProfile]; ok && profile.SSHJumpHost.Addr != "" {
			return profile.SSHJumpHost
		}
	}

	if appConfig.ActiveProfile != "" {
		if profile, ok := appConfig.Profiles[appConfig.ActiveProfile]; ok && profile.SSHJumpHost.Addr != "" {
			return profile.SSHJumpHost
		}
	}

	return appConfig.SSHJumpHost
}
