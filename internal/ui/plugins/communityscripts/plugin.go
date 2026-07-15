package communityscripts

import (
	"context"
	"fmt"

	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// PluginID identifies the community scripts plugin when referencing it from configuration.
const PluginID = "community-scripts"

// Plugin implements the components.Plugin interface to provide community script functionality.
type Plugin struct {
	app *components.App
}

// New returns a fresh plugin instance.
func New() *Plugin {
	return &Plugin{}
}

// ID returns the stable plugin identifier.
func (p *Plugin) ID() string {
	return PluginID
}

// Name returns a human-friendly plugin name.
func (p *Plugin) Name() string {
	return "Community Scripts"
}

// Description describes the plugin's purpose.
func (p *Plugin) Description() string {
	return "Install and manage community-provided Proxmox scripts via the node context menu."
}

// Initialize wires the plugin into the host application.
func (p *Plugin) Initialize(ctx context.Context, app *components.App, registrar components.PluginRegistrar) error {
	p.app = app

	registrar.RegisterNodeAction(components.NodeAction{
		ID:       "community-scripts.install",
		Label:    "Install Community Script",
		Shortcut: 'i',
		IsAvailable: func(node *api.Node) bool {
			return node != nil
		},
		Handler: p.openSelector,
	})

	registrar.RegisterGuestAction(components.GuestAction{
		ID:       "community-scripts.install-guest",
		Label:    "Install Community Script",
		Shortcut: 'i',
		IsAvailable: func(node *api.Node, vm *api.VM) bool {
			return node != nil && vm != nil && vm.Type == api.VMTypeLXC
		},
		Handler: p.openSelectorForLXC,
	})

	return nil
}

// ModalPageNames returns the list of modal page names this plugin registers.
func (p *Plugin) ModalPageNames() []string {
	return []string{
		"scriptInfo",
		"scriptSelector",
	}
}

// Shutdown releases resources held by the plugin.
func (p *Plugin) Shutdown(ctx context.Context) error {
	p.app = nil

	return nil
}

func (p *Plugin) openSelector(ctx context.Context, app *components.App, node *api.Node) error {
	if app == nil {
		return fmt.Errorf("application context unavailable")
	}

	if node == nil {
		return fmt.Errorf("no node selected")
	}

	sshUser := resolveSSHUser(app, node)
	if sshUser == "" {
		app.ShowMessageSafe("SSH user not configured. Please set PROXMOX_SSH_USER or profile ssh_user.")

		return nil
	}

	selector := NewScriptSelector(app, node, nil, sshUser)
	selector.Show()

	return nil
}

func (p *Plugin) openSelectorForLXC(ctx context.Context, app *components.App, node *api.Node, vm *api.VM) error {
	if app == nil {
		return fmt.Errorf("application context unavailable")
	}
	if node == nil || vm == nil {
		return fmt.Errorf("node/vm not provided")
	}
	if vm.Type != api.VMTypeLXC {
		return fmt.Errorf("community scripts guest action supports LXC only")
	}

	sshUser := resolveSSHUser(app, node)
	if sshUser == "" {
		app.ShowMessageSafe("SSH user not configured. Please set PROXMOX_SSH_USER or profile ssh_user.")
		return nil
	}

	selector := NewScriptSelector(app, node, vm, sshUser)
	selector.Show()
	return nil
}

// resolveSSHUser determines the SSH user for a node, preferring the node's source profile when present.
func resolveSSHUser(app *components.App, node *api.Node) string {
	cfg := app.Config()
	if cfg == nil {
		return ""
	}

	sourceProfile := ""
	if node != nil {
		sourceProfile = node.SourceProfile
	}

	return cfg.ResolveSSHSettings(sourceProfile).SSHUser
}
