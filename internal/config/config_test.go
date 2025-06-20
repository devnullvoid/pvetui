package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name:    "empty environment",
			envVars: map[string]string{},
			expected: &Config{
				Realm:   "pam",
				ApiPath: "/api2/json",
			},
		},
		{
			name: "all environment variables set",
			envVars: map[string]string{
				"PROXMOX_ADDR":         "https://proxmox.example.com:8006",
				"PROXMOX_USER":         "testuser",
				"PROXMOX_PASSWORD":     "testpass",
				"PROXMOX_TOKEN_ID":     "testtoken",
				"PROXMOX_TOKEN_SECRET": "testsecret",
				"PROXMOX_REALM":        "ldap",
				"PROXMOX_API_PATH":     "/api2/json/custom",
				"PROXMOX_INSECURE":     "true",
				"PROXMOX_SSH_USER":     "sshuser",
				"PROXMOX_DEBUG":        "true",
				"PROXMOX_CACHE_DIR":    "/tmp/cache",
			},
			expected: &Config{
				Addr:        "https://proxmox.example.com:8006",
				User:        "testuser",
				Password:    "testpass",
				TokenID:     "testtoken",
				TokenSecret: "testsecret",
				Realm:       "ldap",
				ApiPath:     "/api2/json/custom",
				Insecure:    true,
				SSHUser:     "sshuser",
				Debug:       true,
				CacheDir:    "/tmp/cache",
			},
		},
		{
			name: "boolean environment variables with different cases",
			envVars: map[string]string{
				"PROXMOX_INSECURE": "TRUE",
				"PROXMOX_DEBUG":    "True",
			},
			expected: &Config{
				Realm:    "pam",
				ApiPath:  "/api2/json",
				Insecure: true,
				Debug:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearProxmoxEnvVars()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Create config
			config := NewConfig()

			// Verify results
			assert.Equal(t, tt.expected.Addr, config.Addr)
			assert.Equal(t, tt.expected.User, config.User)
			assert.Equal(t, tt.expected.Password, config.Password)
			assert.Equal(t, tt.expected.TokenID, config.TokenID)
			assert.Equal(t, tt.expected.TokenSecret, config.TokenSecret)
			assert.Equal(t, tt.expected.Realm, config.Realm)
			assert.Equal(t, tt.expected.ApiPath, config.ApiPath)
			assert.Equal(t, tt.expected.Insecure, config.Insecure)
			assert.Equal(t, tt.expected.SSHUser, config.SSHUser)
			assert.Equal(t, tt.expected.Debug, config.Debug)
			assert.Equal(t, tt.expected.CacheDir, config.CacheDir)

			// Clean up
			clearProxmoxEnvVars()
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with password auth",
			config: &Config{
				Addr:     "https://proxmox.example.com:8006",
				User:     "testuser",
				Password: "testpass",
			},
			expectError: false,
		},
		{
			name: "valid config with token auth",
			config: &Config{
				Addr:        "https://proxmox.example.com:8006",
				User:        "testuser",
				TokenID:     "testtoken",
				TokenSecret: "testsecret",
			},
			expectError: false,
		},
		{
			name: "missing address",
			config: &Config{
				User:     "testuser",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "proxmox address required",
		},
		{
			name: "missing user",
			config: &Config{
				Addr:     "https://proxmox.example.com:8006",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "proxmox username required",
		},
		{
			name: "missing authentication",
			config: &Config{
				Addr: "https://proxmox.example.com:8006",
				User: "testuser",
			},
			expectError: true,
			errorMsg:    "authentication required",
		},
		{
			name: "conflicting authentication methods",
			config: &Config{
				Addr:        "https://proxmox.example.com:8006",
				User:        "testuser",
				Password:    "testpass",
				TokenID:     "testtoken",
				TokenSecret: "testsecret",
			},
			expectError: true,
			errorMsg:    "conflicting authentication methods",
		},
		{
			name: "incomplete token auth - missing secret",
			config: &Config{
				Addr:    "https://proxmox.example.com:8006",
				User:    "testuser",
				TokenID: "testtoken",
			},
			expectError: true,
			errorMsg:    "authentication required",
		},
		{
			name: "incomplete token auth - missing ID",
			config: &Config{
				Addr:        "https://proxmox.example.com:8006",
				User:        "testuser",
				TokenSecret: "testsecret",
			},
			expectError: true,
			errorMsg:    "authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_IsUsingTokenAuth(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "using token auth",
			config: &Config{
				TokenID:     "testtoken",
				TokenSecret: "testsecret",
			},
			expected: true,
		},
		{
			name: "using password auth",
			config: &Config{
				Password: "testpass",
			},
			expected: false,
		},
		{
			name: "incomplete token auth - missing secret",
			config: &Config{
				TokenID: "testtoken",
			},
			expected: false,
		},
		{
			name: "incomplete token auth - missing ID",
			config: &Config{
				TokenSecret: "testsecret",
			},
			expected: false,
		},
		{
			name:     "empty config",
			config:   &Config{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsUsingTokenAuth()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_GetAPIToken(t *testing.T) {
	config := &Config{
		User:        "testuser",
		Realm:       "pam",
		TokenID:     "testtoken",
		TokenSecret: "testsecret",
	}

	expected := "PVEAPIToken=testuser@pam!testtoken=testsecret"
	result := config.GetAPIToken()
	assert.Equal(t, expected, result)
}

func TestConfig_MergeWithFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		initialConfig  *Config
		fileContent    string
		expectedConfig *Config
		expectError    bool
	}{
		{
			name: "merge with valid YAML file",
			initialConfig: &Config{
				Addr: "https://initial.example.com:8006",
				User: "initialuser",
			},
			fileContent: `
addr: "https://merged.example.com:8006"
password: "mergedpass"
debug: true
insecure: true
`,
			expectedConfig: &Config{
				Addr:     "https://merged.example.com:8006",
				User:     "initialuser", // Should keep initial value
				Password: "mergedpass",
				Debug:    true,
				Insecure: true,
			},
			expectError: false,
		},
		{
			name: "merge with empty file path",
			initialConfig: &Config{
				Addr: "https://initial.example.com:8006",
				User: "initialuser",
			},
			fileContent: "",
			expectedConfig: &Config{
				Addr: "https://initial.example.com:8006",
				User: "initialuser",
			},
			expectError: false,
		},
		{
			name: "merge with invalid YAML",
			initialConfig: &Config{
				Addr: "https://initial.example.com:8006",
			},
			fileContent: `
invalid: yaml: content:
  - malformed
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if tt.fileContent != "" {
				// Create temporary file
				file, err := os.CreateTemp(tempDir, "config-*.yml")
				require.NoError(t, err)
				defer os.Remove(file.Name())

				_, err = file.WriteString(tt.fileContent)
				require.NoError(t, err)
				file.Close()

				filePath = file.Name()
			}

			// Test merge
			err := tt.initialConfig.MergeWithFile(filePath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedConfig != nil {
					assert.Equal(t, tt.expectedConfig.Addr, tt.initialConfig.Addr)
					assert.Equal(t, tt.expectedConfig.User, tt.initialConfig.User)
					assert.Equal(t, tt.expectedConfig.Password, tt.initialConfig.Password)
					assert.Equal(t, tt.expectedConfig.Debug, tt.initialConfig.Debug)
					assert.Equal(t, tt.expectedConfig.Insecure, tt.initialConfig.Insecure)
				}
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test that cache directory is set to XDG-compliant path
	assert.NotEmpty(t, config.CacheDir)
	assert.Contains(t, config.CacheDir, "proxmox-tui")
}

func TestGetXDGCacheDir(t *testing.T) {
	// Save original environment
	originalXDGCache := os.Getenv("XDG_CACHE_HOME")
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("XDG_CACHE_HOME", originalXDGCache)
		os.Setenv("HOME", originalHome)
	}()

	tests := []struct {
		name           string
		xdgCache       string
		home           string
		expectedSuffix string
	}{
		{
			name:           "XDG_CACHE_HOME set",
			xdgCache:       "/custom/cache",
			home:           "/home/user",
			expectedSuffix: "/custom/cache/proxmox-tui",
		},
		{
			name:           "HOME set, no XDG_CACHE_HOME",
			xdgCache:       "",
			home:           "/home/user",
			expectedSuffix: "/home/user/.cache/proxmox-tui",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("XDG_CACHE_HOME", tt.xdgCache)
			os.Setenv("HOME", tt.home)

			result := getXDGCacheDir()
			assert.Equal(t, tt.expectedSuffix, result)
		})
	}
}

func TestGetXDGConfigDir(t *testing.T) {
	// Save original environment
	originalXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalXDGConfig)
		os.Setenv("HOME", originalHome)
	}()

	tests := []struct {
		name           string
		xdgConfig      string
		home           string
		expectedSuffix string
	}{
		{
			name:           "XDG_CONFIG_HOME set",
			xdgConfig:      "/custom/config",
			home:           "/home/user",
			expectedSuffix: "/custom/config/proxmox-tui",
		},
		{
			name:           "HOME set, no XDG_CONFIG_HOME",
			xdgConfig:      "",
			home:           "/home/user",
			expectedSuffix: "/home/user/.config/proxmox-tui",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("XDG_CONFIG_HOME", tt.xdgConfig)
			os.Setenv("HOME", tt.home)

			result := getXDGConfigDir()
			assert.Equal(t, tt.expectedSuffix, result)
		})
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	result := GetDefaultConfigPath()
	assert.Contains(t, result, "proxmox-tui")
	assert.Contains(t, result, "config.yml")
	assert.True(t, filepath.IsAbs(result))
}

// Helper function to clear all Proxmox environment variables
func clearProxmoxEnvVars() {
	envVars := []string{
		"PROXMOX_ADDR",
		"PROXMOX_USER",
		"PROXMOX_PASSWORD",
		"PROXMOX_TOKEN_ID",
		"PROXMOX_TOKEN_SECRET",
		"PROXMOX_REALM",
		"PROXMOX_API_PATH",
		"PROXMOX_INSECURE",
		"PROXMOX_SSH_USER",
		"PROXMOX_DEBUG",
		"PROXMOX_CACHE_DIR",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
