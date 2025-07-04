// Package config provides configuration management for the Proxmox TUI application.
//
// This package handles loading configuration from multiple sources with proper
// precedence ordering:
//  1. Command-line flags (highest priority)
//  2. Environment variables
//  3. Configuration files (YAML format)
//  4. Default values (lowest priority)
//
// The package follows XDG Base Directory Specification for configuration and
// cache file locations, providing a clean and predictable user experience.
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
//   - PROXMOX_CACHE_DIR: Custom cache directory path
//
// Configuration File Format (YAML):
//
//	addr: "https://pve.example.com:8006"
//	user: "root"
//	password: "secret"
//	realm: "pam"
//	insecure: false
//	debug: true
//	cache_dir: "/custom/cache/path"
//
// XDG Directory Support:
//
// The package automatically determines appropriate directories for configuration
// and cache files based on XDG specifications:
//   - Config: $XDG_CONFIG_HOME/proxmox-tui or ~/.config/proxmox-tui
//   - Cache: $XDG_CACHE_HOME/proxmox-tui or ~/.cache/proxmox-tui
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

	"github.com/devnullvoid/proxmox-tui/internal/keys"
	"gopkg.in/yaml.v3"
)

// DebugEnabled is a global flag to enable debug logging throughout the application.
//
// This variable is set during configuration parsing and used by various
// components to determine whether to emit debug-level log messages.
var DebugEnabled bool

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
	Shell             string `yaml:"shell"`        // Open shell session
	VNC               string `yaml:"vnc"`          // Open VNC console
	Scripts           string `yaml:"scripts"`      // Install community scripts
	Refresh           string `yaml:"refresh"`      // Manual refresh
	AutoRefresh       string `yaml:"auto_refresh"` // Toggle auto-refresh
	Search            string `yaml:"search"`       // Activate search
	Help              string `yaml:"help"`         // Toggle help modal
	Quit              string `yaml:"quit"`         // Quit application
}

// Config represents the complete application configuration with support for
// multiple authentication methods and XDG-compliant directory handling.
//
// The configuration supports both password-based and API token authentication
// for Proxmox VE. All fields can be populated from environment variables,
// command-line flags, or YAML configuration files.
//
// Authentication Methods:
//   - Password: Use User + Password + Realm
//   - API Token: Use User + TokenID + TokenSecret + Realm
//
// Example configuration:
//
//	config := &Config{
//		Addr:     "https://pve.example.com:8006",
//		User:     "root",
//		Password: "secret",
//		Realm:    "pam",
//		Insecure: false,
//		Debug:    true,
//	}
type Config struct {
	Addr        string      `yaml:"addr"`         // Proxmox server URL (e.g., "https://pve.example.com:8006")
	User        string      `yaml:"user"`         // Username for authentication (without realm)
	Password    string      `yaml:"password"`     // Password for password-based authentication
	TokenID     string      `yaml:"token_id"`     // API token ID for token-based authentication
	TokenSecret string      `yaml:"token_secret"` // API token secret for token-based authentication
	Realm       string      `yaml:"realm"`        // Authentication realm (e.g., "pam", "pve")
	ApiPath     string      `yaml:"api_path"`     // API base path (default: "/api2/json")
	Insecure    bool        `yaml:"insecure"`     // Skip TLS certificate verification
	SSHUser     string      `yaml:"ssh_user"`     // SSH username for shell connections
	Debug       bool        `yaml:"debug"`        // Enable debug logging
	CacheDir    string      `yaml:"cache_dir"`    // Custom cache directory path
	KeyBindings KeyBindings `yaml:"key_bindings"` // Customizable key bindings
}

