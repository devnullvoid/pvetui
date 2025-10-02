package components

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestPluginRegistry_NodeActions(t *testing.T) {
	registry := newPluginRegistry()

	action := NodeAction{
		ID:       "test-action",
		Label:    "Test",
		Shortcut: 't',
		IsAvailable: func(node *api.Node) bool {
			return node != nil
		},
		Handler: func(ctx context.Context, app *App, node *api.Node) error {
			return nil
		},
	}

	registry.RegisterNodeAction(action)

	actions := registry.NodeActions()
	require.Len(t, actions, 1)
	require.Equal(t, "test-action", actions[0].ID)

	available := registry.NodeActionsForNode(&api.Node{})
	require.Len(t, available, 1)

	unavailable := registry.NodeActionsForNode(nil)
	require.Len(t, unavailable, 0)
}
