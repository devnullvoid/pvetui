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
		app.ShowMessage("SSH user not configured. Please set PROXMOX_SSH_USER or profile ssh_user.")

		return nil
	}

	selector := NewScriptSelector(app, node, nil, sshUser)
	selector.Show()

	return nil
}

// resolveSSHUser determines the SSH user for a node, preferring the node's source profile when present.
func resolveSSHUser(app *components.App, node *api.Node) string {
	cfg := app.Config()
	if cfg == nil {
		return ""
	}

	// If the node originated from a specific profile (e.g., group mode), honor that profile's ssh_user.
	if node != nil && node.SourceProfile != "" {
		if prof, ok := cfg.Profiles[node.SourceProfile]; ok && prof.SSHUser != "" {
			return prof.SSHUser
		}
	}

	// Fall back to active profile's ssh_user
	if cfg.ActiveProfile != "" {
		if prof, ok := cfg.Profiles[cfg.ActiveProfile]; ok && prof.SSHUser != "" {
			return prof.SSHUser
		}
	}

	// Finally, use global ssh_user (legacy)
	return cfg.SSHUser
}
