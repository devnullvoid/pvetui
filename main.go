package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"gopkg.in/yaml.v3"
	"github.com/rivo/tview"
	"github.com/lonepie/proxmox-util/pkg/api"
	"github.com/lonepie/proxmox-util/pkg/ui"
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

	addr := flag.String("addr", defaultAddr, "Proxmox API URL (env PROXMOX_ADDR)")
	user := flag.String("user", defaultUser, "Proxmox username (env PROXMOX_USER)")
	password := flag.String("password", defaultPassword, "Proxmox password (env PROXMOX_PASSWORD)")
	insecure := flag.Bool("insecure", defaultInsecure, "Skip TLS verification (env PROXMOX_INSECURE)")
	apiPath := flag.String("api-path", defaultAPIPath, "Proxmox API path (env PROXMOX_API_PATH)")
	configPath := flag.String("config", "", "Path to YAML config file")
	flag.Parse()

	// Load config file first if provided
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("Error reading config file: %v", err)
		}
		var cfg struct {
			Addr     string `yaml:"addr"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			APIPath  string `yaml:"api_path"`
			Insecure bool   `yaml:"insecure"`
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Fatalf("Error parsing config file: %v", err)
		}
		if cfg.Addr != "" {
			*addr = cfg.Addr
		}
		if cfg.User != "" {
			*user = cfg.User
		}
		if cfg.Password != "" {
			*password = cfg.Password
		}
		if cfg.APIPath != "" {
			*apiPath = cfg.APIPath
		}
		if cfg.Insecure {
			*insecure = true
		}
	}

	// Now validate required fields
	if *addr == "" {
		log.Fatal("Proxmox address required: set via -addr flag, PROXMOX_ADDR env var, or config file")
	}
	if *user == "" || *password == "" {
		log.Fatal("Credentials required: set -user & -password flags, PROXMOX_USER/PROXMOX_PASSWORD env vars, or config file")
	}

	// Construct full API URL
	apiURL := strings.TrimRight(*addr, "/") + "/" + strings.TrimPrefix(*apiPath, "/")
	client, err := api.NewClient(apiURL, *user, *password, *insecure)
	if err != nil {
		log.Fatalf("API client error: %v", err)
	}

	app := tview.NewApplication()
	root := ui.NewAppUI(app, client)
	if err := app.SetRoot(root, true).Run(); err != nil {
		log.Fatalf("Error running app: %v", err)
	}
}
