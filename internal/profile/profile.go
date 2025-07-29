// Package profile handles connection profile management and resolution.
//
// This package provides a clean separation of concerns for managing
// multiple connection profiles and resolving which profile to use.
package profile

import (
	"fmt"
	"os"

	"github.com/devnullvoid/proxmox-tui/internal/config"
)

// ResolveProfile determines which profile to use based on command line flags,
// environment variables, and configuration defaults.
func ResolveProfile(flagProfile string, cfg *config.Config) (string, error) {
	// Check command line flag first (highest priority)
	if flagProfile != "" {
		return flagProfile, nil
	}

	// Check environment variable
	if envProfile := os.Getenv("PROXMOX_TUI_PROFILE"); envProfile != "" {
		return envProfile, nil
	}

	// Check configuration default
	if cfg.DefaultProfile != "" {
		return cfg.DefaultProfile, nil
	}

	// If no explicit profile but profiles exist, use "default"
	if len(cfg.Profiles) > 0 {
		return "default", nil
	}

	// No profile selected
	return "", nil
}

// ValidateProfile ensures the selected profile exists and is valid.
func ValidateProfile(profileName string, cfg *config.Config) error {
	if profileName == "" {
		return nil // No profile selected is valid
	}

	if cfg.Profiles == nil {
		return fmt.Errorf("no profiles configured")
	}

	if _, exists := cfg.Profiles[profileName]; !exists {
		return fmt.Errorf("profile '%s' not found", profileName)
	}

	return nil
}

// ListProfiles returns a list of available profile names.
func ListProfiles(cfg *config.Config) []string {
	if cfg.Profiles == nil {
		return nil
	}

	profiles := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		profiles = append(profiles, name)
	}

	return profiles
}

// GetProfileConfig returns the configuration for a specific profile.
func GetProfileConfig(profileName string, cfg *config.Config) (*config.ProfileConfig, error) {
	if err := ValidateProfile(profileName, cfg); err != nil {
		return nil, err
	}

	profile := cfg.Profiles[profileName]
	return &profile, nil
}
