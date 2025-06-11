// Package adapters provides adapter implementations that bridge internal components
// with the external API package interfaces, enabling dependency injection and
// clean architecture patterns.
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
	internalLogger, err := logger.NewInternalLoggerWithCacheDir(level, cfg.CacheDir)
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
