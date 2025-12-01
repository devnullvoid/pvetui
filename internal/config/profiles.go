// Package config provides profile management functionality.
//
// This file contains profile-related types and functions that were
// previously part of the main config.go file.
package config

import (
	"fmt"
	"sort"
	"strings"
)

// ProfileConfig holds a single connection profile's settings.
type ProfileConfig struct {
	Addr        string `yaml:"addr"`
	User        string `yaml:"user"`
	Password    string `yaml:"password"`
	TokenID     string `yaml:"token_id"`
	TokenSecret string `yaml:"token_secret"`
	Realm       string `yaml:"realm"`
	ApiPath     string `yaml:"api_path"`
	Insecure    bool   `yaml:"insecure"`
	SSHUser     string `yaml:"ssh_user"`
	VMSSHUser   string `yaml:"vm_ssh_user"`

	// Aggregate is an optional group identifier for aggregate cluster mode.
	// Profiles with the same non-empty Aggregate value will be combined
	// into a single "aggregate cluster" view, showing all nodes and VMs
	// from all grouped profiles together.
	//
	// Example: Setting aggregate: "homelab" on multiple profiles will
	// allow selecting "homelab" as an aggregate view in the profile picker.
	//
	// Leave empty for standalone profile behavior (default).
	Aggregate string `yaml:"aggregate,omitempty"`
}

// ApplyProfile applies the settings from a named profile to the main config.
func (c *Config) ApplyProfile(profileName string) error {
	if c.Profiles == nil {
		return fmt.Errorf("no profiles configured")
	}

	profile, exists := c.Profiles[profileName]
	if !exists {
		return fmt.Errorf("profile '%s' not found", profileName)
	}

	// Apply profile settings to main config (legacy compatibility)
	c.Addr = profile.Addr
	c.User = profile.User
	c.Password = profile.Password
	c.TokenID = profile.TokenID
	c.TokenSecret = profile.TokenSecret
	c.Realm = profile.Realm
	c.ApiPath = profile.ApiPath
	c.Insecure = profile.Insecure
	c.SSHUser = profile.SSHUser
	c.VMSSHUser = profile.VMSSHUser

	// Mark runtime active profile so getters resolve to this profile without changing persisted default
	c.ActiveProfile = profileName

	return nil
}

// MigrateLegacyToProfiles migrates legacy configuration fields to the new profile system.
func (c *Config) MigrateLegacyToProfiles() bool {
	// Check if we have legacy fields but no profiles
	hasLegacyFields := c.Addr != "" || c.User != "" || c.Password != "" ||
		c.TokenID != "" || c.TokenSecret != "" || c.Realm != "" ||
		c.ApiPath != "" || c.SSHUser != "" || c.VMSSHUser != ""

	if !hasLegacyFields || len(c.Profiles) > 0 {
		return false
	}

	// Create profiles map if it doesn't exist
	if c.Profiles == nil {
		c.Profiles = make(map[string]ProfileConfig)
	}

	// Create a "default" profile from legacy fields
	c.Profiles["default"] = ProfileConfig{
		Addr:        c.Addr,
		User:        c.User,
		Password:    c.Password,
		TokenID:     c.TokenID,
		TokenSecret: c.TokenSecret,
		Realm:       c.Realm,
		ApiPath:     c.ApiPath,
		Insecure:    c.Insecure,
		SSHUser:     c.SSHUser,
		VMSSHUser:   c.VMSSHUser,
	}

	// Set default profile
	c.DefaultProfile = "default"

	// Clear legacy fields
	c.Addr = ""
	c.User = ""
	c.Password = ""
	c.TokenID = ""
	c.TokenSecret = ""
	c.Realm = ""
	c.ApiPath = ""
	c.Insecure = false
	c.SSHUser = ""
	c.VMSSHUser = ""

	return true
}

// Validate validates a single profile configuration.
func (p *ProfileConfig) Validate() error {
	if p.Addr == "" {
		return fmt.Errorf("profile address is required")
	}

	// Normalize address
	p.Addr = strings.TrimRight(p.Addr, "/")

	if p.User == "" {
		return fmt.Errorf("profile username is required")
	}

	// Check authentication method
	hasPassword := p.Password != ""
	hasToken := p.TokenID != "" && p.TokenSecret != ""

	if !hasPassword && !hasToken {
		return fmt.Errorf("profile must have either password or token authentication")
	}

	if hasPassword && hasToken {
		return fmt.Errorf("profile cannot have both password and token authentication")
	}

	if p.Realm == "" {
		p.Realm = "pam" // Default realm
	}

	return nil
}

// GetProfileNames returns a list of available profile names.
func (c *Config) GetProfileNames() []string {
	if c.Profiles == nil {
		return nil
	}

	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}

	return names
}

// HasProfiles returns true if the configuration has any profiles defined.
func (c *Config) HasProfiles() bool {
	return c.Profiles != nil
}

// GetActiveProfile returns the name of the currently active profile.
func (c *Config) GetActiveProfile() string {
	if c.ActiveProfile != "" {
		return c.ActiveProfile
	}
	return c.DefaultProfile
}

// GetAggregateGroups returns a map of aggregate group names to their member profile names.
// Only includes profiles that have a non-empty Aggregate field.
func (c *Config) GetAggregateGroups() map[string][]string {
	groups := make(map[string][]string)

	for name, profile := range c.Profiles {
		if profile.Aggregate != "" {
			groups[profile.Aggregate] = append(groups[profile.Aggregate], name)
		}
	}

	// Sort profile names within each group for consistent ordering
	for groupName := range groups {
		sort.Strings(groups[groupName])
	}

	return groups
}

// GetProfilesInAggregate returns all profiles belonging to a specific aggregate group.
func (c *Config) GetProfilesInAggregate(aggregateName string) []ProfileConfig {
	var profiles []ProfileConfig

	for _, profile := range c.Profiles {
		if profile.Aggregate == aggregateName {
			profiles = append(profiles, profile)
		}
	}

	return profiles
}

// GetProfileNamesInAggregate returns profile names belonging to a specific aggregate group.
func (c *Config) GetProfileNamesInAggregate(aggregateName string) []string {
	var names []string

	for name, profile := range c.Profiles {
		if profile.Aggregate == aggregateName {
			names = append(names, name)
		}
	}

	sort.Strings(names)
	return names
}

// IsAggregateGroup checks if a name refers to an aggregate group rather than a profile.
func (c *Config) IsAggregateGroup(name string) bool {
	groups := c.GetAggregateGroups()
	_, exists := groups[name]
	return exists
}

// HasAggregateGroups returns true if any profiles are configured with aggregate groups.
func (c *Config) HasAggregateGroups() bool {
	for _, profile := range c.Profiles {
		if profile.Aggregate != "" {
			return true
		}
	}
	return false
}

// ValidateAggregateGroups checks that aggregate group configurations are valid.
func (c *Config) ValidateAggregateGroups() error {
	groups := c.GetAggregateGroups()

	for groupName := range groups {
		// Check for naming conflicts between profiles and aggregate groups
		if _, exists := c.Profiles[groupName]; exists {
			return fmt.Errorf("aggregate group name '%s' conflicts with profile name", groupName)
		}
	}

	return nil
}
