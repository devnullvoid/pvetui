package scripts

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/devnullvoid/proxmox-tui/internal/cache"
	"github.com/devnullvoid/proxmox-tui/pkg/api/testutils"
	"golang.org/x/term"
)

func isTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// Test data for mock GitHub API responses
var mockMetadataFiles = []GitHubContent{
	{
		Name:        "nextcloud.json",
		Path:        "frontend/public/json/nextcloud.json",
		Type:        "file",
		DownloadURL: "https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/frontend/public/json/nextcloud.json",
	},
	{
		Name:        "homeassistant.json",
		Path:        "frontend/public/json/homeassistant.json",
		Type:        "file",
		DownloadURL: "https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/frontend/public/json/homeassistant.json",
	},
	{
		Name:        "metadata.json", // Should be filtered out
		Path:        "frontend/public/json/metadata.json",
		Type:        "file",
		DownloadURL: "https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/frontend/public/json/metadata.json",
	},
}

var mockScript = Script{
	Name:          "Nextcloud",
	Slug:          "nextcloud",
	Description:   "Cloud storage and collaboration platform",
	Categories:    []int{1, 2},
	Type:          "ct",
	Updateable:    true,
	Privileged:    false,
	InterfacePort: 80,
	Documentation: "https://nextcloud.com/",
	Website:       "https://nextcloud.com/",
	ScriptPath:    "ct/nextcloud.sh",
	DateCreated:   "2023-01-01",
}

func TestGetScriptCategories(t *testing.T) {
	categories := GetScriptCategories()

	// Test that we get expected categories
	assert.NotEmpty(t, categories)
	assert.GreaterOrEqual(t, len(categories), 3) // At least container, VM, tools

	// Test specific categories exist
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	assert.Contains(t, categoryNames, "Container Templates")
	assert.Contains(t, categoryNames, "Virtual Machines")
	assert.Contains(t, categoryNames, "Tools")

	// Test category structure
	for _, category := range categories {
		assert.NotEmpty(t, category.Name)
		assert.NotEmpty(t, category.Description)
		assert.NotEmpty(t, category.Path)
	}
}

// Note: Complex integration tests for GitHub API calls are moved to
// a separate integration test file to avoid network dependencies in unit tests

func TestValidateConnection(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		nodeIP      string
		expectError bool
	}{
		{
			name:        "empty user",
			user:        "",
			nodeIP:      "192.168.1.100",
			expectError: true,
		},
		{
			name:        "empty nodeIP",
			user:        "root",
			nodeIP:      "",
			expectError: true,
		},
		{
			name:        "invalid nodeIP",
			user:        "root",
			nodeIP:      "invalid-ip",
			expectError: true,
		},
		{
			name:        "non-existent host",
			user:        "root",
			nodeIP:      "192.168.254.254", // Non-routable IP for faster timeout
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConnection(tt.user, tt.nodeIP)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				// Note: This would require actual SSH connectivity in a real test
				// In a unit test, we'd mock the exec.Command
				t.Skip("Skipping actual SSH connection test")
			}
		})
	}
}

func TestInstallScript_Validation(t *testing.T) {
	if os.Getenv("CI") != "" || !isTerminal(os.Stdin.Fd()) {
		t.Skip("Skipping InstallScript validation in CI or non-interactive environment")
	}
	tests := []struct {
		name        string
		scriptPath  string
		expectError bool
		errorMsg    string
		skipSSH     bool // Skip SSH connection attempt for validation-only tests
	}{
		{
			name:        "valid script path",
			scriptPath:  "ct/nextcloud.sh",
			expectError: true, // Will fail on SSH, but should not have validation error
			skipSSH:     false,
		},
		{
			name:        "script path with subdirectory",
			scriptPath:  "tools/backup/simple.sh",
			expectError: true, // Will fail on SSH, but should not have validation error
			skipSSH:     false,
		},
		{
			name:        "invalid character - semicolon",
			scriptPath:  "ct/test;rm -rf /.sh",
			expectError: true,
			errorMsg:    "invalid script path character",
			skipSSH:     true, // Should fail validation before SSH attempt
		},
		{
			name:        "invalid character - pipe",
			scriptPath:  "ct/test|malicious.sh",
			expectError: true,
			errorMsg:    "invalid script path character",
			skipSSH:     true,
		},
		{
			name:        "invalid character - ampersand",
			scriptPath:  "ct/test&background.sh",
			expectError: true,
			errorMsg:    "invalid script path character",
			skipSSH:     true,
		},
		{
			name:        "invalid character - dollar",
			scriptPath:  "ct/test$var.sh",
			expectError: true,
			errorMsg:    "invalid script path character",
			skipSSH:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a non-routable IP for faster timeout
			err := InstallScript("testuser", "192.168.254.254", tt.scriptPath)

			assert.Error(t, err)
			if tt.errorMsg != "" {
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else if !tt.skipSSH {
				// For valid paths, error should be SSH-related, not validation
				assert.NotContains(t, err.Error(), "invalid script path character")
			}
		})
	}
}

