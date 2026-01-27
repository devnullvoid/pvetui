// Package config provides configuration management for the pvetui application.
//
// This package handles loading configuration from multiple sources with proper
// precedence ordering:
//  1. Command-line flags (highest priority)
//  2. Environment variables
//  3. Configuration files (YAML format)
//  4. Default values (lowest priority)
//
// The package follows platform-appropriate standards for configuration and
// cache file locations, providing a clean and predictable user experience across
// Windows, macOS, and Linux.
//
// Configuration Sources:
//
// Environment Variables:
//   - PROXMOX_ADDR: Proxmox server URL
//   - PROXMOX_USER: Username for authentication
//   - PROXMOX_PASSWORD: Password for password-based auth
//   - PROXMOX_TOKEN_ID: API token ID for token-based auth
//   - PROXMOX_TOKEN_SECRET: API token secret
//   - PROXMOX_REALM: Authentication realm (default: "pam")
//   - PROXMOX_INSECURE: Skip TLS verification ("true"/"false")
//   - PROXMOX_DEBUG: Enable debug logging ("true"/"false")
//   - PROXMOX_CACHE_DIR: Custom cache directory (overrides platform defaults)
//   - PVETUI_SHOW_ICONS: Show icons/emojis in the UI ("true"/"false", default: "true")
//
// Configuration File Format (YAML):
//
//	addr: "https://pve.example.com:8006"
//	user: "root"
//	password: "secret"
//	realm: "pam"
//	insecure: false
//	debug: true
//	cache_dir: "/custom/cache/path"  # Optional: overrides platform defaults
//	show_icons: true  # Optional: show icons/emojis in UI (default: true)
//
// Platform Directory Support:
//
// The package automatically determines appropriate directories for configuration
// and cache files based on platform standards:
//   - Windows: Config in %APPDATA%/pvetui, Cache in %LOCALAPPDATA%/pvetui
//   - macOS: Config in $XDG_CONFIG_HOME/pvetui or ~/.config/pvetui, Cache in $XDG_CACHE_HOME/pvetui or ~/.cache/pvetui
//   - Linux: Config in $XDG_CONFIG_HOME/pvetui or ~/.config/pvetui, Cache in $XDG_CACHE_HOME/pvetui or ~/.cache/pvetui
//
// Authentication Methods:
//
// The package supports both password and API token authentication:
//   - Password: Requires user + password + realm
//   - API Token: Requires user + token_id + token_secret + realm
//
// Example usage:
//
//	// Load configuration with automatic source detection
//	config := NewConfig()
//	config.ParseFlags()
//
//	// Merge with config file if specified
//	if configPath != "" {
//		err := config.MergeWithFile(configPath)
//		if err != nil {
//			log.Fatal("Failed to load config file:", err)
//		}
//	}
//
//	// Set defaults and validate
//	config.SetDefaults()
//	if err := config.Validate(); err != nil {
//		log.Fatal("Invalid configuration:", err)
//	}
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devnullvoid/pvetui/internal/keys"
	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

const (
	defaultRealm   = "pam"
	defaultApiPath = "/api2/json"
	trueString     = "true"
)

// DebugEnabled is a global flag to enable debug logging throughout the application.
//
// This variable is set during configuration parsing and used by various
// components to determine whether to emit debug-level log messages.
var DebugEnabled bool

// Config represents the complete application configuration, including multiple profiles.
type Config struct {
	Profiles       map[string]ProfileConfig `yaml:"profiles"`
	DefaultProfile string                   `yaml:"default_profile"`
	// ActiveProfile holds the currently active profile at runtime.
	// It is not persisted to disk and is used to resolve getters when set.
	ActiveProfile string `yaml:"-"`
	// hasCleartextSensitive tracks whether the last-loaded config file contained
	// unencrypted sensitive data. It is ignored by YAML marshaling.
	hasCleartextSensitive bool `yaml:"-"`
	// The following fields are global settings, not per-profile
	Debug    bool   `yaml:"debug"`
	CacheDir string `yaml:"cache_dir"`
	// AgeDir overrides the directory used to store age identity and recipient files.
	AgeDir      string       `yaml:"age_dir,omitempty"`
	KeyBindings KeyBindings  `yaml:"key_bindings"`
	Theme       ThemeConfig  `yaml:"theme"`
	Plugins     PluginConfig `yaml:"plugins"`
	ShowIcons   bool         `yaml:"show_icons"`
	// Deprecated: legacy single-profile fields for migration
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
}

