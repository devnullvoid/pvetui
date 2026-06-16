package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/internal/config"
	core "github.com/devnullvoid/pvetui/internal/plugins/communityscripts"
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