func TestScript_Methods(t *testing.T) {
	script := &Script{
		Name:        "Test Script",
		Type:        "ct",
		ScriptPath:  "ct/test.sh",
		Categories:  []int{1, 2, 3},
		Updateable:  true,
		Privileged:  false,
		DateCreated: "2023-01-01",
	}

	// Test basic properties
	assert.Equal(t, "Test Script", script.Name)
	assert.Equal(t, "ct", script.Type)
	assert.Equal(t, "ct/test.sh", script.ScriptPath)
	assert.True(t, script.Updateable)
	assert.False(t, script.Privileged)
}

func TestScriptCategory_Methods(t *testing.T) {
	category := &ScriptCategory{
		Name:        "Container Templates",
		Description: "LXC container templates",
		Path:        "ct",
	}

	assert.Equal(t, "Container Templates", category.Name)
	assert.Equal(t, "LXC container templates", category.Description)
	assert.Equal(t, "ct", category.Path)
}

// Benchmark tests for performance profiling
func BenchmarkGetScriptCategories(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetScriptCategories()
	}
}

// BenchmarkCacheOperations benchmarks cache set/get performance
func BenchmarkCacheOperations(b *testing.B) {
	testCache := testutils.NewInMemoryCache()

	b.Run("cache_set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-key-%d", i)
			_ = testCache.Set(key, mockScript, time.Hour)
		}
	})

	b.Run("cache_get", func(b *testing.B) {
		// Pre-populate cache
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("benchmark-key-%d", i)
			_ = testCache.Set(key, mockScript, time.Hour)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-key-%d", i%1000)
			var result Script
			_, _ = testCache.Get(key, &result)
		}
	})
}

// BenchmarkScriptValidation benchmarks the script path validation
func BenchmarkScriptValidation(b *testing.B) {
	validPaths := []string{
		"ct/nextcloud.sh",
		"vm/ubuntu.sh",
		"tools/backup.sh",
		"misc/utility.sh",
	}

	invalidPaths := []string{
		"ct/test;malicious.sh",
		"vm/test|bad.sh",
		"tools/test&evil.sh",
	}

	b.Run("valid_paths", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			path := validPaths[i%len(validPaths)]
			// Test just the validation logic by checking for invalid characters
			for _, c := range path {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '/' || c == '.' || c == '_' || c == '-') {
					break
				}
			}
		}
	})

	b.Run("invalid_paths", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			path := invalidPaths[i%len(invalidPaths)]
			// Test just the validation logic by checking for invalid characters
			for _, c := range path {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '/' || c == '.' || c == '_' || c == '-') {
					break
				}
			}
		}
	})
}

func BenchmarkGetScriptMetadataFiles_WithMemoryCache(b *testing.B) {
	// Create a new memory cache instance for this benchmark
	memCache := cache.NewMemoryCache()

	// Pre-populate cache with mock data
	_ = memCache.Set(ScriptListCacheKey, mockMetadataFiles, ScriptListTTL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This benchmark only tests cache hits since we can't make real GitHub API calls
		// For real network benchmarks, use integration tests
		_, _ = GetScriptMetadataFiles()
	}
}

func BenchmarkGetScriptMetadata_WithMemoryCache(b *testing.B) {
	// Create a new memory cache instance for this benchmark
	memCache := cache.NewMemoryCache()

	// Pre-populate cache with mock data
	cacheKey := ScriptCacheKeyPrefix + "test_url"
	_ = memCache.Set(cacheKey, mockScript, ScriptMetadataTTL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This benchmark only tests cache hits since we can't make real GitHub API calls
		_, _ = GetScriptMetadata("test_url")
	}
}

// Test helper functions
func TestScriptsLogger(t *testing.T) {
	logger1 := getScriptsLogger()
	logger2 := getScriptsLogger()

	// Should return the same instance (singleton pattern)
	assert.Equal(t, logger1, logger2)
	assert.NotNil(t, logger1)
}

