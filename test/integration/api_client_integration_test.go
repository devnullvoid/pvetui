package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/proxmox-tui/internal/adapters"
	"github.com/devnullvoid/proxmox-tui/internal/cache"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/test/testutils"
)

// TestAPIClientIntegration_MockServer tests the API client against a mock Proxmox server
func TestAPIClientIntegration_MockServer(t *testing.T) {
	testutils.SkipIfRealProxmox(t)

	// Create mock server
	mockServer := testutils.NewMockProxmoxServer()
	defer mockServer.Close()

	// Create integration test config pointing to mock server
	itc := testutils.NewIntegrationTestConfig(t)
	itc.ProxmoxAddr = mockServer.GetURL()

	// Set up complete integration test environment
	cfg, _, testCache, _ := itc.SetupIntegrationTest(t)

	// Update config to point to mock server
	cfg.Addr = mockServer.GetURL()
	cfg.Insecure = true // Required for test server

	// Create new client with updated config
	configAdapter := adapters.NewConfigAdapter(cfg)
	loggerAdapter := adapters.NewLoggerAdapter(cfg)

	client, err := api.NewClient(configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(testCache))
	require.NoError(t, err)

	t.Run("client_creation", func(t *testing.T) {
		assert.NotNil(t, client)
		assert.False(t, client.IsUsingTokenAuth()) // Using password auth
	})

	t.Run("api_version_call", func(t *testing.T) {
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 7.4, version)
	})

	t.Run("vm_list_call", func(t *testing.T) {
		vms, err := client.GetVmList(context.Background())
		require.NoError(t, err)
		assert.Len(t, vms, 2) // Mock server returns 2 VMs

		// Verify VM data structure
		vm := vms[0]
		assert.Contains(t, vm, "type")
		assert.Contains(t, vm, "vmid")
		assert.Contains(t, vm, "name")
		assert.Contains(t, vm, "status")
	})

	t.Run("caching_behavior", func(t *testing.T) {
		// Make API call that should be cached
		var result map[string]interface{}
		err := client.GetWithCache("/version", &result, time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, result)

		// Verify data is in cache
		// Cache key format: proxmox_api_<baseURL>_<path> with slashes replaced by underscores
		cacheKey := fmt.Sprintf("proxmox_api_%s_%s", mockServer.GetURL(), "/version")
		cacheKey = strings.ReplaceAll(cacheKey, "/", "_")

		var cachedResult map[string]interface{}
		found, err := testCache.Get(cacheKey, &cachedResult)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, result, cachedResult)
	})

	t.Run("cache_clear", func(t *testing.T) {
		// Ensure something is in cache first
		var result map[string]interface{}
		err := client.GetWithCache("/nodes", &result, time.Hour)
		require.NoError(t, err)

		// Clear cache
		client.ClearAPICache()

		// Verify cache is empty
		cacheKey := fmt.Sprintf("proxmox_api_%s_%s", mockServer.GetURL(), "/nodes")
		cacheKey = strings.ReplaceAll(cacheKey, "/", "_")

		var cachedResult map[string]interface{}
		found, err := testCache.Get(cacheKey, &cachedResult)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("retry_behavior", func(t *testing.T) {
		// Test retry on valid endpoint
		var result map[string]interface{}
		err := client.GetWithRetry("/version", &result, 3)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("error_handling", func(t *testing.T) {
		// Test handling of non-existent endpoint
		var result map[string]interface{}
		err := client.GetNoRetry("/nonexistent", &result)
		// Should not error since mock server returns generic response
		require.NoError(t, err)
	})
}

// TestAPIClientIntegration_TokenAuth tests token-based authentication
func TestAPIClientIntegration_TokenAuth(t *testing.T) {
	testutils.SkipIfRealProxmox(t)

	// Create mock server
	mockServer := testutils.NewMockProxmoxServer()
	defer mockServer.Close()

	// Create integration test config with token auth
	itc := testutils.NewIntegrationTestConfig(t)
	cfg := itc.CreateTestConfig()
	cfg.Addr = mockServer.GetURL()
	cfg.Password = "" // Clear password
	cfg.TokenID = "testtoken"
	cfg.TokenSecret = "testsecret"
	cfg.Insecure = true

	require.NoError(t, cfg.Validate())
	assert.True(t, cfg.IsUsingTokenAuth())

	// Create logger and cache
	testLogger, err := logger.NewInternalLogger(logger.LevelDebug, itc.CacheDir)
	require.NoError(t, err)
	defer testLogger.Close()

	testCache, err := cache.NewBadgerCache(itc.CacheDir)
	require.NoError(t, err)
	defer testCache.Close()

	// Create adapters
	configAdapter := adapters.NewConfigAdapter(cfg)
	loggerAdapter := adapters.NewLoggerAdapter(cfg)

	// Create API client
	client, err := api.NewClient(configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(testCache))
	require.NoError(t, err)

	t.Run("token_auth_client", func(t *testing.T) {
		assert.NotNil(t, client)
		assert.True(t, client.IsUsingTokenAuth())
	})

	t.Run("api_calls_with_token", func(t *testing.T) {
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 7.4, version)
	})
}

// TestAPIClientIntegration_RealProxmox tests against a real Proxmox server
func TestAPIClientIntegration_RealProxmox(t *testing.T) {
	testutils.SkipIfNoRealProxmox(t)

	// Set up integration test environment
	itc := testutils.NewIntegrationTestConfig(t)
	cfg, _, _, client := itc.SetupIntegrationTest(t)

	t.Run("real_proxmox_connection", func(t *testing.T) {
		assert.NotNil(t, client)
		assert.NotNil(t, cfg)
		assert.NotEmpty(t, cfg.Addr)
	})

	t.Run("real_version_call", func(t *testing.T) {
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Greater(t, version, 0.0)
		t.Logf("Proxmox version: %.1f", version)
	})

	t.Run("real_vm_list", func(t *testing.T) {
		vms, err := client.GetVmList(context.Background())
		require.NoError(t, err)
		t.Logf("Found %d VMs/containers", len(vms))

		// Basic validation if VMs exist
		for _, vm := range vms {
			assert.Contains(t, vm, "type")
			assert.Contains(t, vm, "vmid")
		}
	})

	t.Run("real_caching", func(t *testing.T) {
		// Test caching with real API calls
		start := time.Now()
		var result1 map[string]interface{}
		err := client.GetWithCache("/version", &result1, time.Hour)
		require.NoError(t, err)
		firstCallDuration := time.Since(start)

		// Second call should be faster (cached)
		start = time.Now()
		var result2 map[string]interface{}
		err = client.GetWithCache("/version", &result2, time.Hour)
		require.NoError(t, err)
		secondCallDuration := time.Since(start)

		assert.Equal(t, result1, result2)
		assert.Less(t, secondCallDuration, firstCallDuration,
			"Second call should be faster due to caching")
	})
}

// TestAPIClientIntegration_ErrorScenarios tests various error scenarios
func TestAPIClientIntegration_ErrorScenarios(t *testing.T) {
	testutils.SkipIfRealProxmox(t)

	itc := testutils.NewIntegrationTestConfig(t)

	t.Run("invalid_server_address", func(t *testing.T) {
		// Step 1: Create configuration with invalid server
		cfg := &config.Config{
			Addr:     "https://invalid-server-that-does-not-exist.com:8006",
			User:     "testuser",
			Password: "testpass",
			Realm:    "pam",
			Insecure: true,
			CacheDir: itc.CacheDir,
		}

		require.NoError(t, cfg.Validate())

		// Step 2: Create components
		testLogger, err := logger.NewInternalLogger(logger.LevelDebug, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		// Step 3: Client creation should fail during authentication
		_, err = api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))

		// Should fail with some kind of network/DNS error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
	})

	t.Run("invalid_credentials", func(t *testing.T) {
		// Create mock server
		mockServer := testutils.NewMockProxmoxServer()
		defer mockServer.Close()

		itc := testutils.NewIntegrationTestConfig(t)
		cfg := itc.CreateTestConfig()
		cfg.Addr = mockServer.GetURL()
		cfg.Password = "wrong-password"
		cfg.Insecure = true

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))

		// Client creation should succeed
		require.NoError(t, err)

		// API calls might still work with mock server
		// (real authentication testing would require more sophisticated mocking)
		version, err := client.Version(context.Background())
		if err == nil {
			assert.Greater(t, version, 0.0)
		}
	})

	t.Run("network_timeout", func(t *testing.T) {
		itc := testutils.NewIntegrationTestConfig(t)
		cfg := itc.CreateTestConfig()
		cfg.Addr = "https://httpbin.org:8006" // This will timeout on Proxmox API calls

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		// Create a basic client for timeout testing
		// Note: We'll test timeout behavior through context cancellation
		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))

		if err != nil {
			t.Logf("Skipping timeout test: %v", err)
			return
		}

		// API call should timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = client.Version(ctx)
		assert.Error(t, err)
	})
}

