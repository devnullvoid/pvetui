package components

import "github.com/devnullvoid/proxmox-tui/internal/ui/models"

// restoreSelection restores node and VM selections after a refresh.
func (a *App) restoreSelection(hasVM bool, vmID int, vmNode string, vmState *models.SearchState,
	hasNode bool, nodeName string, nodeState *models.SearchState,
) {
	if hasVM {
		for i, vm := range a.vmList.GetVMs() {
			if vm != nil && vm.ID == vmID && vm.Node == vmNode {
				a.vmList.SetCurrentItem(i)
				if vmState != nil {
					vmState.SelectedIndex = i
				}
				break
			}
		}
	}
	if hasNode {
		for i, node := range a.nodeList.GetNodes() {
			if node != nil && node.Name == nodeName {
				a.nodeList.SetCurrentItem(i)
				if nodeState != nil {
					nodeState.SelectedIndex = i
				}
				break
			}
		}
	}
}
