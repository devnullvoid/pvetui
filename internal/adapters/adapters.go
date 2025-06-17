// Package adapters provides bridge implementations that connect internal application
// components with the external API package interfaces, enabling dependency injection and
// clean separation of concerns.
//
// This package implements the Adapter pattern to translate between the internal
// configuration, logging, and caching systems and the standardized interfaces
// expected by the API client. This design allows for:
//
//   - Clean dependency injection throughout the application
//   - Easy testing with mock implementations
//   - Loose coupling between internal and external components
//   - Consistent interface contracts across the codebase
//
// The adapters handle the translation of internal types to interface implementations
// while maintaining type safety and proper error handling.
//
// Available Adapters:
//
//   - ConfigAdapter: Wraps internal.Config to implement interfaces.Config
//   - LoggerAdapter: Wraps internal.Logger to implement interfaces.Logger
//   - CacheAdapter: Wraps internal.Cache to implement interfaces.Cache
//
// Example usage:
//
//	// Create internal components
//	config := config.NewConfig()
//	logger := logger.NewInternalLogger(logger.LevelInfo, config.CacheDir)
//	cache := cache.NewFileCache(config.CacheDir, false)
//
//	// Wrap with adapters for API client
//	configAdapter := adapters.NewConfigAdapter(config)
//	loggerAdapter := adapters.NewLoggerAdapter(logger)
//	cacheAdapter := adapters.NewCacheAdapter(cache)
//
//	// Use with API client
//	client, err := api.NewClient(configAdapter,
//		api.WithLogger(loggerAdapter),
//		api.WithCache(cacheAdapter))
//
// Thread Safety:
//
// All adapters are designed to be thread-safe and delegate thread safety
// concerns to their underlying implementations. The adapters themselves
// add no additional synchronization overhead.
package adapters

import (
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/cache"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api/interfaces"
)

// ConfigAdapter adapts our internal config to the API interface
type ConfigAdapter struct {
	*config.Config
}

// NewConfigAdapter creates a new config adapter
func NewConfigAdapter(cfg *config.Config) interfaces.Config {
	return &ConfigAdapter{Config: cfg}
}

// LoggerAdapter adapts our internal logging to the API interface
type LoggerAdapter struct {
	logger *logger.Logger
}

// NewLoggerAdapter creates a new logger adapter with the given configuration
func NewLoggerAdapter(cfg *config.Config) interfaces.Logger {
	// Determine log level based on debug setting
	level := logger.LevelInfo
	if cfg.Debug {
		level = logger.LevelDebug
	}

	// Use the new cache-aware logger system
	internalLogger, err := logger.NewInternalLogger(level, cfg.CacheDir)
	if err != nil {
		// Fallback to simple logger if file logging fails
		internalLogger = logger.NewSimpleLogger(level)
	}

	return &LoggerAdapter{
		logger: internalLogger,
	}
}

// NewSimpleLoggerAdapter creates a logger adapter with simple stdout logging
func NewSimpleLoggerAdapter(debugEnabled bool) interfaces.Logger {
	level := logger.LevelInfo
	if debugEnabled {
		level = logger.LevelDebug
	}
	return &LoggerAdapter{
		logger: logger.NewSimpleLogger(level),
	}
}

func (l *LoggerAdapter) Debug(format string, args ...interface{}) {
	l.logger.Debug(format, args...)
}

func (l *LoggerAdapter) Info(format string, args ...interface{}) {
	l.logger.Info(format, args...)
}

func (l *LoggerAdapter) Error(format string, args ...interface{}) {
	l.logger.Error(format, args...)
}

// CacheAdapter adapts our internal cache to the API interface
type CacheAdapter struct {
	cache cache.Cache
}

// NewCacheAdapter creates a new cache adapter
func NewCacheAdapter() interfaces.Cache {
	return &CacheAdapter{
		cache: cache.GetGlobalCache(),
	}
}

func (c *CacheAdapter) Get(key string, dest interface{}) (bool, error) {
	return c.cache.Get(key, dest)
}

func (c *CacheAdapter) Set(key string, value interface{}, ttl time.Duration) error {
	return c.cache.Set(key, value, ttl)
}

func (c *CacheAdapter) Delete(key string) error {
	return c.cache.Delete(key)
}

func (c *CacheAdapter) Clear() error {
	return c.cache.Clear()
}
