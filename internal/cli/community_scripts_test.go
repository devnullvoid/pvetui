package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/internal/config"
	core "github.com/devnullvoid/pvetui/internal/plugins/communityscripts"
	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestEnsureCommunityScriptsEnabled(t *testing.T) {
	session := &cliSession{cfg: &config.Config{}}
	require.ErrorContains(t, ensureCommunityScriptsEnabled(session), "not enabled")

	session.cfg.Plugins.Enabled = []string{"ansible", communityScriptsPluginID}
	require.NoError(t, ensureCommunityScriptsEnabled(session))
}

func TestCommunityScriptToOutput(t *testing.T) {
	out := communityScriptToOutput(core.Script{
		Name:          "AFFiNE",
		Slug:          "affine",
		Description:   "Docs",
		Categories:    []string{"productivity"},
		Type:          "ct",
		ScriptPath:    "ct/affine.sh",
		Website:       "https://affine.pro",
		Documentation: "https://docs.affine.pro",
		ConfigPath:    "/opt/affine/.env",
		InterfacePort: 3010,
		Updateable:    true,
		IsDev:         true,
	})

	require.Equal(t, "AFFiNE", out.Name)
	require.Equal(t, "affine", out.Slug)
	require.Equal(t, "community-scripts/ProxmoxVED", out.SourceRepo)
	require.Equal(t, core.RawGitHubDevRepo+"/ct/affine.sh", out.ScriptURL)
	require.Equal(t, []string{"productivity"}, out.Categories)
	require.True(t, out.Updateable)
}

func TestRedactCommunityScriptEnv(t *testing.T) {
	env := redactCommunityScriptEnv([]core.EnvOverride{
		{Name: "var_hostname", Value: "grafana"},
		{Name: "var_pw", Value: "secret"},
		{Name: "var_github_token", Value: "ghp_secret"},
	})

	require.Equal(t, "grafana", env[0].Value)
	require.Equal(t, "[redacted]", env[1].Value)
	require.Equal(t, "[redacted]", env[2].Value)
}

func TestDetectCreatedGuests(t *testing.T) {
	before := []*api.VM{
		{ID: 100, Name: "old", Node: "pve01", Type: api.VMTypeLXC, SourceProfile: "pve01"},
		{ID: 102, Name: "existing-duplicate-id", Node: "pve01", Type: api.VMTypeLXC, SourceProfile: "pve01"},
	}
	after := []*api.VM{
		{ID: 100, Name: "old", Node: "pve01", Type: api.VMTypeLXC, SourceProfile: "pve01"},
		{ID: 102, Name: "existing-duplicate-id", Node: "pve01", Type: api.VMTypeLXC, SourceProfile: "pve01"},
		{ID: 102, Name: "prometheus", Node: "pve02", Type: api.VMTypeLXC, Status: api.VMStatusRunning, SourceProfile: "pve02"},
		{ID: 103, Name: "other-node", Node: "pve03", Type: api.VMTypeLXC, SourceProfile: "pve03"},
	}

	created := detectCreatedGuests(before, after, "pve02")
	require.Len(t, created, 1)
	require.Equal(t, 102, created[0].ID)
	require.Equal(t, "prometheus", created[0].Name)
}
