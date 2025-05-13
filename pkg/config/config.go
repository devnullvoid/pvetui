package config

import (
	"errors"
	"flag"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var DebugEnabled bool

func DebugLog(format string, v ...interface{}) {
	if DebugEnabled {
		log.Printf("[DEBUG] "+format, v...)
	}
}

type Config struct {
	Addr     string `yaml:"addr"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Realm    string `yaml:"realm"`
	APIPath  string `yaml:"api-path"`
	Insecure bool   `yaml:"insecure"`
	SSHUser  string `yaml:"ssh-user"`
	Debug    bool   `yaml:"debug"`
}

func NewConfig() Config {
	return Config{
		Addr:     os.Getenv("PROXMOX_ADDR"),
		User:     os.Getenv("PROXMOX_USER"),
		Password: os.Getenv("PROXMOX_PASSWORD"),
		Realm:    getEnvWithDefault("PROXMOX_REALM", "pam"),
		APIPath:  getEnvWithDefault("PROXMOX_API_PATH", "/api2/json"),
		Insecure: strings.ToLower(os.Getenv("PROXMOX_INSECURE")) == "true",
		SSHUser:  os.Getenv("PROXMOX_SSH_USER"),
		Debug:    strings.ToLower(os.Getenv("PROXMOX_DEBUG")) == "true",
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

func (c *Config) SetupFlags() {
	flag.StringVar(&c.Addr, "addr", c.Addr, "Proxmox API URL (env PROXMOX_ADDR)")
	flag.StringVar(&c.User, "user", c.User, "Proxmox username (env PROXMOX_USER)")
	flag.StringVar(&c.Password, "password", c.Password, "Proxmox password (env PROXMOX_PASSWORD)")
	flag.BoolVar(&c.Insecure, "insecure", c.Insecure, "Skip TLS verification (env PROXMOX_INSECURE)")
	flag.StringVar(&c.APIPath, "api-path", c.APIPath, "Proxmox API path (env PROXMOX_API_PATH)")
	flag.StringVar(&c.SSHUser, "ssh-user", c.SSHUser, "SSH username (env PROXMOX_SSH_USER)")
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
	if fileConfig.APIPath != "" {
		c.APIPath = fileConfig.APIPath
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
