// Package testutils provides utilities and helpers for integration testing.
//
// This package contains helper functions, mock servers, and utilities
// that are commonly used across integration tests. It follows the same
// patterns as the main testutils package but focuses on integration
// testing scenarios.
package testutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/peevetui/internal/adapters"
	"github.com/devnullvoid/peevetui/internal/cache"
	"github.com/devnullvoid/peevetui/internal/config"
	"github.com/devnullvoid/peevetui/internal/logger"
	"github.com/devnullvoid/peevetui/pkg/api"
	"github.com/devnullvoid/peevetui/pkg/api/interfaces"
)

// IntegrationTestConfig holds configuration for integration tests.
type IntegrationTestConfig struct {
	TempDir        string
	ConfigFile     string
	CacheDir       string
	LogFile        string
	UseRealProxmox bool
	ProxmoxAddr    string
	ProxmoxUser    string
	ProxmoxPass    string
}

// NewIntegrationTestConfig creates a new integration test configuration.
func NewIntegrationTestConfig(t *testing.T) *IntegrationTestConfig {
	tempDir := t.TempDir()

	return &IntegrationTestConfig{
		TempDir:        tempDir,
		ConfigFile:     filepath.Join(tempDir, "test-config.yml"),
		CacheDir:       filepath.Join(tempDir, "cache"),
		LogFile:        filepath.Join(tempDir, "test.log"),
		UseRealProxmox: os.Getenv("PROXMOX_INTEGRATION_TEST") == api.StringTrue,
		ProxmoxAddr:    getEnvOrDefault("PROXMOX_TEST_ADDR", "https://test.example.com:8006"),
		ProxmoxUser:    getEnvOrDefault("PROXMOX_TEST_USER", "testuser@pam"),
		ProxmoxPass:    getEnvOrDefault("PROXMOX_TEST_PASS", "testpass"),
	}
}

// CreateTestConfigFile creates a configuration file for testing.
func (itc *IntegrationTestConfig) CreateTestConfigFile(t *testing.T, configContent string) {
	if configContent == "" {
		configContent = fmt.Sprintf(`
addr: "%s"
user: "%s"
password: "%s"
realm: "pam"
insecure: true
debug: true
cache_dir: "%s"
`, itc.ProxmoxAddr, itc.ProxmoxUser, itc.ProxmoxPass, itc.CacheDir)
	}

	err := os.WriteFile(itc.ConfigFile, []byte(configContent), 0o644)
	require.NoError(t, err)
}

// CreateTestConfig creates a test configuration object.
func (itc *IntegrationTestConfig) CreateTestConfig() *config.Config {
	cfg := &config.Config{
		Addr:     itc.ProxmoxAddr,
		User:     itc.ProxmoxUser,
		Password: itc.ProxmoxPass,
		Realm:    "pam",
		Insecure: true,
		Debug:    true,
		CacheDir: itc.CacheDir,
	}

	// Ensure cache directory exists
	_ = os.MkdirAll(itc.CacheDir, 0o750)

	return cfg
}

// SetupIntegrationTest sets up a complete integration test environment.
func (itc *IntegrationTestConfig) SetupIntegrationTest(t *testing.T) (*config.Config, interfaces.Logger, interfaces.Cache, *api.Client) {
	// Create configuration
	cfg := itc.CreateTestConfig()
	require.NoError(t, cfg.Validate())

	// Create logger
	testLogger, err := logger.NewInternalLogger(logger.LevelDebug, itc.CacheDir)
	require.NoError(t, err)
	t.Cleanup(func() { testLogger.Close() })

	// Create cache
	testCache, err := cache.NewBadgerCache(itc.CacheDir)
	require.NoError(t, err)
	t.Cleanup(func() { testCache.Close() })

	// Create adapters
	configAdapter := adapters.NewConfigAdapter(cfg)
	loggerAdapter := adapters.NewLoggerAdapter(cfg)

	// Create API client
	client, err := api.NewClient(configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(testCache))

	if itc.UseRealProxmox {
		require.NoError(t, err, "Failed to create client for real Proxmox integration test")
	} else {
		// For mock tests, we expect this might fail, that's ok
		if err != nil {
			t.Logf("Expected error creating client with test credentials: %v", err)
		}
	}

	return cfg, testLogger, testCache, client
}