// ValidateKeyBindings checks if all key specifications are valid.
func ValidateKeyBindings(kb KeyBindings) error {
	bindings := map[string]string{
		"switch_view":         kb.SwitchView,
		"switch_view_reverse": kb.SwitchViewReverse,
		"nodes_page":          kb.NodesPage,
		"guests_page":         kb.GuestsPage,
		"tasks_page":          kb.TasksPage,
		"menu":                kb.Menu,
		"shell":               kb.Shell,
		"vnc":                 kb.VNC,
		"scripts":             kb.Scripts,
		"refresh":             kb.Refresh,
		"auto_refresh":        kb.AutoRefresh,
		"search":              kb.Search,
		"help":                kb.Help,
		"quit":                kb.Quit,
	}

	defaults := map[string]string{
		"switch_view":         "]",
		"switch_view_reverse": "[",
		"nodes_page":          "F1",
		"guests_page":         "F2",
		"tasks_page":          "F3",
		"menu":                "m",
		"shell":               "s",
		"vnc":                 "v",
		"scripts":             "c",
		"refresh":             "F5",
		"auto_refresh":        "a",
		"search":              "/",
		"help":                "?",
		"quit":                "q",
	}

	seen := make(map[string]string)

	for name, spec := range bindings {
		if spec == "" {
			continue
		}
		key, r, mod, err := keys.Parse(spec)
		if err != nil {
			return fmt.Errorf("invalid key binding %s: %w", name, err)
		}

		if keys.IsReserved(key, r, mod) && spec != defaults[name] {
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

// getXDGCacheDir returns the XDG-compliant cache directory for the application.
//
// This function follows the XDG Base Directory Specification for determining
// the appropriate cache directory:
//  1. Uses $XDG_CACHE_HOME/proxmox-tui if XDG_CACHE_HOME is set
//  2. Falls back to $HOME/.cache/proxmox-tui if HOME is available
//  3. Uses system temp directory as final fallback
//
// The returned directory may not exist yet - callers should create it as needed.
//
// Returns the absolute path to the cache directory.
func getXDGCacheDir() string {
	// Check XDG_CACHE_HOME first
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "proxmox-tui")
	}

	// Fallback to $HOME/.cache/proxmox-tui
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".cache", "proxmox-tui")
	}

	// Final fallback to temp directory
	return filepath.Join(os.TempDir(), "proxmox-tui-cache")
}

// getXDGConfigDir returns the XDG-compliant configuration directory for the application.
//
// This function follows the XDG Base Directory Specification for determining
// the appropriate configuration directory:
//  1. Uses $XDG_CONFIG_HOME/proxmox-tui if XDG_CONFIG_HOME is set
//  2. Falls back to $HOME/.config/proxmox-tui if HOME is available
//  3. Uses current directory as final fallback
//
// The returned directory may not exist yet - callers should create it as needed.
//
// Returns the absolute path to the configuration directory.
func getXDGConfigDir() string {
	// Check XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "proxmox-tui")
	}

	// Fallback to $HOME/.config/proxmox-tui
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".config", "proxmox-tui")
	}

	// Final fallback to current directory
	return "."
}

// GetDefaultConfigPath returns the default path for the application configuration file.
//
// This function combines the XDG-compliant configuration directory with the
// standard configuration filename "config.yml". The path follows XDG Base
// Directory Specification conventions.
//
// The returned path may not exist - callers should check for file existence
// before attempting to read from it.
//
// Returns the absolute path to the default configuration file.
//
// Example usage:
//
//	configPath := GetDefaultConfigPath()
//	if _, err := os.Stat(configPath); err == nil {
//		// Config file exists, load it
//		err := config.MergeWithFile(configPath)
//	}
func GetDefaultConfigPath() string {
	return filepath.Join(getXDGConfigDir(), "config.yml")
}

// NewConfig creates a new Config instance populated with values from environment variables.
//
// This function reads all supported environment variables and creates a Config
// with those values. Environment variables that are not set will result in
// zero values for the corresponding fields.
//
// Environment variables read:
//   - PROXMOX_ADDR: Server URL
//   - PROXMOX_USER: Username
//   - PROXMOX_PASSWORD: Password for password auth
//   - PROXMOX_TOKEN_ID: Token ID for token auth
//   - PROXMOX_TOKEN_SECRET: Token secret for token auth
//   - PROXMOX_REALM: Authentication realm (default: "pam")
//   - PROXMOX_API_PATH: API base path (default: "/api2/json")
//   - PROXMOX_INSECURE: Skip TLS verification ("true"/"false")
//   - PROXMOX_SSH_USER: SSH username
//   - PROXMOX_DEBUG: Enable debug logging ("true"/"false")
//   - PROXMOX_CACHE_DIR: Custom cache directory
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
	return &Config{
		Addr:        os.Getenv("PROXMOX_ADDR"),
		User:        os.Getenv("PROXMOX_USER"),
		Password:    os.Getenv("PROXMOX_PASSWORD"),
		TokenID:     os.Getenv("PROXMOX_TOKEN_ID"),
		TokenSecret: os.Getenv("PROXMOX_TOKEN_SECRET"),
		Realm:       getEnvWithDefault("PROXMOX_REALM", "pam"),
		ApiPath:     getEnvWithDefault("PROXMOX_API_PATH", "/api2/json"),
		Insecure:    strings.ToLower(os.Getenv("PROXMOX_INSECURE")) == "true",
		SSHUser:     os.Getenv("PROXMOX_SSH_USER"),
		Debug:       strings.ToLower(os.Getenv("PROXMOX_DEBUG")) == "true",
		CacheDir:    os.Getenv("PROXMOX_CACHE_DIR"),
		KeyBindings: KeyBindings{
			SwitchView:        "]",
			SwitchViewReverse: "[",
			NodesPage:         "F1",
			GuestsPage:        "F2",
			TasksPage:         "F3",
			Menu:              "M",
			Shell:             "S",
			VNC:               "V",
			Scripts:           "C",
			Refresh:           "F5",
			AutoRefresh:       "A",
			Search:            "/",
			Help:              "?",
			Quit:              "Q",
		},
	}
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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

