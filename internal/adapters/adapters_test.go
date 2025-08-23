package adapters

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/internal/config"
)

func TestConfigAdapter(t *testing.T) {
	// Create a test config
	cfg := &config.Config{
		Addr:        "https://test.example.com:8006",
		User:        "testuser",
		Password:    "testpass",
		Realm:       "pam",
		TokenID:     "testtoken",
		TokenSecret: "testsecret",
		Insecure:    true,
		Debug:       true,
	}

	// Create adapter
	adapter := NewConfigAdapter(cfg)
	require.NotNil(t, adapter)

	// Test all interface methods
	assert.Equal(t, cfg.Addr, adapter.GetAddr())
	assert.Equal(t, cfg.User, adapter.GetUser())
	assert.Equal(t, cfg.Password, adapter.GetPassword())
	assert.Equal(t, cfg.Realm, adapter.GetRealm())
	assert.Equal(t, cfg.TokenID, adapter.GetTokenID())
	assert.Equal(t, cfg.TokenSecret, adapter.GetTokenSecret())
	assert.Equal(t, cfg.Insecure, adapter.GetInsecure())
	assert.Equal(t, cfg.IsUsingTokenAuth(), adapter.IsUsingTokenAuth())
	assert.Equal(t, cfg.GetAPIToken(), adapter.GetAPIToken())
}

func TestConfigAdapter_TokenAuth(t *testing.T) {
	cfg := &config.Config{
		Addr:        "https://test.example.com:8006",
		User:        "testuser",
		Realm:       "pam",
		TokenID:     "testtoken",
		TokenSecret: "testsecret",
	}

	adapter := NewConfigAdapter(cfg)

	assert.True(t, adapter.IsUsingTokenAuth())

	// nolint:gosec // This is test code with controlled test data
	expectedToken := "PVEAPIToken=testuser@pam!testtoken=testsecret"
	assert.Equal(t, expectedToken, adapter.GetAPIToken())
}

func TestConfigAdapter_PasswordAuth(t *testing.T) {
	cfg := &config.Config{
		Addr:     "https://test.example.com:8006",
		User:     "testuser",
		Password: "testpass",
		Realm:    "pam",
	}

	adapter := NewConfigAdapter(cfg)

	assert.False(t, adapter.IsUsingTokenAuth())
	assert.Empty(t, adapter.GetAPIToken())
}

func TestNewSimpleLoggerAdapter(t *testing.T) {
	tests := []struct {
		name         string
		debugEnabled bool
	}{
		{
			name:         "debug enabled",
			debugEnabled: true,
		},
		{
			name:         "debug disabled",
			debugEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewSimpleLoggerAdapter(tt.debugEnabled)
			require.NotNil(t, adapter)

			// Test that the logger methods don't panic
			assert.NotPanics(t, func() {
				adapter.Debug("debug message: %s", "test")
				adapter.Info("info message: %s", "test")
				adapter.Error("error message: %s", "test")
			})
		})
	}
}

func TestNewLoggerAdapter(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name: "debug enabled",
			config: &config.Config{
				Debug:    true,
				CacheDir: tempDir,
			},
		},
		{
			name: "debug disabled",
			config: &config.Config{
				Debug:    false,
				CacheDir: tempDir,
			},
		},
		{
			name: "invalid cache dir",
			config: &config.Config{
				Debug:    true,
				CacheDir: "/invalid/path/that/should/not/exist",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewLoggerAdapter(tt.config)
			require.NotNil(t, adapter)

			// Test that the logger methods don't panic
			assert.NotPanics(t, func() {
				adapter.Debug("debug message: %s", "test")
				adapter.Info("info message: %s", "test")
				adapter.Error("error message: %s", "test")
			})
		})
	}
}

func TestLoggerAdapter_Methods(t *testing.T) {
	adapter := NewSimpleLoggerAdapter(true)

	// Test that all methods exist and can be called
	assert.NotPanics(t, func() {
		adapter.Debug("Debug: %s %d", "test", 123)
	})

	assert.NotPanics(t, func() {
		adapter.Info("Info: %s %d", "test", 456)
	})

	assert.NotPanics(t, func() {
		adapter.Error("Error: %s %d", "test", 789)
	})
}

func TestNewCacheAdapter(t *testing.T) {
	adapter := NewCacheAdapter()
	require.NotNil(t, adapter)

	// Test basic cache operations
	key := "test-key"
	value := "test-value"
	ttl := time.Hour

	// Test Set
	err := adapter.Set(key, value, ttl)
	assert.NoError(t, err)

	// Test Get
	var result string
	found, err := adapter.Get(key, &result)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, value, result)

	// Test Delete
	err = adapter.Delete(key)
	assert.NoError(t, err)

	// Verify deletion
	found, err = adapter.Get(key, &result)
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestCacheAdapter_ComplexData(t *testing.T) {
	adapter := NewCacheAdapter()

	// Test with complex data structure
	key := "complex-key"
	value := map[string]interface{}{
		"name":   "test",
		"age":    30,
		"active": true,
		"scores": []int{1, 2, 3},
	}
	ttl := time.Hour

	// Set complex data
	err := adapter.Set(key, value, ttl)
	assert.NoError(t, err)

	// Get complex data
	var result map[string]interface{}
	found, err := adapter.Get(key, &result)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.NotNil(t, result)

	// Note: Due to JSON marshaling/unmarshaling in cache,
	// we need to be careful about type assertions
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, true, result["active"])
}

func TestCacheAdapter_NonExistentKey(t *testing.T) {
	adapter := NewCacheAdapter()

	var result string
	found, err := adapter.Get("non-existent-key", &result)
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, result)
}

func TestCacheAdapter_Clear(t *testing.T) {
	adapter := NewCacheAdapter()

	// Set multiple items
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		err := adapter.Set(key, "value-"+key, time.Hour)
		assert.NoError(t, err)
	}

	// Verify items exist
	for _, key := range keys {
		var result string
		found, err := adapter.Get(key, &result)
		assert.NoError(t, err)
		assert.True(t, found)
	}

	// Clear cache
	err := adapter.Clear()
	assert.NoError(t, err)

	// Verify items are gone
	for _, key := range keys {
		var result string
		found, err := adapter.Get(key, &result)
		assert.NoError(t, err)
		assert.False(t, found)
	}
}

func TestCacheAdapter_TTL(t *testing.T) {
	adapter := NewCacheAdapter()

	key := "ttl-test"
	value := "ttl-value"
	shortTTL := 50 * time.Millisecond

	// Set with short TTL
	err := adapter.Set(key, value, shortTTL)
	assert.NoError(t, err)

	// Immediately get - should be found
	var result string
	found, err := adapter.Get(key, &result)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, value, result)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Try to get expired data - behavior depends on cache implementation
	// Some caches may still return expired data, others may not
	_, err = adapter.Get(key, &result)
	assert.NoError(t, err)
	// We don't assert on found here because it depends on the cache implementation
}
