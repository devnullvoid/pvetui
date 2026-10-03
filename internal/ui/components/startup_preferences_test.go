package components

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestStartupPage(t *testing.T) {
	for setting, page := range map[string]string{
		"": api.PageNodes, "nodes": api.PageNodes, "guests": api.PageGuests,
		"tasks": api.PageTasks, "storage": api.PageStorage,
	} {
		t.Run(setting, func(t *testing.T) {
			a := &App{
				config: config.Config{StartupPage: setting},
				pages:  tview.NewPages(),
				header: NewHeader(), footer: NewFooter(),
				clusterStatus: NewClusterStatus(),
				nodeList:      NewNodeList(), nodeDetails: NewNodeDetails(),
				vmList: NewVMList(), vmDetails: NewVMDetails(),
				tasksList: NewTasksList(), storageBrowser: NewStorageBrowser(),
			}
			a.createMainLayout()
			name, _ := a.pages.GetFrontPage()
			require.Equal(t, page, name)
		})
	}
}

func TestRefreshInterval(t *testing.T) {
	a := &App{config: config.Config{AutoRefresh: config.AutoRefreshConfig{Interval: 30}}}
	require.Equal(t, 30, a.refreshInterval())
	require.Contains(t, buildHelpText(config.DefaultKeyBindings(), a.refreshInterval()), "30s interval")
	a.config.AutoRefresh.Interval = 0
	require.Equal(t, 10, a.refreshInterval())
}
