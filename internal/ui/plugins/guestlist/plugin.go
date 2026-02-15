package guestlist

import (
	"context"
	"fmt"
	"strings"

	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// PluginID identifies the Guest Insights plugin for configuration toggles.
const PluginID = "guest-insights"

// LegacyPluginID is accepted for backwards compatibility with older configs.
const LegacyPluginID = "demo-guest-list"

// guestListModalPageName is the page identifier registered with the global keyboard handler.
const guestListModalPageName = "plugin.demoGuestList.modal"

// Plugin is a tiny example plugin that contributes a node action.
type Plugin struct {
	app *components.App
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
	return "Guest Insights"
}

// Description summarises the plugin's behaviour.
func (p *Plugin) Description() string {
	return "Inspect guests on the selected node with live metrics, filtering, and quick navigation."
}

// Initialize registers the plugin's node-level action.
func (p *Plugin) Initialize(ctx context.Context, app *components.App, registrar components.PluginRegistrar) error {
	p.app = app

	registrar.RegisterNodeAction(components.NodeAction{
		ID:       "demo.guestlist.show",
		Label:    "Guest Insights",
		Shortcut: 'I',
		IsAvailable: func(node *api.Node) bool {
			return node != nil
		},
		Handler: p.handleShowGuests,
	})

	return nil
}

// ModalPageNames returns the list of modal page names this plugin registers.
func (p *Plugin) ModalPageNames() []string {
	return []string{guestListModalPageName}
}

// Shutdown releases resources associated with the plugin.
func (p *Plugin) Shutdown(ctx context.Context) error {
	p.app = nil

	return nil
}

func (p *Plugin) handleShowGuests(ctx context.Context, app *components.App, node *api.Node) error {
	if app == nil {
		return fmt.Errorf("application context unavailable")
	}

	if node == nil {
		return fmt.Errorf("no node selected")
	}

	rows := buildGuestRows(node)
	if len(rows) == 0 {
		displayName := strings.TrimSpace(node.Name)
		if displayName == "" {
			displayName = "selected node"
		}
		app.ShowMessageSafe(fmt.Sprintf("No guests found on %s.", displayName))

		return nil
	}

	view := newGuestListView(app, node, rows)
	view.show(ctx)

	return nil
}
