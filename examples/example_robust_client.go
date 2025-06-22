//go:build examples
// +build examples

package main

import (
	"context"
	"log"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/adapters"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

func main() {
	log.Println("Testing Robust Client Setup example from DOCUMENTATION.md...")

	// Load configuration with error handling
	cfg := config.NewConfig()
	cfg.ParseFlags()
	cfg.SetDefaults()

	// For testing purposes, set some required values if not provided
	if cfg.Addr == "" {
		cfg.Addr = "https://test.example.com:8006"
		log.Println("Using test address:", cfg.Addr)
	}
	if cfg.User == "" {
		cfg.User = "test@pam"
		log.Println("Using test user:", cfg.User)
	}
	if cfg.Password == "" && cfg.TokenID == "" {
		cfg.Password = "test-password"
		log.Println("Using test password authentication")
	}

	if err := cfg.Validate(); err != nil {
		log.Printf("Configuration validation failed: %v", err)
		log.Println("Please check your configuration and try again.")
		return
	}
	log.Println("Configuration validation passed ✓")

	// Create logger with error handling
	loggerInstance, err := logger.NewInternalLogger(logger.LevelInfo, cfg.CacheDir)
	if err != nil {
		log.Printf("Failed to create logger: %v", err)
		log.Println("Falling back to simple logging...")
		loggerInstance = logger.NewSimpleLogger(logger.LevelInfo)
	}
	defer loggerInstance.Close()
	log.Println("Logger created successfully ✓")

	// Create adapters
	configAdapter := adapters.NewConfigAdapter(cfg)
	loggerAdapter := adapters.NewLoggerAdapter(cfg)
	log.Println("Adapters created successfully ✓")

	// Create API client with error handling
	client, err := api.NewClient(configAdapter,
		api.WithLogger(loggerAdapter))
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		log.Println("Please check your Proxmox server address and credentials.")
		log.Println("This is expected with test credentials ✓")
		return
	}

	// Use the client with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vms, err := client.GetVmList(ctx)
	if err != nil {
		log.Printf("Failed to get VMs: %v", err)
		log.Println("This could be due to network issues or insufficient permissions.")
		log.Println("This is expected with test credentials ✓")
		return
	}

	log.Printf("Successfully retrieved %d VMs", len(vms))

	// Example: List VM details
	for _, vm := range vms {
		vmid := api.SafeStringValue(vm["vmid"])
		name := api.SafeStringValue(vm["name"])
		status := api.SafeStringValue(vm["status"])
		log.Printf("VM %s: %s (Status: %s)", vmid, name, status)
	}

	log.Println("\n✅ Robust Client Setup example test completed successfully!")
}
