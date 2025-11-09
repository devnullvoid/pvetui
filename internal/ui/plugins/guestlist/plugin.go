package guestlist

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// PluginID identifies the demo guest list plugin for configuration toggles.
const PluginID = "demo-guest-list"

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
	return "Demo Guest List"
}

// Description summarises the plugin's behaviour.
func (p *Plugin) Description() string {
	return "Show a simple modal listing running guests on the selected node."
}

// Initialize registers the plugin's node-level action.
func (p *Plugin) Initialize(ctx context.Context, app *components.App, registrar components.PluginRegistrar) error {
	p.app = app

	registrar.RegisterNodeAction(components.NodeAction{
		ID:       "demo.guestlist.show",
		Label:    "Show Running Guests (Demo)",
		Shortcut: 0,
		IsAvailable: func(node *api.Node) bool {
			return node != nil
		},
		Handler: p.handleShowGuests,
	})

	return nil
}

// ModalPageNames returns the list of modal page names this plugin registers.
// This plugin doesn't add any custom modal pages.
func (p *Plugin) ModalPageNames() []string {
	return []string{}
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

	lines := runningGuestSummaries(node)
	displayName := node.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = "selected node"
	}

	if len(lines) == 0 {
		app.ShowMessageSafe(fmt.Sprintf("No running guests on %s.", displayName))

		return nil
	}

	message := fmt.Sprintf("Running guests on %s (%d):\n\n%s", displayName, len(lines), strings.Join(lines, "\n"))
	app.ShowMessageSafe(message)

	return nil
}

// runningGuestSummaries collects display strings for running guests on the provided node.
func runningGuestSummaries(node *api.Node) []string {
	if node == nil {
		return nil
	}

	summaries := make([]string, 0, len(node.VMs))
	for _, vm := range node.VMs {
		if vm == nil {
			continue
		}

		if strings.ToLower(vm.Status) != api.VMStatusRunning {
			continue
		}

		name := strings.TrimSpace(vm.Name)
		if name == "" {
			name = fmt.Sprintf("VM %d", vm.ID)
		}

		typeLabel := strings.ToUpper(strings.TrimSpace(vm.Type))
		if typeLabel == "" {
			typeLabel = "VM"
		}

		var details []string
		if vm.IP != "" {
			details = append(details, vm.IP)
		}

		summary := fmt.Sprintf("%s (ID %d, %s)", name, vm.ID, typeLabel)
		if len(details) > 0 {
			summary = fmt.Sprintf("%s [%s]", summary, strings.Join(details, ", "))
		}

		summaries = append(summaries, summary)
	}

	sort.Strings(summaries)

	return summaries
}
