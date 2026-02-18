package components

import (
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestGuestSelectionKey(t *testing.T) {
	vm := &api.VM{
		ID:            101,
		Node:          "pve-node-a",
		SourceProfile: "lab",
	}

	require.Equal(t, "lab|pve-node-a|101", guestSelectionKey(vm))
	require.Equal(t, "", guestSelectionKey(nil))
}

func TestToggleGuestSelection(t *testing.T) {
	app := &App{
		guestSelections: make(map[string]struct{}),
	}
	vm := &api.VM{ID: 100, Node: "node1", SourceProfile: "default"}

	selected := app.toggleGuestSelection(vm)
	require.True(t, selected)
	require.True(t, app.isGuestSelected(vm))
	require.Equal(t, 1, app.guestSelectionCount())

	selected = app.toggleGuestSelection(vm)
	require.False(t, selected)
	require.False(t, app.isGuestSelected(vm))
	require.Equal(t, 0, app.guestSelectionCount())
}

func TestReconcileGuestSelection(t *testing.T) {
	vm1 := &api.VM{ID: 100, Node: "node1", SourceProfile: "default"}
	vm2 := &api.VM{ID: 101, Node: "node1", SourceProfile: "default"}

	app := &App{
		guestSelections: map[string]struct{}{
			guestSelectionKey(vm1): {},
			guestSelectionKey(vm2): {},
		},
	}

	app.reconcileGuestSelection([]*api.VM{vm1})

	require.True(t, app.isGuestSelected(vm1))
	require.False(t, app.isGuestSelected(vm2))
	require.Equal(t, 1, app.guestSelectionCount())
}
