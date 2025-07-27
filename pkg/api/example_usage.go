package api

import (
	"context"
	"fmt"
	"log"
)

// ExampleConfig demonstrates how to implement the Config interface.
type ExampleConfig struct {
	addr        string
	user        string
	password    string
	realm       string
	tokenID     string
	tokenSecret string
	insecure    bool
}

func NewExampleConfig(addr, user, password, realm string, insecure bool) *ExampleConfig {
	return &ExampleConfig{
		addr:     addr,
		user:     user,
		password: password,
		realm:    realm,
		insecure: insecure,
	}
}

func (c *ExampleConfig) GetAddr() string        { return c.addr }
func (c *ExampleConfig) GetUser() string        { return c.user }
func (c *ExampleConfig) GetPassword() string    { return c.password }
func (c *ExampleConfig) GetRealm() string       { return c.realm }
func (c *ExampleConfig) GetTokenID() string     { return c.tokenID }
func (c *ExampleConfig) GetTokenSecret() string { return c.tokenSecret }
func (c *ExampleConfig) GetInsecure() bool      { return c.insecure }
func (c *ExampleConfig) IsUsingTokenAuth() bool { return c.tokenID != "" && c.tokenSecret != "" }
func (c *ExampleConfig) GetAPIToken() string {
	if !c.IsUsingTokenAuth() {
		return ""
	}

	return fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s", c.user, c.realm, c.tokenID, c.tokenSecret)
}

// ExampleLogger demonstrates how to implement the Logger interface.
type ExampleLogger struct{}

func (l *ExampleLogger) Debug(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}

func (l *ExampleLogger) Info(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func (l *ExampleLogger) Error(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

// ExampleUsage demonstrates how to use the reusable API package.
func ExampleUsage() {
	// Create configuration
	config := NewExampleConfig(
		"https://proxmox.example.com:8006",
		"root",
		"password",
		"pam",
		true, // insecure
	)

	// Create logger
	logger := &ExampleLogger{}

	// Create client with custom logger (cache will use NoOpCache by default)
	client, err := NewClient(config, WithLogger(logger))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Use the client
	version, err := client.Version(context.Background())
	if err != nil {
		log.Fatalf("Failed to get version: %v", err)
	}

	logger.Info("Proxmox version: %.1f", version)

	// Get VM list
	vms, err := client.GetVmList(context.Background())
	if err != nil {
		log.Fatalf("Failed to get VMs: %v", err)
	}

	logger.Info("Found %d VMs", len(vms))
}
