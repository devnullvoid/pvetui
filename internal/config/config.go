package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DebugEnabled is a global flag to enable debug logging
var DebugEnabled bool

// Config represents the application configuration
type Config struct {
	Addr        string `yaml:"addr"`
	User        string `yaml:"user"`
	Password    string `yaml:"password"`
	TokenID     string `yaml:"token_id"`
	TokenSecret string `yaml:"token_secret"`
	Realm       string `yaml:"realm"`
	ApiPath     string `yaml:"api_path"`
	Insecure    bool   `yaml:"insecure"`
	SSHUser     string `yaml:"ssh_user"`
	Debug       bool   `yaml:"debug"`
	CacheDir    string `yaml:"cache_dir"` // Directory for caching data
}

// NewConfig creates a Config with values from environment variables
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
	configFs.Parse(os.Args[1:]) // Parse just the --config flag first
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
	flag.StringVar(&c.CacheDir, "cache-dir", c.CacheDir, "Cache directory path (env PROXMOX_CACHE_DIR)")
}

func (c *Config) MergeWithFile(path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var fileConfig Config
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
	if fileConfig.ApiPath != "" {
		c.ApiPath = fileConfig.ApiPath
	}
	if fileConfig.Insecure {
		c.Insecure = true
	}
	if fileConfig.SSHUser != "" {
		c.SSHUser = fileConfig.SSHUser
	}
	if fileConfig.Debug {
		c.Debug = true
	}
	if fileConfig.CacheDir != "" {
		c.CacheDir = fileConfig.CacheDir
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
		// Default to a subdirectory in the user's home directory
		homeDir, err := os.UserHomeDir()
		if err == nil {
			// Use a stable cache directory to maintain cache between runs
			c.CacheDir = filepath.Join(homeDir, ".proxmox-tui", "cache", "badger-store")
		} else {
			// Fallback to a temporary directory if home directory isn't available
			c.CacheDir = filepath.Join(os.TempDir(), "proxmox-tui-cache", "badger-store")
		}
	}
}
