package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/internal/cache"
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/display"
	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/internal/ui"
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// Options configures the Run function.
type Options struct {
	NoCache      bool
	InitialGroup string
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
	fmt.Println(display.IconText("🔧", "Initializing API client...", cfg.ShowIcons))

	var client *api.Client
	var profilesToTry []string

	if opts.InitialGroup != "" {
		profilesToTry = cfg.GetProfileNamesInGroup(opts.InitialGroup)
	} else {
		// Just try the current config (which is already set up)
		profilesToTry = []string{""}
	}

	var connected bool
	var lastErr error

	for _, profileName := range profilesToTry {
		if profileName != "" {
			if err := cfg.ApplyProfile(profileName); err != nil {
				mainLogger.Error("Failed to apply profile %s: %v", profileName, err)
				continue
			}
			// Normalize the API URL again as ApplyProfile might have reset it
			cfg.Addr = strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.ApiPath, "/")
		}

		client, err = api.NewClient(
			configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(cacheAdapter),
		)
		if err != nil {
			lastErr = err
			// Provide more specific error messages
			if strings.Contains(err.Error(), "authentication failed") {
				mainLogger.Error("Authentication failed for %s: %v", profileName, err)
			} else if strings.Contains(err.Error(), "missing port") {
				mainLogger.Error("Invalid address format for %s: %v", profileName, err)
			} else {
				mainLogger.Error("Failed to initialize API client for %s: %v", profileName, err)
			}
			continue
		}

		if profileName != "" {
			fmt.Println(display.IconText("🔗", fmt.Sprintf("Testing connection to %s (%s)...", profileName, strings.TrimSuffix(cfg.Addr, "/api2/json")), cfg.ShowIcons))
		} else {
			fmt.Println(display.IconText("🔗", fmt.Sprintf("Testing connection to %s...", strings.TrimSuffix(cfg.Addr, "/api2/json")), cfg.ShowIcons))
		}

		// Try a simple API call to verify connectivity and authentication
		var result map[string]interface{}
		if testErr := client.GetNoRetry("/version", &result); testErr != nil {
			lastErr = testErr
			if strings.Contains(testErr.Error(), "authentication failed") || strings.Contains(testErr.Error(), "Unauthorized") {
				mainLogger.Error("Authentication failed for %s: invalid credentials", profileName)
			} else if strings.Contains(testErr.Error(), "connection") || strings.Contains(testErr.Error(), "timeout") || strings.Contains(testErr.Error(), "dial") || strings.Contains(testErr.Error(), "name resolution") {
				mainLogger.Error("Connection failed for %s: %v", profileName, testErr)
			} else {
				mainLogger.Error("API test failed for %s: %v", profileName, testErr)
			}
			continue
		}

		connected = true
		fmt.Println(display.IconText("✅", "API client initialized", cfg.ShowIcons))
		break
	}

	if !connected {
		if lastErr != nil {
			if strings.Contains(lastErr.Error(), "authentication failed") {
				return fmt.Errorf("authentication failed: invalid credentials")
			}
			return fmt.Errorf("connection failed: %w", lastErr)
		}
		return fmt.Errorf("failed to connect to any profile")
	}

	fmt.Println(display.IconText("✅", "Connected successfully", cfg.ShowIcons))
	fmt.Println(display.IconText("✅", "Authentication successful", cfg.ShowIcons))

	autoEncryptConfig(cfg, configPath)

	fmt.Println(display.IconText("🖥️", "Loading interface...", cfg.ShowIcons))
	if opts.InitialGroup != "" {
		fmt.Println(display.IconText("🔄", fmt.Sprintf("Will automatically switch to group mode: %s", opts.InitialGroup), cfg.ShowIcons))
	}
	fmt.Println()

	// Start the UI
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return ui.RunApp(ctx, client, cfg, configPath, opts.InitialGroup)
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
			fmt.Println(display.IconText("⚠️", fmt.Sprintf("Warning: Failed to encrypt and save config: %v", err), cfg.ShowIcons))
		}
		return
	}

	fmt.Println(display.IconText("🔐", "Encrypted sensitive fields in config file", cfg.ShowIcons))
	cfg.MarkSensitiveDataEncrypted()
}
