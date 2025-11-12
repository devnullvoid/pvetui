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
	VMLinux   []string `yaml:"vm_linux,omitempty"`
	VMWindows []string `yaml:"vm_windows,omitempty"`
}

// DefaultConfig returns the default configuration for the command runner plugin
func DefaultConfig() Config {
	linuxVMCommands := []string{
		"uptime",
		"df -h",
		"free -h",
		"systemctl status {service}",
		"journalctl -u {service} -n 50",
		"ps aux | head -20",
		"ip addr show",
	}
	windowsVMCommands := []string{
		"Get-ComputerInfo | Select-Object CsName, OsName, OsVersion, WindowsProductName",
		"Get-Process | Sort-Object CPU -Descending | Select-Object -First 15 Name, CPU, PM | Format-Table",
		"Get-Service | Sort-Object Status, DisplayName | Format-Table -AutoSize -First 40",
		"Get-EventLog -LogName System -Newest 50 | Format-Table TimeGenerated, EntryType, Source, Message -AutoSize",
		"Get-Volume | Select-Object DriveLetter, FileSystemLabel, FileSystem, SizeRemaining, Size | Format-Table -AutoSize",
		"Get-NetIPAddress | Select-Object InterfaceAlias, IPAddress, PrefixLength | Format-Table -AutoSize",
		"Get-NetAdapter | Select-Object Name, InterfaceDescription, Status, LinkSpeed | Format-Table -AutoSize",
	}

	return Config{
		Enabled:       false, // Opt-in by default
		Timeout:       30 * time.Second,
		MaxOutputSize: 1048576, // 1MB
		AllowedCommands: AllowedCommands{
			Host: []string{
				"uptime",
				"df -h",
				"free -h",
				"pveversion -v",
				"systemctl status {service}",
				"journalctl -u {service} -n 50",
				"top -bn1 | head -20",
				"iostat -x 1 5",
				"netstat -tulpn",
				"ss -tulpn",
				"ip addr show",
			},
			Container: []string{
				"uptime",
				"df -h",
				"free -h",
				"ps aux",
				"top -bn1 | head -20",
				"systemctl status {service}",
				"journalctl -u {service} -n 50",
				"dpkg -l | grep {package}",
				"ip addr show",
				"netstat -tulpn",
				"ss -tulpn",
			},
			VM:        append([]string{}, linuxVMCommands...),
			VMLinux:   linuxVMCommands,
			VMWindows: windowsVMCommands,
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
