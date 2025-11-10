package commandrunner

import (
	"context"
	"time"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// APIClientAdapter adapts pkg/api.Client to the ProxmoxAPIClient interface
type APIClientAdapter struct {
	client *api.Client
}

// NewAPIClientAdapter creates a new API client adapter
func NewAPIClientAdapter(client *api.Client) *APIClientAdapter {
	return &APIClientAdapter{
		client: client,
	}
}

// ExecuteGuestAgentCommand executes a command via QEMU guest agent
func (a *APIClientAdapter) ExecuteGuestAgentCommand(ctx context.Context, vm VM, command []string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	// Convert our minimal VM struct to pkg/api.VM
	apiVM := &api.VM{
		ID:           vm.ID,
		Node:         vm.Node,
		Type:         vm.Type,
		Status:       vm.Status,
		AgentEnabled: vm.AgentEnabled,
		AgentRunning: vm.AgentRunning,
	}

	return a.client.ExecuteGuestAgentCommand(ctx, apiVM, command, timeout)
}
