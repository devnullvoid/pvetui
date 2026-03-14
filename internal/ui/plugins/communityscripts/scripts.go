package communityscripts

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	GitHubRepo             = "https://github.com/community-scripts/ProxmoxVE"
	GitHubAPIRepo          = "https://api.github.com/repos/community-scripts/ProxmoxVE"
	RawGitHubRepo          = "https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main"
	RawGitHubDevRepo       = "https://raw.githubusercontent.com/community-scripts/ProxmoxVED/main"
	MetadataPocketBaseBase = "https://db.community-scripts.org"
	MetadataPocketBaseAPI  = MetadataPocketBaseBase + "/api/collections"
)

// Cache TTLs.
const (
	ScriptMetadataTTL = 24 * time.Hour // Cache script metadata for 24 hours
	ScriptListTTL     = 12 * time.Hour // Cache script list for 12 hours
)

// Cache keys.
const (
	ScriptMetadataListCacheKey = "communityscripts_script_metadata_list_v3"
	ScriptListCacheKey         = "communityscripts_script_list_v3"
	ScriptCacheKeyPrefix       = "communityscripts_script_"
)

var allowedMetadataHosts = map[string]struct{}{
	"db.community-scripts.org":  {},
	"api.github.com":            {},
	"raw.githubusercontent.com": {},
}

// ScriptCategory represents a category of Proxmox scripts.
type ScriptCategory struct {
	Name        string
	Description string
	Path        string
}

// Script represents a single script from the repository.
type Script struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Description   string   `json:"description"`
	Categories    []string `json:"categories"`
	Type          string   `json:"type"` // "ct" for containers, "vm" for VMs
	Updateable    bool     `json:"updateable"`
	Privileged    bool     `json:"privileged"`
	InterfacePort int      `json:"interface_port"`
	Documentation string   `json:"documentation"`
	Website       string   `json:"website"`
	ConfigPath    string   `json:"config_path"`
	Logo          string   `json:"logo"`
	ScriptPath    string   // Added for our use, not in the JSON
	DateCreated   string   `json:"date_created"`
	IsDev         bool     `json:"is_dev"`
	IsDisabled    bool     `json:"is_disabled"`
	IsDeleted     bool     `json:"is_deleted"`
}

// GitHubContent represents a file or directory in the GitHub API.
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

type pocketBaseRelation struct {
	Type string `json:"type"`
}

type pocketBaseScriptRecord struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Slug              string                 `json:"slug"`
	Description       string                 `json:"description"`
	Categories        []string               `json:"categories"`
	Type              string                 `json:"type"`
	Updateable        bool                   `json:"updateable"`
	Privileged        bool                   `json:"privileged"`
	Port              int                    `json:"port"`
	Documentation     string                 `json:"documentation"`
	Website           string                 `json:"website"`
	ConfigPath        string                 `json:"config_path"`
	Logo              string                 `json:"logo"`
	Created           string                 `json:"created"`
	IsDev             bool                   `json:"is_dev"`
	IsDisabled        bool                   `json:"is_disabled"`
	IsDeleted         bool                   `json:"is_deleted"`
	InstallMethodsRaw json.RawMessage        `json:"install_methods_json"`
	NotesRaw          json.RawMessage        `json:"notes_json"`
	Expand            pocketBaseScriptExpand `json:"expand"`
}

type pocketBaseScriptExpand struct {
	Type pocketBaseRelation `json:"type"`
}

type pocketBaseResponse struct {
	Page       int                      `json:"page"`
	PerPage    int                      `json:"perPage"`
	TotalPages int                      `json:"totalPages"`
	TotalItems int                      `json:"totalItems"`
	Items      []pocketBaseScriptRecord `json:"items"`
}

// Scripts logger instance.
var (
	scriptsLogger     interfaces.Logger
	scriptsLoggerOnce sync.Once
	pluginCache       cache.Cache
	pluginCacheOnce   sync.Once
)

