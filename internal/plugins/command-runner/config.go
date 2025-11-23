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
	linuxCommonCommands := []string{
		// System overview
		"uptime",
		"df -Th",
		"lsblk",
		"free -h",

		// Process and resource inspection
		"ps aux",
		"top -bn1 | head -20",
		"ps -eo pid,ppid,cmd,%cpu,%mem --sort=-%cpu | head -20",
		"ps -eo pid,ppid,cmd,%cpu,%mem --sort=-%mem | head -20",

		// Services, logs, packages
		"systemctl list-units --type=service --state=running",
		"systemctl list-unit-files --state=enabled",
		"systemctl status {service}",
		"journalctl -n 100",
		"journalctl -u {service} -n 50",
		"dpkg -l | grep {package}",

		// Networking & connectivity
		"ip addr show",
		"ip route show",
		"ip link show",
		"ss -tulpn",
		"ss -s",
		"netstat -tulpn",
		"cat /etc/resolv.conf",
		"resolvectl status 2>/dev/null || systemd-resolve --status 2>/dev/null",
		"curl https://ifconfig.me/all",
		"nslookup {hostname}",

		// User activity
		"who",
		"last -n 20",
	}
	hostCommands := append([]string{}, linuxCommonCommands...)
	hostCommands = append(hostCommands,
		"pveversion -v",
		"iostat -x 1 5",
	)
	containerCommands := append([]string{}, linuxCommonCommands...)
	linuxVMCommands := append([]string{}, linuxCommonCommands...)
	windowsVMCommands := []string{
		"Get-ComputerInfo | Select-Object CsName, OsName, OsVersion, WindowsProductName",
		"Get-Process | Sort-Object CPU -Descending | Select-Object -First 15 Name, CPU, PM | Format-Table",
		"Get-Process | Sort-Object PM -Descending | Select-Object -First 15 Name, CPU, PM | Format-Table",
		"Get-Service | Sort-Object Status, DisplayName | Format-Table -AutoSize -First 40",
		"Get-EventLog -LogName System -Newest 50 | Format-Table TimeGenerated, EntryType, Source, Message -AutoSize",
		"Get-Volume | Select-Object DriveLetter, FileSystemLabel, FileSystem, SizeRemaining, Size | Format-Table -AutoSize",
		"Get-NetIPAddress | Select-Object InterfaceAlias, IPAddress, PrefixLength | Format-Table -AutoSize",
		"Get-NetAdapter | Select-Object Name, InterfaceDescription, Status, LinkSpeed | Format-Table -AutoSize",
		"Get-NetIPConfiguration | Format-Table -AutoSize",
		"Get-NetRoute | Sort-Object DestinationPrefix | Format-Table -AutoSize",
		"Get-DnsClientServerAddress | Select-Object InterfaceAlias, ServerAddresses | Format-Table -AutoSize",
		"ipconfig /all",
		"netstat -ano | findstr LISTEN",
		"Test-NetConnection -ComputerName {hostname} -InformationLevel Detailed",
		"Resolve-DnsName {hostname} -Type A",
		"Invoke-RestMethod -Uri http://ifconfig.me/all",
	}

	return Config{
		Enabled:       false, // Opt-in by default
		Timeout:       30 * time.Second,
		MaxOutputSize: 1048576, // 1MB
		AllowedCommands: AllowedCommands{
			Host:      hostCommands,
			Container: containerCommands,
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
