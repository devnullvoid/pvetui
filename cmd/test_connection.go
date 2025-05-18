package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

func main() {
	// Test configuration - REPLACE THESE VALUES
	config := struct {
		Address  string
		User     string
		Password string
		Realm    string
		Insecure bool
	}{
		Address:  "https://jupiter.devnullvoid.local:8006/api2/json", // Replace with your Proxmox IP/hostname
		User:     "jon",                                              // Proxmox admin user
		Password: "Ch3rryB1rb@#",                                     // Proxmox password
		Realm:    "pam",                                              // Authentication realm
		Insecure: true,                                               // Keep true for self-signed certs
	}

	fmt.Println("Starting connection test...")

	// 1. Create client
	client, err := api.NewClient(
		config.Address,
		config.User,
		config.Password,
		config.Realm,
		config.Insecure,
	)
	if err != nil {
		log.Fatalf("Client creation failed: %v", err)
	}
	fmt.Println("✅ Client created successfully")

	// 2. Debug connection parameters
	fmt.Printf("\nConnection Details:\n- Endpoint: %s\n- User: %s@%s\n- TLS Verify: %v\n\n",
		config.Address,
		config.User,
		config.Realm,
		!config.Insecure,
	)

	// 3. Test API version endpoint
	ctx := context.Background()
	version, err := client.ProxClient.Version(ctx)
	if err != nil {
		log.Fatalf("API connection failed: %v\nCheck:\n1. Network connectivity\n2. TLS certificate\n3. Firewall rules", err)
	}
	fmt.Printf("✅ Proxmox API version: %.2f\n", version)

	// 4. Test authentication with nodes endpoint
	nodes, err := client.ProxClient.GetVmList(ctx)
	if err != nil {
		log.Fatalf("Authentication failed: %v\nCheck:\n1. username@realm format\n2. Password\n3. API permissions", err)
	}
	fmt.Printf("✅ Authenticated successfully\nFound %d VMs/containers\n", len(nodes))

	fmt.Println("\nAll tests passed successfully!")
}
