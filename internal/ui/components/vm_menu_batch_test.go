package components

import (
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestIsEligibleForBatchAction(t *testing.T) {
	runningQemu := &api.VM{Status: api.VMStatusRunning, Type: api.VMTypeQemu}
	runningLXC := &api.VM{Status: api.VMStatusRunning, Type: api.VMTypeLXC}
	stoppedQemu := &api.VM{Status: api.VMStatusStopped, Type: api.VMTypeQemu}

	require.True(t, isEligibleForBatchAction(stoppedQemu, vmActionStart))
	require.False(t, isEligibleForBatchAction(runningQemu, vmActionStart))

	require.True(t, isEligibleForBatchAction(runningQemu, vmActionShutdown))
	require.True(t, isEligibleForBatchAction(runningQemu, vmActionStop))
	require.True(t, isEligibleForBatchAction(runningQemu, vmActionRestart))
	require.False(t, isEligibleForBatchAction(stoppedQemu, vmActionShutdown))

	require.True(t, isEligibleForBatchAction(runningQemu, vmActionReset))
	require.False(t, isEligibleForBatchAction(runningLXC, vmActionReset))
	require.False(t, isEligibleForBatchAction(stoppedQemu, vmActionReset))
	require.False(t, isEligibleForBatchAction(nil, vmActionReset))
	require.False(t, isEligibleForBatchAction(runningQemu, "unknown"))
}
