package components

import (
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestGuestTemplateHelpers(t *testing.T) {
	templateVM := &api.VM{Type: api.VMTypeQemu, Status: api.VMStatusStopped, Template: true}
	stoppedVM := &api.VM{Type: api.VMTypeQemu, Status: api.VMStatusStopped}

	require.Equal(t, "Template", guestStatusLabel(templateVM))
	require.Equal(t, "QEMU Template", guestTypeLabel(templateVM))
	require.False(t, canStartGuest(templateVM))

	require.Equal(t, "Stopped", guestStatusLabel(stoppedVM))
	require.Equal(t, "QEMU", guestTypeLabel(stoppedVM))
	require.True(t, canStartGuest(stoppedVM))
}
