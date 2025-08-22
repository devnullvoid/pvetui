package scripts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/devnullvoid/pvetui/internal/cache"
	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// GitHubRepo is the URL to the Proxmox Community Scripts repository.
const (
	GitHubRepo    = "https://github.com/community-scripts/ProxmoxVE"
	GitHubAPIRepo = "https://api.github.com/repos/community-scripts/ProxmoxVE"
	RawGitHubRepo = "https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main"
)

// Cache TTLs.
const (
	ScriptMetadataTTL = 24 * time.Hour // Cache script metadata for 24 hours
	ScriptListTTL     = 12 * time.Hour // Cache script list for 12 hours
)

// Cache keys.
const (
	ScriptListCacheKey   = "github_script_list"
	ScriptCacheKeyPrefix = "github_script_"
)

// ScriptCategory represents a category of Proxmox scripts.
type ScriptCategory struct {
	Name        string
	Description string
	Path        string
}

// Script represents a single script from the repository.
type Script struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	Categories    []int  `json:"categories"`
	Type          string `json:"type"` // "ct" for containers, "vm" for VMs
	Updateable    bool   `json:"updateable"`
	Privileged    bool   `json:"privileged"`
	InterfacePort int    `json:"interface_port"`
	Documentation string `json:"documentation"`
	Website       string `json:"website"`
	ConfigPath    string `json:"config_path"`
	Logo          string `json:"logo"`
	ScriptPath    string // Added for our use, not in the JSON
	DateCreated   string `json:"date_created"`
}

// GitHubContent represents a file or directory in the GitHub API.
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

// Scripts logger instance.
var (
	scriptsLogger     interfaces.Logger
	scriptsLoggerOnce sync.Once
)

// getScriptsLogger returns the scripts logger, initializing it if necessary.
func getScriptsLogger() interfaces.Logger {
	scriptsLoggerOnce.Do(func() {
		// Use the global logger system for unified logging
		scriptsLogger = logger.GetPackageLogger("scripts")
	})

	return scriptsLogger
}

// GetScriptCategories returns the available script categories.
func GetScriptCategories() []ScriptCategory {
	return []ScriptCategory{
		{
			Name:        "Container Templates",
			Description: "LXC container templates",
			Path:        "ct",
		},
		{
			Name:        "Virtual Machines",
			Description: "VM installation scripts",
			Path:        "vm",
		},
		// {
		// 	Name:        "Utilities",
		// 	Description: "Utility scripts for Proxmox",
		// 	Path:        "misc",
		// },
		// {
		// 	Name:        "Installation",
		// 	Description: "Installation scripts for Proxmox",
		// 	Path:        "install",
		// },
		{
			Name:        "Tools",
			Description: "Tool scripts for Proxmox",
			Path:        "tools",
		},
	}
}

// GetScriptMetadataFiles fetches the list of script metadata JSON files from the repository.
func GetScriptMetadataFiles() ([]GitHubContent, error) {
	// Check cache first
	c := cache.GetGlobalCache()

	var cachedFiles []GitHubContent

	found, err := c.Get(ScriptListCacheKey, &cachedFiles)
	if err != nil {
		getScriptsLogger().Debug("Cache error for script list: %v", err)
	} else if found && len(cachedFiles) > 0 {
		getScriptsLogger().Debug("Using cached script list (%d items)", len(cachedFiles))

		return cachedFiles, nil
	}

	// The GitHub API URL for the JSON metadata directory
	url := GitHubAPIRepo + "/contents/frontend/public/json"

	// Create a new request with GitHub API headers
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add User-Agent header to avoid GitHub API rate limiting
	req.Header.Add("User-Agent", "pvetui")

	// Execute the request
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch script metadata list: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	// Check for GitHub API rate limiting
	if resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		resetTime := resp.Header.Get("X-RateLimit-Reset")

		return nil, fmt.Errorf("GitHub API rate limit exceeded. Please try again later (reset at %s)", resetTime)
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
	}

	// Parse the JSON response
	var contents []GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	// Filter for JSON files only, but exclude metadata.json and versions.json
	var jsonFiles []GitHubContent

	for _, content := range contents {
		if content.Type == "file" && strings.HasSuffix(content.Name, ".json") {
			// Skip the special metadata files that have different structures
			if content.Name == "metadata.json" || content.Name == "versions.json" {
				getScriptsLogger().Debug("Skipping special metadata file: %s", content.Name)

				continue
			}

			jsonFiles = append(jsonFiles, content)
		}
	}

	// Cache the results
	if len(jsonFiles) > 0 {
		if err := c.Set(ScriptListCacheKey, jsonFiles, ScriptListTTL); err != nil {
			getScriptsLogger().Debug("Failed to cache script list: %v", err)
		} else {
			getScriptsLogger().Debug("Cached script list with %d items", len(jsonFiles))
		}
	}

	return jsonFiles, nil
}

