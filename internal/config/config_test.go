package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"filippo.io/age"
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
					// After migration, check profile-based values instead of legacy fields
					if len(tt.initialConfig.Profiles) > 0 {
						// Check profile-based configuration
						defaultProfile, exists := tt.initialConfig.Profiles["default"]
						assert.True(t, exists, "Default profile should exist after migration")
						assert.Equal(t, tt.expectedConfig.Addr, defaultProfile.Addr)
						assert.Equal(t, tt.expectedConfig.User, defaultProfile.User)
						assert.Equal(t, tt.expectedConfig.Password, defaultProfile.Password)
						assert.Equal(t, tt.expectedConfig.Insecure, defaultProfile.Insecure)
					} else {
						// Check legacy fields (if no migration occurred)
						assert.Equal(t, tt.expectedConfig.Addr, tt.initialConfig.Addr)
						assert.Equal(t, tt.expectedConfig.User, tt.initialConfig.User)
						assert.Equal(t, tt.expectedConfig.Password, tt.initialConfig.Password)
						assert.Equal(t, tt.expectedConfig.Insecure, tt.initialConfig.Insecure)
					}
					assert.Equal(t, tt.expectedConfig.Debug, tt.initialConfig.Debug)
				}
			}
		})
	}
}

func TestConfig_MergeWithEncryptedFile(t *testing.T) {
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("sops binary not available")
	}

	if os.Getenv("CI") != "" {
		t.Skip("Skipping encrypted file test in CI environment")
	}

	tempDir := t.TempDir()

	id, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	keyPath := filepath.Join(tempDir, "key.txt")
	require.NoError(t, os.WriteFile(keyPath, []byte(id.String()), 0o600))

	plainPath := filepath.Join(tempDir, "plain.yaml")
	require.NoError(t, os.WriteFile(plainPath, []byte("addr: https://enc.example.com\nuser: encuser\n"), 0o600))

	// nolint:gosec // This is test code with controlled input
	cmd := exec.Command("sops", "--encrypt", "--input-type", "yaml", "--output-type", "yaml", "--age", id.Recipient().String(), plainPath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("sops encryption failed: %v\nstderr: %s", err, string(out))
	}

	encPath := filepath.Join(tempDir, "config.enc.yaml")
	require.NoError(t, os.WriteFile(encPath, out, 0o600))

	cfg := &Config{}
	err = cfg.MergeWithFile(encPath)
	require.NoError(t, err)
	assert.Equal(t, "https://enc.example.com", cfg.Addr)
	assert.Equal(t, "encuser", cfg.User)
}

func TestConfig_SetDefaults(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test that cache directory is set to XDG-compliant path
	assert.NotEmpty(t, config.CacheDir)
	assert.Contains(t, config.CacheDir, "pvetui")
}

// testXDGPathHelper runs tests for XDG path functions with common setup and teardown.
func testXDGPathHelper(t *testing.T, envVar string, testFunc func() string, expectedSuffix string) {
	// Save original environment
	originalEnv := os.Getenv(envVar)
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv(envVar, originalEnv)
		os.Setenv("HOME", originalHome)
	}()

	tests := []struct {
		name           string
		envValue       string
		home           string
		expectedSuffix string
	}{
		{
			name:           envVar + " set",
			envValue:       "/custom/path",
			home:           "/home/user",
			expectedSuffix: "/custom/path/pvetui",
		},
		{
			name:           "HOME set, no " + envVar,
			envValue:       "",
			home:           "/home/user",
			expectedSuffix: expectedSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(envVar, tt.envValue)
			os.Setenv("HOME", tt.home)

			result := testFunc()
			assert.Equal(t, tt.expectedSuffix, result)
		})
	}
}

func TestGetXDGCacheDir(t *testing.T) {
	testXDGPathHelper(t, "XDG_CACHE_HOME", getXDGCacheDir, "/home/user/.cache/pvetui")
}

func TestGetXDGConfigDir(t *testing.T) {
	testXDGPathHelper(t, "XDG_CONFIG_HOME", getXDGConfigDir, "/home/user/.config/pvetui")
}

func TestGetCacheDir(t *testing.T) {
	testXDGPathHelper(t, "XDG_CACHE_HOME", getCacheDir, "/home/user/.cache/pvetui")
}

func TestGetConfigDir(t *testing.T) {
	testXDGPathHelper(t, "XDG_CONFIG_HOME", getConfigDir, "/home/user/.config/pvetui")
}