func (c *Config) HasCleartextSensitiveData() bool {
	return c.hasCleartextSensitive
}

func (c *Config) MarkSensitiveDataEncrypted() {
	c.hasCleartextSensitive = false
}

// KeyBindings defines customizable key mappings for common actions.
// Each field represents a single keyboard key that triggers the action.
// Only single characters and function keys (e.g. "F1") are supported.
type KeyBindings struct {
	SwitchView        string `yaml:"switch_view"` // Switch between pages
	SwitchViewReverse string `yaml:"switch_view_reverse"`
	NodesPage         string `yaml:"nodes_page"`   // Jump to Nodes page
	GuestsPage        string `yaml:"guests_page"`  // Jump to Guests page
	TasksPage         string `yaml:"tasks_page"`   // Jump to Tasks page
	Menu              string `yaml:"menu"`         // Open context menu
	GlobalMenu        string `yaml:"global_menu"`  // Open global context menu
	Shell             string `yaml:"shell"`        // Open shell session
	VNC               string `yaml:"vnc"`          // Open VNC console
	Refresh           string `yaml:"refresh"`      // Manual refresh
	AutoRefresh       string `yaml:"auto_refresh"` // Toggle auto-refresh
	Search            string `yaml:"search"`       // Activate search
	Help              string `yaml:"help"`         // Toggle help modal
	Quit              string `yaml:"quit"`         // Quit application
}

// ThemeConfig defines theme-related configuration options.
type ThemeConfig struct {
	// Name specifies the built-in theme to use as a base (e.g., "default", "catppuccin-mocha").
	// If empty, defaults to "default".
	Name string `yaml:"name"`
	// Colors specifies the color overrides for theme elements.
	// Users can use any tcell-supported color value (ANSI name, W3C name, or hex code).
	Colors map[string]string `yaml:"colors"`
}

// PluginConfig holds plugin related configuration options.
type PluginConfig struct {
	// Enabled lists plugin identifiers that should be activated.
	Enabled []string `yaml:"enabled"`
}

// DefaultKeyBindings returns a KeyBindings struct with the default key mappings.
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		SwitchView:        "]",
		SwitchViewReverse: "[",
		NodesPage:         "Alt+1",
		GuestsPage:        "Alt+2",
		TasksPage:         "Alt+3",
		Menu:              "m",
		GlobalMenu:        "g",
		Shell:             "s",
		VNC:               "v",
		Refresh:           "Ctrl+r",
		AutoRefresh:       "a",
		Search:            "/",
		Help:              "?",
		Quit:              "q",
	}
}

// keyBindingsToMap converts a KeyBindings struct to a map for validation.
func keyBindingsToMap(kb KeyBindings) map[string]string {
	return map[string]string{
		"switch_view":         kb.SwitchView,
		"switch_view_reverse": kb.SwitchViewReverse,
		"nodes_page":          kb.NodesPage,
		"guests_page":         kb.GuestsPage,
		"tasks_page":          kb.TasksPage,
		"menu":                kb.Menu,
		"global_menu":         kb.GlobalMenu,
		"shell":               kb.Shell,
		"vnc":                 kb.VNC,
		"refresh":             kb.Refresh,
		"auto_refresh":        kb.AutoRefresh,
		"search":              kb.Search,
		"help":                kb.Help,
		"quit":                kb.Quit,
	}
}

// ValidateKeyBindings checks if all key specifications are valid.
func ValidateKeyBindings(kb KeyBindings) error {
	bindings := keyBindingsToMap(kb)
	defaultMap := keyBindingsToMap(DefaultKeyBindings())

	seen := make(map[string]string)

	for name, spec := range bindings {
		if spec == "" {
			continue
		}

		key, r, mod, err := keys.Parse(spec)
		if err != nil {
			return fmt.Errorf("invalid key binding %s: %w", name, err)
		}

		if keys.IsReserved(key, r, mod) && spec != defaultMap[name] {
			return fmt.Errorf("key binding %s uses reserved key %s", name, spec)
		}

		id := keys.CanonicalID(key, r, mod)
		if other, ok := seen[id]; ok {
			return fmt.Errorf("key binding %s duplicates %s", name, other)
		}

		seen[id] = name
	}

	return nil
}