// GetScriptMetadata fetches and parses the metadata for a specific script.
func GetScriptMetadata(metadataURL string) (*Script, error) {
	// Generate a cache key based on the URL
	cacheKey := ScriptCacheKeyPrefix + strings.ReplaceAll(metadataURL, "/", "_")

	// Check cache first
	c := cache.GetGlobalCache()

	var cachedScript Script

	found, err := c.Get(cacheKey, &cachedScript)
	if err != nil {
		getScriptsLogger().Debug("Cache error for script %s: %v", metadataURL, err)
	} else if found && cachedScript.Name != "" {
		getScriptsLogger().Debug("Using cached script metadata for %s", cachedScript.Name)

		return &cachedScript, nil
	}

	// Create a new request with GitHub API headers
	req, err := http.NewRequest(http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add User-Agent header to avoid GitHub API rate limiting
	req.Header.Add("User-Agent", "pvetui")

	// Execute the request
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch script metadata: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	// Check for GitHub API rate limiting
	if resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		resetTime := resp.Header.Get("X-RateLimit-Reset")

		return nil, fmt.Errorf("GitHub API rate limit exceeded. Please try again later (reset at %s)", resetTime)
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
	}

	// Read the response body into a byte slice so we can use it multiple times
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the JSON for the basic script info
	var script Script
	if err := json.Unmarshal(bodyBytes, &script); err != nil {
		return nil, fmt.Errorf("failed to parse script metadata: %w", err)
	}

	// Extract the script path from the install_methods if available
	type InstallMethod struct {
		Type   string `json:"type"`
		Script string `json:"script"`
	}

	type ScriptWithInstallMethods struct {
		InstallMethods []InstallMethod `json:"install_methods"`
	}

	// Parse again to extract install methods
	var scriptWithMethods ScriptWithInstallMethods
	if err := json.Unmarshal(bodyBytes, &scriptWithMethods); err != nil {
		return nil, fmt.Errorf("failed to parse script install methods: %w", err)
	}

	// Extract the script path from the first install method
	if len(scriptWithMethods.InstallMethods) > 0 {
		script.ScriptPath = scriptWithMethods.InstallMethods[0].Script
	} else {
		// If no install methods found, try to guess based on the slug
		if script.Type == "ct" {
			script.ScriptPath = fmt.Sprintf("ct/%s.sh", script.Slug)
		} else if script.Type == "vm" {
			script.ScriptPath = fmt.Sprintf("vm/%s.sh", script.Slug)
		} else {
			// For other types, we might not be able to determine the script path
			getScriptsLogger().Debug("Warning: No install method found for script %s, might not be installable", script.Name)
		}
	}

	// Cache the script metadata
	if script.Name != "" && script.ScriptPath != "" {
		if err := c.Set(cacheKey, script, ScriptMetadataTTL); err != nil {
			getScriptsLogger().Debug("Failed to cache script metadata for %s: %v", script.Name, err)
		} else {
			getScriptsLogger().Debug("Cached script metadata for %s", script.Name)
		}
	}

	return &script, nil
}

