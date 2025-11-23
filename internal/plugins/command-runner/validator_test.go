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
			command:    "df -Th",
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

func TestValidator_ValidateVMCommandByOSType(t *testing.T) {
	config := DefaultConfig()
	config.AllowedCommands.VMLinux = []string{"linux-cmd"}
	config.AllowedCommands.VMWindows = []string{"Get-Service"}
	config.AllowedCommands.VM = []string{"fallback"}
	validator := NewValidator(config)

	linuxVM := VM{OSType: "l26"}
	if err := validator.ValidateVMCommand(linuxVM, "linux-cmd"); err != nil {
		t.Fatalf("expected linux command to be allowed, got %v", err)
	}

	windowsVM := VM{OSType: "win11"}
	if err := validator.ValidateVMCommand(windowsVM, "Get-Service"); err != nil {
		t.Fatalf("expected windows command to be allowed, got %v", err)
	}

	if err := validator.ValidateVMCommand(windowsVM, "linux-cmd"); err == nil {
		t.Fatalf("expected linux command to be rejected on windows VM")
	}

	defaultVM := VM{OSType: "unknown"}
	allowed := validator.GetAllowedVMCommands(defaultVM)
	if len(allowed) != 1 || allowed[0] != "fallback" {
		t.Fatalf("expected fallback VM commands, got %v", allowed)
	}
}
