package config

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var DebugEnabled bool

func DebugLog(format string, args ...interface{}) {
	if DebugEnabled {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Config represents the application configuration
type Config struct {
	Addr     string `yaml:"addr"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Realm    string `yaml:"realm"`
	ApiPath  string `yaml:"api_path"`
	Insecure bool   `yaml:"insecure"`
	SSHUser  string `yaml:"ssh_user"`
	Debug    bool   `yaml:"debug"`
	CacheDir string `yaml:"cache_dir"` // Directory for caching data
}

// NewConfig creates a Config with values from environment variables
func NewConfig() *Config {
	return &Config{
		Addr:     os.Getenv("PROXMOX_ADDR"),
		User:     os.Getenv("PROXMOX_USER"),
		Password: os.Getenv("PROXMOX_PASSWORD"),
		Realm:    getEnvWithDefault("PROXMOX_REALM", "pam"),
		ApiPath:  getEnvWithDefault("PROXMOX_API_PATH", "/api2/json"),
		Insecure: strings.ToLower(os.Getenv("PROXMOX_INSECURE")) == "true",
		SSHUser:  os.Getenv("PROXMOX_SSH_USER"),
		Debug:    strings.ToLower(os.Getenv("PROXMOX_DEBUG")) == "true",
		CacheDir: os.Getenv("PROXMOX_CACHE_DIR"),
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
	if c.User == "" || c.Password == "" {
		return errors.New("credentials required: set -user & -password flags, PROXMOX_USER/PROXMOX_PASSWORD env vars, or config file")
	}
	return nil
}

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

// InitLogging sets up logging to redirect to a file when in TUI mode
func InitLogging(cacheDir string) error {
	// Create a logs directory in the cache directory
	// logsDir := filepath.Join(cacheDir, "logs")
	logsDir := cacheDir
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create a log file with timestamp in the name
	// logFileName := fmt.Sprintf("proxmox-tui-%s.log", time.Now().Format("2006-01-02-15-04-05"))
	logFileName := "proxmox-tui.log"
	logFilePath := filepath.Join(logsDir, logFileName)

	// Open the log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Set the log output to the file
	log.SetOutput(logFile)

	// Log that we've started logging to a file
	log.Printf("Logging started to %s", logFilePath)

	return nil
}