// NewConfig creates a new Config instance populated with values from environment variables.
//
// This function reads all supported environment variables and creates a Config
// with those values. Environment variables that are not set will result in
// zero values for the corresponding fields.
//
// Environment variables read:
//   - PVETUI_ADDR: Server URL
//   - PVETUI_USER: Username
//   - PVETUI_PASSWORD: Password for password auth
//   - PVETUI_TOKEN_ID: Token ID for token auth
//   - PVETUI_TOKEN_SECRET: Token secret for token auth
//   - PVETUI_REALM: Authentication realm (default: "pam")
//   - PVETUI_API_PATH: API base path (default: "/api2/json")
//   - PVETUI_INSECURE: Skip TLS verification ("true"/"false")
//   - PVETUI_SSH_USER: SSH username
//   - PVETUI_AGE_DIR: Custom age key directory (overrides platform defaults)
//   - PVETUI_DEBUG: Enable debug logging ("true"/"false")
//   - PVETUI_CACHE_DIR: Custom cache directory (overrides platform defaults)
//   - PVETUI_SHOW_ICONS: Show icons/emojis in the UI ("true"/"false", default: "true")
//
// The returned Config should typically be further configured with command-line
// flags and/or configuration files before validation.
//
// Returns a new Config instance with environment variable values.
//
// Example usage:
//
//	config := NewConfig()
//	config.ParseFlags()
//	config.SetDefaults()
//	if err := config.Validate(); err != nil {
//		log.Fatal("Invalid config:", err)
//	}
func NewConfig() *Config {
	config := &Config{
		Profiles:       make(map[string]ProfileConfig),
		DefaultProfile: "default",
		// Read environment variables for legacy fields
		Addr:        os.Getenv("PVETUI_ADDR"),
		User:        os.Getenv("PVETUI_USER"),
		Password:    os.Getenv("PVETUI_PASSWORD"),
		TokenID:     os.Getenv("PVETUI_TOKEN_ID"),
		TokenSecret: os.Getenv("PVETUI_TOKEN_SECRET"),
		Realm:       os.Getenv("PVETUI_REALM"),
		ApiPath:     os.Getenv("PVETUI_API_PATH"),
		Insecure:    strings.ToLower(os.Getenv("PVETUI_INSECURE")) == trueString,
		SSHUser:     os.Getenv("PVETUI_SSH_USER"),
		AgeDir:      ExpandHomePath(os.Getenv("PVETUI_AGE_DIR")),
		SSHJumpHost: SSHJumpHost{
			Addr:    os.Getenv("PVETUI_SSH_JUMPHOST_ADDR"),
			User:    os.Getenv("PVETUI_SSH_JUMPHOST_USER"),
			Keyfile: os.Getenv("PVETUI_SSH_JUMPHOST_KEYFILE"),
		},
		Debug:       strings.ToLower(os.Getenv("PVETUI_DEBUG")) == trueString,
		CacheDir:    ExpandHomePath(os.Getenv("PVETUI_CACHE_DIR")),
		KeyBindings: DefaultKeyBindings(),
		ShowIcons:   strings.ToLower(os.Getenv("PVETUI_SHOW_ICONS")) != "false",
	}
	// Set default values for Realm and ApiPath only
	if config.Realm == "" {
		config.Realm = defaultRealm
	}
	if config.ApiPath == "" {
		config.ApiPath = defaultApiPath
	}
	if config.AgeDir != "" {
		SetAgeDirOverride(config.AgeDir)
	}

	return config
}

var (
	configPath string
	configFs   = flag.NewFlagSet("config", flag.ContinueOnError)
)

func init() {
	configFs.StringVar(&configPath, "config", "", "Path to YAML config file")
}

func ParseConfigFlags() {
	_ = configFs.Parse(os.Args[1:]) // Parse just the --config flag first, ignore errors
}

