package commandrunner

import (
	"fmt"
	"strings"
	"time"
)

// Config holds the configuration for the command runner plugin
type Config struct {
	Enabled         bool            `yaml:"enabled"`
	Timeout         time.Duration   `yaml:"timeout"`
	MaxOutputSize   int             `yaml:"max_output_size"`
	AllowedCommands AllowedCommands `yaml:"allowed_commands"`
}

// AllowedCommands defines whitelisted commands for different target types
type AllowedCommands struct {
	Host      []string `yaml:"host"`
	Container []string `yaml:"container"`
	VM        []string `yaml:"vm"`
}

// DefaultConfig returns the default configuration for the command runner plugin
func DefaultConfig() Config {
	return Config{
		Enabled:       false, // Opt-in by default
		Timeout:       30 * time.Second,
		MaxOutputSize: 1048576, // 1MB
		AllowedCommands: AllowedCommands{
			Host: []string{
				"uptime",
				"df -h",
				"free -h",
				"systemctl status {service}",
				"journalctl -n 50",
			},
			Container: []string{
				"ps aux",
				"df -h",
				"apt list --upgradable",
			},
			VM: []string{
				"systemctl status {service}",
			},
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.MaxOutputSize <= 0 {
		return fmt.Errorf("max_output_size must be positive")
	}
	return nil
}

// CommandTemplate represents a command with parameter placeholders
type CommandTemplate struct {
	Template   string
	Parameters []string
}

// ParseTemplate extracts parameter names from a command template
// e.g., "systemctl status {service}" -> ["service"]
func ParseTemplate(cmd string) CommandTemplate {
	var params []string
	start := -1

	for i, ch := range cmd {
		if ch == '{' {
			start = i
		} else if ch == '}' && start != -1 {
			param := cmd[start+1 : i]
			params = append(params, param)
			start = -1
		}
	}

	return CommandTemplate{
		Template:   cmd,
		Parameters: params,
	}
}

// FillTemplate replaces placeholders with actual values
// e.g., "systemctl status {service}" + {"service": "nginx"} -> "systemctl status nginx"
func (t *CommandTemplate) FillTemplate(values map[string]string) (string, error) {
	result := t.Template

	for _, param := range t.Parameters {
		value, ok := values[param]
		if !ok {
			return "", fmt.Errorf("missing parameter: %s", param)
		}

		// Basic sanitization: no shell metacharacters
		if containsShellMetachars(value) {
			return "", fmt.Errorf("parameter %s contains invalid characters", param)
		}

		placeholder := fmt.Sprintf("{%s}", param)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// containsShellMetachars checks for dangerous shell characters
func containsShellMetachars(s string) bool {
	dangerous := []string{";", "&", "|", "$", "`", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerous {
		if strings.Contains(s, char) {
			return true
		}
	}
	return false
}
