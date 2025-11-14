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
	NoCache bool
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

	// Auto-encrypt and save sensitive fields if not using SOPS and config file exists
	if configPath != "" {
		// nolint:gosec // configPath is validated and comes from trusted sources (user input or default paths)
		if data, err := os.ReadFile(configPath); err == nil {
			if !config.IsSOPSEncrypted(configPath, data) {
				// Check if there are any cleartext sensitive fields that need encryption
				hasCleartext := false
				for _, profile := range cfg.Profiles {
					if (profile.Password != "" && !strings.HasPrefix(profile.Password, "age1:")) ||
						(profile.TokenSecret != "" && !strings.HasPrefix(profile.TokenSecret, "age1:")) {
						hasCleartext = true
						break
					}
				}
				if !hasCleartext {
					if (cfg.Password != "" && !strings.HasPrefix(cfg.Password, "age1:")) ||
						(cfg.TokenSecret != "" && !strings.HasPrefix(cfg.TokenSecret, "age1:")) {
						hasCleartext = true
					}
				}

				if hasCleartext {
					// Create a copy to avoid modifying the original config in memory
					cfgCopy := *cfg
					cfgCopy.Profiles = make(map[string]config.ProfileConfig)
					for k, v := range cfg.Profiles {
						cfgCopy.Profiles[k] = v
					}

					// Encrypt and save config
					if err := config.SaveConfigFile(&cfgCopy, configPath); err == nil {
						fmt.Println("🔐 Encrypted sensitive fields in config file")
						// Update the in-memory config with encrypted values
						cfg.Password = cfgCopy.Password
						cfg.TokenSecret = cfgCopy.TokenSecret
						for k, v := range cfgCopy.Profiles {
							if profile, exists := cfg.Profiles[k]; exists {
								profile.Password = v.Password
								profile.TokenSecret = v.TokenSecret
								cfg.Profiles[k] = profile
							}
						}
					} else if config.DebugEnabled {
						fmt.Printf("⚠️  Warning: Failed to encrypt and save config: %v\n", err)
					}
				}
			}
		}
	}

	fmt.Println("🖥️  Loading interface...")
	fmt.Println()

	// Start the UI
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return ui.RunApp(ctx, client, cfg, configPath)
}
