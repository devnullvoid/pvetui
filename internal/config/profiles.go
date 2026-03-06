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

// SSHJumpHost holds configuration for an SSH jump server.
type SSHJumpHost struct {
	Addr    string `yaml:"addr,omitempty"`
	User    string `yaml:"user,omitempty"`
	Keyfile string `yaml:"keyfile,omitempty"`
	Port    int    `yaml:"port,omitempty"`
}

// ProfileConfig holds a single connection profile's settings.
type ProfileConfig struct {
	Addr        string      `yaml:"addr"`
	User        string      `yaml:"user"`
	Password    string      `yaml:"password"`
	TokenID     string      `yaml:"token_id"`
	TokenSecret string      `yaml:"token_secret"`
	Realm       string      `yaml:"realm"`
	ApiPath     string      `yaml:"api_path"`
	Insecure    bool        `yaml:"insecure"`
	SSHUser     string      `yaml:"ssh_user"`
	VMSSHUser   string      `yaml:"vm_ssh_user"`
	SSHJumpHost SSHJumpHost `yaml:"ssh_jump_host,omitempty"`

	// Groups is a list of group identifiers.
	// This allows a profile to belong to multiple groups.
	// Profiles in the same group will be combined into a single "group cluster" view.
	Groups []string `yaml:"groups,omitempty"`
}


// GroupMode constants define the operational mode for a group.
const (
	// GroupModeAggregate is the default mode: all profiles connect simultaneously
	// and their data is merged into a unified view (multi-cluster).
	GroupModeAggregate = "aggregate"

	// GroupModeCluster connects to a single profile at a time with automatic
	// failover to the next profile when the active one becomes unreachable.
	// This is intended for multiple nodes of the same Proxmox cluster.
	GroupModeCluster = "cluster"
)

// GroupSettingsConfig holds per-group configuration options.
type GroupSettingsConfig struct {
	// Mode determines the operational mode for the group.
	// "aggregate" (default): connect to all profiles, merge data.
	// "cluster": connect to one profile at a time with HA failover.
	Mode string `yaml:"mode"`
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
	c.SSHJumpHost = profile.SSHJumpHost

	// Mark runtime active profile so getters resolve to this profile without changing persisted default
	c.ActiveProfile = profileName

	return nil
}

// MigrateLegacyToProfiles migrates legacy configuration fields to the new profile system.
func (c *Config) MigrateLegacyToProfiles() bool {
	// Check if we have legacy fields but no profiles
	hasLegacyFields := c.Addr != "" || c.User != "" || c.Password != "" ||
		c.TokenID != "" || c.TokenSecret != "" || c.Realm != "" ||
		c.ApiPath != "" || c.SSHUser != "" || c.VMSSHUser != "" || c.SSHJumpHost.Addr != ""

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
		SSHJumpHost: c.SSHJumpHost,
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
	c.SSHJumpHost = SSHJumpHost{}

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

// GetGroups returns a map of group names to their member profile names.
// Only includes profiles that have non-empty Groups.
func (c *Config) GetGroups() map[string][]string {
	groups := make(map[string][]string)

	for name, profile := range c.Profiles {
		for _, g := range profile.Groups {
			if g != "" {
				groups[g] = append(groups[g], name)
			}
		}
	}

	// Sort profile names within each group for consistent ordering
	for groupName := range groups {
		sort.Strings(groups[groupName])
	}

	return groups
}

// GetProfilesInGroup returns all profiles belonging to a specific group.
func (c *Config) GetProfilesInGroup(groupName string) []ProfileConfig {
	var profiles []ProfileConfig

	for _, profile := range c.Profiles {
		inGroup := false
		for _, g := range profile.Groups {
			if g == groupName {
				inGroup = true
				break
			}
		}

		if inGroup {
			profiles = append(profiles, profile)
		}
	}

	return profiles
}

// GetProfileNamesInGroup returns profile names belonging to a specific group.
func (c *Config) GetProfileNamesInGroup(groupName string) []string {
	var names []string

	for name, profile := range c.Profiles {
		inGroup := false
		for _, g := range profile.Groups {
			if g == groupName {
				inGroup = true
				break
			}
		}

		if inGroup {
			names = append(names, name)
		}
	}

	sort.Strings(names)
	return names
}

// IsGroup checks if a name refers to a group rather than a profile.
func (c *Config) IsGroup(name string) bool {
	groups := c.GetGroups()
	_, exists := groups[name]
	return exists
}

// HasGroups returns true if any profiles are configured with groups.
func (c *Config) HasGroups() bool {
	for _, profile := range c.Profiles {
		if len(profile.Groups) > 0 {
			return true
		}
	}
	return false
}

// ValidateGroups checks that group configurations are valid.
// This validates naming conflicts and group_settings entries.
func (c *Config) ValidateGroups() error {
	groups := c.GetGroups()
	for groupName := range groups {
		// Check for naming conflicts between profiles and groups
		if _, exists := c.Profiles[groupName]; exists {
			return fmt.Errorf("group name '%s' conflicts with profile name", groupName)
		}
	}

	// Validate group_settings entries
	for name, settings := range c.GroupSettings {
		// Group settings must reference an actual group
		if _, exists := groups[name]; !exists {
			return fmt.Errorf("group_settings '%s' does not match any group", name)
		}

		// Validate mode value
		switch settings.Mode {
		case GroupModeAggregate, GroupModeCluster, "":
			// valid
		default:
			return fmt.Errorf("group_settings '%s' has invalid mode '%s' (must be '%s' or '%s')",
				name, settings.Mode, GroupModeAggregate, GroupModeCluster)
		}

	}
	return nil
}

// FindGroupProfileNameConflicts returns group names that also exist as profile names.
func (c *Config) FindGroupProfileNameConflicts() []string {
	groups := c.GetGroups()
	conflicts := make([]string, 0)

	for groupName := range groups {
		if _, exists := c.Profiles[groupName]; exists {
			conflicts = append(conflicts, groupName)
		}
	}

	sort.Strings(conflicts)
	return conflicts
}

// GetGroupMode returns the operational mode for a group.
// Returns GroupModeCluster if configured, otherwise GroupModeAggregate (default).
func (c *Config) GetGroupMode(groupName string) string {
	if settings, exists := c.GroupSettings[groupName]; exists {
		if settings.Mode == GroupModeCluster {
			return GroupModeCluster
		}
	}
	return GroupModeAggregate
}

// IsClusterGroup returns true if the named group is configured in cluster (HA failover) mode.
func (c *Config) IsClusterGroup(groupName string) bool {
	return c.GetGroupMode(groupName) == GroupModeCluster
}

// SetGroupMode sets the operational mode for a group.
// Creates the GroupSettings map if needed.
func (c *Config) SetGroupMode(groupName string, mode string) {
	if c.GroupSettings == nil {
		c.GroupSettings = make(map[string]GroupSettingsConfig)
	}
	if mode == GroupModeAggregate || mode == "" {
		// Aggregate is the default — remove the entry to keep config clean
		delete(c.GroupSettings, groupName)
		return
	}
	c.GroupSettings[groupName] = GroupSettingsConfig{Mode: mode}
}
