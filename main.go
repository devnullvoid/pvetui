package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/lonepie/proxmox-tui/pkg/config"
	"github.com/lonepie/proxmox-tui/pkg/ui"
	"github.com/rivo/tview"
)

func main() {
	// Set defaults from environment
	defaultAddr := os.Getenv("PROXMOX_ADDR")
	defaultUser := os.Getenv("PROXMOX_USER")
	defaultPassword := os.Getenv("PROXMOX_PASSWORD")
	defaultInsecure := strings.ToLower(os.Getenv("PROXMOX_INSECURE")) == "true"
	defaultAPIPath := os.Getenv("PROXMOX_API_PATH")
	if defaultAPIPath == "" {
		defaultAPIPath = "/api2/json"
	}

	cfg := config.NewConfig()

	flag.StringVar(&cfg.Addr, "addr", defaultAddr, "Proxmox API URL (env PROXMOX_ADDR)")
	flag.StringVar(&cfg.User, "user", defaultUser, "Proxmox username (env PROXMOX_USER)")
	flag.StringVar(&cfg.Password, "password", defaultPassword, "Proxmox password (env PROXMOX_PASSWORD)")
	flag.BoolVar(&cfg.Insecure, "insecure", defaultInsecure, "Skip TLS verification (env PROXMOX_INSECURE)")
	flag.StringVar(&cfg.APIPath, "api-path", defaultAPIPath, "Proxmox API path (env PROXMOX_API_PATH)")
	flag.StringVar(&cfg.SSHUser, "ssh-user", os.Getenv("PROXMOX_SSH_USER"), "SSH username (env PROXMOX_SSH_USER)")
	configPath := flag.String("config", "", "Path to YAML config file")
	flag.Parse()

	// Load config file first if provided
	if *configPath != "" {
		if err := cfg.MergeWithFile(*configPath); err != nil {
			log.Fatalf("Error loading config file: %v", err)
		}
	}

	// Now validate required fields
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	// Construct full API URL
	apiURL := strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.APIPath, "/")
	client, err := api.NewClient(apiURL, cfg.User, cfg.Password, cfg.Insecure)
	if err != nil {
		log.Fatalf("API client error: %v", err)
	}

	app := tview.NewApplication()
	root := ui.NewAppUI(app, client, cfg)
	if err := app.SetRoot(root, true).Run(); err != nil {
		log.Fatalf("Error running app: %v", err)
	}
}
