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
	"strconv"
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
	// globalMenuConfigured indicates key_bindings.global_menu was explicitly set
	// in config input, including an empty string to intentionally disable it.
	globalMenuConfigured bool `yaml:"-"`
	// The following fields are global settings, not per-profile
	Debug    bool   `yaml:"debug"`
	CacheDir string `yaml:"cache_dir"`
	// AgeDir overrides the directory used to store age identity and recipient files.
	AgeDir        string                         `yaml:"age_dir,omitempty"`
	KeyBindings   KeyBindings                    `yaml:"key_bindings"`
	Theme         ThemeConfig                    `yaml:"theme"`
	Plugins       PluginConfig                   `yaml:"plugins"`
	ShowIcons     bool                           `yaml:"show_icons"`
	GroupSettings map[string]GroupSettingsConfig `yaml:"group_settings,omitempty"`
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
	SwitchView          string `yaml:"switch_view"` // Switch between pages
	SwitchViewReverse   string `yaml:"switch_view_reverse"`
	NodesPage           string `yaml:"nodes_page"`            // Jump to Nodes page
	GuestsPage          string `yaml:"guests_page"`           // Jump to Guests page
	TasksPage           string `yaml:"tasks_page"`            // Jump to Tasks page
	StoragePage         string `yaml:"storage_page"`          // Jump to Storage page
	TasksToggleQueue    string `yaml:"tasks_toggle_queue"`    // Toggle active queue panel in Tasks page
	TaskStopCancel      string `yaml:"task_stop_cancel"`      // Stop running task / cancel queued task
	Menu                string `yaml:"menu"`                  // Open context menu
	GlobalMenu          string `yaml:"global_menu"`           // Open global context menu
	Shell               string `yaml:"shell"`                 // Open shell session
	VNC                 string `yaml:"vnc"`                   // Open VNC console
	Refresh             string `yaml:"refresh"`               // Manual refresh
	AutoRefresh         string `yaml:"auto_refresh"`          // Toggle auto-refresh
	Search              string `yaml:"search"`                // Activate search
	AdvancedGuestFilter string `yaml:"advanced_guest_filter"` // Open advanced guest filter modal
	Help                string `yaml:"help"`                  // Toggle help modal
	Quit                string `yaml:"quit"`                  // Quit application
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
	Enabled []string            `yaml:"enabled"`
	Ansible AnsiblePluginConfig `yaml:"ansible,omitempty"`
}

// AnsiblePluginConfig holds configuration for the ansible plugin.
type AnsiblePluginConfig struct {
	InventoryFormat   string                 `yaml:"inventory_format,omitempty"`
	InventoryStyle    string                 `yaml:"inventory_style,omitempty"`
	InventoryVars     map[string]string      `yaml:"inventory_vars,omitempty"`
	DefaultUser       string                 `yaml:"default_user,omitempty"`
	DefaultPassword   string                 `yaml:"default_password,omitempty"`
	SSHPrivateKeyFile string                 `yaml:"ssh_private_key_file,omitempty"`
	DefaultLimitMode  string                 `yaml:"default_limit_mode,omitempty"`
	AskPass           bool                   `yaml:"ask_pass,omitempty"`
	AskBecomePass     bool                   `yaml:"ask_become_pass,omitempty"`
	ExtraArgs         []string               `yaml:"extra_args,omitempty"`
	Bootstrap         AnsibleBootstrapConfig `yaml:"bootstrap,omitempty"`
}

// AnsibleBootstrapConfig holds bootstrap access workflow settings for ansible.
type AnsibleBootstrapConfig struct {
	Enabled              bool   `yaml:"enabled,omitempty"`
	Username             string `yaml:"username,omitempty"`
	Shell                string `yaml:"shell,omitempty"`
	CreateHome           bool   `yaml:"create_home,omitempty"`
	ExcludeWindowsGuests bool   `yaml:"exclude_windows_guests,omitempty"`
	SSHPublicKeyFile     string `yaml:"ssh_public_key_file,omitempty"`
	InstallAuthorizedKey bool   `yaml:"install_authorized_key,omitempty"`
	SetPassword          bool   `yaml:"set_password,omitempty"`
	Password             string `yaml:"password,omitempty"`
	GrantSudoNOPASSWD    bool   `yaml:"grant_sudo_nopasswd,omitempty"`
	SudoersFileMode      string `yaml:"sudoers_file_mode,omitempty"`
	DryRunDefault        bool   `yaml:"dry_run_default,omitempty"`
	Parallelism          int    `yaml:"parallelism,omitempty"`
	Timeout              string `yaml:"timeout,omitempty"`
	FailFast             bool   `yaml:"fail_fast,omitempty"`
}