func (c *Config) MergeWithFile(path string) error {
	if path == "" {
		return nil
	}

	// Reset cleartext tracking; it will be re-evaluated based on file contents.
	c.hasCleartextSensitive = false

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	isSOPSEncrypted := IsSOPSEncrypted(path, data)
	if isSOPSEncrypted {
		decrypted, derr := decrypt.File(path, "yaml")
		if derr != nil {
			return derr
		}

		data = decrypted

		fmt.Printf("🔐 Decrypted config file: %s\n", path)
	}

	// Use a struct with pointers to distinguish between unset and explicitly set values
	var fileConfig struct {
		Profiles       map[string]ProfileConfig `yaml:"profiles"`
		DefaultProfile string                   `yaml:"default_profile"`
		Debug          *bool                    `yaml:"debug"`
		CacheDir       string                   `yaml:"cache_dir"`
		AgeDir         string                   `yaml:"age_dir"`
		KeyBindings    struct {
			SwitchView        string `yaml:"switch_view"`
			SwitchViewReverse string `yaml:"switch_view_reverse"`
			NodesPage         string `yaml:"nodes_page"`
			GuestsPage        string `yaml:"guests_page"`
			TasksPage         string `yaml:"tasks_page"`
			Menu              string `yaml:"menu"`
			GlobalMenu        string `yaml:"global_menu"`
			Shell             string `yaml:"shell"`
			VNC               string `yaml:"vnc"`
			Scripts           string `yaml:"scripts"`
			Refresh           string `yaml:"refresh"`
			AutoRefresh       string `yaml:"auto_refresh"`
			Search            string `yaml:"search"`
			Help              string `yaml:"help"`
			Quit              string `yaml:"quit"`
		} `yaml:"key_bindings"`
		Theme struct {
			Name   string            `yaml:"name"`
			Colors map[string]string `yaml:"colors"`
		} `yaml:"theme"`
		Plugins struct {
			Enabled []string `yaml:"enabled"`
		} `yaml:"plugins"`
		ShowIcons *bool `yaml:"show_icons"`
		// Legacy fields for migration
		Addr        string      `yaml:"addr"`
		User        string      `yaml:"user"`
		Password    string      `yaml:"password"`
		TokenID     string      `yaml:"token_id"`
		TokenSecret string      `yaml:"token_secret"`
		Realm       string      `yaml:"realm"`
		ApiPath     string      `yaml:"api_path"`
		Insecure    *bool       `yaml:"insecure"`
		SSHUser     string      `yaml:"ssh_user"`
		VMSSHUser   string      `yaml:"vm_ssh_user"`
		SSHJumpHost SSHJumpHost `yaml:"ssh_jump_host,omitempty"`
	}

	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return err
	}

	if !isSOPSEncrypted && detectCleartextSensitive(fileConfig.Profiles, fileConfig.Password, fileConfig.TokenSecret) {
		c.hasCleartextSensitive = true
	}

	if fileConfig.AgeDir != "" && c.AgeDir == "" {
		c.AgeDir = ExpandHomePath(fileConfig.AgeDir)
	}
	if c.AgeDir != "" {
		SetAgeDirOverride(c.AgeDir)
	}

	// Load profiles and default_profile
	if fileConfig.Profiles != nil {
		// Initialize profiles map if it doesn't exist
		if c.Profiles == nil {
			c.Profiles = make(map[string]ProfileConfig)
		}

		// Merge profiles from file into existing profiles
		for name, fileProfile := range fileConfig.Profiles {
			// Get existing profile or create new one
			existingProfile, exists := c.Profiles[name]
			if !exists {
				// If profile doesn't exist, just add it
				c.Profiles[name] = fileProfile
			} else {
				// Merge fields from file profile into existing profile
				if fileProfile.Addr != "" {
					existingProfile.Addr = fileProfile.Addr
				}
				if fileProfile.User != "" {
					existingProfile.User = fileProfile.User
				}
				if fileProfile.Password != "" {
					existingProfile.Password = fileProfile.Password
				}
				if fileProfile.TokenID != "" {
					existingProfile.TokenID = fileProfile.TokenID
				}
				if fileProfile.TokenSecret != "" {
					existingProfile.TokenSecret = fileProfile.TokenSecret
				}
				if fileProfile.Realm != "" {
					existingProfile.Realm = fileProfile.Realm
				}
				if fileProfile.ApiPath != "" {
					existingProfile.ApiPath = fileProfile.ApiPath
				}
				if fileProfile.Insecure {
					existingProfile.Insecure = fileProfile.Insecure
				}
				if fileProfile.SSHUser != "" {
					existingProfile.SSHUser = fileProfile.SSHUser
				}
				if fileProfile.VMSSHUser != "" {
					existingProfile.VMSSHUser = fileProfile.VMSSHUser
				}
				if fileProfile.SSHJumpHost.Addr != "" {
					existingProfile.SSHJumpHost = fileProfile.SSHJumpHost
				}

				c.Profiles[name] = existingProfile
			}
		}
	}

	if fileConfig.DefaultProfile != "" {
		c.DefaultProfile = fileConfig.DefaultProfile
	}

	// Merge legacy fields if no profiles are present
	if len(c.Profiles) == 0 && (fileConfig.Addr != "" || fileConfig.User != "") {
		if fileConfig.Addr != "" {
			c.Addr = fileConfig.Addr
		}
		if fileConfig.User != "" {
			c.User = fileConfig.User
		}
		if fileConfig.Password != "" {
			c.Password = fileConfig.Password
		}
		if fileConfig.TokenID != "" {
			c.TokenID = fileConfig.TokenID
		}
		if fileConfig.TokenSecret != "" {
			c.TokenSecret = fileConfig.TokenSecret
		}
		if fileConfig.Realm != "" {
			c.Realm = fileConfig.Realm
		}
		if fileConfig.ApiPath != "" {
			c.ApiPath = fileConfig.ApiPath
		}

		if fileConfig.Insecure != nil {
			c.Insecure = *fileConfig.Insecure
		}

		if fileConfig.SSHUser != "" {
			c.SSHUser = fileConfig.SSHUser
		}
		if fileConfig.VMSSHUser != "" {
			c.VMSSHUser = fileConfig.VMSSHUser
		}
		if fileConfig.SSHJumpHost.Addr != "" {
			c.SSHJumpHost = fileConfig.SSHJumpHost
		}
	}

	// Merge global settings
	if fileConfig.Debug != nil {
		c.Debug = *fileConfig.Debug
	}

	if fileConfig.CacheDir != "" {
		c.CacheDir = ExpandHomePath(fileConfig.CacheDir)
	}

	// Migrate legacy configuration to profile-based if needed
	if migrated := c.MigrateLegacyToProfiles(); migrated {
		fmt.Printf("🔄 Migrated legacy configuration to profile-based format\n")
	}

	// Merge key bindings if provided
	if kb := fileConfig.KeyBindings; kb != struct {
		SwitchView        string `yaml:"switch_view"`
		SwitchViewReverse string `yaml:"switch_view_reverse"`
		NodesPage         string `yaml:"nodes_page"`
		GuestsPage        string `yaml:"guests_page"`
		TasksPage         string `yaml:"tasks_page"`
		Menu              string `yaml:"menu"`
		GlobalMenu        string `yaml:"global_menu"`
		Shell             string `yaml:"shell"`
		VNC               string `yaml:"vnc"`
		Scripts           string `yaml:"scripts"`
		Refresh           string `yaml:"refresh"`
		AutoRefresh       string `yaml:"auto_refresh"`
		Search            string `yaml:"search"`
		Help              string `yaml:"help"`
		Quit              string `yaml:"quit"`
	}{} {
		if kb.SwitchView != "" {
			c.KeyBindings.SwitchView = kb.SwitchView
		}

		if kb.SwitchViewReverse != "" {
			c.KeyBindings.SwitchViewReverse = kb.SwitchViewReverse
		}

		if kb.NodesPage != "" {
			c.KeyBindings.NodesPage = kb.NodesPage
		}

		if kb.GuestsPage != "" {
			c.KeyBindings.GuestsPage = kb.GuestsPage
		}

		if kb.TasksPage != "" {
			c.KeyBindings.TasksPage = kb.TasksPage
		}

		if kb.Menu != "" {
			c.KeyBindings.Menu = kb.Menu
		}

		if kb.GlobalMenu != "" {
			c.KeyBindings.GlobalMenu = kb.GlobalMenu
		}

		if kb.Shell != "" {
			c.KeyBindings.Shell = kb.Shell
		}

		if kb.VNC != "" {
			c.KeyBindings.VNC = kb.VNC
		}

		if kb.Refresh != "" {
			c.KeyBindings.Refresh = kb.Refresh
		}

		if kb.AutoRefresh != "" {
			c.KeyBindings.AutoRefresh = kb.AutoRefresh
		}

		if kb.Search != "" {
			c.KeyBindings.Search = kb.Search
		}

		if kb.Help != "" {
			c.KeyBindings.Help = kb.Help
		}

		if kb.Quit != "" {
			c.KeyBindings.Quit = kb.Quit
		}
	}

	// Merge plugin configuration if provided
	if fileConfig.Plugins.Enabled != nil {
		c.Plugins.Enabled = append([]string{}, fileConfig.Plugins.Enabled...)
	}

	// Merge show_icons configuration if provided
	if fileConfig.ShowIcons != nil {
		c.ShowIcons = *fileConfig.ShowIcons
	}

	// Merge theme configuration if provided
	c.Theme.Name = fileConfig.Theme.Name
	c.Theme.Colors = make(map[string]string)

	for k, v := range fileConfig.Theme.Colors {
		c.Theme.Colors[k] = v
	}

	// Decrypt sensitive fields if not using SOPS
	// SOPS handles encryption/decryption itself, so we only decrypt age-encrypted fields
	if !IsSOPSEncrypted(path, data) {
		if err := DecryptConfigSensitiveFields(c); err != nil {
			// Log error but don't fail - allow cleartext to work
			if DebugEnabled {
				fmt.Printf("⚠️  Warning: Failed to decrypt some fields: %v\n", err)
			}
		}
	}

	return nil
}

