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

	return nil
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

	cfg := app.Config()
	if cfg == nil || cfg.SSHUser == "" {
		app.ShowMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")

		return nil
	}

	selector := NewScriptSelector(app, node, nil, cfg.SSHUser)
	selector.Show()

	return nil
}
