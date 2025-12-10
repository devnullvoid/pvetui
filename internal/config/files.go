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
	"runtime"
	"strings"

	"github.com/devnullvoid/pvetui/internal/logger"
	"gopkg.in/yaml.v3"
)

//go:embed config.tpl.yml
var templateFS embed.FS

// GetDefaultConfigPath returns the default configuration file path.
func GetDefaultConfigPath() string {
	return filepath.Join(getConfigDir(), "config.yml")
}

// CreateDefaultConfigFile creates a default configuration file and returns its path.
func CreateDefaultConfigFile() (string, error) {
	configDir := getConfigDir()
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

// getConfigDir returns the appropriate config directory path for the current platform.
func getConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		// Windows: Use %APPDATA% (Roaming) for config files
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "pvetui")
		}
		// Fallback to user home directory
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, "AppData", "Roaming", "pvetui")
		}
	default:
		// macOS, Linux, and other Unix-like systems: Use XDG Base Directory Specification
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, "pvetui")
		}
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, ".config", "pvetui")
		}
	}

	// Ultimate fallback
	return filepath.Join(".config", "pvetui")
}

// getCacheDir returns the appropriate cache directory path for the current platform.
func getCacheDir() string {
	switch runtime.GOOS {
	case "windows":
		// Windows: Use %LOCALAPPDATA% for cache files
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "pvetui")
		}
		// Fallback to user home directory
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, "AppData", "Local", "pvetui")
		}
	default:
		// macOS, Linux, and other Unix-like systems: Use XDG Base Directory Specification
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, "pvetui")
		}
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, ".cache", "pvetui")
		}
	}

	// Ultimate fallback
	return filepath.Join(".cache", "pvetui")
}

// getXDGConfigDir returns the XDG config directory path.
// * Deprecated: Use getConfigDir() instead for cross-platform support
func getXDGConfigDir() string {
	return getConfigDir()
}

// getXDGCacheDir returns the XDG cache directory path.
// * Deprecated: Use getCacheDir() instead for cross-platform support
func getXDGCacheDir() string {
	return getCacheDir()
}

// IsSOPSEncrypted checks if a file is SOPS-encrypted.
func IsSOPSEncrypted(path string, data []byte) bool {
	// Check for SOPS header patterns in the entire content
	content := string(data)
	// Check for "sops:" YAML key (most common) or "\"sops\":" JSON key
	hasSopsKey := contains(content, "sops:") || contains(content, "\"sops\":")
	// Check for ENC[ marker which is specific to SOPS/Age
	hasEnc := contains(content, "ENC[")

	if DebugEnabled {
		// Use global logger for debug output to avoid UI corruption
		if globalLogger := logger.GetGlobalLogger(); globalLogger != nil {
			globalLogger.Debug("SOPS detection for %s: hasSopsKey=%v, hasEnc=%v", path, hasSopsKey, hasEnc)
		}
	}

	return hasSopsKey || hasEnc
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

// SaveConfigFile saves the config to a file with optional encryption (if not using SOPS).
// This is a lower-level function that can be used from packages that can't import UI components.
func SaveConfigFile(cfg *Config, path string) error {
	// Check if SOPS is being used
	data, err := os.ReadFile(path)
	isSOPS := false
	if err == nil {
		isSOPS = IsSOPSEncrypted(path, data)
	}

	// If not using SOPS, encrypt sensitive fields before saving
	if !isSOPS {
		if err := EncryptConfigSensitiveFields(cfg); err != nil {
			// Log warning but continue - allow saving even if encryption fails
			if DebugEnabled {
				fmt.Printf("⚠️  Warning: Failed to encrypt some fields: %v\n", err)
			}
		}
	}

	// Marshal to YAML
	data, err = yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config to YAML: %w", err)
	}

	// Ensure the target directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	if cfg != nil {
		cfg.hasCleartextSensitive = false
	}

	return nil
}
