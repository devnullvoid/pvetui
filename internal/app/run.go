package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/internal/cache"
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/internal/ui"
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// Options configures the Run function.
type Options struct {
	NoCache          bool
	InitialAggregate string
}

// RunWithStartupVerification constructs the API client, performs connectivity verification with user feedback, and starts the TUI.
func RunWithStartupVerification(cfg *config.Config, configPath string, opts Options) error {
	// Initialize logger first (but don't output startup messages in debug mode)
	level := logger.LevelInfo
	if cfg.Debug {
		level = logger.LevelDebug
	}

	mainLogger, err := logger.NewInternalLogger(level, cfg.CacheDir)
	if err != nil {
		mainLogger = logger.NewSimpleLogger(level)
	}

	loggerAdapter := adapters.NewLoggerAdapter(cfg)
	models.SetUILogger(loggerAdapter)

	// Create cache directory
	if cfg.CacheDir != "" {
		if mkdirErr := os.MkdirAll(cfg.CacheDir, 0o750); mkdirErr != nil {
			return fmt.Errorf("create cache dir: %w", mkdirErr)
		}
	}

	// Initialize cache
	if !opts.NoCache {
		if cacheErr := cache.InitGlobalCache(cfg.CacheDir); cacheErr != nil {
			mainLogger.Error("failed to initialize cache: %v", cacheErr)
		}
	}

	// Initialize global logger
	if loggerErr := logger.InitGlobalLogger(level, cfg.CacheDir); loggerErr != nil {
		mainLogger.Error("failed to init global logger: %v", loggerErr)
	}

	// Normalize the API URL
	cfg.Addr = strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.ApiPath, "/")

	// Create adapters
	configAdapter := adapters.NewConfigAdapter(cfg)
	cacheAdapter := adapters.NewCacheAdapter()

	// Initialize API client (this just sets up the client, doesn't test connectivity)
	fmt.Println("🔧 Initializing API client...")

	client, err := api.NewClient(
		configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(cacheAdapter),
	)
	if err != nil {
		// Provide more specific error messages
		if strings.Contains(err.Error(), "authentication failed") {
			return fmt.Errorf("authentication failed: %w", err)
		} else if strings.Contains(err.Error(), "missing port") {
			return fmt.Errorf("invalid address format (missing port): %w", err)
		}

		return fmt.Errorf("failed to initialize API client: %w", err)
	}

	fmt.Println("✅ API client initialized")

	// Now test actual connectivity and authentication
	fmt.Printf("🔗 Testing connection to %s...\n", strings.TrimSuffix(cfg.Addr, "/api2/json"))

	// Try a simple API call to verify connectivity and authentication
	var result map[string]interface{}
	if testErr := client.GetNoRetry("/version", &result); testErr != nil {
		if strings.Contains(testErr.Error(), "authentication failed") || strings.Contains(testErr.Error(), "Unauthorized") {
			return fmt.Errorf("authentication failed: invalid credentials")
		} else if strings.Contains(testErr.Error(), "connection") || strings.Contains(testErr.Error(), "timeout") || strings.Contains(testErr.Error(), "dial") || strings.Contains(testErr.Error(), "name resolution") {
			return fmt.Errorf("connection failed: %w", testErr)
		}

		return fmt.Errorf("API test failed: %w", testErr)
	}

	fmt.Println("✅ Connected successfully")
	fmt.Println("✅ Authentication successful")

	autoEncryptConfig(cfg, configPath)

	fmt.Println("🖥️  Loading interface...")
	if opts.InitialAggregate != "" {
		fmt.Printf("🔄 Will automatically switch to aggregate mode: %s\n", opts.InitialAggregate)
	}
	fmt.Println()

	// Start the UI
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return ui.RunApp(ctx, client, cfg, configPath, opts.InitialAggregate)
}

func autoEncryptConfig(cfg *config.Config, configPath string) {
	if configPath == "" || !cfg.HasCleartextSensitiveData() {
		return
	}

	// nolint:gosec // configPath is validated and comes from trusted sources (user input or default paths)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	if config.IsSOPSEncrypted(configPath, data) {
		cfg.MarkSensitiveDataEncrypted()
		return
	}

	cfgCopy := *cfg
	cfgCopy.Profiles = make(map[string]config.ProfileConfig)
	for k, v := range cfg.Profiles {
		cfgCopy.Profiles[k] = v
	}

	if err := config.SaveConfigFile(&cfgCopy, configPath); err != nil {
		if config.DebugEnabled {
			fmt.Printf("⚠️  Warning: Failed to encrypt and save config: %v\n", err)
		}
		return
	}

	fmt.Println("🔐 Encrypted sensitive fields in config file")
	cfg.MarkSensitiveDataEncrypted()
}
