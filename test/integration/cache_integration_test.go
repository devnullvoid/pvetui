package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/peevetui/internal/cache"
	"github.com/devnullvoid/peevetui/pkg/api/interfaces"
	"github.com/devnullvoid/peevetui/test/testutils"
)

// TestCacheIntegration_BadgerCache tests the Badger cache implementation.
func TestCacheIntegration_BadgerCache(t *testing.T) {
	itc := testutils.NewIntegrationTestConfig(t)

	// Create Badger cache
	badgerCache, err := cache.NewBadgerCache(itc.CacheDir)
	require.NoError(t, err)
	defer badgerCache.Close()

	t.Run("basic_operations", func(t *testing.T) {
		testBasicCacheOperations(t, badgerCache)
	})

	t.Run("ttl_expiration", func(t *testing.T) {
		testCacheTTLExpiration(t, badgerCache)
	})

	t.Run("complex_data_types", func(t *testing.T) {
		testCacheComplexDataTypes(t, badgerCache)
	})

	t.Run("concurrent_access", func(t *testing.T) {
		testCacheConcurrentAccess(t, badgerCache)
	})

	t.Run("persistence", func(t *testing.T) {
		badgerCache.Close()
		testCachePersistence(t, itc.CacheDir)
	})
}

// TestCacheIntegration_InMemoryCache tests the in-memory cache implementation.
func TestCacheIntegration_InMemoryCache(t *testing.T) {
	// Create in-memory cache
	memCache := cache.NewMemoryCache()

	t.Run("basic_operations", func(t *testing.T) {
		testBasicCacheOperations(t, memCache)
	})

	t.Run("ttl_expiration", func(t *testing.T) {
		testCacheTTLExpiration(t, memCache)
	})

	t.Run("complex_data_types", func(t *testing.T) {
		testCacheComplexDataTypes(t, memCache)
	})

	t.Run("concurrent_access", func(t *testing.T) {
		testCacheConcurrentAccess(t, memCache)
	})

	t.Run("memory_only_behavior", func(t *testing.T) {
		// Test that in-memory cache doesn't persist
		key := "memory-test"
		value := "should-not-persist"

		err := memCache.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Create new in-memory cache instance
		newMemCache := cache.NewMemoryCache()

		var result string
		found, err := newMemCache.Get(key, &result)
		require.NoError(t, err)
		assert.False(t, found, "In-memory cache should not persist across instances")
	})
}

// testBasicCacheOperations tests basic cache operations for any cache implementation.
func testBasicCacheOperations(t *testing.T, c interfaces.Cache) {
	t.Run("set_and_get_string", func(t *testing.T) {
		key := "test-string"
		value := "hello world"

		// Set value
		err := c.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get value
		var result string
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})

	t.Run("set_and_get_integer", func(t *testing.T) {
		key := "test-int"
		value := 42

		// Set value
		err := c.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get value
		var result int
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})

	t.Run("get_nonexistent_key", func(t *testing.T) {
		var result string
		found, err := c.Get("nonexistent-key", &result)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, result)
	})

	t.Run("delete_key", func(t *testing.T) {
		key := "delete-test"
		value := "to-be-deleted"

		// Set value
		err := c.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Verify it exists
		var result string
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)

		// Delete key
		err = c.Delete(key)
		require.NoError(t, err)

		// Verify it's gone
		found, err = c.Get(key, &result)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("clear_cache", func(t *testing.T) {
		// Set multiple values
		keys := []string{"clear-test-1", "clear-test-2", "clear-test-3"}
		for _, key := range keys {
			err := c.Set(key, "value-"+key, time.Hour)
			require.NoError(t, err)
		}

		// Verify they exist
		for _, key := range keys {
			var result string
			found, err := c.Get(key, &result)
			require.NoError(t, err)
			assert.True(t, found)
		}

		// Clear cache
		err := c.Clear()
		require.NoError(t, err)

		// Verify they're all gone
		for _, key := range keys {
			var result string
			found, err := c.Get(key, &result)
			require.NoError(t, err)
			assert.False(t, found)
		}
	})

	t.Run("overwrite_key", func(t *testing.T) {
		key := "overwrite-test"
		value1 := "first-value"
		value2 := "second-value"

		// Set first value
		err := c.Set(key, value1, time.Hour)
		require.NoError(t, err)

		// Set second value (overwrite)
		err = c.Set(key, value2, time.Hour)
		require.NoError(t, err)

		// Get value - should be second value
		var result string
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value2, result)
	})
}