func TestValidateKeyBindings(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		kb := KeyBindings{SwitchView: "F1", NodesPage: "F1"}
		err := ValidateKeyBindings(kb)
		assert.Error(t, err)
	})

	t.Run("reserved", func(t *testing.T) {
		kb := KeyBindings{Menu: "h"}
		err := ValidateKeyBindings(kb)
		assert.Error(t, err)
	})

	t.Run("system reserved", func(t *testing.T) {
		kb := KeyBindings{Quit: "Ctrl+C"}
		err := ValidateKeyBindings(kb)
		assert.Error(t, err)
	})

	t.Run("valid", func(t *testing.T) {
		kb := KeyBindings{SwitchView: "Ctrl+A", NodesPage: "F1"}
		err := ValidateKeyBindings(kb)
		assert.NoError(t, err)
	})
}

func TestConfig_ProfileBasedConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid profile-based config with password auth",
			config: &Config{
				Profiles: map[string]ProfileConfig{
					"default": {
						Addr:     "https://proxmox.example.com:8006",
						User:     "testuser",
						Password: "testpass",
					},
				},
				DefaultProfile: "default",
			},
			expectError: false,
		},
		{
			name: "valid profile-based config with token auth",
			config: &Config{
				Profiles: map[string]ProfileConfig{
					"default": {
						Addr:        "https://proxmox.example.com:8006",
						User:        "testuser",
						TokenID:     "testtoken",
						TokenSecret: "testsecret",
					},
				},
				DefaultProfile: "default",
			},
			expectError: false,
		},
		{
			name: "missing default profile",
			config: &Config{
				Profiles: map[string]ProfileConfig{
					"profile1": {
						Addr:     "https://proxmox.example.com:8006",
						User:     "testuser",
						Password: "testpass",
					},
				},
				DefaultProfile: "nonexistent",
			},
			expectError: true,
			errorMsg:    "default profile 'nonexistent' not found",
		},
		{
			name: "empty profiles map",
			config: &Config{
				Profiles:       map[string]ProfileConfig{},
				DefaultProfile: "default",
			},
			expectError: true,
			errorMsg:    "proxmox address required: set via -addr flag, PROXMOX_ADDR env var, or config file",
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

func TestConfig_MergeWithFile_ProfileBased(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Test merging with profile-based configuration
	initialConfig := &Config{
		Profiles: map[string]ProfileConfig{
			"default": {
				Addr: "https://initial.example.com:8006",
				User: "initialuser",
			},
		},
		DefaultProfile: "default",
	}

	fileContent := `
profiles:
  default:
    addr: "https://merged.example.com:8006"
    password: "mergedpass"
  secondary:
    addr: "https://secondary.example.com:8006"
    user: "secondaryuser"
    password: "secondarypass"
default_profile: "default"
debug: true
`

	// Create temporary file
	file, err := os.CreateTemp(tempDir, "config-*.yml")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	_, err = file.WriteString(fileContent)
	require.NoError(t, err)
	file.Close()

	// Test merge
	err = initialConfig.MergeWithFile(file.Name())
	assert.NoError(t, err)

	// Verify profiles were merged correctly
	assert.Len(t, initialConfig.Profiles, 2)

	// Check default profile
	defaultProfile, exists := initialConfig.Profiles["default"]
	assert.True(t, exists)
	assert.Equal(t, "https://merged.example.com:8006", defaultProfile.Addr)
	assert.Equal(t, "initialuser", defaultProfile.User) // Should keep initial value
	assert.Equal(t, "mergedpass", defaultProfile.Password)

	// Check secondary profile
	secondaryProfile, exists := initialConfig.Profiles["secondary"]
	assert.True(t, exists)
	assert.Equal(t, "https://secondary.example.com:8006", secondaryProfile.Addr)
	assert.Equal(t, "secondaryuser", secondaryProfile.User)
	assert.Equal(t, "secondarypass", secondaryProfile.Password)

	// Check global settings
	assert.True(t, initialConfig.Debug)
}

func TestConfig_MigrateLegacyToProfiles(t *testing.T) {
	// Test that legacy configuration gets migrated to profile-based
	cfg := &Config{
		Addr:     "https://test.example.com:8006",
		User:     "testuser",
		Password: "testpass",
		Realm:    "pam",
		Insecure: true,
		SSHUser:  "sshuser",
	}

	// Initially should have no profiles
	assert.Equal(t, 0, len(cfg.Profiles))
	assert.Equal(t, "", cfg.DefaultProfile)

	// Migrate should succeed
	migrated := cfg.MigrateLegacyToProfiles()
	assert.True(t, migrated)

	// Should now have a default profile
	assert.Equal(t, "default", cfg.DefaultProfile)
	assert.Equal(t, 1, len(cfg.Profiles))

	// Check that the default profile has the legacy values
	defaultProfile, exists := cfg.Profiles["default"]
	assert.True(t, exists)
	assert.Equal(t, "https://test.example.com:8006", defaultProfile.Addr)
	assert.Equal(t, "testuser", defaultProfile.User)
	assert.Equal(t, "testpass", defaultProfile.Password)
	assert.Equal(t, "pam", defaultProfile.Realm)
	assert.True(t, defaultProfile.Insecure)
	assert.Equal(t, "sshuser", defaultProfile.SSHUser)

	// Legacy fields should be cleared
	assert.Equal(t, "", cfg.Addr)
	assert.Equal(t, "", cfg.User)
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, "", cfg.Realm)
	assert.False(t, cfg.Insecure)
	assert.Equal(t, "", cfg.SSHUser)
}