func detectCleartextSensitive(profiles map[string]ProfileConfig, legacyPassword, legacyTokenSecret string) bool {
	if hasCleartextSensitiveProfiles(profiles) {
		return true
	}
	return hasCleartextSensitiveValue(legacyPassword) || hasCleartextSensitiveValue(legacyTokenSecret)
}

func hasCleartextSensitiveProfiles(profiles map[string]ProfileConfig) bool {
	for _, profile := range profiles {
		if hasCleartextSensitiveValue(profile.Password) || hasCleartextSensitiveValue(profile.TokenSecret) {
			return true
		}
	}
	return false
}

func hasCleartextSensitiveValue(value string) bool {
	return value != "" && !isEncrypted(value)
}

func (c *Config) Validate() error {
	// Validate profile-based configuration if profiles exist.
	if len(c.Profiles) > 0 {
		if err := c.ValidateGroups(); err != nil {
			return err
		}

		// Prefer active profile for validation; fall back to default.
		selection := c.ActiveProfile
		label := "selected profile"
		if selection == "" {
			selection = c.DefaultProfile
			label = "default profile"
		}

		if selection != "" {
			// Allow selecting an aggregate group as the active/default startup target.
			if c.IsGroup(selection) {
				memberNames := c.GetProfileNamesInGroup(selection)
				if len(memberNames) == 0 {
					return fmt.Errorf("%s group '%s' has no member profiles", label, selection)
				}

				for _, member := range memberNames {
					selectedProfile, exists := c.Profiles[member]
					if !exists {
						return fmt.Errorf("%s group '%s' references missing profile '%s'", label, selection, member)
					}

					if selectedProfile.Addr == "" {
						return fmt.Errorf("proxmox address required in %s group '%s' (profile '%s')", label, selection, member)
					}

					if selectedProfile.User == "" {
						return fmt.Errorf("proxmox username required in %s group '%s' (profile '%s')", label, selection, member)
					}

					hasPassword := selectedProfile.Password != ""
					hasToken := selectedProfile.TokenID != "" && selectedProfile.TokenSecret != ""

					if !hasPassword && !hasToken {
						return fmt.Errorf("authentication required in %s group '%s' (profile '%s'): provide either password or API token", label, selection, member)
					}

					if hasPassword && hasToken {
						return fmt.Errorf("conflicting authentication methods in %s group '%s' (profile '%s'): provide either password or API token, not both", label, selection, member)
					}
				}
			} else {
				selectedProfile, exists := c.Profiles[selection]
				if !exists {
					return fmt.Errorf("%s '%s' not found", label, selection)
				}

				if selectedProfile.Addr == "" {
					return errors.New("proxmox address required in " + label)
				}

				if selectedProfile.User == "" {
					return errors.New("proxmox username required in " + label)
				}

				hasPassword := selectedProfile.Password != ""
				hasToken := selectedProfile.TokenID != "" && selectedProfile.TokenSecret != ""

				if !hasPassword && !hasToken {
					return errors.New("authentication required in " + label + ": provide either password or API token")
				}

				if hasPassword && hasToken {
					return errors.New("conflicting authentication methods in " + label + ": provide either password or API token, not both")
				}
			}
		}
	} else {
		// Validate legacy configuration.
		if c.Addr == "" {
			return errors.New("proxmox address required: set via -addr flag, PVETUI_ADDR env var, or config file")
		}

		if c.User == "" {
			return errors.New("proxmox username required: set via -user flag, PVETUI_USER env var, or config file")
		}

		hasPassword := c.Password != ""
		hasToken := c.TokenID != "" && c.TokenSecret != ""

		if !hasPassword && !hasToken {
			return errors.New("authentication required: provide either password (-password flag, PVETUI_PASSWORD env var) or API token (-token-id/-token-secret flags, PVETUI_TOKEN_ID/PVETUI_TOKEN_SECRET env vars, or config file)")
		}

		if hasPassword && hasToken {
			return errors.New("conflicting authentication methods: provide either password or API token, not both")
		}
	}

	if err := ValidateKeyBindings(c.KeyBindings); err != nil {
		return err
	}

	return nil
}