// getScriptsLogger returns the scripts logger, initializing it if necessary.
func getScriptsLogger() interfaces.Logger {
	scriptsLoggerOnce.Do(func() {
		// Use the global logger system for unified logging
		scriptsLogger = logger.GetPackageLogger("scripts")
	})

	return scriptsLogger
}

// getPluginCache returns a cache instance dedicated to the community scripts plugin.
func getPluginCache() cache.Cache {
	pluginCacheOnce.Do(func() {
		pluginCache = cache.GetNamespacedCache("communityscripts")
	})

	return pluginCache
}

func validateMetadataURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("missing URL host")
	}

	if _, ok := allowedMetadataHosts[strings.ToLower(host)]; !ok {
		return "", fmt.Errorf("unsupported metadata host %q", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		return "", fmt.Errorf("IP hosts are not allowed")
	}

	return parsed.String(), nil
}

func buildScriptPath(scriptType, slug string) string {
	switch scriptType {
	case "lxc":
		return fmt.Sprintf("ct/%s.sh", slug)
	case "vm":
		return fmt.Sprintf("vm/%s.sh", slug)
	case "addon":
		return fmt.Sprintf("tools/addon/%s.sh", slug)
	case "pve":
		return fmt.Sprintf("tools/pve/%s.sh", slug)
	case "turnkey":
		return "turnkey/turnkey.sh"
	default:
		return ""
	}
}

func normalizeScriptType(scriptType string) string {
	switch scriptType {
	case "lxc":
		return "ct"
	case "vm":
		return "vm"
	case "addon", "pve":
		return "tools"
	default:
		return scriptType
	}
}

func mapPocketBaseRecord(record pocketBaseScriptRecord) Script {
	sourceType := record.Expand.Type.Type
	if sourceType == "" {
		sourceType = record.Type
	}

	return Script{
		Name:          record.Name,
		Slug:          record.Slug,
		Description:   record.Description,
		Categories:    record.Categories,
		Type:          normalizeScriptType(sourceType),
		Updateable:    record.Updateable,
		Privileged:    record.Privileged,
		InterfacePort: record.Port,
		Documentation: record.Documentation,
		Website:       record.Website,
		ConfigPath:    record.ConfigPath,
		Logo:          record.Logo,
		ScriptPath:    buildScriptPath(sourceType, record.Slug),
		DateCreated:   record.Created,
		IsDev:         record.IsDev,
		IsDisabled:    record.IsDisabled,
		IsDeleted:     record.IsDeleted,
	}
}

func fetchPocketBase(url string, target any) error {
	url, err := validateMetadataURL(url)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("User-Agent", "pvetui")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch metadata: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("metadata API error: %s - %s", resp.Status, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to parse metadata response: %w", err)
	}

	return nil
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
	c := getPluginCache()

	var cachedFiles []GitHubContent

	found, err := c.Get(ScriptMetadataListCacheKey, &cachedFiles)
	if err != nil {
		getScriptsLogger().Debug("Cache error for script list: %v", err)
	} else if found && len(cachedFiles) > 0 {
		getScriptsLogger().Debug("Using cached script list (%d items)", len(cachedFiles))

		return cachedFiles, nil
	}

	const perPage = 200
	page := 1
	records := []GitHubContent{}

	for {
		endpoint := fmt.Sprintf("%s/script_scripts/records?perPage=%d&page=%d&sort=slug", MetadataPocketBaseAPI, perPage, page)

		var response pocketBaseResponse
		if err := fetchPocketBase(endpoint, &response); err != nil {
			return nil, fmt.Errorf("failed to fetch script metadata list: %w", err)
		}

		for _, item := range response.Items {
			records = append(records, GitHubContent{
				Name:        item.Slug,
				Path:        "script_scripts/" + item.ID,
				Type:        "record",
				DownloadURL: MetadataPocketBaseAPI + "/script_scripts/records/" + item.ID + "?expand=type",
			})
		}

		if page >= response.TotalPages || len(response.Items) == 0 {
			break
		}

		page++
	}

	// Cache the results
	if len(records) > 0 {
		if err := c.Set(ScriptMetadataListCacheKey, records, ScriptListTTL); err != nil {
			getScriptsLogger().Debug("Failed to cache script list: %v", err)
		} else {
			getScriptsLogger().Debug("Cached script list with %d items", len(records))
		}
	}

	return records, nil
}

