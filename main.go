package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/cache"
	"github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/devnullvoid/proxmox-tui/pkg/ui"
)

func main() {
	// Get configuration from environment and command-line flags
	cfg := config.NewConfig()

	// Parse command-line flags
	cfg.ParseFlags()

	// Flag for config file path
	configPath := flag.String("config", "", "Path to YAML config file")

	// Special flags not in the config struct
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching")

	// Parse flags
	flag.Parse()

	// Load config file first if provided
	if *configPath != "" {
		if err := cfg.MergeWithFile(*configPath); err != nil {
			log.Fatalf("Error loading config file: %v", err)
		}
		// Add debug output to check SSH user value
		log.Printf("[DEBUG] Config loaded from file: SSH user = '%s'", cfg.SSHUser)
	}

	// Set defaults after merging config file
	cfg.SetDefaults()

	// Set debug mode from the configuration
	config.DebugEnabled = cfg.Debug
	config.DebugLog("Debug mode enabled")

	// Create cache directory if it doesn't exist
	if cfg.CacheDir != "" {
		if err := os.MkdirAll(cfg.CacheDir, 0755); err != nil {
			log.Fatalf("Error creating cache directory: %v", err)
		}

		// Initialize logging to file
		if err := config.InitLogging(cfg.CacheDir); err != nil {
			// Just print a warning and continue with standard logging
			log.Printf("Warning: Could not initialize logging to file: %v", err)
		}
	}

	// Initialize cache system
	if !*noCacheFlag {
		config.DebugLog("Initializing cache in %s", cfg.CacheDir)
		if err := cache.InitGlobalCache(cfg.CacheDir); err != nil {
			log.Printf("Warning: Failed to initialize cache: %v", err)
			// Continue without persistent cache, we'll fall back to in-memory cache
		}

		// Verify the cache type in use
		_, isBadger := cache.GetBadgerCache()
		if isBadger {
			config.DebugLog("Using BadgerDB cache for persistence")
		} else {
			config.DebugLog("Using file-based cache for persistence")
		}
	} else {
		config.DebugLog("Caching disabled with --no-cache flag")
	}

	// Set up a graceful shutdown to close the BadgerDB
	setupGracefulShutdown()

	// Now validate required fields
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	// Construct full API URL
	apiURL := strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.ApiPath, "/")
	config.DebugLog("Creating API client for %s", apiURL)

	// Update the config with the full API URL
	cfg.Addr = apiURL

	client, err := api.NewClientFromConfig(cfg)
	if err != nil {
		log.Fatalf("API client error: %v", err)
	}

	// Run the application using the component-based UI architecture
	if err := ui.RunApp(client, cfg); err != nil {
		log.Fatalf("Error running app: %v", err)
	}

	// Ensure BadgerDB is properly closed on exit
	closeBadgerDB()
}

// setupGracefulShutdown sets up signal handling to ensure proper cleanup on exit
func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		config.DebugLog("Shutting down gracefully...")
		closeBadgerDB()
		os.Exit(0)
	}()
}

// closeBadgerDB closes the BadgerDB if it's being used
func closeBadgerDB() {
	badgerCache, ok := cache.GetBadgerCache()
	if ok {
		config.DebugLog("Closing BadgerDB...")
		if err := badgerCache.Close(); err != nil {
			log.Printf("Error closing BadgerDB: %v", err)
		}
	}
}