// IsUsingTokenAuth returns true if the configuration is set up for API token authentication.
func (c *Config) IsUsingTokenAuth() bool {
	return c.TokenID != "" && c.TokenSecret != ""
}

// GetAPIToken returns the full API token string in the format required by Proxmox
// Format: PVEAPIToken=USER@REALM!TOKENID=SECRET.
func (c *Config) GetAPIToken() string {
	if !c.IsUsingTokenAuth() {
		return ""
	}

	return fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s", c.User, c.Realm, c.TokenID, c.TokenSecret)
}

// GetAddr returns the configured server address.
func (c *Config) GetAddr() string {
	// Prefer active profile if set
	if len(c.Profiles) > 0 {
		if c.ActiveProfile != "" {
			if profile, exists := c.Profiles[c.ActiveProfile]; exists {
				return profile.Addr
			}
		}
		if c.DefaultProfile != "" {
			if profile, exists := c.Profiles[c.DefaultProfile]; exists {
				return profile.Addr
			}
		}
	}
	return c.Addr
}

// GetUser returns the configured username.
func (c *Config) GetUser() string {
	if len(c.Profiles) > 0 {
		if c.ActiveProfile != "" {
			if profile, exists := c.Profiles[c.ActiveProfile]; exists {
				return profile.User
			}
		}
		if c.DefaultProfile != "" {
			if profile, exists := c.Profiles[c.DefaultProfile]; exists {
				return profile.User
			}
		}
	}
	return c.User
}