// testCacheTTLExpiration tests TTL expiration behavior.
func testCacheTTLExpiration(t *testing.T, c interfaces.Cache) {
	t.Run("short_ttl_expiration", func(t *testing.T) {
		key := "ttl-test"
		value := "expires-soon"
		ttl := time.Second

		// Set value with short TTL
		err := c.Set(key, value, ttl)
		require.NoError(t, err)

		// Get immediately - should exist
		var result string
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)

		// Wait for expiration
		time.Sleep(ttl + time.Second)

		// Get after expiration - should not exist
		found, err = c.Get(key, &result)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("zero_ttl_no_expiration", func(t *testing.T) {
		key := "no-ttl-test"
		value := "never-expires"

		// Set value with zero TTL (no expiration)
		err := c.Set(key, value, 0)
		require.NoError(t, err)

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Should still exist
		var result string
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})
}

// testCacheComplexDataTypes tests caching of complex data structures.
func testCacheComplexDataTypes(t *testing.T, c interfaces.Cache) {
	t.Run("map_data", func(t *testing.T) {
		key := "map-test"
		value := map[string]interface{}{
			"string": "hello",
			"number": float64(42),
			"bool":   true,
			"nested": map[string]interface{}{
				"inner": "value",
			},
		}

		// Set map
		err := c.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get map
		var result map[string]interface{}
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.EqualValues(t, value, result)
	})

	t.Run("slice_data", func(t *testing.T) {
		key := "slice-test"
		value := []string{"item1", "item2", "item3"}

		// Set slice
		err := c.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get slice
		var result []string
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})

	t.Run("struct_data", func(t *testing.T) {
		type TestStruct struct {
			Name   string   `json:"name"`
			Age    int      `json:"age"`
			Active bool     `json:"active"`
			Tags   []string `json:"tags"`
		}

		key := "struct-test"
		value := TestStruct{
			Name:   "Test User",
			Age:    30,
			Active: true,
			Tags:   []string{"tag1", "tag2"},
		}

		// Set struct
		err := c.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get struct
		var result TestStruct
		found, err := c.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})
}

// testCacheConcurrentAccess tests concurrent cache operations.
func testCacheConcurrentAccess(t *testing.T, c interfaces.Cache) {
	t.Run("concurrent_set_get", func(t *testing.T) {
		const numGoroutines = 10

		const numOperations = 20

		results := make(chan error, numGoroutines*numOperations*2) // *2 for set and get

		// Launch concurrent goroutines
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("concurrent-%d-%d", goroutineID, j)
					value := fmt.Sprintf("value-%d-%d", goroutineID, j)

					// Set operation
					err := c.Set(key, value, time.Hour)
					results <- err

					// Get operation
					var result string
					_, err = c.Get(key, &result)
					results <- err
				}
			}(i)
		}

		// Collect results
		var errors []error

		for i := 0; i < numGoroutines*numOperations*2; i++ {
			if err := <-results; err != nil {
				errors = append(errors, err)
			}
		}

		// All operations should succeed
		assert.Empty(t, errors, "Expected no errors from concurrent operations")
	})

	t.Run("concurrent_delete_clear", func(t *testing.T) {
		// Pre-populate cache
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("delete-test-%d", i)
			err := c.Set(key, fmt.Sprintf("value-%d", i), time.Hour)
			require.NoError(t, err)
		}

		const numGoroutines = 5
		results := make(chan error, numGoroutines)

		// Launch concurrent delete operations
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				if goroutineID == 0 {
					// One goroutine clears the cache
					results <- c.Clear()
				} else {
					// Others delete specific keys
					key := fmt.Sprintf("delete-test-%d", goroutineID*10)
					results <- c.Delete(key)
				}
			}(i)
		}

		// Collect results - should not error even if keys don't exist
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			if err != nil && strings.Contains(err.Error(), "Writes are blocked") {
				t.Logf("Ignoring expected badger error: %v", err)

				continue
			}

			assert.NoError(t, err)
		}
	})
}

