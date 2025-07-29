// Package config provides file operations for configuration management.
//
// This file contains file-related functions that were previously part
// of the main config.go file.
package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/logger"
)

//go:embed config.tpl.yml
var templateFS embed.FS

// GetDefaultConfigPath returns the default configuration file path.
func GetDefaultConfigPath() string {
	return filepath.Join(getXDGConfigDir(), "config.yml")
}

// CreateDefaultConfigFile creates a default configuration file and returns its path.
func CreateDefaultConfigFile() (string, error) {
	configDir := getXDGConfigDir()
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil // File already exists
	}

	// Read template
	templateData, err := templateFS.ReadFile("config.tpl.yml")
	if err != nil {
		return "", fmt.Errorf("read template: %w", err)
	}

	// Write template to config file
	if err := os.WriteFile(configPath, templateData, 0o600); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}

	return configPath, nil
}

// FindDefaultConfigPath finds the default configuration file path.
func FindDefaultConfigPath() (string, bool) {
	configPath := GetDefaultConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		return configPath, true
	}

	// Check for config in current directory
	if _, err := os.Stat("config.yml"); err == nil {
		return "config.yml", true
	}

	return "", false
}

// getXDGConfigDir returns the XDG config directory path.
func getXDGConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "proxmox-tui")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "proxmox-tui")
	}

	return filepath.Join(homeDir, ".config", "proxmox-tui")
}

// getXDGCacheDir returns the XDG cache directory path.
func getXDGCacheDir() string {
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "proxmox-tui")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cache", "proxmox-tui")
	}

	return filepath.Join(homeDir, ".cache", "proxmox-tui")
}

// IsSOPSEncrypted checks if a file is SOPS-encrypted.
func IsSOPSEncrypted(path string, data []byte) bool {
	// Check for SOPS header patterns in the entire content
	content := string(data)
	hasSops := contains(content, "sops")
	hasEnc := contains(content, "ENC[")

	if DebugEnabled {
		// Use global logger for debug output to avoid UI corruption
		if globalLogger := logger.GetGlobalLogger(); globalLogger != nil {
			globalLogger.Debug("SOPS detection for %s: hasSops=%v, hasEnc=%v", path, hasSops, hasEnc)
		}
	}

	return hasSops || hasEnc
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// FindSOPSRule checks if a SOPS rule file exists in the given directory or its parents.
func FindSOPSRule(startDir string) bool {
	current := startDir
	for {
		rulePath := filepath.Join(current, ".sops.yaml")
		if _, err := os.Stat(rulePath); err == nil {
			return true
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // Reached root
		}
		current = parent
	}
	return false
}