// ParseFlags adds command-line flag definitions to a Config object
func (c *Config) ParseFlags() {
	flag.StringVar(&c.Addr, "addr", c.Addr, "Proxmox API URL (env PROXMOX_ADDR)")
	flag.StringVar(&c.User, "user", c.User, "Proxmox username (env PROXMOX_USER)")
	flag.StringVar(&c.Password, "password", c.Password, "Proxmox password (env PROXMOX_PASSWORD)")
	flag.StringVar(&c.TokenID, "token-id", c.TokenID, "Proxmox API token ID (env PROXMOX_TOKEN_ID)")
	flag.StringVar(&c.TokenSecret, "token-secret", c.TokenSecret, "Proxmox API token secret (env PROXMOX_TOKEN_SECRET)")
	flag.StringVar(&c.Realm, "realm", c.Realm, "Proxmox realm (env PROXMOX_REALM)")
	flag.BoolVar(&c.Insecure, "insecure", c.Insecure, "Skip TLS verification (env PROXMOX_INSECURE)")
	flag.StringVar(&c.ApiPath, "api-path", c.ApiPath, "Proxmox API path (env PROXMOX_API_PATH)")
	flag.StringVar(&c.SSHUser, "ssh-user", c.SSHUser, "SSH username (env PROXMOX_SSH_USER)")
	flag.BoolVar(&c.Debug, "debug", c.Debug, "Enable debug logging (env PROXMOX_DEBUG)")
	flag.StringVar(&c.CacheDir, "cache-dir", c.CacheDir, "Cache directory path (env PROXMOX_CACHE_DIR, default: $XDG_CACHE_HOME/proxmox-tui or ~/.cache/proxmox-tui)")
}