func TestConfig_MigrateLegacyToProfiles_NoLegacyFields(t *testing.T) {
	// Test that migration doesn't happen when no legacy fields are present
	cfg := &Config{
		Profiles:       make(map[string]ProfileConfig),
		DefaultProfile: "existing",
	}

	// Should not migrate when no legacy fields
	migrated := cfg.MigrateLegacyToProfiles()
	assert.False(t, migrated)

	// Should not have changed
	assert.Equal(t, "existing", cfg.DefaultProfile)
	assert.Equal(t, 0, len(cfg.Profiles))
}

func TestConfig_MigrateLegacyToProfiles_AlreadyHasProfiles(t *testing.T) {
	// Test that migration doesn't happen when profiles already exist
	cfg := &Config{
		Addr:     "https://legacy.example.com:8006",
		User:     "legacyuser",
		Password: "legacypass",
		Profiles: map[string]ProfileConfig{
			"existing": {
				Addr: "https://existing.example.com:8006",
				User: "existinguser",
			},
		},
		DefaultProfile: "existing",
	}

	// Should not migrate when profiles already exist
	migrated := cfg.MigrateLegacyToProfiles()
	assert.False(t, migrated)

	// Should not have changed
	assert.Equal(t, "existing", cfg.DefaultProfile)
	assert.Equal(t, 1, len(cfg.Profiles))
	assert.Equal(t, "https://legacy.example.com:8006", cfg.Addr) // Legacy fields preserved
}

func TestConfig_MergeWithFile_MigratesLegacyConfig(t *testing.T) {
	// Test that loading a legacy config file automatically migrates it
	legacyConfigContent := `
addr: "https://legacy.example.com:8006"
user: "legacyuser"
password: "legacypass"
realm: "pam"
insecure: true
ssh_user: "sshuser"
debug: true
cache_dir: "/tmp/test-cache"
`

	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "legacy-config.yml")
	err := os.WriteFile(configFile, []byte(legacyConfigContent), 0o644)
	require.NoError(t, err)

	// Load the legacy config
	cfg := NewConfig()
	err = cfg.MergeWithFile(configFile)
	require.NoError(t, err)

	// Should now have a default profile with the legacy values
	assert.Equal(t, "default", cfg.DefaultProfile)
	assert.Equal(t, 1, len(cfg.Profiles))

	defaultProfile, exists := cfg.Profiles["default"]
	assert.True(t, exists)
	assert.Equal(t, "https://legacy.example.com:8006", defaultProfile.Addr)
	assert.Equal(t, "legacyuser", defaultProfile.User)
	assert.Equal(t, "legacypass", defaultProfile.Password)
	assert.Equal(t, "pam", defaultProfile.Realm)
	assert.True(t, defaultProfile.Insecure)
	assert.Equal(t, "sshuser", defaultProfile.SSHUser)

	// Legacy fields should be cleared
	assert.Equal(t, "", cfg.Addr)
	assert.Equal(t, "", cfg.User)
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, "", cfg.Realm)
	assert.False(t, cfg.Insecure)
	assert.Equal(t, "", cfg.SSHUser)

	// Global settings should be preserved
	assert.True(t, cfg.Debug)
	assert.Equal(t, "/tmp/test-cache", cfg.CacheDir)

	// Should validate successfully
	err = cfg.Validate()
	assert.NoError(t, err)
}

// Helper function to clear all Proxmox environment variables.
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
