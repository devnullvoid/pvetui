package plugins

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/plugins/communityscripts"
	"github.com/devnullvoid/pvetui/internal/ui/plugins/guestlist"
)

func TestEnabledFromConfig_Default(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	plugins, missing := EnabledFromConfig(cfg)

	require.Empty(t, missing)
	require.Empty(t, plugins)
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
	require.Contains(t, ids, guestlist.PluginID)
}

func TestAvailableMetadata(t *testing.T) {
	infos := AvailableMetadata()
	require.GreaterOrEqual(t, len(infos), 2)

	var prevName string
	var foundCommunity, foundGuest bool

	for _, info := range infos {
		require.NotEmpty(t, info.ID)
		require.NotEmpty(t, info.Name)
		require.NotEmpty(t, info.Description)

		if prevName != "" {
			require.LessOrEqual(t, prevName, info.Name)
		}

		if info.ID == communityscripts.PluginID {
			foundCommunity = true
		}
		if info.ID == guestlist.PluginID {
			foundGuest = true
		}

		prevName = info.Name
	}

	require.True(t, foundCommunity)
	require.True(t, foundGuest)
}