// MockProxmoxServer creates a mock Proxmox API server for testing.
type MockProxmoxServer struct {
	*httptest.Server

	responses map[string]interface{}
	authToken string
}

// NewMockProxmoxServer creates a new mock Proxmox server.
func NewMockProxmoxServer() *MockProxmoxServer {
	server := &MockProxmoxServer{
		responses: make(map[string]interface{}),
		authToken: "test-auth-token",
	}

	mux := http.NewServeMux()

	// Authentication endpoint
	mux.HandleFunc("/api2/json/access/ticket", server.handleAuth)

	// Version endpoint
	mux.HandleFunc("/api2/json/version", server.handleVersion)

	// Cluster resources endpoint
	mux.HandleFunc("/api2/json/cluster/resources", server.handleClusterResources)

	// Nodes endpoint
	mux.HandleFunc("/api2/json/nodes", server.handleNodes)

	// Status endpoint
	mux.HandleFunc("/api2/json/cluster/status", server.handleClusterStatus)

	// Generic handler for other endpoints
	mux.HandleFunc("/", server.handleGeneric)

	server.Server = httptest.NewTLSServer(mux)

	return server
}

// SetResponse sets a mock response for a specific endpoint.
func (m *MockProxmoxServer) SetResponse(path string, response interface{}) {
	m.responses[path] = response
}

// handleAuth handles authentication requests.
func (m *MockProxmoxServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"ticket":              m.authToken,
			"CSRFPreventionToken": "test-csrf-token",
			"username":            "testuser@pam",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleVersion handles version requests.
func (m *MockProxmoxServer) handleVersion(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"version": "7.4",
			"release": "1",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleClusterResources handles cluster resources requests.
func (m *MockProxmoxServer) handleClusterResources(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{
				"id":     "qemu/100",
				"type":   "qemu",
				"vmid":   100,
				"name":   "test-vm",
				"status": "running",
				"node":   "test-node",
			},
			map[string]interface{}{
				"id":     "lxc/101",
				"type":   "lxc",
				"vmid":   101,
				"name":   "test-container",
				"status": "stopped",
				"node":   "test-node",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleNodes handles nodes requests.
func (m *MockProxmoxServer) handleNodes(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{
				"node":   "test-node",
				"status": "online",
				"type":   "node",
				"level":  "",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleClusterStatus handles cluster status requests.
func (m *MockProxmoxServer) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{
				"name":    "test-cluster",
				"type":    "cluster",
				"quorate": 1,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleGeneric handles generic requests.
func (m *MockProxmoxServer) handleGeneric(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Remove /api2/json prefix for lookup
	if len(path) > 10 && path[:10] == "/api2/json" {
		path = path[10:]
	}

	if response, exists := m.responses[path]; exists {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)

		return
	}

	// Default response for unknown endpoints
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"message": "Mock response for " + r.URL.Path,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// GetURL returns the mock server URL.
func (m *MockProxmoxServer) GetURL() string {
	return m.Server.URL
}

// Close closes the mock server.
func (m *MockProxmoxServer) Close() {
	m.Server.Close()
}

// WaitForCondition waits for a condition to be true with timeout.
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	start := time.Now()
	for !condition() {
		if time.Since(start) > timeout {
			t.Fatalf("Timeout waiting for condition: %s", message)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// AssertEventuallyTrue asserts that a condition becomes true within a timeout.
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	WaitForCondition(t, condition, timeout, message)
}

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

// SkipIfNoRealProxmox skips the test if not running against real Proxmox.
func SkipIfNoRealProxmox(t *testing.T) {
	if os.Getenv("PROXMOX_INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test - set PROXMOX_INTEGRATION_TEST=true to run against real Proxmox")
	}
}

// SkipIfRealProxmox skips the test if running against real Proxmox.
func SkipIfRealProxmox(t *testing.T) {
	if os.Getenv("PROXMOX_INTEGRATION_TEST") == "true" {
		t.Skip("Skipping mock test - running against real Proxmox")
	}
}
