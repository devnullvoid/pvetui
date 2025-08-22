package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/peevetui/internal/cache"
	"github.com/devnullvoid/peevetui/test/testutils"
)

// Test constants for repeated strings.
const (
	testKey   = "test-string"
	testValue = "hello world"
)

// TestCacheIntegration_Simple tests basic cache functionality.
func TestCacheIntegration_Simple(t *testing.T) {
	itc := testutils.NewIntegrationTestConfig(t)

	t.Run("badger_cache_basic", func(t *testing.T) {
		// Create Badger cache
		badgerCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer badgerCache.Close()

		// Test basic string operations
		key := testKey
		value := testValue

		// Set value
		err = badgerCache.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get value
		var result string
		found, err := badgerCache.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)

		// Delete value
		err = badgerCache.Delete(key)
		require.NoError(t, err)

		// Verify it's gone
		found, err = badgerCache.Get(key, &result)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("memory_cache_basic", func(t *testing.T) {
		// Create memory cache
		memCache := cache.NewMemoryCache()

		// Test basic string operations
		key := testKey
		value := testValue

		// Set value
		err := memCache.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get value
		var result string
		found, err := memCache.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)

		// Delete value
		err = memCache.Delete(key)
		require.NoError(t, err)

		// Verify it's gone
		found, err = memCache.Get(key, &result)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("struct_data", func(t *testing.T) {
		type TestData struct {
			Name string `json:"name"`
			ID   int    `json:"id"`
		}

		memCache := cache.NewMemoryCache()

		key := "test-struct"
		value := TestData{Name: "test", ID: 123}

		// Set struct
		err := memCache.Set(key, value, time.Hour)
		require.NoError(t, err)

		// Get struct
		var result TestData
		found, err := memCache.Get(key, &result)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value.Name, result.Name)
		assert.Equal(t, value.ID, result.ID)
	})

	t.Run("cache_clear", func(t *testing.T) {
		memCache := cache.NewMemoryCache()

		// Set multiple values
		keys := []string{"key1", "key2", "key3"}
		for i, key := range keys {
			err := memCache.Set(key, "value"+string(rune(i)), time.Hour)
			require.NoError(t, err)
		}

		// Verify they exist
		for _, key := range keys {
			var result string
			found, err := memCache.Get(key, &result)
			require.NoError(t, err)
			assert.True(t, found)
		}

		// Clear cache
		err := memCache.Clear()
		require.NoError(t, err)

		// Verify they're gone
		for _, key := range keys {
			var result string
			found, err := memCache.Get(key, &result)
			require.NoError(t, err)
			assert.False(t, found)
		}
	})
}