// FetchScripts fetches all available scripts from the repository.
func FetchScripts() ([]Script, error) {
	// Get all metadata files
	metadataFiles, err := GetScriptMetadataFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch script metadata files: %w", err)
	}

	// Check if we got any files
	if len(metadataFiles) == 0 {
		return nil, fmt.Errorf("no script metadata files found, GitHub API may be unavailable")
	}

	// Fetch metadata for each script
	var scripts []Script

	var errorCount int

	for _, file := range metadataFiles {
		script, err := GetScriptMetadata(file.DownloadURL)
		if err != nil {
			// Skip this script but log the error
			getScriptsLogger().Debug("Error fetching metadata for %s: %v", file.Name, err)

			errorCount++

			// If we're getting too many errors, something might be wrong with GitHub API
			if errorCount > 5 {
				return scripts, fmt.Errorf("multiple GitHub API errors, rate limit may have been exceeded")
			}

			continue
		}

		if script != nil && script.ScriptPath != "" {
			scripts = append(scripts, *script)
		}
	}

	// If no scripts were found but we had metadata files, it's probably a data issue
	if len(scripts) == 0 && len(metadataFiles) > 0 {
		return nil, fmt.Errorf("no valid scripts found in %d metadata files", len(metadataFiles))
	}

	return scripts, nil
}

// GetScriptsByCategory returns scripts for a specific category.
func GetScriptsByCategory(category string) ([]Script, error) {
	allScripts, err := FetchScripts()
	if err != nil {
		return nil, err
	}

	// Filter scripts by category
	var categoryScripts []Script

	for _, script := range allScripts {
		// If the script path starts with the category name or the type matches
		if strings.HasPrefix(script.ScriptPath, category+"/") || script.Type == category {
			categoryScripts = append(categoryScripts, script)
		}
	}

	if len(categoryScripts) == 0 {
		return nil, fmt.Errorf("no scripts found for category: %s", category)
	}

	return categoryScripts, nil
}

// InstallScript installs a script on a Proxmox node interactively.
func InstallScript(user, nodeIP, scriptPath string) error {
	// Validate script path for security
	for _, c := range scriptPath {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '/' || c == '.' || c == '_' || c == '-') {
			return fmt.Errorf("invalid script path character: %c", c)
		}
	}

	getScriptsLogger().Debug("Installing script: %s on node %s", scriptPath, nodeIP)

	// Build the script installation command using curl (matches official instructions)
	scriptURL := fmt.Sprintf("%s/%s", RawGitHubRepo, scriptPath)
	// Switch to root user completely and run in bash environment
	installCmd := fmt.Sprintf("sudo su - root -c \"SHELL=/bin/bash /bin/bash -c \\\"\\$(curl -fsSL %s)\\\"\"", scriptURL)

	// Use SSH to run the script installation command interactively with proper terminal environment
	sshCmd := exec.Command("ssh", "-t", fmt.Sprintf("%s@%s", user, nodeIP), installCmd)

	// Connect stdin/stdout/stderr for interactive session
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Set environment variables for better terminal compatibility
	// Override TERM to xterm-256color for better compatibility with remote systems
	// This fixes issues with terminals like Kitty (xterm-kitty) that aren't recognized on all systems
	sshCmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Run the command interactively
	err := sshCmd.Run()

	// Show completion status and wait for user input before returning
	utils.WaitForEnterToReturn(err, "Script installation completed successfully!", "Script installation failed")

	getScriptsLogger().Debug("Script installation completed, returning to TUI")

	if err != nil {
		return fmt.Errorf("script installation failed: %w", err)
	}

	return nil
}

// ValidateConnection checks if SSH connection to the node is possible.
func ValidateConnection(user, nodeIP string) error {
	// Simple command to test SSH connection with timeout
	// Use similar SSH options as InstallScript for consistency
	cmd := exec.Command("ssh",
		"-o", "ConnectTimeout=5", // 5 second connection timeout
		"-o", "ServerAliveInterval=2", // Send keepalive every 2 seconds
		"-o", "ServerAliveCountMax=1", // Give up after 1 failed keepalive
		"-o", "BatchMode=yes", // Don't prompt for passwords
		"-o", "StrictHostKeyChecking=no", // Don't prompt for host key verification
		"-o", "UserKnownHostsFile=/dev/null", // Don't save host keys
		"-o", "LogLevel=ERROR", // Reduce SSH verbosity
		fmt.Sprintf("%s@%s", user, nodeIP),
		"echo 'Connection test successful'")

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	return nil
}