// GetPassword returns the configured password.
func (c *Config) GetPassword() string {
	if len(c.Profiles) > 0 {
		if c.ActiveProfile != "" {
			if profile, exists := c.Profiles[c.ActiveProfile]; exists {
				return profile.Password
			}
		}
		if c.DefaultProfile != "" {
			if profile, exists := c.Profiles[c.DefaultProfile]; exists {
				return profile.Password
			}
		}
	}
	return c.Password
}

// GetRealm returns the configured realm.
func (c *Config) GetRealm() string {
	if len(c.Profiles) > 0 {
		if c.ActiveProfile != "" {
			if profile, exists := c.Profiles[c.ActiveProfile]; exists {
				return profile.Realm
			}
		}
		if c.DefaultProfile != "" {
			if profile, exists := c.Profiles[c.DefaultProfile]; exists {
				return profile.Realm
			}
		}
	}
	return c.Realm
}

// GetTokenID returns the configured token ID.
func (c *Config) GetTokenID() string {
	if len(c.Profiles) > 0 {
		if c.ActiveProfile != "" {
			if profile, exists := c.Profiles[c.ActiveProfile]; exists {
				return profile.TokenID
			}
		}
		if c.DefaultProfile != "" {
			if profile, exists := c.Profiles[c.DefaultProfile]; exists {
				return profile.TokenID
			}
		}
	}
	return c.TokenID
}

