package ansible

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAdhocArgs_UsesPatternModuleAndModuleArgs(t *testing.T) {
	args := buildAdhocArgs("/tmp/inventory.yml", AdhocOptions{
		Pattern:    "all",
		Module:     "shell",
		ModuleArgs: "systemctl status pvedaemon",
		Limit:      "proxmox_nodes",
		ExtraArgs:  []string{"-b", "-o"},
	})

	require.Equal(t, []string{
		"-i", "/tmp/inventory.yml",
		"all",
		"-m", "shell",
		"-a", "systemctl status pvedaemon",
		"--limit", "proxmox_nodes",
		"-b", "-o",
	}, args)
}

func TestBuildAdhocArgs_DefaultsPatternToAll(t *testing.T) {
	args := buildAdhocArgs("/tmp/inventory.ini", AdhocOptions{
		Module: "ping",
	})

	require.Equal(t, []string{
		"-i", "/tmp/inventory.ini",
		"all",
		"-m", "ping",
	}, args)
}

func TestBuildPlaybookArgs_IncludesCheckModeAndLimit(t *testing.T) {
	args := buildPlaybookArgs("/tmp/inventory.yml", "site.yml", PlaybookOptions{
		Limit:     "web",
		CheckMode: true,
		ExtraArgs: []string{"-vv"},
	})

	require.Equal(t, []string{
		"-i", "/tmp/inventory.yml",
		"site.yml",
		"--limit", "web",
		"--check",
		"-vv",
	}, args)
}

func TestWriteTempInventoryResult_UsesProxmoxInventoryExtension(t *testing.T) {
	path, cleanup, err := writeTempInventoryResult(InventoryResult{
		Source: InventorySourceProxmox,
		Format: InventoryFormatYAML,
		Text:   "plugin: community.proxmox.proxmox\n",
	})
	require.NoError(t, err)
	defer cleanup()

	require.Equal(t, ".yml", filepath.Ext(path))
	require.Contains(t, filepath.Base(path), ".proxmox.")
}
