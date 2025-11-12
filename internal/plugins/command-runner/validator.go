package commandrunner

import (
	"fmt"
	"strings"
)

// TargetType represents where the command will be executed
type TargetType string

const (
	TargetHost      TargetType = "host"
	TargetContainer TargetType = "container"
	TargetVM        TargetType = "vm"
)

// Validator handles command whitelist validation
type Validator struct {
	config Config
}

// NewValidator creates a new command validator
func NewValidator(config Config) *Validator {
	return &Validator{
		config: config,
	}
}

// ValidateCommand checks if a command is allowed for the given target type
func (v *Validator) ValidateCommand(targetType TargetType, command string) error {
	var allowedCommands []string

	switch targetType {
	case TargetHost:
		allowedCommands = v.config.AllowedCommands.Host
	case TargetContainer:
		allowedCommands = v.config.AllowedCommands.Container
	case TargetVM:
		allowedCommands = v.config.AllowedCommands.VM
	default:
		return fmt.Errorf("invalid target type: %s", targetType)
	}

	// Check if command matches any allowed pattern
	for _, allowed := range allowedCommands {
		if v.matchesPattern(command, allowed) {
			return nil
		}
	}

	return fmt.Errorf("command not in whitelist: %s", command)
}

// matchesPattern checks if a command matches an allowed pattern
// Supports exact matches and template patterns like "systemctl status {service}"
func (v *Validator) matchesPattern(command, pattern string) bool {
	// Exact match
	if command == pattern {
		return true
	}

	// Template match
	template := ParseTemplate(pattern)
	if len(template.Parameters) == 0 {
		// No parameters, must be exact match
		return false
	}

	// Check if command structure matches template
	return v.matchesTemplateStructure(command, template)
}

// matchesTemplateStructure checks if a command matches the structure of a template
func (v *Validator) matchesTemplateStructure(command string, template CommandTemplate) bool {
	// Split template into parts around parameters
	parts := v.splitTemplate(template.Template)

	// Command must start with first part
	if !strings.HasPrefix(command, parts[0]) {
		return false
	}

	// Check each part exists in order
	pos := 0
	for _, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(command[pos:], part)
		if idx == -1 {
			return false
		}
		pos += idx + len(part)
	}

	return true
}

// splitTemplate splits a template string into static parts
// e.g., "systemctl status {service}" -> ["systemctl status ", ""]
func (v *Validator) splitTemplate(template string) []string {
	var parts []string
	var current strings.Builder
	inParam := false

	for _, ch := range template {
		if ch == '{' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			inParam = true
		} else if ch == '}' {
			inParam = false
		} else if !inParam {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// GetAllowedCommands returns the list of allowed commands for a target type
func (v *Validator) GetAllowedCommands(targetType TargetType) []string {
	switch targetType {
	case TargetHost:
		return v.config.AllowedCommands.Host
	case TargetContainer:
		return v.config.AllowedCommands.Container
	case TargetVM:
		return v.config.AllowedCommands.VM
	default:
		return nil
	}
}

// ValidateVMCommand applies OS-aware validation for VM commands.
func (v *Validator) ValidateVMCommand(vm VM, command string) error {
	allowed := v.allowedCommandsForVM(vm)
	for _, pattern := range allowed {
		if v.matchesPattern(command, pattern) {
			return nil
		}
	}
	return fmt.Errorf("command not in whitelist: %s", command)
}

// GetAllowedVMCommands returns the whitelist for a VM after considering its OS.
func (v *Validator) GetAllowedVMCommands(vm VM) []string {
	return append([]string{}, v.allowedCommandsForVM(vm)...)
}

func (v *Validator) allowedCommandsForVM(vm VM) []string {
	switch detectOSFamily(vm.OSType) {
	case OSFamilyWindows:
		if len(v.config.AllowedCommands.VMWindows) > 0 {
			return v.config.AllowedCommands.VMWindows
		}
	case OSFamilyLinux:
		if len(v.config.AllowedCommands.VMLinux) > 0 {
			return v.config.AllowedCommands.VMLinux
		}
	}
	return v.config.AllowedCommands.VM
}
