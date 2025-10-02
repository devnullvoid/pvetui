package plugins

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/plugins/communityscripts"
)

func TestEnabledFromConfig_Default(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	plugins, missing := EnabledFromConfig(cfg)

	require.Empty(t, missing)
	require.Len(t, plugins, 1)
	require.Equal(t, communityscripts.PluginID, plugins[0].ID())
}

func TestEnabledFromConfig_CustomList(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	cfg.Plugins.Enabled = []string{"unknown", communityscripts.PluginID, communityscripts.PluginID}

	plugins, missing := EnabledFromConfig(cfg)

	require.Equal(t, []string{"unknown"}, missing)
	require.Len(t, plugins, 1)
	require.Equal(t, communityscripts.PluginID, plugins[0].ID())
}

func TestAvailableIDs(t *testing.T) {
	ids := AvailableIDs()
	require.Contains(t, ids, communityscripts.PluginID)
}
