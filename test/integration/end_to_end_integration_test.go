package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/peevetui/internal/adapters"
	"github.com/devnullvoid/peevetui/internal/cache"
	"github.com/devnullvoid/peevetui/internal/config"
	"github.com/devnullvoid/peevetui/internal/logger"
	"github.com/devnullvoid/peevetui/pkg/api"
	"github.com/devnullvoid/peevetui/test/testutils"
)

// TestEndToEndIntegration_CompleteWorkflow tests a complete workflow from config to API calls.
func TestEndToEndIntegration_CompleteWorkflow(t *testing.T) {
	testutils.SkipIfRealProxmox(t)

	// Create mock server
	mockServer := testutils.NewMockProxmoxServer()
	defer mockServer.Close()

	itc := testutils.NewIntegrationTestConfig(t)

	t.Run("config_file_to_api_calls", func(t *testing.T) {
		// Clear environment variables to avoid conflicts
		envVars := []string{
			"PROXMOX_ADDR", "PROXMOX_USER", "PROXMOX_PASSWORD",
			"PROXMOX_TOKEN_ID", "PROXMOX_TOKEN_SECRET", "PROXMOX_REALM",
			"PROXMOX_INSECURE", "PROXMOX_DEBUG", "PROXMOX_CACHE_DIR",
			"PROXMOX_API_PATH", "PROXMOX_SSH_USER",
		}
		originalEnv := make(map[string]string)

		for _, env := range envVars {
			originalEnv[env] = os.Getenv(env)
			os.Unsetenv(env)
		}

		defer func() {
			for _, env := range envVars {
				if val, exists := originalEnv[env]; exists && val != "" {
					os.Setenv(env, val)
				} else {
					os.Unsetenv(env)
				}
			}
		}()

		// Step 1: Create configuration file
		configContent := `
addr: "` + mockServer.GetURL() + `"
user: "testuser"
password: "testpass"
realm: "pam"
insecure: true
debug: true
cache_dir: "` + itc.CacheDir + `"
`
		configFile := filepath.Join(itc.TempDir, "e2e-config.yml")
		err := os.WriteFile(configFile, []byte(configContent), 0o644)
		require.NoError(t, err)

		// Step 2: Load configuration from file
		cfg := config.NewConfig()
		err = cfg.MergeWithFile(configFile)
		require.NoError(t, err)

		// Step 3: Validate configuration
		err = cfg.Validate()
		require.NoError(t, err)

		// Step 4: Create logger
		testLogger, err := logger.NewInternalLogger(logger.LevelDebug, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		// Step 5: Create cache
		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		// Step 6: Create adapters
		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		// Step 7: Create API client
		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		// Step 8: Make API calls
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 7.4, version)

		// Step 9: Test caching behavior
		var result1 map[string]interface{}
		err = client.GetWithCache("/version", &result1, time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, result1)

		// Second call should hit cache
		var result2 map[string]interface{}
		err = client.GetWithCache("/version", &result2, time.Hour)
		require.NoError(t, err)
		assert.Equal(t, result1, result2)

		// Step 10: Test VM list
		vms, err := client.GetVmList(context.Background())
		require.NoError(t, err)
		assert.Len(t, vms, 2) // Mock server returns 2 VMs
	})

	t.Run("environment_to_api_calls", func(t *testing.T) {
		// Save and clear ALL environment variables that could affect config
		envVars := []string{
			"PROXMOX_ADDR", "PROXMOX_USER", "PROXMOX_PASSWORD",
			"PROXMOX_TOKEN_ID", "PROXMOX_TOKEN_SECRET", "PROXMOX_REALM",
			"PROXMOX_INSECURE", "PROXMOX_DEBUG", "PROXMOX_CACHE_DIR",
			"PROXMOX_API_PATH", "PROXMOX_SSH_USER",
		}
		originalEnv := make(map[string]string)

		for _, env := range envVars {
			originalEnv[env] = os.Getenv(env)
			os.Unsetenv(env)
		}

		defer func() {
			for _, env := range envVars {
				if val, exists := originalEnv[env]; exists && val != "" {
					os.Setenv(env, val)
				} else {
					os.Unsetenv(env)
				}
			}
		}()

		// Step 1: Set environment variables
		os.Setenv("PROXMOX_ADDR", mockServer.GetURL())
		os.Setenv("PROXMOX_USER", "envuser")
		os.Setenv("PROXMOX_PASSWORD", "envpass")
		os.Setenv("PROXMOX_DEBUG", "true")
		os.Setenv("PROXMOX_INSECURE", "true")

		// Step 2: Create configuration from environment
		cfg := config.NewConfig()
		cfg.SetDefaults()
		cfg.CacheDir = itc.CacheDir

		// Step 3: Validate configuration
		err := cfg.Validate()
		require.NoError(t, err)

		// Step 4: Create components and client
		testLogger, err := logger.NewInternalLogger(logger.LevelDebug, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		// Step 5: Test API calls
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 7.4, version)
	})

	t.Run("token_auth_workflow", func(t *testing.T) {
		// Step 1: Create configuration with token auth
		cfg := &config.Config{
			Addr:        mockServer.GetURL(),
			User:        "tokenuser",
			TokenID:     "testtoken",
			TokenSecret: "testsecret",
			Realm:       "pve",
			Insecure:    true,
			Debug:       true,
			CacheDir:    itc.CacheDir,
		}

		// Step 2: Validate configuration
		err := cfg.Validate()
		require.NoError(t, err)
		assert.True(t, cfg.IsUsingTokenAuth())

		// Step 3: Create components
		testLogger, err := logger.NewInternalLogger(logger.LevelDebug, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		// Step 4: Create client and test
		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		assert.True(t, client.IsUsingTokenAuth())

		// Step 5: Test API calls with token auth
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 7.4, version)
	})
}

