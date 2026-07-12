package communityscripts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/devnullvoid/pvetui/internal/cache"
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/internal/ssh"
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

// InstallOptions describes a community script installation over SSH.
type InstallOptions struct {
	User           string
	Host           string
	Keyfile        string
	JumpHost       config.SSHJumpHost
	Script         Script
	Env            []EnvOverride
	Preset         string
	NonInteractive bool
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
}

// EnvOverride is a validated Community Scripts environment variable override.
type EnvOverride struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var allowedEnvOverrideNames = map[string]struct{}{
	"var_apt_cacher": {}, "var_apt_cacher_ip": {}, "var_brg": {}, "var_cpu": {}, "var_disk": {},
	"var_fuse": {}, "var_gateway": {}, "var_github_token": {}, "var_gpu": {}, "var_hostname": {},
	"var_http_no_proxy": {}, "var_http_proxy": {}, "var_ipv6_method": {}, "var_keyctl": {},
	"var_mac": {}, "var_mknod": {}, "var_mount_fs": {}, "var_mtu": {}, "var_net": {},
	"var_nesting": {}, "var_ns": {}, "var_os": {}, "var_post_install": {}, "var_protection": {},
	"var_pw": {}, "var_ram": {}, "var_sdn_vnet": {}, "var_searchdomain": {}, "var_ssh": {},
	"var_ssh_authorized_key": {}, "var_tags": {}, "var_template_storage": {}, "var_timezone": {},
	"var_tun": {}, "var_unprivileged": {}, "var_verbose": {}, "var_version": {}, "var_vlan": {},
	"var_container_storage": {},
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

// SearchScripts returns scripts whose name, slug, or description contains query.
func SearchScripts(scripts []Script, query string) []Script {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]Script(nil), scripts...)
	}

	matches := make([]Script, 0)
	for _, script := range scripts {
		if strings.Contains(strings.ToLower(script.Name), query) ||
			strings.Contains(strings.ToLower(script.Slug), query) ||
			strings.Contains(strings.ToLower(script.Description), query) {
			matches = append(matches, script)
		}
	}

	return matches
}

// FindScript finds a script by exact slug or case-insensitive name.
func FindScript(scripts []Script, nameOrSlug string) (Script, error) {
	target := strings.ToLower(strings.TrimSpace(nameOrSlug))
	if target == "" {
		return Script{}, fmt.Errorf("script name or slug is required")
	}

	for _, script := range scripts {
		if strings.ToLower(script.Slug) == target || strings.ToLower(script.Name) == target {
			return script, nil
		}
	}

	matches := SearchScripts(scripts, target)
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		labels := make([]string, 0, len(matches))
		for _, match := range matches {
			labels = append(labels, match.Slug)
		}

		return Script{}, fmt.Errorf("script %q is ambiguous; matches: %s", nameOrSlug, strings.Join(labels, ", "))
	}

	return Script{}, fmt.Errorf("script %q not found", nameOrSlug)
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

// ParseEnvOverride parses and validates a KEY=VALUE Community Scripts override.
func ParseEnvOverride(raw string) (EnvOverride, error) {
	name, value, ok := strings.Cut(raw, "=")
	if !ok {
		return EnvOverride{}, fmt.Errorf("override %q must use KEY=VALUE format", raw)
	}

	override := EnvOverride{
		Name:  strings.TrimSpace(name),
		Value: value,
	}
	if err := ValidateEnvOverride(override); err != nil {
		return EnvOverride{}, err
	}

	return override, nil
}

// ValidateEnvOverride validates a Community Scripts var_* override.
func ValidateEnvOverride(override EnvOverride) error {
	if override.Name == "" {
		return fmt.Errorf("override name is required")
	}
	if _, ok := allowedEnvOverrideNames[override.Name]; !ok {
		return fmt.Errorf("unsupported community script override %q", override.Name)
	}
	if strings.ContainsAny(override.Value, "\x00\r\n") {
		return fmt.Errorf("override %q contains unsupported control characters", override.Name)
	}

	return nil
}

// IsSensitiveEnvOverride reports whether an override value should be redacted in output.
func IsSensitiveEnvOverride(name string) bool {
	switch name {
	case "var_pw", "var_github_token":
		return true
	default:
		return false
	}
}

// RedactEnvOverrides returns a copy with sensitive values replaced.
func RedactEnvOverrides(env []EnvOverride) []EnvOverride {
	redacted := make([]EnvOverride, 0, len(env))
	for _, override := range env {
		if IsSensitiveEnvOverride(override.Name) {
			override.Value = "[redacted]"
		}
		redacted = append(redacted, override)
	}

	return redacted
}