func (c *Config) MergeWithFile(path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Use a struct with pointers to distinguish between unset and explicitly set values
	var fileConfig struct {
		Addr        string `yaml:"addr"`
		User        string `yaml:"user"`
		Password    string `yaml:"password"`
		TokenID     string `yaml:"token_id"`
		TokenSecret string `yaml:"token_secret"`
		Realm       string `yaml:"realm"`
		ApiPath     string `yaml:"api_path"`
		Insecure    *bool  `yaml:"insecure"`
		SSHUser     string `yaml:"ssh_user"`
		Debug       *bool  `yaml:"debug"`
		CacheDir    string `yaml:"cache_dir"`
		KeyBindings struct {
			SwitchView        string `yaml:"switch_view"`
			SwitchViewReverse string `yaml:"switch_view_reverse"`
			NodesPage         string `yaml:"nodes_page"`
			GuestsPage        string `yaml:"guests_page"`
			TasksPage         string `yaml:"tasks_page"`
			Menu              string `yaml:"menu"`
			Shell             string `yaml:"shell"`
			VNC               string `yaml:"vnc"`
			Scripts           string `yaml:"scripts"`
			Refresh           string `yaml:"refresh"`
			AutoRefresh       string `yaml:"auto_refresh"`
			Search            string `yaml:"search"`
			Help              string `yaml:"help"`
			Quit              string `yaml:"quit"`
		} `yaml:"key_bindings"`
	}

	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return err
	}

	// Merge fields where file has values
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
	if fileConfig.Debug != nil {
		c.Debug = *fileConfig.Debug
	}
	if fileConfig.CacheDir != "" {
		c.CacheDir = fileConfig.CacheDir
	}
	// Merge key bindings if provided
	if kb := fileConfig.KeyBindings; kb != struct {
		SwitchView        string `yaml:"switch_view"`
		SwitchViewReverse string `yaml:"switch_view_reverse"`
		NodesPage         string `yaml:"nodes_page"`
		GuestsPage        string `yaml:"guests_page"`
		TasksPage         string `yaml:"tasks_page"`
		Menu              string `yaml:"menu"`
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
		if kb.Shell != "" {
			c.KeyBindings.Shell = kb.Shell
		}
		if kb.VNC != "" {
			c.KeyBindings.VNC = kb.VNC
		}
		if kb.Scripts != "" {
			c.KeyBindings.Scripts = kb.Scripts
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

	return nil
}

func (c *Config) Validate() error {
	if c.Addr == "" {
		return errors.New("proxmox address required: set via -addr flag, PROXMOX_ADDR env var, or config file")
	}
	if c.User == "" {
		return errors.New("proxmox username required: set via -user flag, PROXMOX_USER env var, or config file")
	}

	// Check that either password or token authentication is provided
	hasPassword := c.Password != ""
	hasToken := c.TokenID != "" && c.TokenSecret != ""

	if !hasPassword && !hasToken {
		return errors.New("authentication required: provide either password (-password flag, PROXMOX_PASSWORD env var) or API token (-token-id/-token-secret flags, PROXMOX_TOKEN_ID/PROXMOX_TOKEN_SECRET env vars, or config file)")
	}

	if hasPassword && hasToken {
		return errors.New("conflicting authentication methods: provide either password or API token, not both")
	}

	if err := ValidateKeyBindings(c.KeyBindings); err != nil {
		return err
	}

	return nil
}

// IsUsingTokenAuth returns true if the configuration is set up for API token authentication
func (c *Config) IsUsingTokenAuth() bool {
	return c.TokenID != "" && c.TokenSecret != ""
}

// GetAPIToken returns the full API token string in the format required by Proxmox
// Format: PVEAPIToken=USER@REALM!TOKENID=SECRET
func (c *Config) GetAPIToken() string {
	if !c.IsUsingTokenAuth() {
		return ""
	}
	return fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s", c.User, c.Realm, c.TokenID, c.TokenSecret)
}

// Getter methods for API client compatibility
func (c *Config) GetAddr() string        { return c.Addr }
func (c *Config) GetUser() string        { return c.User }
func (c *Config) GetPassword() string    { return c.Password }
func (c *Config) GetRealm() string       { return c.Realm }
func (c *Config) GetTokenID() string     { return c.TokenID }
func (c *Config) GetTokenSecret() string { return c.TokenSecret }
func (c *Config) GetInsecure() bool      { return c.Insecure }

// SetDefaults sets default values for unspecified configuration options
func (c *Config) SetDefaults() {
	if c.Realm == "" {
		c.Realm = "pam"
	}
	if c.ApiPath == "" {
		c.ApiPath = "/api2/json"
	}
	if c.CacheDir == "" {
		// Use XDG_CACHE_HOME for cache directory
		c.CacheDir = getXDGCacheDir()
	}

	// Apply default key bindings if not set
	if c.KeyBindings.SwitchView == "" {
		c.KeyBindings.SwitchView = "]"
	}
	if c.KeyBindings.SwitchViewReverse == "" {
		c.KeyBindings.SwitchViewReverse = "["
	}
	if c.KeyBindings.NodesPage == "" {
		c.KeyBindings.NodesPage = "F1"
	}
	if c.KeyBindings.GuestsPage == "" {
		c.KeyBindings.GuestsPage = "F2"
	}
	if c.KeyBindings.TasksPage == "" {
		c.KeyBindings.TasksPage = "F3"
	}
	if c.KeyBindings.Menu == "" {
		c.KeyBindings.Menu = "M"
	}
	if c.KeyBindings.Shell == "" {
		c.KeyBindings.Shell = "S"
	}
	if c.KeyBindings.VNC == "" {
		c.KeyBindings.VNC = "V"
	}
	if c.KeyBindings.Scripts == "" {
		c.KeyBindings.Scripts = "C"
	}
	if c.KeyBindings.Refresh == "" {
		c.KeyBindings.Refresh = "F5"
	}
	if c.KeyBindings.AutoRefresh == "" {
		c.KeyBindings.AutoRefresh = "A"
	}
	if c.KeyBindings.Search == "" {
		c.KeyBindings.Search = "/"
	}
	if c.KeyBindings.Help == "" {
		c.KeyBindings.Help = "?"
	}
	if c.KeyBindings.Quit == "" {
		c.KeyBindings.Quit = "Q"
	}
}
