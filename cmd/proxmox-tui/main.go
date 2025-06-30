package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/devnullvoid/proxmox-tui/internal/adapters"
	"github.com/devnullvoid/proxmox-tui/internal/cache"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/internal/ui"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// Version information (set by build flags)
var (
	version    = "dev"
	buildDate  = "unknown"
	commitHash = "unknown"
)

// Package-level logger for use throughout main and shutdown functions
var mainLogger *logger.Logger

func main() {
	// Get configuration from environment and command-line flags
	cfg := config.NewConfig()

	// Parse command-line flags
	cfg.ParseFlags()

	// Flag for config file path
	configPath := flag.String("config", "", "Path to YAML config file (default: $XDG_CONFIG_HOME/proxmox-tui/config.yml or ~/.config/proxmox-tui/config.yml)")

	// Special flags not in the config struct
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching")
	versionFlag := flag.Bool("version", false, "Show version information")

	// Parse flags
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("proxmox-tui version %s\n", version)
		fmt.Printf("Build date: %s\n", buildDate)
		fmt.Printf("Commit: %s\n", commitHash)
		os.Exit(0)
	}

	// Load config file - either specified or default XDG location
	configFileToLoad := *configPath
	if configFileToLoad == "" {
		// Try default XDG config location
		defaultConfigPath := config.GetDefaultConfigPath()
		if _, err := os.Stat(defaultConfigPath); err == nil {
			configFileToLoad = defaultConfigPath
		}
	}

	if configFileToLoad != "" {
		if err := cfg.MergeWithFile(configFileToLoad); err != nil {
			log.Fatalf("Error loading config file %s: %v", configFileToLoad, err)
		}
	}

	// Set defaults after merging config file
	cfg.SetDefaults()

	// Set debug mode from the configuration
	config.DebugEnabled = cfg.Debug

	// Create logger early for main function logging using our new system
	level := logger.LevelInfo
	if cfg.Debug {
		level = logger.LevelDebug
	}
	var err error
	mainLogger, err = logger.NewInternalLogger(level, cfg.CacheDir)
	if err != nil {
		// Fallback to simple logger if file logging fails
		mainLogger = logger.NewSimpleLogger(level)
	}
	mainLogger.Debug("Debug mode enabled")

	// Set the shared logger for UI components
	loggerAdapter := adapters.NewLoggerAdapter(cfg)
	models.SetUILogger(loggerAdapter)

	// Create cache directory if it doesn't exist
	if cfg.CacheDir != "" {
		if mkdirErr := os.MkdirAll(cfg.CacheDir, 0755); mkdirErr != nil {
			log.Fatalf("Error creating cache directory: %v", mkdirErr)
		}
	}

	// Initialize cache system
	if !*noCacheFlag {
		mainLogger.Debug("Initializing cache in %s", cfg.CacheDir)
		if cacheErr := cache.InitGlobalCache(cfg.CacheDir); cacheErr != nil {
			mainLogger.Error("Warning: Failed to initialize cache: %v", cacheErr)
			// Continue without persistent cache, we'll fall back to in-memory cache
		}

		// Verify the cache type in use
		_, isBadger := cache.GetBadgerCache()
		if isBadger {
			mainLogger.Debug("Using BadgerDB cache for persistence")
		} else {
			mainLogger.Debug("Using file-based cache for persistence")
		}
	} else {
		mainLogger.Debug("Caching disabled with --no-cache flag")
	}

	// Initialize scripts logger with the same cache directory for unified logging
	if err = logger.InitGlobalLogger(level, cfg.CacheDir); err != nil {
		mainLogger.Error("Failed to initialize global logger: %v", err)
		// Continue execution as this is not critical
	}

	// Set up a graceful shutdown to close the BadgerDB
	setupGracefulShutdown()

	// Now validate required fields
	if validationErr := cfg.Validate(); validationErr != nil {
		log.Fatal(validationErr)
	}

	// Construct full API URL
	apiURL := strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.ApiPath, "/")
	mainLogger.Debug("Creating API client for %s", apiURL)

	// Update the config with the full API URL
	cfg.Addr = apiURL

	// Use the new dependency-injected API client
	mainLogger.Debug("Using new dependency-injected API client")

	// Create adapters for our internal implementations
	configAdapter := adapters.NewConfigAdapter(cfg)
	cacheAdapter := adapters.NewCacheAdapter()

	// Create the new client with dependency injection
	client, err := api.NewClient(
		configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(cacheAdapter),
	)
	if err != nil {
		// Show user-friendly error message for connection issues
		fmt.Fprintf(os.Stderr, "\n❌ Failed to connect to Proxmox API\n\n")
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Please check:\n")
		fmt.Fprintf(os.Stderr, "• Network connectivity to Proxmox server\n")
		fmt.Fprintf(os.Stderr, "• Proxmox server address (PROXMOX_ADDR): %s\n", cfg.Addr)
		fmt.Fprintf(os.Stderr, "• Authentication credentials (PROXMOX_USER/PROXMOX_PASSWORD)\n")
		fmt.Fprintf(os.Stderr, "• TLS certificate settings (PROXMOX_INSECURE): %v\n", cfg.Insecure)
		fmt.Fprintf(os.Stderr, "• Firewall rules allowing access to port 8006\n")
		fmt.Fprintf(os.Stderr, "• Proxmox web interface is accessible\n\n")
		fmt.Fprintf(os.Stderr, "For more help, see the README.md file or check your Proxmox server logs.\n")
		os.Exit(1)
	}

	// Ensure we have a valid client before starting the UI
	if client == nil {
		fmt.Fprintf(os.Stderr, "\n❌ Failed to create Proxmox API client\n")
		os.Exit(1)
	}

	// Connection testing will be handled by the UI components during initialization
	// No need to test connection here since app.go will call FastGetClusterStatus with proper error handling

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
		// Use the main logger for shutdown logging
		if mainLogger != nil {
			mainLogger.Debug("Shutting down gracefully...")
		}
		closeBadgerDB()
		os.Exit(0)
	}()
}

// closeBadgerDB closes the BadgerDB if it's being used
func closeBadgerDB() {
	badgerCache, ok := cache.GetBadgerCache()
	if ok {
		// Use the main logger for shutdown logging
		if mainLogger != nil {
			mainLogger.Debug("Closing BadgerDB...")
		}
		if err := badgerCache.Close(); err != nil {
			if mainLogger != nil {
				mainLogger.Error("Error closing BadgerDB: %v", err)
			} else {
				log.Printf("Error closing BadgerDB: %v", err)
			}
		}
	}
}
