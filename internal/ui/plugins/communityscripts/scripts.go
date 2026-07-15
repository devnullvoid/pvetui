package communityscripts

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/devnullvoid/pvetui/internal/logger"
	core "github.com/devnullvoid/pvetui/internal/plugins/communityscripts"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

const (
	GitHubRepo                 = core.GitHubRepo
	GitHubAPIRepo              = core.GitHubAPIRepo
	RawGitHubRepo              = core.RawGitHubRepo
	RawGitHubDevRepo           = core.RawGitHubDevRepo
	MetadataPocketBaseBase     = core.MetadataPocketBaseBase
	MetadataPocketBaseAPI      = core.MetadataPocketBaseAPI
	ScriptMetadataTTL          = core.ScriptMetadataTTL
	ScriptListTTL              = core.ScriptListTTL
	ScriptMetadataListCacheKey = core.ScriptMetadataListCacheKey
	ScriptListCacheKey         = core.ScriptListCacheKey
	ScriptCacheKeyPrefix       = core.ScriptCacheKeyPrefix
)

type ScriptCategory = core.ScriptCategory
type Script = core.Script
type GitHubContent = core.GitHubContent

func GetScriptCategories() []ScriptCategory {
	return core.GetScriptCategories()
}

func GetScriptMetadataFiles() ([]GitHubContent, error) {
	return core.GetScriptMetadataFiles()
}

func GetScriptMetadata(metadataURL string) (*Script, error) {
	return core.GetScriptMetadata(metadataURL)
}

func FetchScripts() ([]Script, error) {
	return core.FetchScripts()
}

func GetScriptsByCategory(category string) ([]Script, error) {
	return core.GetScriptsByCategory(category)
}

func InstallScript(user, nodeIP string, script Script, skipWait bool) (int, error) {
	exitCode, err := core.InstallScript(user, nodeIP, script)
	if !skipWait {
		utils.WaitForEnterToReturn(err, "Script installation completed successfully!", "Script installation failed")
	}

	return exitCode, err
}

func InstallScriptInLXC(user, nodeIP string, vmid int, script Script, skipWait bool) (int, error) {
	exitCode, err := core.InstallScriptInLXC(user, nodeIP, vmid, script)
	if !skipWait {
		utils.WaitForEnterToReturn(err, "Script installation completed successfully!", "Script installation failed")
	}

	return exitCode, err
}

func ValidateConnection(user, nodeIP string) error {
	return core.ValidateConnection(user, nodeIP)
}

func shellSingleQuote(s string) string {
	return core.ShellSingleQuote(s)
}

func wrapRemoteCommandWithBash(cmd string) string {
	return core.WrapRemoteCommandWithBash(cmd)
}

func rawRepoForScript(script Script) string {
	return core.RawRepoForScript(script)
}

func rawScriptURL(script Script) string {
	return core.RawScriptURL(script)
}

func buildInstallScriptCommand(scriptURL string) string {
	return core.BuildInstallScriptCommand(scriptURL)
}

func scriptURLExists(script Script) (bool, error) {
	return core.ScriptURLExists(script)
}

func getScriptsLogger() interfaces.Logger {
	return logger.GetPackageLogger("scripts")
}

type pocketBaseRelation struct {
	Type string `json:"type"`
}

type pocketBaseScriptExpand struct {
	Type pocketBaseRelation `json:"type"`
}

type pocketBaseScriptRecord struct {
	Name          string                 `json:"name"`
	Slug          string                 `json:"slug"`
	Description   string                 `json:"description"`
	Categories    []string               `json:"categories"`
	Type          string                 `json:"type"`
	Updateable    bool                   `json:"updateable"`
	Privileged    bool                   `json:"privileged"`
	Port          int                    `json:"port"`
	Documentation string                 `json:"documentation"`
	Website       string                 `json:"website"`
	ConfigPath    string                 `json:"config_path"`
	Logo          string                 `json:"logo"`
	Created       string                 `json:"created"`
	IsDev         bool                   `json:"is_dev"`
	IsDisabled    bool                   `json:"is_disabled"`
	IsDeleted     bool                   `json:"is_deleted"`
	Expand        pocketBaseScriptExpand `json:"expand"`
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

func inferInstallTarget(scriptType string) string {
	switch scriptType {
	case "lxc", "vm", "turnkey":
		return "node-create"
	case "pve":
		return "node"
	case "addon":
		return "node-or-guest"
	default:
		return "unknown"
	}
}

func cleanDescription(description string) string {
	description = html.UnescapeString(description)
	description = htmlTagPattern.ReplaceAllString(description, " ")
	return strings.Join(strings.Fields(description), " ")
}

func mapPocketBaseRecord(record pocketBaseScriptRecord) Script {
	sourceType := record.Expand.Type.Type
	if sourceType == "" {
		sourceType = record.Type
	}

	return Script{
		Name:          record.Name,
		Slug:          record.Slug,
		Description:   cleanDescription(record.Description),
		Categories:    record.Categories,
		Type:          normalizeScriptType(sourceType),
		SourceType:    sourceType,
		Target:        inferInstallTarget(sourceType),
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