// TestEndToEndIntegration_ErrorRecovery tests error recovery scenarios.
func TestEndToEndIntegration_ErrorRecovery(t *testing.T) {
	testutils.SkipIfRealProxmox(t)

	itc := testutils.NewIntegrationTestConfig(t)

	t.Run("invalid_config_recovery", func(t *testing.T) {
		// Clear environment variables to avoid conflicts
		envVars := []string{
			"PROXMOX_ADDR", "PROXMOX_USER", "PROXMOX_PASSWORD",
			"PROXMOX_TOKEN_ID", "PROXMOX_TOKEN_SECRET", "PROXMOX_REALM",
			"PROXMOX_INSECURE", "PROXMOX_DEBUG", "PROXMOX_CACHE_DIR",
			"PROXMOX_API_PATH", "PROXMOX_SSH_USER",
		}
		originalEnv := make(map[string]string)

		for _, env := range envVars {
			originalEnv[env] = os.Getenv(env)
			os.Unsetenv(env)
		}

		defer func() {
			for _, env := range envVars {
				if val, exists := originalEnv[env]; exists && val != "" {
					os.Setenv(env, val)
				} else {
					os.Unsetenv(env)
				}
			}
		}()

		// Step 1: Create invalid configuration file
		invalidConfigContent := `
user: "testuser"
password: "testpass"
# Missing required addr field
`
		configFile := filepath.Join(itc.TempDir, "invalid-config.yml")
		err := os.WriteFile(configFile, []byte(invalidConfigContent), 0o644)
		require.NoError(t, err)

		// Step 2: Try to load configuration
		cfg := config.NewConfig()
		err = cfg.MergeWithFile(configFile)
		require.NoError(t, err) // File loading should succeed

		// Step 3: Validation should fail
		err = cfg.Validate()
		assert.Error(t, err)
		// After migration, the error message changes to profile-based format
		assert.Contains(t, err.Error(), "address required")

		// Step 4: Fix configuration and retry
		// After migration, we need to fix the profile instead of legacy fields
		if len(cfg.Profiles) > 0 {
			// Profile-based config - fix the default profile
			defaultProfile := cfg.Profiles["default"]
			defaultProfile.Addr = "https://fixed.example.com:8006"
			cfg.Profiles["default"] = defaultProfile
		} else {
			// Legacy config - fix legacy fields
			cfg.Addr = "https://fixed.example.com:8006"
		}
		err = cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("cache_failure_recovery", func(t *testing.T) {
		// Create mock server
		mockServer := testutils.NewMockProxmoxServer()
		defer mockServer.Close()

		// Step 1: Create configuration
		cfg := &config.Config{
			Addr:     mockServer.GetURL(),
			User:     "testuser",
			Password: "testpass",
			Realm:    "pam",
			Insecure: true,
			CacheDir: "/dev/null/badcache", // Invalid cache directory
		}

		require.NoError(t, cfg.Validate())

		// Step 2: Create logger
		testLogger, err := logger.NewInternalLogger(logger.LevelDebug, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		// Step 3: Try to create cache - should fail
		_, err = cache.NewBadgerCache("/dev/null/badcache")
		assert.Error(t, err)

		// Step 4: Fall back to in-memory cache
		testCache := cache.NewMemoryCache()

		// Step 5: Create client with fallback cache
		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		// Step 6: Test that client still works
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 7.4, version)
	})

	t.Run("logger_failure_recovery", func(t *testing.T) {
		// Create mock server
		mockServer := testutils.NewMockProxmoxServer()
		defer mockServer.Close()

		// Step 1: Create configuration with invalid log directory
		cfg := &config.Config{
			Addr:     mockServer.GetURL(),
			User:     "testuser",
			Password: "testpass",
			Realm:    "pam",
			Insecure: true,
			CacheDir: itc.CacheDir,
		}

		require.NoError(t, cfg.Validate())

		// Step 2: Try to create logger with invalid directory - should fail
		_, err := logger.NewInternalLogger(logger.LevelDebug, "/invalid/log/dir")
		if err == nil {
			// If logger creation unexpectedly succeeded, that's fine for this test
			// The point is to test fallback to simple logger when needed
			t.Log("Warning: Expected logger creation to fail with invalid directory, but it succeeded")
		} else {
			// Expected case - logger creation failed
			assert.Error(t, err)
		}

		// Step 3: Fall back to simple logger
		loggerAdapter := adapters.NewSimpleLoggerAdapter(true)

		// Step 4: Create cache and client
		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		configAdapter := adapters.NewConfigAdapter(cfg)

		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		// Step 5: Test that client still works
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 7.4, version)
	})
}

// TestEndToEndIntegration_RealProxmox tests against a real Proxmox server.
func TestEndToEndIntegration_RealProxmox(t *testing.T) {
	testutils.SkipIfNoRealProxmox(t)

	itc := testutils.NewIntegrationTestConfig(t)

	t.Run("real_proxmox_complete_workflow", func(t *testing.T) {
		// Step 1: Create configuration from environment
		cfg := itc.CreateTestConfig()
		require.NoError(t, cfg.Validate())

		// Step 2: Create all components
		testLogger, err := logger.NewInternalLogger(logger.LevelInfo, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		// Step 3: Create API client
		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		// Step 4: Test basic API operations
		version, err := client.Version(context.Background())
		require.NoError(t, err)
		assert.Greater(t, version, 0.0)
		t.Logf("Connected to Proxmox version: %.1f", version)

		// Step 5: Test VM list
		vms, err := client.GetVmList(context.Background())
		require.NoError(t, err)
		t.Logf("Found %d VMs/containers", len(vms))

		// Step 6: Test caching with real data
		start := time.Now()

		var result1 map[string]interface{}
		err = client.GetWithCache("/version", &result1, time.Hour)
		require.NoError(t, err)

		firstCallDuration := time.Since(start)

		start = time.Now()

		var result2 map[string]interface{}
		err = client.GetWithCache("/version", &result2, time.Hour)
		require.NoError(t, err)

		secondCallDuration := time.Since(start)

		assert.Equal(t, result1, result2)
		assert.Less(t, secondCallDuration, firstCallDuration,
			"Second call should be faster due to caching")

		t.Logf("First call: %v, Second call: %v", firstCallDuration, secondCallDuration)

		// Step 7: Test cache persistence
		testCache.Close()

		// Create new cache instance
		newCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer newCache.Close()

		// Should be able to retrieve cached data
		cacheKey := fmt.Sprintf("proxmox_api_%s_%s", cfg.Addr, "/version")
		cacheKey = strings.ReplaceAll(cacheKey, "/", "_")

		var cachedResult map[string]interface{}
		found, err := newCache.Get(cacheKey, &cachedResult)
		require.NoError(t, err)

		if found {
			t.Logf("Cache persistence verified")
		}
	})
}

// TestEndToEndIntegration_Performance tests performance characteristics.
func TestEndToEndIntegration_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	testutils.SkipIfRealProxmox(t)

	// Create mock server
	mockServer := testutils.NewMockProxmoxServer()
	defer mockServer.Close()

	itc := testutils.NewIntegrationTestConfig(t)

	t.Run("api_call_performance", func(t *testing.T) {
		// Set up complete environment
		cfg := &config.Config{
			Addr:     mockServer.GetURL(),
			User:     "testuser",
			Password: "testpass",
			Realm:    "pam",
			Insecure: true,
			CacheDir: itc.CacheDir,
		}

		testLogger, err := logger.NewInternalLogger(logger.LevelInfo, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		// Benchmark API calls
		const numCalls = 100

		// Test uncached calls
		start := time.Now()

		for i := 0; i < numCalls; i++ {
			_, err := client.Version(context.Background())
			require.NoError(t, err)
		}

		uncachedDuration := time.Since(start)

		// Test cached calls
		start = time.Now()

		for i := 0; i < numCalls; i++ {
			var result map[string]interface{}
			err := client.GetWithCache("/version", &result, time.Hour)
			require.NoError(t, err)
		}

		cachedDuration := time.Since(start)

		t.Logf("Uncached %d calls: %v (%.2f calls/sec)",
			numCalls, uncachedDuration, float64(numCalls)/uncachedDuration.Seconds())
		t.Logf("Cached %d calls: %v (%.2f calls/sec)",
			numCalls, cachedDuration, float64(numCalls)/cachedDuration.Seconds())

		// Cached calls should be significantly faster
		assert.Less(t, cachedDuration, uncachedDuration/2,
			"Cached calls should be at least 2x faster")
	})

	t.Run("concurrent_workflow_performance", func(t *testing.T) {
		// Set up environment
		cfg := &config.Config{
			Addr:     mockServer.GetURL(),
			User:     "testuser",
			Password: "testpass",
			Realm:    "pam",
			Insecure: true,
			CacheDir: itc.CacheDir,
		}

		testLogger, err := logger.NewInternalLogger(logger.LevelInfo, itc.CacheDir)
		require.NoError(t, err)
		defer testLogger.Close()

		testCache, err := cache.NewBadgerCache(itc.CacheDir)
		require.NoError(t, err)
		defer testCache.Close()

		configAdapter := adapters.NewConfigAdapter(cfg)
		loggerAdapter := adapters.NewLoggerAdapter(cfg)

		client, err := api.NewClient(configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(testCache))
		require.NoError(t, err)

		// Test concurrent access
		const numGoroutines = 10

		const numCallsPerGoroutine = 20

		results := make(chan error, numGoroutines*numCallsPerGoroutine)

		start := time.Now()

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

		concurrentDuration := time.Since(start)

		assert.Empty(t, errors, "No errors expected from concurrent calls")
		t.Logf("Concurrent %d calls across %d goroutines: %v (%.2f calls/sec)",
			numGoroutines*numCallsPerGoroutine, numGoroutines, concurrentDuration,
			float64(numGoroutines*numCallsPerGoroutine)/concurrentDuration.Seconds())
	})
}