// Test cache TTL constants
func TestCacheTTLConstants(t *testing.T) {
	assert.Equal(t, 24*time.Hour, ScriptMetadataTTL)
	assert.Equal(t, 12*time.Hour, ScriptListTTL)
	assert.Equal(t, "github_script_list", ScriptListCacheKey)
	assert.Equal(t, "github_script_", ScriptCacheKeyPrefix)
}

// Test GitHub repository constants
func TestGitHubConstants(t *testing.T) {
	assert.Contains(t, GitHubRepo, "github.com/community-scripts/ProxmoxVE")
	assert.Contains(t, GitHubAPIRepo, "api.github.com/repos/community-scripts/ProxmoxVE")
	assert.Contains(t, RawGitHubRepo, "raw.githubusercontent.com/community-scripts/ProxmoxVE/main")
}

// Test GitHubContent struct
func TestGitHubContent(t *testing.T) {
	content := GitHubContent{
		Name:        "test.json",
		Path:        "path/to/test.json",
		Type:        "file",
		DownloadURL: "https://example.com/test.json",
	}

	assert.Equal(t, "test.json", content.Name)
	assert.Equal(t, "path/to/test.json", content.Path)
	assert.Equal(t, "file", content.Type)
	assert.Equal(t, "https://example.com/test.json", content.DownloadURL)
}

// Test script filtering logic (without external dependencies)
func TestScriptFiltering(t *testing.T) {
	scripts := []Script{
		{Name: "Nextcloud", Type: "ct", ScriptPath: "ct/nextcloud.sh"},
		{Name: "Ubuntu VM", Type: "vm", ScriptPath: "vm/ubuntu.sh"},
		{Name: "Backup Tool", Type: "tool", ScriptPath: "tools/backup.sh"},
		{Name: "LXC Container", Type: "ct", ScriptPath: "ct/lxc.sh"},
	}

	// Filter by container type
	var ctScripts []Script
	for _, script := range scripts {
		if script.Type == "ct" {
			ctScripts = append(ctScripts, script)
		}
	}

	assert.Len(t, ctScripts, 2)
	assert.Equal(t, "Nextcloud", ctScripts[0].Name)
	assert.Equal(t, "LXC Container", ctScripts[1].Name)

	// Filter by VM type
	var vmScripts []Script
	for _, script := range scripts {
		if script.Type == "vm" {
			vmScripts = append(vmScripts, script)
		}
	}

	assert.Len(t, vmScripts, 1)
	assert.Equal(t, "Ubuntu VM", vmScripts[0].Name)
}

// Test caching functions using testutils in-memory cache
func TestCacheIntegrationWithTestUtils(t *testing.T) {
	// Create a test cache
	testCache := testutils.NewInMemoryCache()

	// Test basic cache operations
	t.Run("cache_set_and_get", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		err := testCache.Set(key, value, time.Hour)
		assert.NoError(t, err)

		var result string
		found, err := testCache.Get(key, &result)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})

	t.Run("cache_complex_data", func(t *testing.T) {
		key := "complex-key"
		value := map[string]interface{}{
			"name": "test",
			"id":   123,
		}

		err := testCache.Set(key, value, time.Hour)
		assert.NoError(t, err)

		var result map[string]interface{}
		found, err := testCache.Get(key, &result)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})

	t.Run("cache_clear", func(t *testing.T) {
		// Set some data
		_ = testCache.Set("key1", "value1", time.Hour)
		_ = testCache.Set("key2", "value2", time.Hour)

		// Clear cache
		err := testCache.Clear()
		assert.NoError(t, err)

		// Verify data is gone
		var result string
		found, err := testCache.Get("key1", &result)
		assert.NoError(t, err)
		assert.False(t, found)

		found, err = testCache.Get("key2", &result)
		assert.NoError(t, err)
		assert.False(t, found)
	})
}

// Integration tests that require network access should be in separate file
// or marked with build tags for optional execution
func TestGetScriptMetadataFiles_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would test actual GitHub API calls
	// For now, we skip to avoid network dependencies in unit tests
	t.Skip("Integration test - requires network access to GitHub API")
}

func TestFetchScripts_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would test actual script fetching
	// For now, we skip to avoid network dependencies in unit tests
	t.Skip("Integration test - requires network access to GitHub API")
}
