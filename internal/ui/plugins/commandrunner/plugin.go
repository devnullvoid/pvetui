package commandrunner

import (
	"context"
	"fmt"

	commandrunner "github.com/devnullvoid/pvetui/internal/plugins/command-runner"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// PluginID identifies the command runner plugin for configuration toggles.
const PluginID = "command-runner"

// Plugin provides command execution capabilities on Proxmox hosts.
type Plugin struct {
	app    *components.App
	runner *commandrunner.Plugin
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

	// Get SSH credentials from app config
	appConfig := app.Config()
	sshUser := appConfig.SSHUser
	if sshUser == "" {
		sshUser = appConfig.GetUser() // Fall back to Proxmox user
	}

	// Create SSH client
	sshClient := commandrunner.NewSSHClient(commandrunner.SSHClientConfig{
		Username: sshUser,
		// Note: Using SSH key-based auth (no password)
		// SSH keys should be configured in ~/.ssh/ for the current user
		Timeout: config.Timeout,
		Port:    22,
	})

	// Create command runner
	// Pass the full app which implements the UIApp interface
	runner, err := commandrunner.NewPlugin(config, sshClient, app)
	if err != nil {
		return fmt.Errorf("failed to initialize command runner: %w", err)
	}

	p.runner = runner

	// Register node action
	registrar.RegisterNodeAction(components.NodeAction{
		ID:       "commandrunner.run_command",
		Label:    "Run Command (SSH)",
		Shortcut: 'C', // 'C' for command
		IsAvailable: func(node *api.Node) bool {
			return node != nil && node.Online
		},
		Handler: p.handleRunCommand,
	})

	// Register guest action for containers
	registrar.RegisterGuestAction(components.GuestAction{
		ID:       "commandrunner.run_container_command",
		Label:    "Run Command in Container (SSH)",
		Shortcut: 'C', // 'C' for command (same as host)
		IsAvailable: func(node *api.Node, guest *api.VM) bool {
			// Only available for LXC containers on online nodes
			return node != nil && node.Online && guest != nil && guest.Type == "lxc"
		},
		Handler: p.handleRunContainerCommand,
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

	// Show command menu
	p.runner.ShowHostCommandMenu(node.Name, nil)

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

	// Show container command menu
	p.runner.ShowContainerCommandMenu(node.Name, guest.ID, nil)

	return nil
}
