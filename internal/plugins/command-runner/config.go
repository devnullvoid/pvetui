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

// CommandInfo holds a command and its user-friendly description
type CommandInfo struct {
	Command     string
	Description string
}

// GetCommandDescription returns a user-friendly description for a command.
// If no description is found, returns a generic description based on the command.
func GetCommandDescription(cmd string) string {
	// Map of command patterns to descriptions
	descriptions := map[string]string{
		// System overview
		"uptime":  "Show system uptime and load average",
		"df -Th":  "Show disk space usage by filesystem",
		"lsblk":   "List all block devices",
		"free -h": "Show memory usage (RAM and swap)",

		// Process and resource inspection
		"ps aux":              "Show all running processes",
		"top -bn1 | head -20": "Show top 20 processes by resource usage",
		"ps -eo pid,ppid,cmd,%cpu,%mem --sort=-%cpu | head -20": "Show top 20 CPU-consuming processes",
		"ps -eo pid,ppid,cmd,%cpu,%mem --sort=-%mem | head -20": "Show top 20 memory-consuming processes",

		// Services, logs, packages
		"systemctl list-units --type=service --state=running": "List all running systemd services",
		"systemctl list-unit-files --state=enabled":           "List enabled systemd services",
		"systemctl status {service}":                          "Show status of a specific service",
		"journalctl -n 100":                                   "Show last 100 system log entries",
		"journalctl -u {service} -n 50":                       "Show last 50 log entries for a service",
		"dpkg -l | grep {package}":                            "Search for installed Debian package",

		// Networking & connectivity
		"ip addr show":         "Show network interfaces and IP addresses",
		"ip route show":        "Show routing table",
		"ip link show":         "Show network interface status",
		"ss -tulpn":            "Show listening TCP/UDP ports",
		"ss -s":                "Show socket statistics summary",
		"netstat -tulpn":       "Show listening ports (netstat)",
		"cat /etc/resolv.conf": "Show DNS resolver configuration",
		"resolvectl status 2>/dev/null || systemd-resolve --status 2>/dev/null": "Show DNS resolution status",
		"curl https://ifconfig.me/all":                                          "Check public IP and network info",
		"nslookup {hostname}":                                                   "Look up DNS records for a hostname",

		// User activity
		"who":        "Show logged-in users",
		"last -n 20": "Show last 20 user logins",

		// Proxmox host-specific
		"pveversion -v": "Show Proxmox VE version and packages",
		"iostat -x 1 5": "Show disk I/O statistics (5 samples)",

		// Windows PowerShell commands
		"Get-ComputerInfo | Select-Object CsName, OsName, OsVersion, WindowsProductName":                                    "Show Windows system information",
		"Get-Process | Sort-Object CPU -Descending | Select-Object -First 15 Name, CPU, PM | Format-Table":                  "Show top 15 processes by CPU usage",
		"Get-Process | Sort-Object PM -Descending | Select-Object -First 15 Name, CPU, PM | Format-Table":                   "Show top 15 processes by memory usage",
		"Get-Service | Sort-Object Status, DisplayName | Format-Table -AutoSize -First 40":                                  "List Windows services (first 40)",
		"Get-EventLog -LogName System -Newest 50 | Format-Table TimeGenerated, EntryType, Source, Message -AutoSize":        "Show last 50 system event log entries",
		"Get-Volume | Select-Object DriveLetter, FileSystemLabel, FileSystem, SizeRemaining, Size | Format-Table -AutoSize": "Show disk volumes and space",
		"Get-NetIPAddress | Select-Object InterfaceAlias, IPAddress, PrefixLength | Format-Table -AutoSize":                 "Show network IP addresses",
		"Get-NetAdapter | Select-Object Name, InterfaceDescription, Status, LinkSpeed | Format-Table -AutoSize":             "Show network adapters status",
		"Get-NetIPConfiguration | Format-Table -AutoSize":                                                                   "Show network configuration",
		"Get-NetRoute | Sort-Object DestinationPrefix | Format-Table -AutoSize":                                             "Show network routing table",
		"Get-DnsClientServerAddress | Select-Object InterfaceAlias, ServerAddresses | Format-Table -AutoSize":               "Show DNS server configuration",
		"ipconfig /all":                 "Show detailed network configuration",
		"netstat -ano | findstr LISTEN": "Show listening ports",
		"Test-NetConnection -ComputerName {hostname} -InformationLevel Detailed": "Test network connection to a host",
		"Resolve-DnsName {hostname} -Type A":                                     "Resolve DNS A records for a hostname",
		"Invoke-RestMethod -Uri http://ifconfig.me/all":                          "Check public IP and network info",
	}

	// Check for exact match first
	if desc, ok := descriptions[cmd]; ok {
		return desc
	}

	// For commands not in the map, try to generate a description from the command itself
	// Extract the main command (first word)
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		mainCmd := parts[0]

		// Check if it has template parameters
		if strings.Contains(cmd, "{") && strings.Contains(cmd, "}") {
			return fmt.Sprintf("Run: %s (requires parameters)", mainCmd)
		}

		return fmt.Sprintf("Run: %s", strings.TrimSpace(cmd))
	}

	return "Execute command"
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
