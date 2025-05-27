package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/dgraph-io/badger/v4"
)

// BadgerCache implements the Cache interface using Badger DB
type BadgerCache struct {
	db *badger.DB
}

// NewBadgerCache creates a new Badger-based cache
func NewBadgerCache(dir string) (*BadgerCache, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create badger directory: %w", err)
	}

	// If there's a lock file but no process using it, remove it
	lockFilePath := dir + "/LOCK"
	if _, err := os.Stat(lockFilePath); err == nil {
		// Lock file exists, check if it's stale
		if isStale, err := isLockFileStale(lockFilePath); err != nil {
			config.DebugLog("Failed to check if lock file is stale: %v", err)
		} else if isStale {
			config.DebugLog("Removing stale lock file")
			if err := os.Remove(lockFilePath); err != nil {
				return nil, fmt.Errorf("failed to remove stale lock file: %w", err)
			}
		}
	}

	// Open the Badger database with default options
	opts := badger.DefaultOptions(dir)
	// Reduce logging noise
	opts.Logger = nil
	// Use a smaller value size to reduce memory usage
	opts.ValueLogFileSize = 1 << 20 // 1MB
	// Use memory mapping for values to improve performance
	// Note: In badger v4, these options may have been renamed or removed
	// opts.ValueLogLoadingMode = badger.MemoryMap

	db, err := badger.Open(opts)
	if err != nil {
		// Check if the error is due to resource temporarily unavailable
		if os.IsExist(err) || isErrorTemporarilyUnavailable(err) {
			return nil, fmt.Errorf("failed to open badger database (likely another process is using it): %w", err)
		}
		return nil, fmt.Errorf("failed to open badger database: %w", err)
	}

	// Run garbage collection in the background
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			err := db.RunValueLogGC(0.5) // Run GC if 50% or more space can be reclaimed
			if err != nil && err != badger.ErrNoRewrite {
				config.DebugLog("Badger value log GC failed: %v", err)
			}
		}
	}()

	return &BadgerCache{
		db: db,
	}, nil
}

// isLockFileStale checks if the lock file exists but no process is using it
func isLockFileStale(lockFilePath string) (bool, error) {
	// This is a simple implementation and might not be completely reliable
	// A more robust solution would involve parsing the lock file contents
	_, err := os.Stat(lockFilePath)
	return err == nil, nil
}

// isErrorTemporarilyUnavailable checks if an error is due to a resource being temporarily unavailable
func isErrorTemporarilyUnavailable(err error) bool {
	if err == nil {
		return false
	}

	// Check if the error is EAGAIN or EWOULDBLOCK (resource temporarily unavailable)
	if pathErr, ok := err.(*os.PathError); ok {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			return errno == syscall.EAGAIN || errno == syscall.EWOULDBLOCK
		}
	}

	// Also check the error string as a fallback
	return err.Error() == "resource temporarily unavailable"
}

// Get retrieves data from the cache
func (c *BadgerCache) Get(key string, dest interface{}) (bool, error) {
	var found bool
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			config.DebugLog("Cache miss for: %s", key)
			return nil
		}
		if err != nil {
			return fmt.Errorf("badger get operation: %w", err)
		}

		return item.Value(func(val []byte) error {
			// Parse the cache item
			var cacheItem CacheItem
			if err := json.Unmarshal(val, &cacheItem); err != nil {
				return fmt.Errorf("unmarshal cache item: %w", err)
			}

			// Check if the item is expired
			if cacheItem.TTL > 0 && time.Now().Unix()-cacheItem.Timestamp > cacheItem.TTL {
				config.DebugLog("Cache item expired: %s", key)
				// Item is expired, we'll handle deletion outside this transaction
				return nil
			}

			// Item is valid
			found = true
			config.DebugLog("Cache hit for: %s", key)

			// Unmarshal the data into the destination
			bytes, err := json.Marshal(cacheItem.Data)
			if err != nil {
				return fmt.Errorf("marshal cache data: %w", err)
			}

			if err := json.Unmarshal(bytes, dest); err != nil {
				return fmt.Errorf("unmarshal into destination: %w", err)
			}

			return nil
		})
	})

	// If the item was expired, delete it in a separate transaction
	if err == nil && !found {
		// We don't care about errors here, as it's just cleanup
		_ = c.Delete(key)
	}

	return found, err
}

// Set stores data in the cache
func (c *BadgerCache) Set(key string, data interface{}, ttl time.Duration) error {
	// Create cache item
	item := &CacheItem{
		Data:      data,
		Timestamp: time.Now().Unix(),
		TTL:       int64(ttl.Seconds()),
	}

	// Convert to JSON
	bytes, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal cache item: %w", err)
	}

	// Store in Badger
	err = c.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), bytes)
	})

	if err != nil {
		return fmt.Errorf("badger set operation: %w", err)
	}

	config.DebugLog("Cached item: %s with TTL %v", key, ttl)
	return nil
}

// Delete removes an item from the cache
func (c *BadgerCache) Delete(key string) error {
	err := c.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
	if err != nil {
		return fmt.Errorf("badger delete operation: %w", err)
	}
	config.DebugLog("Deleted cache item: %s", key)
	return nil
}

// Clear removes all items from the cache
func (c *BadgerCache) Clear() error {
	config.DebugLog("Clearing all cache items")
	return c.db.DropAll()
}

// Close closes the badger database
func (c *BadgerCache) Close() error {
	config.DebugLog("Closing Badger database")
	return c.db.Close()
}
