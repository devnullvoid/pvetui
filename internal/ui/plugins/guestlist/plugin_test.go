package guestlist

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/pkg/api"
)

type stubRegistrar struct {
	actions []components.NodeAction
}

func (s *stubRegistrar) RegisterNodeAction(action components.NodeAction) {
	s.actions = append(s.actions, action)
}

func TestRunningGuestSummaries(t *testing.T) {
	node := &api.Node{
		Name: "pve01",
		VMs: []*api.VM{
			{ID: 100, Name: "db", Type: api.VMTypeQemu, Status: api.VMStatusRunning, IP: "10.0.0.10"},
			{ID: 101, Name: "app", Type: api.VMTypeQemu, Status: api.VMStatusStopped},
			{ID: 102, Name: "cache", Type: api.VMTypeLXC, Status: api.VMStatusRunning},
		},
	}

	summaries := runningGuestSummaries(node)

	require.Len(t, summaries, 2)
	require.Contains(t, summaries, "cache (ID 102, LXC)")
	require.Contains(t, summaries, "db (ID 100, QEMU) [10.0.0.10]")
}

func TestPluginRegistersNodeAction(t *testing.T) {
	plugin := New()
	registrar := &stubRegistrar{}

	require.NoError(t, plugin.Initialize(context.Background(), &components.App{}, registrar))
	require.Len(t, registrar.actions, 1)
	require.Equal(t, "demo.guestlist.show", registrar.actions[0].ID)
}
