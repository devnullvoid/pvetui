package communityscripts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchScripts(t *testing.T) {
	scripts := []Script{
		{Name: "Nextcloud", Slug: "nextcloud", Description: "Cloud storage"},
		{Name: "Home Assistant", Slug: "homeassistant", Description: "Automation hub"},
		{Name: "Docker", Slug: "docker", Description: "Container runtime"},
	}

	matches := SearchScripts(scripts, "cloud")
	require.Len(t, matches, 1)
	require.Equal(t, "nextcloud", matches[0].Slug)

	matches = SearchScripts(scripts, "HOME")
	require.Len(t, matches, 1)
	require.Equal(t, "homeassistant", matches[0].Slug)

	matches = SearchScripts(scripts, "")
	require.Len(t, matches, 3)
}

func TestFindScript(t *testing.T) {
	scripts := []Script{
		{Name: "Nextcloud", Slug: "nextcloud"},
		{Name: "Nextcloud Backup", Slug: "nextcloud-backup"},
		{Name: "Home Assistant", Slug: "homeassistant"},
	}

	script, err := FindScript(scripts, "Home Assistant")
	require.NoError(t, err)
	require.Equal(t, "homeassistant", script.Slug)

	script, err = FindScript(scripts, "nextcloud")
	require.NoError(t, err)
	require.Equal(t, "Nextcloud", script.Name)

	_, err = FindScript(scripts, "next")
	require.ErrorContains(t, err, "ambiguous")

	_, err = FindScript(scripts, "missing")
	require.ErrorContains(t, err, "not found")
}

func TestParseEnvOverride(t *testing.T) {
	override, err := ParseEnvOverride("var_hostname=grafana")
	require.NoError(t, err)
	require.Equal(t, EnvOverride{Name: "var_hostname", Value: "grafana"}, override)

	_, err = ParseEnvOverride("var_unknown=value")
	require.ErrorContains(t, err, "unsupported")

	_, err = ParseEnvOverride("var_hostname")
	require.ErrorContains(t, err, "KEY=VALUE")

	_, err = ParseEnvOverride("var_hostname=bad\nvalue")
	require.ErrorContains(t, err, "control characters")
}

func TestBuildInstallScriptCommandWithEnv(t *testing.T) {
	cmd, err := BuildInstallScriptCommandWithEnv("https://example.invalid/script.sh", []EnvOverride{
		{Name: "var_hostname", Value: "grafana"},
		{Name: "var_tags", Value: "monitoring;grafana"},
	})
	require.NoError(t, err)
	require.Equal(t, "set -o pipefail && curl -fsSL https://example.invalid/script.sh | TERM='xterm-256color' var_hostname='grafana' var_tags='monitoring;grafana' /bin/bash", cmd)
}

func TestBuildInstallScriptCommandWithStorageDefaultsPrelude(t *testing.T) {
	cmd, err := BuildInstallScriptCommandWithEnv("https://example.invalid/script.sh", []EnvOverride{
		{Name: "var_template_storage", Value: "bigdiggus-ssd"},
		{Name: "var_container_storage", Value: "pool0"},
		{Name: "var_hostname", Value: "alpine-vlan10-test"},
	})
	require.NoError(t, err)

	require.Contains(t, cmd, "cs_defaults=/usr/local/community-scripts/default.vars")
	require.Contains(t, cmd, "cs_backup=$(mktemp)")
	require.Contains(t, cmd, "trap 'if [ -n \"$cs_backup\" ]")
	require.Contains(t, cmd, "var_template_storage=bigdiggus-ssd")
	require.Contains(t, cmd, "var_container_storage=pool0")
	require.Contains(t, cmd, "curl -fsSL https://example.invalid/script.sh")
	require.Contains(t, cmd, "var_hostname='alpine-vlan10-test'")
}

func TestBuildRemoteInstallCommandEscapesNonRootCommand(t *testing.T) {
	cmd, err := BuildRemoteInstallCommand("admin", Script{ScriptPath: "ct/grafana.sh"}, []EnvOverride{
		{Name: "var_hostname", Value: "grafana's"},
	}, "default")
	require.NoError(t, err)
	require.Contains(t, cmd, "/bin/bash -lc")
	require.Contains(t, cmd, "sudo su - root -c")
	require.Contains(t, cmd, "grafana")
	require.Contains(t, cmd, "-s --")
	require.Contains(t, cmd, "default")
}