// AllowedEnvOverrideNames returns the supported Community Scripts var_* keys.
func AllowedEnvOverrideNames() []string {
	names := make([]string, 0, len(allowedEnvOverrideNames))
	for name := range allowedEnvOverrideNames {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// ShellSingleQuote escapes a value for inclusion inside a single-quoted shell string.
func ShellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'"'"'`)
}

// ShellQuote wraps a value in single quotes for POSIX shell usage.
func ShellQuote(s string) string {
	return "'" + ShellSingleQuote(s) + "'"
}

// WrapRemoteCommandWithBash wraps a remote command in /bin/bash -lc.
func WrapRemoteCommandWithBash(cmd string) string {
	return fmt.Sprintf("/bin/bash -lc '%s'", ShellSingleQuote(cmd))
}

func wrapRemoteCommandWithBash(cmd string) string {
	return WrapRemoteCommandWithBash(cmd)
}

// RawRepoForScript returns the raw GitHub repository URL for a script.
func RawRepoForScript(script Script) string {
	if script.IsDev {
		return RawGitHubDevRepo
	}

	return RawGitHubRepo
}

// RawScriptURL returns the direct raw shell script URL.
func RawScriptURL(script Script) string {
	return fmt.Sprintf("%s/%s", RawRepoForScript(script), script.ScriptPath)
}

func rawScriptURL(script Script) string {
	return RawScriptURL(script)
}

// BuildInstallScriptCommand builds the shell command that runs a remote script.
func BuildInstallScriptCommand(scriptURL string) string {
	cmd, _ := BuildInstallScriptCommandWithEnvAndPreset(scriptURL, nil, "")
	return cmd
}

func buildInstallScriptCommand(scriptURL string) string {
	return BuildInstallScriptCommand(scriptURL)
}

// BuildInstallScriptCommandWithEnv builds the shell command that runs a remote
// script with Community Scripts var_* environment overrides.
func BuildInstallScriptCommandWithEnv(scriptURL string, env []EnvOverride) (string, error) {
	return BuildInstallScriptCommandWithEnvAndPreset(scriptURL, env, "")
}

// BuildInstallScriptCommandWithEnvAndPreset builds the shell command that runs
// a remote script with var_* overrides and an optional Community Scripts preset.
func BuildInstallScriptCommandWithEnvAndPreset(scriptURL string, env []EnvOverride, preset string) (string, error) {
	// Several upstream scripts print ${IP} after creation. The shared
	// description helper normally sets it, but if that helper misses the
	// assignment, set -u can make an otherwise successful install roll back.
	prefix := "TERM=" + ShellQuote("xterm-256color") + " IP='' "
	for _, override := range env {
		if err := ValidateEnvOverride(override); err != nil {
			return "", err
		}
		prefix += override.Name + "=" + ShellQuote(override.Value) + " "
	}

	bashArgs := ""
	if strings.TrimSpace(preset) != "" {
		if err := ValidateInstallPreset(preset); err != nil {
			return "", err
		}
		prefix += "DISABLE_UPDATE=" + ShellQuote("yes") + " "
		prefix += "PHS_SILENT=" + ShellQuote("1") + " "
		prefix += "mode=" + ShellQuote(preset) + " "
		bashArgs = " -s -- " + ShellQuote(preset)
	}

	prelude := buildStorageDefaultsPrelude(env)
	return fmt.Sprintf("set -o pipefail && %scurl -fsSL %s | %s/bin/bash%s", prelude, scriptURL, prefix, bashArgs), nil
}

func buildStorageDefaultsPrelude(env []EnvOverride) string {
	var templateStorage, containerStorage string
	for _, override := range env {
		switch override.Name {
		case "var_template_storage":
			templateStorage = override.Value
		case "var_container_storage":
			containerStorage = override.Value
		}
	}

	if templateStorage == "" || containerStorage == "" {
		return ""
	}

	// Upstream first-run defaults bootstrap reads this file before honoring
	// storage env vars. Seed only the storage lines needed for unattended runs,
	// then restore or remove the file when the installer process exits.
	lines := []string{
		"var_template_storage=" + templateStorage,
		"var_container_storage=" + containerStorage,
	}

	return fmt.Sprintf(
		"cs_defaults=/usr/local/community-scripts/default.vars && cs_backup= && "+
			"if [ -f \"$cs_defaults\" ]; then cs_backup=$(mktemp) && cp \"$cs_defaults\" \"$cs_backup\"; fi && "+
			"mkdir -p /usr/local/community-scripts && "+
			"touch \"$cs_defaults\" && "+
			"sed -i '/^[#[:space:]]*var_template_storage=/d;/^[#[:space:]]*var_container_storage=/d' \"$cs_defaults\" && "+
			"printf '%%s\\n' %s %s >> \"$cs_defaults\" && "+
			"trap 'if [ -n \"$cs_backup\" ]; then cp \"$cs_backup\" \"$cs_defaults\"; rm -f \"$cs_backup\"; else rm -f \"$cs_defaults\"; fi' EXIT && ",
		ShellQuote(lines[0]),
		ShellQuote(lines[1]),
	)
}

// BuildRemoteInstallCommand returns the shell command executed on the target node.
func BuildRemoteInstallCommand(user string, script Script, env []EnvOverride, preset string) (string, error) {
	return BuildRemoteInstallCommandWithMode(user, script, env, preset, false)
}

// BuildRemoteInstallCommandWithMode returns the shell command executed on the
// target node. In non-interactive mode, sudo uses -n so password prompts fail
// immediately instead of blocking automation.
func BuildRemoteInstallCommandWithMode(user string, script Script, env []EnvOverride, preset string, nonInteractive bool) (string, error) {
	if err := validateScriptPath(script.ScriptPath); err != nil {
		return "", err
	}

	installCmd, err := BuildInstallScriptCommandWithEnvAndPreset(RawScriptURL(script), env, preset)
	if err != nil {
		return "", err
	}

	remoteCmd := installCmd
	if !strings.EqualFold(user, "root") {
		quotedInstallCmd := ShellQuote(installCmd)
		sudoCmd := "sudo"
		if nonInteractive {
			sudoCmd = "sudo -n"
		}
		remoteCmd = fmt.Sprintf("if command -v sudo >/dev/null 2>&1; then %s su - root -c %s; else su - root -c %s; fi", sudoCmd, quotedInstallCmd, quotedInstallCmd)
	}

	return WrapRemoteCommandWithBash(remoteCmd), nil
}

// ValidateInstallPreset validates a Community Scripts menu preset argument.
func ValidateInstallPreset(preset string) error {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "", "1", "default", "2", "advanced", "3", "mydefaults", "userdefaults", "4", "appdefaults":
		return nil
	default:
		return fmt.Errorf("unsupported community script install preset %q", preset)
	}
}