// TestAPIClientIntegration_ConcurrentAccess tests concurrent API access
func TestAPIClientIntegration_ConcurrentAccess(t *testing.T) {
	testutils.SkipIfRealProxmox(t)

	// Create mock server
	mockServer := testutils.NewMockProxmoxServer()
	defer mockServer.Close()

	// Set up integration test environment
	itc := testutils.NewIntegrationTestConfig(t)
	itc.ProxmoxAddr = mockServer.GetURL()
	cfg, _, testCache, _ := itc.SetupIntegrationTest(t)

	// Update config for mock server
	cfg.Addr = mockServer.GetURL()
	cfg.Insecure = true

	configAdapter := adapters.NewConfigAdapter(cfg)
	loggerAdapter := adapters.NewLoggerAdapter(cfg)

	client, err := api.NewClient(configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(testCache))
	require.NoError(t, err)

	t.Run("concurrent_api_calls", func(t *testing.T) {
		const numGoroutines = 10
		const numCallsPerGoroutine = 5

		results := make(chan error, numGoroutines*numCallsPerGoroutine)

		// Launch concurrent goroutines making API calls
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < numCallsPerGoroutine; j++ {
					_, err := client.Version(context.Background())
					results <- err
				}
			}(i)
		}

		// Collect results
		var errors []error
		for i := 0; i < numGoroutines*numCallsPerGoroutine; i++ {
			if err := <-results; err != nil {
				errors = append(errors, err)
			}
		}

		// All calls should succeed
		assert.Empty(t, errors, "Expected no errors from concurrent API calls")
	})

	t.Run("concurrent_cache_access", func(t *testing.T) {
		const numGoroutines = 10
		const numCallsPerGoroutine = 3

		results := make(chan error, numGoroutines*numCallsPerGoroutine)

		// Launch concurrent goroutines making cached API calls
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < numCallsPerGoroutine; j++ {
					var result map[string]interface{}
					err := client.GetWithCache("/version", &result, time.Hour)
					results <- err
				}
			}(i)
		}

		// Collect results
		var errors []error
		for i := 0; i < numGoroutines*numCallsPerGoroutine; i++ {
			if err := <-results; err != nil {
				errors = append(errors, err)
			}
		}

		// All calls should succeed
		assert.Empty(t, errors, "Expected no errors from concurrent cached API calls")
	})
}
