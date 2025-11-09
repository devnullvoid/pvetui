package components

import (
	"context"
	"sync"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// NodeActionHandler executes a plugin-provided node-level action.
//
// Implementations receive the application instance along with the currently
// selected node and can leverage the shared application context for
// long-running operations.
type NodeActionHandler func(ctx context.Context, app *App, node *api.Node) error

// NodeAction describes a menu action contributed by a plugin for node targets.
//
// If Shortcut is zero, no keyboard shortcut is registered. When IsAvailable is
// nil the action is always shown for the selected node.
type NodeAction struct {
	ID          string
	Label       string
	Shortcut    rune
	Handler     NodeActionHandler
	IsAvailable func(node *api.Node) bool
}

// PluginRegistrar exposes registration hooks a plugin can use to contribute to
// the UI. It is provided to plugins during initialization.
type PluginRegistrar interface {
	RegisterNodeAction(action NodeAction)
}

// Plugin defines the lifecycle hooks required to extend the UI through the
// plugin subsystem.
//
// Initialize is called once during application startup. Shutdown is invoked as
// part of application teardown and should release any resources acquired by the
// plugin.
//
// ModalPageNames returns a list of page names that this plugin adds to the
// application's page stack. These pages will be treated as modals by the global
// keyboard handler, preventing global keybindings from firing when they are active.
// Return an empty slice if the plugin doesn't add any modal pages.
type Plugin interface {
	ID() string
	Name() string
	Description() string
	Initialize(ctx context.Context, app *App, registrar PluginRegistrar) error
	Shutdown(ctx context.Context) error
	ModalPageNames() []string
}

// pluginRegistry stores plugin contributions and ensures thread-safe access.
type pluginRegistry struct {
	mu             sync.RWMutex
	nodeActions    []NodeAction
	modalPageNames []string
}

func newPluginRegistry() *pluginRegistry {
	return &pluginRegistry{
		modalPageNames: make([]string, 0),
	}
}

// RegisterNodeAction registers a plugin-provided node action.
func (r *pluginRegistry) RegisterNodeAction(action NodeAction) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodeActions = append(r.nodeActions, action)
}

// NodeActions returns a snapshot of all registered node actions.
func (r *pluginRegistry) NodeActions() []NodeAction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	actions := make([]NodeAction, len(r.nodeActions))
	copy(actions, r.nodeActions)

	return actions
}

// NodeActionsForNode filters registered actions by availability for the
// provided node.
func (r *pluginRegistry) NodeActionsForNode(node *api.Node) []NodeAction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodeActions) == 0 {
		return nil
	}

	var filtered []NodeAction
	for _, action := range r.nodeActions {
		if action.IsAvailable == nil || action.IsAvailable(node) {
			filtered = append(filtered, action)
		}
	}

	return filtered
}

// RegisterModalPageNames registers modal page names from a plugin.
// This should be called during plugin initialization.
func (r *pluginRegistry) RegisterModalPageNames(pageNames []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.modalPageNames = append(r.modalPageNames, pageNames...)
}

// IsPluginModal checks if the given page name is registered as a plugin modal.
// Returns true if any plugin has registered this page name as a modal.
func (r *pluginRegistry) IsPluginModal(pageName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range r.modalPageNames {
		if name == pageName {
			return true
		}
	}

	return false
}