// ScriptURLExists checks whether the raw install script exists.
func ScriptURLExists(script Script) (bool, error) {
	scriptURL := RawScriptURL(script)
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

// InstallScript installs a script on a Proxmox node via SSH.
// Returns the remote exit code (0 on success) and any error encountered.
func InstallScript(user, nodeIP string, script Script) (int, error) {
	return InstallScriptWithOptions(context.Background(), InstallOptions{
		User:   user,
		Host:   nodeIP,
		Script: script,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}

// InstallScriptWithOptions installs a script on a Proxmox node via SSH.
func InstallScriptWithOptions(ctx context.Context, opts InstallOptions) (int, error) {
	if opts.User == "" {
		return -1, fmt.Errorf("SSH user is required")
	}
	if opts.Host == "" {
		return -1, fmt.Errorf("SSH host is required")
	}
	if opts.Stdin == nil {
		if opts.NonInteractive {
			opts.Stdin = strings.NewReader("")
		} else {
			opts.Stdin = os.Stdin
		}
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	preset := opts.Preset
	if opts.NonInteractive && strings.TrimSpace(preset) == "" {
		preset = "default"
	}

	remoteCmd, err := BuildRemoteInstallCommandWithMode(opts.User, opts.Script, opts.Env, preset, opts.NonInteractive)
	if err != nil {
		return -1, err
	}

	getScriptsLogger().Debug("Installing script: %s on node %s", opts.Script.ScriptPath, opts.Host)
	logRemoteCmd := remoteCmd
	if redactedCmd, err := BuildRemoteInstallCommandWithMode(opts.User, opts.Script, RedactEnvOverrides(opts.Env), preset, opts.NonInteractive); err == nil {
		logRemoteCmd = redactedCmd
	}
	getScriptsLogger().Debug("community-script install via SSH: user=%s host=%s cmd=%s", opts.User, opts.Host, logRemoteCmd)

	sshArgs := ssh.BuildSSHArgs(opts.User, opts.Host, opts.JumpHost)
	args := make([]string, 0, len(sshArgs)+4)
	if opts.Keyfile != "" {
		args = append(args, "-i", opts.Keyfile)
	}
	if opts.NonInteractive {
		args = append(args, "-T")
	} else {
		args = append(args, "-t")
	}
	args = append(args, sshArgs...)
	args = append(args, remoteCmd)

	// #nosec G204 -- command arguments derive from validated node metadata and trusted plugin configuration.
	sshCmd := exec.CommandContext(ctx, "ssh", args...)

	// Connect stdin/stdout/stderr for interactive session
	sshCmd.Stdin = opts.Stdin
	sshCmd.Stdout = opts.Stdout
	sshCmd.Stderr = opts.Stderr

	// Set environment variables for better terminal compatibility
	// Override TERM to xterm-256color for better compatibility with remote systems
	// This fixes issues with terminals like Kitty (xterm-kitty) that aren't recognized on all systems
	sshCmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Run the command interactively
	err = sshCmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	getScriptsLogger().Debug("Script installation completed")

	if err != nil {
		return exitCode, fmt.Errorf("script installation failed: %w", err)
	}

	return exitCode, nil
}

// InstallScriptInLXC installs a script inside an existing LXC container via pct exec.
// It SSHes to the node, then runs pct exec <vmid> -- bash -c "curl ... | bash".
func InstallScriptInLXC(user, nodeIP string, vmid int, script Script) (int, error) {
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
