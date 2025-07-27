package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api/interfaces"
)

// Cache defines the interface for the caching system.
type Cache interface {
	// Get retrieves data from the cache, returning whether it was found
	Get(key string, dest interface{}) (bool, error)

	// Set stores data in the cache with optional TTL
	Set(key string, data interface{}, ttl time.Duration) error

	// Delete removes an item from the cache
	Delete(key string) error

	// Clear removes all items from the cache
	Clear() error

	// Close closes the cache and releases any resources
	Close() error
}

// CacheItem represents an item in the cache with TTL.
type CacheItem struct {
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
	TTL       int64       `json:"ttl"` // TTL in seconds, 0 means no expiration
}

// FileCache implements a simple file-based cache.
type FileCache struct {
	dir       string
	mutex     sync.RWMutex
	inMemory  map[string]*CacheItem
	persisted bool
}

// NewFileCache creates a new file-based cache.
func NewFileCache(cacheDir string, persisted bool) (*FileCache, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &FileCache{
		dir:       cacheDir,
		inMemory:  make(map[string]*CacheItem),
		persisted: persisted,
	}

	// If persisted, load existing cache files
	if persisted {
		if err := cache.loadCacheFiles(); err != nil {
			// Non-fatal error, just log it
			getCacheLogger().Debug("Warning: Failed to load cache files: %v", err)
		}
	}

	return cache, nil
}

// loadCacheFiles loads all existing cache files into memory.
func (c *FileCache) loadCacheFiles() error {
	files, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		key := file.Name()[:len(file.Name())-5] // Remove .json extension

		// Read the file
		data, err := os.ReadFile(filepath.Join(c.dir, file.Name()))
		if err != nil {
			getCacheLogger().Debug("Warning: Failed to read cache file %s: %v", file.Name(), err)

			continue
		}

		// Parse the item
		var item CacheItem
		if err := json.Unmarshal(data, &item); err != nil {
			getCacheLogger().Debug("Warning: Failed to parse cache file %s: %v", file.Name(), err)

			continue
		}

		// Check if the item is expired
		if item.TTL > 0 && time.Now().Unix()-item.Timestamp > item.TTL {
			// Item is expired, remove the file
			if err := os.Remove(filepath.Join(c.dir, file.Name())); err != nil {
				getCacheLogger().Debug("Warning: Failed to remove expired cache file %s: %v", file.Name(), err)
			}

			continue
		}

		// Add to in-memory cache
		c.inMemory[key] = &item
	}

	return nil
}

// Get retrieves data from the cache.
func (c *FileCache) Get(key string, dest interface{}) (bool, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Check if item exists in memory
	item, exists := c.inMemory[key]
	if !exists {
		getCacheLogger().Debug("Cache miss for: %s", key)

		return false, nil
	}

	// Check if the item is expired
	if item.TTL > 0 && time.Now().Unix()-item.Timestamp > item.TTL {
		// Item is expired, remove it
		delete(c.inMemory, key)
		getCacheLogger().Debug("Cache item expired: %s", key)

		// If persisted, remove the file
		if c.persisted {
			filePath := filepath.Join(c.dir, key+".json")
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return false, fmt.Errorf("failed to remove expired cache file: %w", err)
			}
		}

		return false, nil
	}

	getCacheLogger().Debug("Cache hit for: %s", key)

	// Unmarshal the data into the destination
	bytes, err := json.Marshal(item.Data)
	if err != nil {
		return false, fmt.Errorf("failed to marshal cache data: %w", err)
	}

	if err := json.Unmarshal(bytes, dest); err != nil {
		return false, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return true, nil
}

// Set stores data in the cache.
func (c *FileCache) Set(key string, data interface{}, ttl time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Create cache item
	item := &CacheItem{
		Data:      data,
		Timestamp: time.Now().Unix(),
		TTL:       int64(ttl.Seconds()),
	}

	// Add to in-memory cache
	c.inMemory[key] = item

	// If persisted, write to file
	if c.persisted {
		// Convert to JSON
		bytes, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal cache item: %w", err)
		}

		// Write to file
		filePath := filepath.Join(c.dir, key+".json")
		if err := os.WriteFile(filePath, bytes, 0o600); err != nil {
			return fmt.Errorf("failed to write cache file: %w", err)
		}
	}

	getCacheLogger().Debug("Cached item: %s with TTL %v", key, ttl)

	return nil
}

// Delete removes an item from the cache.
func (c *FileCache) Delete(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Remove from in-memory cache
	delete(c.inMemory, key)

	// If persisted, remove the file
	if c.persisted {
		filePath := filepath.Join(c.dir, key+".json")
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove cache file: %w", err)
		}
	}

	getCacheLogger().Debug("Deleted cache item: %s", key)

	return nil
}

// Clear removes all items from the cache.
func (c *FileCache) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Clear in-memory cache
	c.inMemory = make(map[string]*CacheItem)

	// If persisted, remove all cache files
	if c.persisted {
		files, err := os.ReadDir(c.dir)
		if err != nil {
			return fmt.Errorf("failed to read cache directory: %w", err)
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}

			if err := os.Remove(filepath.Join(c.dir, file.Name())); err != nil {
				return fmt.Errorf("failed to remove cache file %s: %w", file.Name(), err)
			}
		}
	}

	return nil
}

