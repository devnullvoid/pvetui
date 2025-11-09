package api

import (
	"context"
	"fmt"
	"time"
)

// GuestAgentExecRequest represents a request to execute a command via QEMU guest agent.
type GuestAgentExecRequest struct {
	Command []string `json:"command"` // Command and arguments as array
}

// GuestAgentExecResponse represents the response from starting a command execution.
type GuestAgentExecResponse struct {
	PID int `json:"pid"` // Process ID of the started command
}

// GuestAgentExecStatus represents the status and output of an executed command.
type GuestAgentExecStatus struct {
	Exited   bool   `json:"exited"`        // Whether the process has exited
	ExitCode *int   `json:"exitcode"`      // Exit code (only if exited)
	OutData  string `json:"out-data"`      // Stdout data (base64 encoded by Proxmox)
	ErrData  string `json:"err-data"`      // Stderr data (base64 encoded by Proxmox)
	Signal   *int   `json:"signal"`        // Signal number if process was terminated
	OutTrunc bool   `json:"out-truncated"` // Whether stdout was truncated
	ErrTrunc bool   `json:"err-truncated"` // Whether stderr was truncated
}

// ExecuteGuestAgentCommand executes a command via QEMU guest agent and waits for completion.
// This is a convenience wrapper around ExecGuestAgentCommand and GetGuestAgentExecStatus.
func (c *Client) ExecuteGuestAgentCommand(ctx context.Context, vm *VM, command []string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if vm.Type != VMTypeQemu {
		return "", "", -1, fmt.Errorf("guest agent commands only supported for QEMU VMs")
	}

	if vm.Status != VMStatusRunning {
		return "", "", -1, fmt.Errorf("VM must be running to execute guest agent commands")
	}

	if !vm.AgentEnabled {
		return "", "", -1, fmt.Errorf("guest agent is not enabled for this VM")
	}

	// Start the command execution
	pid, err := c.ExecGuestAgentCommand(vm, command)
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to start command: %w", err)
	}

	// Poll for command completion
	pollInterval := 500 * time.Millisecond
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return "", "", -1, ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return "", "", -1, fmt.Errorf("command execution timed out after %v", timeout)
		}

		status, err := c.GetGuestAgentExecStatus(vm, pid)
		if err != nil {
			return "", "", -1, fmt.Errorf("failed to get command status: %w", err)
		}

		if status.Exited {
			code := -1
			if status.ExitCode != nil {
				code = *status.ExitCode
			}

			// Note: Proxmox returns base64-encoded data, but the API client
			// should handle decoding. If not, we may need to decode here.
			return status.OutData, status.ErrData, code, nil
		}

		// Wait before polling again
		time.Sleep(pollInterval)
	}
}

// ExecGuestAgentCommand starts command execution via QEMU guest agent.
// Returns the PID of the started process.
func (c *Client) ExecGuestAgentCommand(vm *VM, command []string) (int, error) {
	if vm.Type != VMTypeQemu {
		return 0, fmt.Errorf("guest agent commands only supported for QEMU VMs")
	}

	endpoint := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec", vm.Node, vm.ID)

	// Build request data
	reqData := map[string]interface{}{
		"command": command,
	}

	var res map[string]interface{}
	if err := c.PostWithResponse(endpoint, reqData, &res); err != nil {
		return 0, fmt.Errorf("failed to execute guest agent command: %w", err)
	}

	// Extract PID from response
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected response format: missing 'data' field")
	}

	pid, ok := data["pid"].(float64) // JSON numbers are float64
	if !ok {
		return 0, fmt.Errorf("unexpected response format: missing or invalid 'pid' field")
	}

	return int(pid), nil
}

// GetGuestAgentExecStatus retrieves the status of a command executed via guest agent.
func (c *Client) GetGuestAgentExecStatus(vm *VM, pid int) (*GuestAgentExecStatus, error) {
	if vm.Type != VMTypeQemu {
		return nil, fmt.Errorf("guest agent commands only supported for QEMU VMs")
	}

	endpoint := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec-status?pid=%d", vm.Node, vm.ID, pid)

	var res map[string]interface{}
	if err := c.Get(endpoint, &res); err != nil {
		return nil, fmt.Errorf("failed to get guest agent exec status: %w", err)
	}

	// Extract status from response
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'data' field")
	}

	status := &GuestAgentExecStatus{}

	// Parse exited status
	if exited, ok := data["exited"].(bool); ok {
		status.Exited = exited
	}

	// Parse exit code (only present if exited)
	if exitCode, ok := data["exitcode"].(float64); ok {
		code := int(exitCode)
		status.ExitCode = &code
	}

	// Parse output data
	if outData, ok := data["out-data"].(string); ok {
		status.OutData = outData
	}

	if errData, ok := data["err-data"].(string); ok {
		status.ErrData = errData
	}

	// Parse signal (if terminated by signal)
	if signal, ok := data["signal"].(float64); ok {
		sig := int(signal)
		status.Signal = &sig
	}

	// Parse truncation flags
	if outTrunc, ok := data["out-truncated"].(bool); ok {
		status.OutTrunc = outTrunc
	}

	if errTrunc, ok := data["err-truncated"].(bool); ok {
		status.ErrTrunc = errTrunc
	}

	return status, nil
}
