package commandrunner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecutor_ExecuteHostCommand(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		name         string
		command      string
		mockOutput   string
		mockError    error
		wantErr      bool
		wantTruncate bool
	}{
		{
			name:       "successful execution",
			command:    "uptime",
			mockOutput: "12:00:00 up 1 day",
			mockError:  nil,
			wantErr:    false,
		},
		{
			name:       "invalid command",
			command:    "rm -rf /",
			mockOutput: "",
			mockError:  nil,
			wantErr:    true,
		},
		{
			name:         "output too large - truncation",
			command:      "uptime",
			mockOutput:   strings.Repeat("x", config.MaxOutputSize+1000),
			mockError:    nil,
			wantErr:      false,
			wantTruncate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockSSHClient{
				ExecuteFunc: func(ctx context.Context, host, command string) (string, error) {
					return tt.mockOutput, tt.mockError
				},
			}

			executor := NewExecutor(config, mockClient)
			result := executor.ExecuteHostCommand(context.Background(), "test-host", tt.command)

			if (result.Error != nil) != tt.wantErr {
				t.Errorf("ExecuteHostCommand() error = %v, wantErr %v", result.Error, tt.wantErr)
			}

			if result.Truncated != tt.wantTruncate {
				t.Errorf("ExecuteHostCommand() truncated = %v, want %v", result.Truncated, tt.wantTruncate)
			}

			if !tt.wantErr && result.Output != tt.mockOutput && !tt.wantTruncate {
				t.Errorf("ExecuteHostCommand() output = %v, want %v", result.Output, tt.mockOutput)
			}

			if tt.wantTruncate && len(result.Output) != config.MaxOutputSize {
				t.Errorf("ExecuteHostCommand() truncated output length = %d, want %d", len(result.Output), config.MaxOutputSize)
			}
		})
	}
}

func TestExecutor_ExecuteTemplatedCommand(t *testing.T) {
	config := DefaultConfig()

	mockClient := &MockSSHClient{
		ExecuteFunc: func(ctx context.Context, host, command string) (string, error) {
			return "service is running", nil
		},
	}

	executor := NewExecutor(config, mockClient)

	tests := []struct {
		name       string
		targetType TargetType
		template   string
		params     map[string]string
		wantErr    bool
	}{
		{
			name:       "valid templated command",
			targetType: TargetHost,
			template:   "systemctl status {service}",
			params:     map[string]string{"service": "nginx"},
			wantErr:    false,
		},
		{
			name:       "missing parameter",
			targetType: TargetHost,
			template:   "systemctl status {service}",
			params:     map[string]string{},
			wantErr:    true,
		},
		{
			name:       "dangerous parameter value",
			targetType: TargetHost,
			template:   "systemctl status {service}",
			params:     map[string]string{"service": "nginx; rm -rf /"},
			wantErr:    true,
		},
		{
			name:       "unsupported target type - container",
			targetType: TargetContainer,
			template:   "ps aux",
			params:     nil,
			wantErr:    true,
		},
		{
			name:       "unsupported target type - vm",
			targetType: TargetVM,
			template:   "systemctl status {service}",
			params:     map[string]string{"service": "nginx"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.ExecuteTemplatedCommand(context.Background(), tt.targetType, "test-host", tt.template, tt.params)

			if (result.Error != nil) != tt.wantErr {
				t.Errorf("ExecuteTemplatedCommand() error = %v, wantErr %v", result.Error, tt.wantErr)
			}
		})
	}
}

func TestExecutor_GetAllowedCommands(t *testing.T) {
	config := DefaultConfig()
	executor := NewExecutor(config, &MockSSHClient{})

	hostCommands := executor.GetAllowedCommands(TargetHost)
	if len(hostCommands) == 0 {
		t.Error("GetAllowedCommands(TargetHost) returned empty list")
	}

	containerCommands := executor.GetAllowedCommands(TargetContainer)
	if len(containerCommands) == 0 {
		t.Error("GetAllowedCommands(TargetContainer) returned empty list")
	}

	vmCommands := executor.GetAllowedCommands(TargetVM)
	if len(vmCommands) == 0 {
		t.Error("GetAllowedCommands(TargetVM) returned empty list")
	}
}

func TestExecutor_ContextTimeout(t *testing.T) {
	config := DefaultConfig()
	config.Timeout = 1 * time.Millisecond // Very short timeout

	mockClient := &MockSSHClient{
		ExecuteFunc: func(ctx context.Context, host, command string) (string, error) {
			// Simulate slow command
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return "slow output", nil
			}
		},
	}

	executor := NewExecutor(config, mockClient)
	result := executor.ExecuteHostCommand(context.Background(), "test-host", "uptime")

	if result.Error == nil {
		t.Error("Expected timeout error, got nil")
	}
}
