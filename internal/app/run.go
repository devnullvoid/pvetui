package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/adapters"
	"github.com/devnullvoid/proxmox-tui/internal/cache"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/internal/ui"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// Options configures the Run function.
type Options struct {
	NoCache bool
}

// Run constructs the API client and starts the TUI using the provided config.
func Run(cfg *config.Config, opts Options) error {
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

	if cfg.CacheDir != "" {
		if err := os.MkdirAll(cfg.CacheDir, 0755); err != nil {
			return fmt.Errorf("create cache dir: %w", err)
		}
	}

	if !opts.NoCache {
		if err := cache.InitGlobalCache(cfg.CacheDir); err != nil {
			mainLogger.Error("failed to initialize cache: %v", err)
		}
	}

	if err := logger.InitGlobalLogger(level, cfg.CacheDir); err != nil {
		mainLogger.Error("failed to init global logger: %v", err)
	}

	cfg.Addr = strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.ApiPath, "/")

	configAdapter := adapters.NewConfigAdapter(cfg)
	cacheAdapter := adapters.NewCacheAdapter()

	client, err := api.NewClient(
		configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(cacheAdapter),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return ui.RunApp(ctx, client, cfg)
}
