package commandrunner

import (
	"testing"
)

func TestValidator_ValidateCommand(t *testing.T) {
	config := DefaultConfig()
	validator := NewValidator(config)

	tests := []struct {
		name       string
		targetType TargetType
		command    string
		wantErr    bool
	}{
		{
			name:       "valid host command - exact match",
			targetType: TargetHost,
			command:    "uptime",
			wantErr:    false,
		},
		{
			name:       "valid host command - with args",
			targetType: TargetHost,
			command:    "df -h",
			wantErr:    false,
		},
		{
			name:       "valid templated command",
			targetType: TargetHost,
			command:    "systemctl status nginx",
			wantErr:    false,
		},
		{
			name:       "invalid command - not in whitelist",
			targetType: TargetHost,
			command:    "rm -rf /",
			wantErr:    true,
		},
		{
			name:       "invalid target type",
			targetType: "invalid",
			command:    "uptime",
			wantErr:    true,
		},
		{
			name:       "container command",
			targetType: TargetContainer,
			command:    "ps aux",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateCommand(tt.targetType, tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_GetAllowedCommands(t *testing.T) {
	config := DefaultConfig()
	validator := NewValidator(config)

	tests := []struct {
		name       string
		targetType TargetType
		wantNil    bool
	}{
		{
			name:       "host commands",
			targetType: TargetHost,
			wantNil:    false,
		},
		{
			name:       "container commands",
			targetType: TargetContainer,
			wantNil:    false,
		},
		{
			name:       "vm commands",
			targetType: TargetVM,
			wantNil:    false,
		},
		{
			name:       "invalid target type",
			targetType: "invalid",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands := validator.GetAllowedCommands(tt.targetType)
			if (commands == nil) != tt.wantNil {
				t.Errorf("GetAllowedCommands() nil = %v, wantNil %v", commands == nil, tt.wantNil)
			}
		})
	}
}