// GetScriptMetadata fetches and parses the metadata for a specific script.
func GetScriptMetadata(metadataURL string) (*Script, error) {
	// Generate a cache key based on the URL
	cacheKey := ScriptCacheKeyPrefix + strings.ReplaceAll(metadataURL, "/", "_")

	// Check cache first
	c := getPluginCache()

	var cachedScript Script

	found, err := c.Get(cacheKey, &cachedScript)
	if err != nil {
		getScriptsLogger().Debug("Cache error for script %s: %v", metadataURL, err)
	} else if found && cachedScript.Name != "" {
		getScriptsLogger().Debug("Using cached script metadata for %s", cachedScript.Name)

		return &cachedScript, nil
	}

	var record pocketBaseScriptRecord
	if err := fetchPocketBase(metadataURL, &record); err != nil {
		return nil, fmt.Errorf("failed to fetch script metadata: %w", err)
	}

	script := mapPocketBaseRecord(record)

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
	c := getPluginCache()

	var cachedScripts []Script
	found, err := c.Get(ScriptListCacheKey, &cachedScripts)
	if err != nil {
		getScriptsLogger().Debug("Cache error for fetched scripts: %v", err)
	} else if found && len(cachedScripts) > 0 {
		getScriptsLogger().Debug("Using cached fetched scripts (%d items)", len(cachedScripts))

		return cachedScripts, nil
	}

	const perPage = 200
	var (
		page    = 1
		scripts []Script
	)

	for {
		endpoint := fmt.Sprintf("%s/script_scripts/records?perPage=%d&page=%d&sort=slug&expand=type", MetadataPocketBaseAPI, perPage, page)

		var response pocketBaseResponse
		if err := fetchPocketBase(endpoint, &response); err != nil {
			return nil, fmt.Errorf("failed to fetch script metadata from PocketBase: %w", err)
		}

		for _, item := range response.Items {
			script := mapPocketBaseRecord(item)
			if script.ScriptPath == "" {
				getScriptsLogger().Debug("Skipping script %s with unsupported type %q", script.Name, item.Expand.Type.Type)
				continue
			}

			scripts = append(scripts, script)
		}

		if page >= response.TotalPages || len(response.Items) == 0 {
			break
		}

		page++
	}

	if len(scripts) == 0 {
		return nil, fmt.Errorf("no valid scripts found in PocketBase metadata")
	}

	if err := c.Set(ScriptListCacheKey, scripts, ScriptListTTL); err != nil {
		getScriptsLogger().Debug("Failed to cache fetched scripts: %v", err)
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
func validateScriptPath(scriptPath string) error {
	for _, c := range scriptPath {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '/' || c == '.' || c == '_' || c == '-') {
			return fmt.Errorf("invalid script path character: %c", c)
		}
	}
	return nil
}

func shellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'"'"'`)
}

func wrapRemoteCommandWithBash(cmd string) string {
	return fmt.Sprintf("/bin/bash -lc '%s'", shellSingleQuote(cmd))
}

func rawRepoForScript(script Script) string {
	if script.IsDev {
		return RawGitHubDevRepo
	}

	return RawGitHubRepo
}

func rawScriptURL(script Script) string {
	return fmt.Sprintf("%s/%s", rawRepoForScript(script), script.ScriptPath)
}

func buildInstallScriptCommand(scriptURL string) string {
	return fmt.Sprintf("set -o pipefail && curl -fsSL %s | /bin/bash", scriptURL)
}