// DefaultKeyBindings returns a KeyBindings struct with the default key mappings.
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		SwitchView:          "]",
		SwitchViewReverse:   "[",
		NodesPage:           "Alt+1",
		GuestsPage:          "Alt+2",
		TasksPage:           "Alt+3",
		StoragePage:         "Alt+4",
		TasksToggleQueue:    "t",
		TaskStopCancel:      "x",
		Menu:                "m",
		GlobalMenu:          "Ctrl+g",
		Shell:               "s",
		VNC:                 "v",
		Refresh:             "Ctrl+r",
		AutoRefresh:         "a",
		Search:              "/",
		AdvancedGuestFilter: "Ctrl+f",
		Help:                "?",
		Quit:                "q",
	}
}

// keyBindingsToMap converts a KeyBindings struct to a map for validation.
func keyBindingsToMap(kb KeyBindings) map[string]string {
	return map[string]string{
		"switch_view":           kb.SwitchView,
		"switch_view_reverse":   kb.SwitchViewReverse,
		"nodes_page":            kb.NodesPage,
		"guests_page":           kb.GuestsPage,
		"tasks_page":            kb.TasksPage,
		"storage_page":          kb.StoragePage,
		"tasks_toggle_queue":    kb.TasksToggleQueue,
		"task_stop_cancel":      kb.TaskStopCancel,
		"menu":                  kb.Menu,
		"global_menu":           kb.GlobalMenu,
		"shell":                 kb.Shell,
		"vnc":                   kb.VNC,
		"refresh":               kb.Refresh,
		"auto_refresh":          kb.AutoRefresh,
		"search":                kb.Search,
		"advanced_guest_filter": kb.AdvancedGuestFilter,
		"help":                  kb.Help,
		"quit":                  kb.Quit,
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
		Profiles: make(map[string]ProfileConfig),
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
			Port:    parseEnvPort("PVETUI_SSH_JUMPHOST_PORT"),
		},
		Debug:       strings.ToLower(os.Getenv("PVETUI_DEBUG")) == trueString,
		CacheDir:    ExpandHomePath(os.Getenv("PVETUI_CACHE_DIR")),
		KeyBindings: DefaultKeyBindings(),
		Plugins: PluginConfig{
			Ansible: AnsiblePluginConfig{
				Bootstrap: AnsibleBootstrapConfig{
					CreateHome:           true,
					ExcludeWindowsGuests: true,
					InstallAuthorizedKey: true,
					DryRunDefault:        true,
				},
			},
		},
		ShowIcons: strings.ToLower(os.Getenv("PVETUI_SHOW_ICONS")) != "false",
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
			SwitchView          string `yaml:"switch_view"`
			SwitchViewReverse   string `yaml:"switch_view_reverse"`
			NodesPage           string `yaml:"nodes_page"`
			GuestsPage          string `yaml:"guests_page"`
			TasksPage           string `yaml:"tasks_page"`
			StoragePage         string `yaml:"storage_page"`
			TasksToggleQueue    string `yaml:"tasks_toggle_queue"`
			TaskStopCancel      string `yaml:"task_stop_cancel"`
			Menu                string `yaml:"menu"`
			GlobalMenu          string `yaml:"global_menu"`
			Shell               string `yaml:"shell"`
			VNC                 string `yaml:"vnc"`
			Scripts             string `yaml:"scripts"`
			Refresh             string `yaml:"refresh"`
			AutoRefresh         string `yaml:"auto_refresh"`
			Search              string `yaml:"search"`
			AdvancedGuestFilter string `yaml:"advanced_guest_filter"`
			Help                string `yaml:"help"`
			Quit                string `yaml:"quit"`
		} `yaml:"key_bindings"`
		Theme struct {
			Name   string            `yaml:"name"`
			Colors map[string]string `yaml:"colors"`
		} `yaml:"theme"`
		Plugins struct {
			Enabled []string `yaml:"enabled"`
			Ansible struct {
				InventoryFormat   string            `yaml:"inventory_format"`
				InventoryStyle    string            `yaml:"inventory_style"`
				InventoryVars     map[string]string `yaml:"inventory_vars"`
				DefaultUser       string            `yaml:"default_user"`
				DefaultPassword   string            `yaml:"default_password"`
				SSHPrivateKeyFile string            `yaml:"ssh_private_key_file"`
				DefaultLimitMode  string            `yaml:"default_limit_mode"`
				AskPass           *bool             `yaml:"ask_pass"`
				AskBecomePass     *bool             `yaml:"ask_become_pass"`
				ExtraArgs         []string          `yaml:"extra_args"`
				Bootstrap         struct {
					Enabled              *bool  `yaml:"enabled"`
					Username             string `yaml:"username"`
					Shell                string `yaml:"shell"`
					CreateHome           *bool  `yaml:"create_home"`
					ExcludeWindowsGuests *bool  `yaml:"exclude_windows_guests"`
					SSHPublicKeyFile     string `yaml:"ssh_public_key_file"`
					InstallAuthorizedKey *bool  `yaml:"install_authorized_key"`
					SetPassword          *bool  `yaml:"set_password"`
					Password             string `yaml:"password"`
					GrantSudoNOPASSWD    *bool  `yaml:"grant_sudo_nopasswd"`
					SudoersFileMode      string `yaml:"sudoers_file_mode"`
					DryRunDefault        *bool  `yaml:"dry_run_default"`
					Parallelism          *int   `yaml:"parallelism"`
					Timeout              string `yaml:"timeout"`
					FailFast             *bool  `yaml:"fail_fast"`
				} `yaml:"bootstrap"`
			} `yaml:"ansible"`
		} `yaml:"plugins"`
		ShowIcons     *bool                          `yaml:"show_icons"`
		GroupSettings map[string]GroupSettingsConfig `yaml:"group_settings"`
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

	var fileConfigRaw struct {
		KeyBindings map[string]any `yaml:"key_bindings"`
	}
	if err := yaml.Unmarshal(data, &fileConfigRaw); err != nil {
		return err
	}
	hasGlobalMenuKey := false
	if fileConfigRaw.KeyBindings != nil {
		_, hasGlobalMenuKey = fileConfigRaw.KeyBindings["global_menu"]
	}

	if !isSOPSEncrypted && detectCleartextSensitive(
		fileConfig.Profiles,
		fileConfig.Password,
		fileConfig.TokenSecret,
		fileConfig.Plugins.Ansible.DefaultPassword,
		fileConfig.Plugins.Ansible.Bootstrap.Password,
	) {
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
	if hasGlobalMenuKey {
		c.globalMenuConfigured = true
		c.KeyBindings.GlobalMenu = fileConfig.KeyBindings.GlobalMenu
	}

	if kb := fileConfig.KeyBindings; kb != struct {
		SwitchView          string `yaml:"switch_view"`
		SwitchViewReverse   string `yaml:"switch_view_reverse"`
		NodesPage           string `yaml:"nodes_page"`
		GuestsPage          string `yaml:"guests_page"`
		TasksPage           string `yaml:"tasks_page"`
		StoragePage         string `yaml:"storage_page"`
		TasksToggleQueue    string `yaml:"tasks_toggle_queue"`
		TaskStopCancel      string `yaml:"task_stop_cancel"`
		Menu                string `yaml:"menu"`
		GlobalMenu          string `yaml:"global_menu"`
		Shell               string `yaml:"shell"`
		VNC                 string `yaml:"vnc"`
		Scripts             string `yaml:"scripts"`
		Refresh             string `yaml:"refresh"`
		AutoRefresh         string `yaml:"auto_refresh"`
		Search              string `yaml:"search"`
		AdvancedGuestFilter string `yaml:"advanced_guest_filter"`
		Help                string `yaml:"help"`
		Quit                string `yaml:"quit"`
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
		if kb.StoragePage != "" {
			c.KeyBindings.StoragePage = kb.StoragePage
		}
		if kb.TasksToggleQueue != "" {
			c.KeyBindings.TasksToggleQueue = kb.TasksToggleQueue
		}
		if kb.TaskStopCancel != "" {
			c.KeyBindings.TaskStopCancel = kb.TaskStopCancel
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
		if kb.AdvancedGuestFilter != "" {
			c.KeyBindings.AdvancedGuestFilter = kb.AdvancedGuestFilter
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
	if fileConfig.Plugins.Ansible.InventoryFormat != "" {
		c.Plugins.Ansible.InventoryFormat = fileConfig.Plugins.Ansible.InventoryFormat
	}
	if fileConfig.Plugins.Ansible.InventoryStyle != "" {
		c.Plugins.Ansible.InventoryStyle = fileConfig.Plugins.Ansible.InventoryStyle
	}
	if fileConfig.Plugins.Ansible.InventoryVars != nil {
		c.Plugins.Ansible.InventoryVars = make(map[string]string, len(fileConfig.Plugins.Ansible.InventoryVars))
		for k, v := range fileConfig.Plugins.Ansible.InventoryVars {
			c.Plugins.Ansible.InventoryVars[k] = v
		}
	}
	if fileConfig.Plugins.Ansible.DefaultUser != "" {
		c.Plugins.Ansible.DefaultUser = fileConfig.Plugins.Ansible.DefaultUser
	}
	if fileConfig.Plugins.Ansible.DefaultPassword != "" {
		c.Plugins.Ansible.DefaultPassword = fileConfig.Plugins.Ansible.DefaultPassword
	}
	if fileConfig.Plugins.Ansible.SSHPrivateKeyFile != "" {
		c.Plugins.Ansible.SSHPrivateKeyFile = ExpandHomePath(fileConfig.Plugins.Ansible.SSHPrivateKeyFile)
	}
	if fileConfig.Plugins.Ansible.DefaultLimitMode != "" {
		c.Plugins.Ansible.DefaultLimitMode = fileConfig.Plugins.Ansible.DefaultLimitMode
	}
	if fileConfig.Plugins.Ansible.AskPass != nil {
		c.Plugins.Ansible.AskPass = *fileConfig.Plugins.Ansible.AskPass
	}
	if fileConfig.Plugins.Ansible.AskBecomePass != nil {
		c.Plugins.Ansible.AskBecomePass = *fileConfig.Plugins.Ansible.AskBecomePass
	}
	if fileConfig.Plugins.Ansible.ExtraArgs != nil {
		c.Plugins.Ansible.ExtraArgs = append([]string{}, fileConfig.Plugins.Ansible.ExtraArgs...)
	}
	if fileConfig.Plugins.Ansible.Bootstrap.Enabled != nil {
		c.Plugins.Ansible.Bootstrap.Enabled = *fileConfig.Plugins.Ansible.Bootstrap.Enabled
	}
	if fileConfig.Plugins.Ansible.Bootstrap.Username != "" {
		c.Plugins.Ansible.Bootstrap.Username = fileConfig.Plugins.Ansible.Bootstrap.Username
	}
	if fileConfig.Plugins.Ansible.Bootstrap.Shell != "" {
		c.Plugins.Ansible.Bootstrap.Shell = fileConfig.Plugins.Ansible.Bootstrap.Shell
	}
	if fileConfig.Plugins.Ansible.Bootstrap.CreateHome != nil {
		c.Plugins.Ansible.Bootstrap.CreateHome = *fileConfig.Plugins.Ansible.Bootstrap.CreateHome
	}
	if fileConfig.Plugins.Ansible.Bootstrap.ExcludeWindowsGuests != nil {
		c.Plugins.Ansible.Bootstrap.ExcludeWindowsGuests = *fileConfig.Plugins.Ansible.Bootstrap.ExcludeWindowsGuests
	}
	if fileConfig.Plugins.Ansible.Bootstrap.SSHPublicKeyFile != "" {
		c.Plugins.Ansible.Bootstrap.SSHPublicKeyFile = ExpandHomePath(fileConfig.Plugins.Ansible.Bootstrap.SSHPublicKeyFile)
	}
	if fileConfig.Plugins.Ansible.Bootstrap.InstallAuthorizedKey != nil {
		c.Plugins.Ansible.Bootstrap.InstallAuthorizedKey = *fileConfig.Plugins.Ansible.Bootstrap.InstallAuthorizedKey
	}
	if fileConfig.Plugins.Ansible.Bootstrap.SetPassword != nil {
		c.Plugins.Ansible.Bootstrap.SetPassword = *fileConfig.Plugins.Ansible.Bootstrap.SetPassword
	}
	if fileConfig.Plugins.Ansible.Bootstrap.Password != "" {
		c.Plugins.Ansible.Bootstrap.Password = fileConfig.Plugins.Ansible.Bootstrap.Password
	}
	if fileConfig.Plugins.Ansible.Bootstrap.GrantSudoNOPASSWD != nil {
		c.Plugins.Ansible.Bootstrap.GrantSudoNOPASSWD = *fileConfig.Plugins.Ansible.Bootstrap.GrantSudoNOPASSWD
	}
	if fileConfig.Plugins.Ansible.Bootstrap.SudoersFileMode != "" {
		c.Plugins.Ansible.Bootstrap.SudoersFileMode = fileConfig.Plugins.Ansible.Bootstrap.SudoersFileMode
	}
	if fileConfig.Plugins.Ansible.Bootstrap.DryRunDefault != nil {
		c.Plugins.Ansible.Bootstrap.DryRunDefault = *fileConfig.Plugins.Ansible.Bootstrap.DryRunDefault
	}
	if fileConfig.Plugins.Ansible.Bootstrap.Parallelism != nil && *fileConfig.Plugins.Ansible.Bootstrap.Parallelism > 0 {
		c.Plugins.Ansible.Bootstrap.Parallelism = *fileConfig.Plugins.Ansible.Bootstrap.Parallelism
	}
	if fileConfig.Plugins.Ansible.Bootstrap.Timeout != "" {
		c.Plugins.Ansible.Bootstrap.Timeout = fileConfig.Plugins.Ansible.Bootstrap.Timeout
	}
	if fileConfig.Plugins.Ansible.Bootstrap.FailFast != nil {
		c.Plugins.Ansible.Bootstrap.FailFast = *fileConfig.Plugins.Ansible.Bootstrap.FailFast
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

	// Merge group_settings configuration if provided
	if fileConfig.GroupSettings != nil {
		c.GroupSettings = make(map[string]GroupSettingsConfig, len(fileConfig.GroupSettings))
		for k, v := range fileConfig.GroupSettings {
			c.GroupSettings[k] = v
		}
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

func detectCleartextSensitive(
	profiles map[string]ProfileConfig,
	legacyPassword,
	legacyTokenSecret,
	ansibleDefaultPassword,
	ansibleBootstrapPassword string,
) bool {
	if hasCleartextSensitiveProfiles(profiles) {
		return true
	}
	return hasCleartextSensitiveValue(legacyPassword) ||
		hasCleartextSensitiveValue(legacyTokenSecret) ||
		hasCleartextSensitiveValue(ansibleDefaultPassword) ||
		hasCleartextSensitiveValue(ansibleBootstrapPassword)
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
	if c.KeyBindings.StoragePage == "" {
		c.KeyBindings.StoragePage = defaults.StoragePage
	}
	if c.KeyBindings.TasksToggleQueue == "" {
		c.KeyBindings.TasksToggleQueue = defaults.TasksToggleQueue
	}
	if c.KeyBindings.TaskStopCancel == "" {
		c.KeyBindings.TaskStopCancel = defaults.TaskStopCancel
	}

	if c.KeyBindings.Menu == "" {
		c.KeyBindings.Menu = defaults.Menu
	}

	if c.KeyBindings.GlobalMenu == "" && !c.globalMenuConfigured {
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
	if c.KeyBindings.AdvancedGuestFilter == "" {
		c.KeyBindings.AdvancedGuestFilter = defaults.AdvancedGuestFilter
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
	if c.Plugins.Ansible.InventoryFormat == "" {
		c.Plugins.Ansible.InventoryFormat = "yaml"
	}
	if c.Plugins.Ansible.InventoryStyle == "" {
		c.Plugins.Ansible.InventoryStyle = "compact"
	}
	if c.Plugins.Ansible.DefaultLimitMode == "" {
		c.Plugins.Ansible.DefaultLimitMode = "selection"
	}
	if c.Plugins.Ansible.SSHPrivateKeyFile != "" {
		c.Plugins.Ansible.SSHPrivateKeyFile = ExpandHomePath(c.Plugins.Ansible.SSHPrivateKeyFile)
	}
	if c.Plugins.Ansible.Bootstrap.Username == "" {
		c.Plugins.Ansible.Bootstrap.Username = "ansible"
	}
	if c.Plugins.Ansible.Bootstrap.Shell == "" {
		c.Plugins.Ansible.Bootstrap.Shell = "/bin/bash"
	}
	if c.Plugins.Ansible.Bootstrap.SudoersFileMode == "" {
		c.Plugins.Ansible.Bootstrap.SudoersFileMode = "0440"
	}
	if c.Plugins.Ansible.Bootstrap.Timeout == "" {
		c.Plugins.Ansible.Bootstrap.Timeout = "2m"
	}
	if c.Plugins.Ansible.Bootstrap.Parallelism <= 0 {
		c.Plugins.Ansible.Bootstrap.Parallelism = 10
	}
	if c.Plugins.Ansible.Bootstrap.SSHPublicKeyFile != "" {
		c.Plugins.Ansible.Bootstrap.SSHPublicKeyFile = ExpandHomePath(c.Plugins.Ansible.Bootstrap.SSHPublicKeyFile)
	}

	// ShowIcons defaults to true (icons enabled).
	// The zero value is treated as enabled unless explicitly set to false by env/config.
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

func parseEnvPort(name string) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0
	}

	return value
}
