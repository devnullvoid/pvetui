package components

import (
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

func (a *App) showStorageBrowser(node *api.Node) {
	if a.storageBrowser != nil {
		a.storageBrowser.SetNodes(append([]*api.Node(nil), models.GlobalState.OriginalNodes...))
		if node != nil {
			a.storageBrowser.SelectNode(node)
			return
		}
	}

	a.pages.SwitchToPage(api.PageStorage)
	if browser, ok := a.storageBrowser.(*StorageBrowser); ok {
		a.SetFocus(browser.tree)
	}
}