func scriptURLExists(script Script) (bool, error) {
	scriptURL := rawScriptURL(script)
	scriptURL, err := validateMetadataURL(scriptURL)
	if err != nil {
		return false, fmt.Errorf("invalid script URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodHead, scriptURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("User-Agent", "pvetui")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to verify script URL: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected script URL status: %s", resp.Status)
	}

	return true, nil
}

// InstallScript installs a script on a Proxmox node.
// Returns the remote exit code (0 on success) and any error encountered.
// When skipWait is true, it will not prompt/await Enter after completion.
func InstallScript(user, nodeIP string, script Script, skipWait bool) (int, error) {
	if err := validateScriptPath(script.ScriptPath); err != nil {
		return -1, err
	}

	getScriptsLogger().Debug("Installing script: %s on node %s", script.ScriptPath, nodeIP)

	// Build the script installation command using curl (matches official instructions)
	scriptURL := rawScriptURL(script)
	// Switch to root user completely and run in bash environment. On PVE, sudo
	// may not be installed; when SSHing as root we don't need elevation.
	installCmd := buildInstallScriptCommand(scriptURL)
	remoteCmd := installCmd
	if !strings.EqualFold(user, "root") {
		remoteCmd = fmt.Sprintf("if command -v sudo >/dev/null 2>&1; then sudo su - root -c '%s'; else su - root -c '%s'; fi", installCmd, installCmd)
	}
	remoteCmd = wrapRemoteCommandWithBash(remoteCmd)
	getScriptsLogger().Debug("community-script install via SSH: user=%s host=%s cmd=%s", user, nodeIP, remoteCmd)

	// Use SSH to run the script installation command interactively with proper terminal environment
	// #nosec G204 -- command arguments derive from validated node metadata and trusted plugin configuration.
	sshCmd := exec.Command("ssh", "-t", fmt.Sprintf("%s@%s", user, nodeIP), remoteCmd)

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
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Show completion status and wait for user input before returning
	if !skipWait {
		utils.WaitForEnterToReturn(err, "Script installation completed successfully!", "Script installation failed")
	}

	getScriptsLogger().Debug("Script installation completed, returning to TUI")

	if err != nil {
		return exitCode, fmt.Errorf("script installation failed: %w", err)
	}

	return exitCode, nil
}

// InstallScriptInLXC installs a script inside an existing LXC container via pct exec.
// It SSHes to the node, then runs pct exec <vmid> -- bash -c "curl ... | bash".
func InstallScriptInLXC(user, nodeIP string, vmid int, script Script, skipWait bool) (int, error) {
	if err := validateScriptPath(script.ScriptPath); err != nil {
		return -1, err
	}

	getScriptsLogger().Debug("Installing script %s in LXC %d on %s", script.ScriptPath, vmid, nodeIP)

	scriptURL := rawScriptURL(script)
	innerCmd := buildInstallScriptCommand(scriptURL)
	pctCmd := fmt.Sprintf("pct exec %d -- %s", vmid, innerCmd)
	if !strings.EqualFold(user, "root") {
		pctCmd = "sudo " + pctCmd
	}
	pctCmd = wrapRemoteCommandWithBash(pctCmd)

	// #nosec G204 -- command arguments are constructed from validated paths and vmid.
	sshCmd := exec.Command("ssh", "-t", fmt.Sprintf("%s@%s", user, nodeIP), pctCmd)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	sshCmd.Env = append(os.Environ(), "TERM=xterm-256color")

	err := sshCmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	if !skipWait {
		utils.WaitForEnterToReturn(err, "Script installation completed successfully!", "Script installation failed")
	}

	if err != nil {
		return exitCode, fmt.Errorf("script installation failed: %w", err)
	}

	return exitCode, nil
}

// ValidateConnection checks if SSH connection to the node is possible.
func ValidateConnection(user, nodeIP string) error {
	// Simple command to test SSH connection with timeout
	// Use similar SSH options as InstallScript for consistency
	// #nosec G204 -- command arguments derive from validated node metadata and trusted plugin configuration.
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
