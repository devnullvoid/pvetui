package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// BadgerCache implements the Cache interface using Badger DB.
type BadgerCache struct {
	db     *badger.DB
	stopGC chan struct{}
}

// NewBadgerCache creates a new Badger-based cache.
func NewBadgerCache(dir string) (*BadgerCache, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create badger directory: %w", err)
	}

	// If there's a lock file but no process using it, remove it
	lockFilePath := dir + "/LOCK"
	if _, err := os.Stat(lockFilePath); err == nil {
		// Lock file exists, check if it's stale
		if isStale, err := isLockFileStale(lockFilePath); err != nil {
			getCacheLogger().Debug("Failed to check if lock file is stale: %v", err)
		} else if isStale {
			getCacheLogger().Debug("Removing stale lock file")

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

	cache := &BadgerCache{
		db:     db,
		stopGC: make(chan struct{}),
	}

	// Run garbage collection in the background with proper cleanup
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := db.RunValueLogGC(0.5) // Run GC if 50% or more space can be reclaimed
				if err != nil && err != badger.ErrNoRewrite {
					getCacheLogger().Debug("Badger value log GC failed: %v", err)
				}
			case <-cache.stopGC:
				getCacheLogger().Debug("Stopping Badger GC goroutine")
				return // Exit goroutine cleanly
			}
		}
	}()

	return cache, nil
}

// isLockFileStale checks if the lock file exists but no process is using it.
// This function attempts to determine if a BadgerDB lock file is stale by
// checking if the process that created it is still running.
func isLockFileStale(lockFilePath string) (bool, error) {
	// Read the lock file to get the PID
	// #nosec G304 -- lockFilePath is constructed internally, not from user input
	data, err := os.ReadFile(lockFilePath)
	if err != nil {
		// Can't read the file, consider it not stale (safer default)
		return false, fmt.Errorf("failed to read lock file: %w", err)
	}

	// BadgerDB lock files typically contain just a PID
	// Try to parse it as an integer
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		// Invalid lock file format, might be corrupted - consider it stale
		getCacheLogger().Debug("Lock file has invalid format, considering stale")
		return true, nil
	}

	// Check if the process exists by trying to find it
	process, err := os.FindProcess(pid)
	if err != nil {
		// Process doesn't exist (on some systems FindProcess always succeeds)
		getCacheLogger().Debug("Process %d not found, lock is stale", pid)
		return true, nil
	}

	// On Unix systems, send signal 0 to check if process is alive
	// Signal 0 doesn't actually send a signal, just checks if we can
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist or we don't have permission to signal it
		getCacheLogger().Debug("Cannot signal process %d: %v, lock is stale", pid, err)
		return true, nil
	}

	// Process exists and is running, lock is NOT stale
	getCacheLogger().Debug("Process %d is running, lock is valid", pid)
	return false, nil
}

// isErrorTemporarilyUnavailable checks if an error is due to a resource being temporarily unavailable.
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

// Get retrieves data from the cache.
func (c *BadgerCache) Get(key string, dest interface{}) (bool, error) {
	var found bool

	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			getCacheLogger().Debug("Cache miss for: %s", key)

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
				getCacheLogger().Debug("Cache item expired: %s", key)
				// Item is expired, we'll handle deletion outside this transaction
				return nil
			}

			// Item is valid
			found = true

			getCacheLogger().Debug("Cache hit for: %s", key)

			// Unmarshal the raw JSON directly into the destination (no double marshaling)
			if err := json.Unmarshal(cacheItem.Data, dest); err != nil {
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

// Set stores data in the cache.
func (c *BadgerCache) Set(key string, data interface{}, ttl time.Duration) error {
	// Marshal data to JSON once (avoids double marshaling on Get)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	// Create cache item with pre-marshaled JSON
	item := &CacheItem{
		Data:      jsonData,
		Timestamp: time.Now().Unix(),
		TTL:       int64(ttl.Seconds()),
	}

	// Convert cache item to JSON
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

	getCacheLogger().Debug("Cached item: %s with TTL %v", key, ttl)

	return nil
}

// Delete removes an item from the cache.
func (c *BadgerCache) Delete(key string) error {
	err := c.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
	if err != nil {
		return fmt.Errorf("badger delete operation: %w", err)
	}

	getCacheLogger().Debug("Deleted cache item: %s", key)

	return nil
}

// Clear removes all items from the cache.
func (c *BadgerCache) Clear() error {
	getCacheLogger().Debug("Clearing all cache items")

	return c.db.DropAll()
}

// Close closes the badger database and stops the background GC goroutine.
func (c *BadgerCache) Close() error {
	getCacheLogger().Debug("Closing Badger database")

	// Signal the GC goroutine to stop
	close(c.stopGC)

	// Close the database
	return c.db.Close()
}
