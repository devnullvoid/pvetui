package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/proxmox-tui/internal/adapters"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/test/testutils"
)

// TestConfigIntegration_FileLoading tests loading configuration from YAML files.
func TestConfigIntegration_FileLoading(t *testing.T) {
	itc := testutils.NewIntegrationTestConfig(t)

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

	tests := []struct {
		name          string
		configContent string
		expectError   bool
		expectedAddr  string
		expectedUser  string
		expectedRealm string
		expectedDebug bool
	}{
		{
			name: "valid_password_config",
			configContent: `
addr: "https://pve.example.com:8006"
user: "admin"
password: "secret123"
realm: "pam"
debug: true
insecure: false
`,
			expectError:   false,
			expectedAddr:  "https://pve.example.com:8006",
			expectedUser:  "admin",
			expectedRealm: "pam",
			expectedDebug: true,
		},
		{
			name: "valid_token_config",
			configContent: `
addr: "https://pve.example.com:8006"
user: "apiuser"
token_id: "mytoken"
token_secret: "secret-token-value"
realm: "pve"
debug: false
`,
			expectError:   false,
			expectedAddr:  "https://pve.example.com:8006",
			expectedUser:  "apiuser",
			expectedRealm: "pve",
			expectedDebug: false,
		},
		{
			name: "missing_required_fields",
			configContent: `
user: "admin"
password: "secret123"
# Missing addr
`,
			expectError: true,
		},
		{
			name: "invalid_yaml",
			configContent: `
addr: "https://pve.example.com:8006"
user: "admin"
password: "secret123"
invalid: [unclosed
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			configFile := filepath.Join(itc.TempDir, "test-config.yml")
			err := os.WriteFile(configFile, []byte(tt.configContent), 0o644)
			require.NoError(t, err)

			// Create config and load from file
			cfg := config.NewConfig()
			err = cfg.MergeWithFile(configFile)

			// If we expect an error and got one during file loading, that's expected
			if tt.expectError && err != nil {
				assert.Error(t, err)

				return
			}

			// File loading should succeed for all our test cases
			require.NoError(t, err)

			// Validate configuration
			err = cfg.Validate()
			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			// Check expected values (use getter methods to handle both legacy and profile-based configs)
			assert.Equal(t, tt.expectedAddr, cfg.GetAddr())
			assert.Equal(t, tt.expectedUser, cfg.GetUser())
			assert.Equal(t, tt.expectedRealm, cfg.GetRealm())
			assert.Equal(t, tt.expectedDebug, cfg.Debug)
		})
	}
}

// TestConfigIntegration_EnvironmentVariables tests configuration from environment variables.
func TestConfigIntegration_EnvironmentVariables(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"PROXMOX_ADDR", "PROXMOX_USER", "PROXMOX_PASSWORD",
		"PROXMOX_TOKEN_ID", "PROXMOX_TOKEN_SECRET", "PROXMOX_REALM",
		"PROXMOX_INSECURE", "PROXMOX_DEBUG", "PROXMOX_CACHE_DIR",
	}

	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}

	// Restore environment after test
	t.Cleanup(func() {
		for _, env := range envVars {
			if val, exists := originalEnv[env]; exists && val != "" {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	})

	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		validate    func(t *testing.T, cfg *config.Config)
	}{
		{
			name: "complete_password_auth_env",
			envVars: map[string]string{
				"PROXMOX_ADDR":     "https://env.example.com:8006",
				"PROXMOX_USER":     "envuser",
				"PROXMOX_PASSWORD": "envpass",
				"PROXMOX_REALM":    "pam",
				"PROXMOX_DEBUG":    "true",
				"PROXMOX_INSECURE": "true",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "https://env.example.com:8006", cfg.GetAddr())
				assert.Equal(t, "envuser", cfg.GetUser())
				assert.Equal(t, "envpass", cfg.GetPassword())
				assert.Equal(t, "pam", cfg.GetRealm())
				assert.True(t, cfg.Debug)
				assert.True(t, cfg.GetInsecure())
				assert.False(t, cfg.IsUsingTokenAuth())
			},
		},
		{
			name: "complete_token_auth_env",
			envVars: map[string]string{
				"PROXMOX_ADDR":         "https://token.example.com:8006",
				"PROXMOX_USER":         "tokenuser",
				"PROXMOX_TOKEN_ID":     "mytoken",
				"PROXMOX_TOKEN_SECRET": "secret123",
				"PROXMOX_REALM":        "pve",
				"PROXMOX_DEBUG":        "false",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "https://token.example.com:8006", cfg.GetAddr())
				assert.Equal(t, "tokenuser", cfg.GetUser())
				assert.Equal(t, "mytoken", cfg.GetTokenID())
				assert.Equal(t, "secret123", cfg.GetTokenSecret())
				assert.Equal(t, "pve", cfg.GetRealm())
				assert.False(t, cfg.Debug)
				assert.True(t, cfg.IsUsingTokenAuth())
				expectedToken := "PVEAPIToken=tokenuser@pve!mytoken=secret123"
				assert.Equal(t, expectedToken, cfg.GetAPIToken())
			},
		},
		{
			name: "boolean_variations",
			envVars: map[string]string{
				"PROXMOX_ADDR":     "https://bool.example.com:8006",
				"PROXMOX_USER":     "booluser",
				"PROXMOX_PASSWORD": "boolpass",
				"PROXMOX_DEBUG":    "TRUE",
				"PROXMOX_INSECURE": "True",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				assert.True(t, cfg.Debug)
				assert.True(t, cfg.Insecure)
			},
		},
		{
			name: "missing_auth_credentials",
			envVars: map[string]string{
				"PROXMOX_ADDR": "https://noauth.example.com:8006",
				"PROXMOX_USER": "noauthuser",
				// No password or token
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Create config from environment
			cfg := config.NewConfig()
			cfg.SetDefaults()

			// Validate configuration
			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			// Run custom validation
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

// TestConfigIntegration_FileAndEnvironmentMerging tests merging file and environment configuration.
func TestConfigIntegration_FileAndEnvironmentMerging(t *testing.T) {
	itc := testutils.NewIntegrationTestConfig(t)

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

	// Create config file
	configContent := `
addr: "https://file.example.com:8006"
user: "fileuser"
password: "filepass"
realm: "pam"
debug: false
`
	configFile := filepath.Join(itc.TempDir, "merge-test.yml")
	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Set environment variables that should be overridden by file values
	os.Setenv("PROXMOX_ADDR", "https://env.example.com:8006")
	os.Setenv("PROXMOX_DEBUG", "true")

	// Create config and merge
	cfg := config.NewConfig() // This loads from environment first
	err = cfg.MergeWithFile(configFile)
	require.NoError(t, err)

	// File should take precedence over environment
	assert.Equal(t, "https://file.example.com:8006", cfg.GetAddr()) // From file (overrides env)
	assert.Equal(t, "fileuser", cfg.GetUser())                      // From file
	assert.Equal(t, "filepass", cfg.GetPassword())                  // From file
	assert.False(t, cfg.Debug)                                      // From file (overrides env)

	// Validate the merged configuration
	err = cfg.Validate()
	require.NoError(t, err)
}

// TestConfigIntegration_AdapterCompatibility tests that config works with adapters.
func TestConfigIntegration_AdapterCompatibility(t *testing.T) {
	itc := testutils.NewIntegrationTestConfig(t)

	t.Run("config_adapter_interface", func(t *testing.T) {
		// Create test configuration with password auth only
		cfg := &config.Config{
			Addr:     "https://adapter.example.com:8006",
			User:     "adapteruser",
			Password: "adapterpass",
			Realm:    "pam",
			Insecure: true,
			Debug:    true,
			CacheDir: itc.CacheDir,
		}

		require.NoError(t, cfg.Validate())

		adapter := adapters.NewConfigAdapter(cfg)
		require.NotNil(t, adapter)

		// Test all interface methods
		assert.Equal(t, cfg.GetAddr(), adapter.GetAddr())
		assert.Equal(t, cfg.GetUser(), adapter.GetUser())
		assert.Equal(t, cfg.GetPassword(), adapter.GetPassword())
		assert.Equal(t, cfg.GetRealm(), adapter.GetRealm())
		assert.Equal(t, cfg.GetTokenID(), adapter.GetTokenID())
		assert.Equal(t, cfg.GetTokenSecret(), adapter.GetTokenSecret())
		assert.Equal(t, cfg.GetInsecure(), adapter.GetInsecure())
		assert.Equal(t, cfg.IsUsingTokenAuth(), adapter.IsUsingTokenAuth())
		assert.Equal(t, cfg.GetAPIToken(), adapter.GetAPIToken())
	})

	t.Run("logger_adapter_integration", func(t *testing.T) {
		cfg := &config.Config{
			Debug:    true,
			CacheDir: itc.CacheDir,
		}
		adapter := adapters.NewLoggerAdapter(cfg)
		require.NotNil(t, adapter)

		// Test that logger methods don't panic
		assert.NotPanics(t, func() {
			adapter.Debug("Debug message: %s", "test")
			adapter.Info("Info message: %s", "test")
			adapter.Error("Error message: %s", "test")
		})
	})

	t.Run("password_auth_config", func(t *testing.T) {
		passwordCfg := &config.Config{
			Addr:     "https://pass.example.com:8006",
			User:     "passuser",
			Password: "password123",
			Realm:    "pam",
		}

		require.NoError(t, passwordCfg.Validate())
		assert.False(t, passwordCfg.IsUsingTokenAuth())

		adapter := adapters.NewConfigAdapter(passwordCfg)
		assert.False(t, adapter.IsUsingTokenAuth())
		assert.Empty(t, adapter.GetAPIToken())
	})

	t.Run("token_auth_config", func(t *testing.T) {
		tokenCfg := &config.Config{
			Addr:        "https://token.example.com:8006",
			User:        "tokenuser",
			TokenID:     "mytoken",
			TokenSecret: "secret123",
			Realm:       "pve",
		}

		require.NoError(t, tokenCfg.Validate())
		assert.True(t, tokenCfg.IsUsingTokenAuth())

		adapter := adapters.NewConfigAdapter(tokenCfg)
		assert.True(t, adapter.IsUsingTokenAuth())

		expectedToken := "PVEAPIToken=tokenuser@pve!mytoken=secret123"
		assert.Equal(t, expectedToken, adapter.GetAPIToken())
	})
}

// TestConfigIntegration_DefaultsAndValidation tests default setting and validation.
func TestConfigIntegration_DefaultsAndValidation(t *testing.T) {
	t.Run("defaults_application", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.SetDefaults()

		// Check that defaults are applied
		assert.Equal(t, "pam", cfg.Realm)
		assert.Equal(t, "/api2/json", cfg.ApiPath)
		assert.NotEmpty(t, cfg.CacheDir)
		assert.Contains(t, cfg.CacheDir, "proxmox-tui")
	})

	t.Run("validation_scenarios", func(t *testing.T) {
		tests := []struct {
			name        string
			config      *config.Config
			expectError bool
			errorMsg    string
		}{
			{
				name: "valid_password_auth",
				config: &config.Config{
					Addr:     "https://valid.example.com:8006",
					User:     "validuser",
					Password: "validpass",
				},
				expectError: false,
			},
			{
				name: "valid_token_auth",
				config: &config.Config{
					Addr:        "https://valid.example.com:8006",
					User:        "validuser",
					TokenID:     "validtoken",
					TokenSecret: "validsecret",
				},
				expectError: false,
			},
			{
				name: "missing_address",
				config: &config.Config{
					User:     "user",
					Password: "pass",
				},
				expectError: true,
				errorMsg:    "address required",
			},
			{
				name: "missing_user",
				config: &config.Config{
					Addr:     "https://example.com:8006",
					Password: "pass",
				},
				expectError: true,
				errorMsg:    "username required",
			},
			{
				name: "missing_auth",
				config: &config.Config{
					Addr: "https://example.com:8006",
					User: "user",
				},
				expectError: true,
				errorMsg:    "authentication required",
			},
			{
				name: "incomplete_token_auth",
				config: &config.Config{
					Addr:    "https://example.com:8006",
					User:    "user",
					TokenID: "token",
					// Missing TokenSecret
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

					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