// Close implements the Cache.Close method for FileCache
// This is a no-op for FileCache since it doesn't maintain any resources that need explicit closing.
func (c *FileCache) Close() error {
	return nil
}

// NewMemoryCache creates an in-memory only cache (no persistence).
func NewMemoryCache() *FileCache {
	return &FileCache{
		inMemory:  make(map[string]*CacheItem),
		persisted: false,
	}
}

// Global singleton cache instance.
var (
	globalCache     Cache
	cacheLogger     interfaces.Logger
	globalCacheDir  string
	once            sync.Once
	cacheLoggerOnce sync.Once
)

// getCacheLogger returns the cache logger, initializing it if necessary.
func getCacheLogger() interfaces.Logger {
	cacheLoggerOnce.Do(func() {
		// Create a logger for cache operations that logs to file
		// Use debug level if config.DebugEnabled is true
		level := logger.LevelInfo
		if config.DebugEnabled {
			level = logger.LevelDebug
		}

		var err error

		// Use the global cache directory if available, otherwise fallback to current directory
		cacheDir := globalCacheDir
		if cacheDir == "" {
			cacheDir = "."
		}

		// Always use our new internal logger system with the cache directory
		cacheLogger, err = logger.NewInternalLogger(level, cacheDir)
		if err != nil {
			// Fallback to simple logger if file logging fails
			cacheLogger = logger.NewSimpleLogger(level)
		}
	})

	return cacheLogger
}

// InitGlobalCache initializes the global cache with the given directory.
func InitGlobalCache(cacheDir string) error {
	var err error

	once.Do(func() {
		// Store the cache directory globally for logger initialization
		globalCacheDir = cacheDir

		// Create cache directory if it doesn't exist
		if err = os.MkdirAll(cacheDir, 0o750); err != nil {
			err = fmt.Errorf("failed to create cache directory: %w", err)

			return
		}

		// Create a Badger database directory
		badgerDir := filepath.Join(cacheDir, "badger")
		if err = os.MkdirAll(badgerDir, 0o750); err != nil {
			err = fmt.Errorf("failed to create badger directory: %w", err)

			return
		}

		// Check if there's an existing process using the badger directory
		lockFilePath := filepath.Join(badgerDir, "LOCK")
		lockFileExists := false

		if _, statErr := os.Stat(lockFilePath); statErr == nil {
			lockFileExists = true

			getCacheLogger().Debug("Found existing BadgerDB lock file")
		}

		// Initialize badger cache
		getCacheLogger().Debug("Attempting to initialize BadgerDB cache at %s", badgerDir)

		badgerCache, badgerErr := NewBadgerCache(badgerDir)
		if badgerErr != nil {
			// If lock file exists and we failed to initialize, it might be a lock contention
			if lockFileExists {
				getCacheLogger().Debug("Lock contention detected, waiting for lock release...")
				// Wait a short time and try again once
				time.Sleep(500 * time.Millisecond)

				badgerCache, badgerErr = NewBadgerCache(badgerDir)
			}

			// If still failed, don't fall back to file cache, use in-memory as temporary solution
			if badgerErr != nil {
				getCacheLogger().Debug("Failed to initialize BadgerDB cache: %v", badgerErr)
				getCacheLogger().Debug("Using temporary in-memory cache - no persistence will be available")

				globalCache = NewMemoryCache()
				err = badgerErr

				return
			}
		}

		getCacheLogger().Debug("Successfully initialized BadgerDB cache")

		globalCache = badgerCache

		// Verify cache is working by writing and reading a test item
		testKey := "_cache_test_" + fmt.Sprintf("%d", time.Now().UnixNano())
		testData := map[string]string{"test": "data"}

		if err = globalCache.Set(testKey, testData, 10*time.Second); err != nil {
			getCacheLogger().Debug("WARNING: Failed to write test item to cache: %v", err)
		} else {
			var result map[string]string

			found, getErr := globalCache.Get(testKey, &result)
			if getErr != nil {
				getCacheLogger().Debug("WARNING: Failed to read test item from cache: %v", getErr)
			} else if !found {
				getCacheLogger().Debug("WARNING: Test item was not found in cache immediately after writing")
			} else {
				getCacheLogger().Debug("Cache verification successful - cache is working properly")
			}
			// Clean up test item
			_ = globalCache.Delete(testKey)
		}
	})

	return err
}

// GetGlobalCache returns the global cache instance.
func GetGlobalCache() Cache {
	if globalCache == nil {
		// If global cache is not initialized, use a temporary in-memory cache
		globalCache = NewMemoryCache()
	}

	return globalCache
}

// GetBadgerCache returns the global cache as a BadgerCache if applicable.
func GetBadgerCache() (*BadgerCache, bool) {
	cache := GetGlobalCache()
	badgerCache, ok := cache.(*BadgerCache)

	return badgerCache, ok
}