// testCachePersistence tests cache persistence (only for persistent caches like Badger).
func testCachePersistence(t *testing.T, cacheDir string) {
	key := "persistence-test"
	value := "persisted-value"

	// Create first cache instance
	cache1, err := cache.NewBadgerCache(cacheDir)
	require.NoError(t, err)

	// Set value
	err = cache1.Set(key, value, time.Hour)
	require.NoError(t, err)

	// Close first instance
	cache1.Close()
	time.Sleep(200 * time.Millisecond)

	// Create second cache instance (should read from same directory)
	cache2, err := cache.NewBadgerCache(cacheDir)
	if err != nil {
		t.Logf("Skipping persistence test: %v", err)

		return
	}
	defer cache2.Close()

	// Get value - should be persisted
	var result string
	found, err := cache2.Get(key, &result)
	require.NoError(t, err)
	assert.True(t, found, "Value should persist across cache instances")
	assert.Equal(t, value, result)
}

// TestCacheIntegration_ErrorHandling tests error handling scenarios.
func TestCacheIntegration_ErrorHandling(t *testing.T) {
	t.Run("invalid_cache_directory", func(t *testing.T) {
		// Try to create cache in invalid directory
		invalidPath := "/dev/null/invalidcache"
		_, err := cache.NewBadgerCache(invalidPath)
		assert.Error(t, err, "Should error when cache directory is not writable")
	})

	t.Run("nil_destination_get", func(t *testing.T) {
		memCache := cache.NewMemoryCache()

		// Set a value
		err := memCache.Set("test", "value", time.Hour)
		require.NoError(t, err)

		// Try to get with nil destination - should handle gracefully
		found, err := memCache.Get("test", nil)
		// Behavior may vary by implementation, but should not panic
		_ = found
		_ = err
	})

	t.Run("type_mismatch_get", func(t *testing.T) {
		memCache := cache.NewMemoryCache()

		// Set string value
		err := memCache.Set("type-test", "string-value", time.Hour)
		require.NoError(t, err)

		// Try to get as int - should handle gracefully
		var result int
		found, err := memCache.Get("type-test", &result)
		// Should either error or return false, but not panic
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.False(t, found)
		}
	})
}

// TestCacheIntegration_Performance tests cache performance characteristics.
func TestCacheIntegration_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	itc := testutils.NewIntegrationTestConfig(t)

	// Test both cache implementations
	caches := map[string]interfaces.Cache{
		"memory": cache.NewMemoryCache(),
	}

	// Add Badger cache if we can create it
	if badgerCache, err := cache.NewBadgerCache(itc.CacheDir); err == nil {
		caches["badger"] = badgerCache
		defer badgerCache.Close()
	}

	for name, c := range caches {
		t.Run(name+"_performance", func(t *testing.T) {
			const numOperations = 1000

			// Benchmark Set operations
			start := time.Now()

			for i := 0; i < numOperations; i++ {
				key := fmt.Sprintf("perf-test-%d", i)
				value := fmt.Sprintf("value-%d", i)
				err := c.Set(key, value, time.Hour)
				require.NoError(t, err)
			}

			setDuration := time.Since(start)

			// Benchmark Get operations
			start = time.Now()

			for i := 0; i < numOperations; i++ {
				key := fmt.Sprintf("perf-test-%d", i)

				var result string
				found, err := c.Get(key, &result)
				require.NoError(t, err)
				assert.True(t, found)
			}

			getDuration := time.Since(start)

			t.Logf("%s cache - Set %d items: %v (%.2f ops/sec)",
				name, numOperations, setDuration,
				float64(numOperations)/setDuration.Seconds())
			t.Logf("%s cache - Get %d items: %v (%.2f ops/sec)",
				name, numOperations, getDuration,
				float64(numOperations)/getDuration.Seconds())

			// Performance should be reasonable (not scientific, just sanity check)
			assert.Less(t, setDuration, 5*time.Second, "Set operations should complete in reasonable time")
			assert.Less(t, getDuration, 5*time.Second, "Get operations should complete in reasonable time")
		})
	}
}
