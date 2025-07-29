package main

import (
	"os"

	"github.com/devnullvoid/proxmox-tui/internal/bootstrap"
)

func main() {
	// Parse command line flags
	opts := bootstrap.ParseFlags()

	// Bootstrap the application
	result, err := bootstrap.Bootstrap(opts)
	if err != nil {
		os.Exit(1)
	}

	// If result is nil, the application should exit (e.g., version flag)
	if result == nil {
		return
	}

	// Start the main application
	if err := bootstrap.StartApplication(result); err != nil {
		os.Exit(1)
	}
}