// GetTokenSecret returns the configured token secret.
func (c *Config) GetTokenSecret() string {
	if len(c.Profiles) > 0 {
		if c.ActiveProfile != "" {
			if profile, exists := c.Profiles[c.ActiveProfile]; exists {
				return profile.TokenSecret
			}
		}
		if c.DefaultProfile != "" {
			if profile, exists := c.Profiles[c.DefaultProfile]; exists {
				return profile.TokenSecret
			}
		}
	}
	return c.TokenSecret
}

// GetInsecure returns the configured insecure flag.
func (c *Config) GetInsecure() bool {
	if len(c.Profiles) > 0 {
		if c.ActiveProfile != "" {
			if profile, exists := c.Profiles[c.ActiveProfile]; exists {
				return profile.Insecure
			}
		}
		if c.DefaultProfile != "" {
			if profile, exists := c.Profiles[c.DefaultProfile]; exists {
				return profile.Insecure
			}
		}
	}
	return c.Insecure
}

// SetDefaults sets default values for unspecified configuration options.
func (c *Config) SetDefaults() {
	if c.Realm == "" {
		c.Realm = "pam"
	}

	if c.ApiPath == "" {
		c.ApiPath = "/api2/json"
	}

	if c.CacheDir != "" {
		c.CacheDir = ExpandHomePath(c.CacheDir)
	}
	if c.CacheDir == "" {
		// Use platform-appropriate cache directory
		c.CacheDir = getCacheDir()
	}
	if c.AgeDir != "" {
		c.AgeDir = ExpandHomePath(c.AgeDir)
		SetAgeDirOverride(c.AgeDir)
	}

	// Apply default key bindings if not set
	defaults := DefaultKeyBindings()
	if c.KeyBindings.SwitchView == "" {
		c.KeyBindings.SwitchView = defaults.SwitchView
	}

	if c.KeyBindings.SwitchViewReverse == "" {
		c.KeyBindings.SwitchViewReverse = defaults.SwitchViewReverse
	}

	if c.KeyBindings.NodesPage == "" {
		c.KeyBindings.NodesPage = defaults.NodesPage
	}

	if c.KeyBindings.GuestsPage == "" {
		c.KeyBindings.GuestsPage = defaults.GuestsPage
	}

	if c.KeyBindings.TasksPage == "" {
		c.KeyBindings.TasksPage = defaults.TasksPage
	}

	if c.KeyBindings.Menu == "" {
		c.KeyBindings.Menu = defaults.Menu
	}

	if c.KeyBindings.GlobalMenu == "" {
		c.KeyBindings.GlobalMenu = defaults.GlobalMenu
	}

	if c.KeyBindings.Shell == "" {
		c.KeyBindings.Shell = defaults.Shell
	}

	if c.KeyBindings.VNC == "" {
		c.KeyBindings.VNC = defaults.VNC
	}

	if c.KeyBindings.Refresh == "" {
		c.KeyBindings.Refresh = defaults.Refresh
	}

	if c.KeyBindings.AutoRefresh == "" {
		c.KeyBindings.AutoRefresh = defaults.AutoRefresh
	}

	if c.KeyBindings.Search == "" {
		c.KeyBindings.Search = defaults.Search
	}

	if c.KeyBindings.Help == "" {
		c.KeyBindings.Help = defaults.Help
	}

	if c.KeyBindings.Quit == "" {
		c.KeyBindings.Quit = defaults.Quit
	}

	// Set default theme configuration only if not already set
	if c.Theme.Colors == nil {
		c.Theme.Colors = make(map[string]string)
	}

	if c.Plugins.Enabled == nil {
		c.Plugins.Enabled = []string{}
	}

	// ShowIcons defaults to true (icons enabled)
	// No explicit default needed since it's already set in NewConfig()
}

// ExpandHomePath expands a leading ~ in paths using the current user's home directory.
func ExpandHomePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}

	if trimmed == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return trimmed
		}
		return home
	}

	if strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return trimmed
		}
		rest := strings.TrimPrefix(trimmed, "~")
		rest = strings.TrimPrefix(rest, "/")
		rest = strings.TrimPrefix(rest, "\\")
		return filepath.Join(home, rest)
	}

	if strings.HasPrefix(trimmed, "~"+string(os.PathSeparator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return trimmed
		}
		rest := strings.TrimPrefix(trimmed, "~"+string(os.PathSeparator))
		return filepath.Join(home, rest)
	}

	return trimmed
}
