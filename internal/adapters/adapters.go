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
	"os"
	"path/filepath"
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

// NewLoggerAdapter creates a new logger adapter with the given configuration.
//
// This function attempts to create a file-based logger using the provided cache
// directory. If the cache directory is invalid or inaccessible, it falls back
// to a simple stdout logger to avoid creating log files in unexpected locations.
//
// Parameters:
//   - cfg: Configuration containing debug settings and cache directory
//
// Returns a logger adapter that implements the interfaces.Logger interface.
func NewLoggerAdapter(cfg *config.Config) interfaces.Logger {
	// Determine log level based on debug setting
	level := logger.LevelInfo
	if cfg.Debug {
		level = logger.LevelDebug
	}

	// Validate cache directory before attempting to use it
	if cfg.CacheDir != "" {
		// Test if we can create the directory and write to it
		if err := os.MkdirAll(cfg.CacheDir, 0o750); err == nil {
			// Test write access by creating a temporary file
			testFile := filepath.Join(cfg.CacheDir, ".write_test")
			if file, err := os.Create(testFile); err == nil {
				if err := file.Close(); err != nil {
					// Log error but continue - this is acceptable in cleanup code
				}
				if err := os.Remove(testFile); err != nil {
					// Log error but continue
				}

				// Cache directory is valid, use file-based logging
				internalLogger, err := logger.NewInternalLogger(level, cfg.CacheDir)
				if err == nil {
					return &LoggerAdapter{logger: internalLogger}
				}
			}
		}
	}

	// Fallback to simple logger if cache directory is invalid or inaccessible
	return &LoggerAdapter{
		logger: logger.NewSimpleLogger(level),
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

// GetInternalLogger returns the internal logger instance for VNC service compatibility
func (l *LoggerAdapter) GetInternalLogger() *logger.Logger {
	return l.logger
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
