package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Addr     string `yaml:"addr"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	APIPath  string `yaml:"api-path"`
	Insecure bool   `yaml:"insecure"`
	SSHUser  string `yaml:"ssh-user"`
}

func NewConfig() Config {
	return Config{
		APIPath: "/api2/json",
		SSHUser: os.Getenv("PROXMOX_SSH_USER"),
	}
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

	return nil
}

func (c *Config) Validate() error {
	if c.Addr == "" {
		return errors.New("Proxmox address required: set via -addr flag, PROXMOX_ADDR env var, or config file")
	}
	if c.User == "" || c.Password == "" {
		return errors.New("Credentials required: set -user & -password flags, PROXMOX_USER/PROXMOX_PASSWORD env vars, or config file")
	}
	return nil
}
