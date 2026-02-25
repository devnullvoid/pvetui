package components

import (
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// restoreSelection restores node and VM selections after a refresh.
func (a *App) restoreSelection(hasVM bool, vmID int, vmNode string, vmState *models.SearchState,
	hasNode bool, nodeName string, nodeState *models.SearchState,
) {
	// If user changed VM selection while refresh was in-flight, preserve the
	// user's newer selection instead of snapping back to stale pre-refresh state.
	if !shouldRestoreVMSelection(hasVM, vmID, vmNode, a.vmList.GetSelectedVM()) {
		hasVM = false
		if vmState != nil {
			vmState.SelectedIndex = a.vmList.GetCurrentItem()
		}
	}

	if hasVM {
		found := false
		for i, vm := range a.vmList.GetVMs() {
			if vm != nil && vm.ID == vmID && vm.Node == vmNode {
				a.vmList.SetCurrentItem(i)

				if vmState != nil {
					vmState.SelectedIndex = i
				}

				// Manually trigger the VM changed callback to update details
				if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
					a.vmDetails.Update(selectedVM)
				}

				found = true
				break
			}
		}

		// Fallback: if previously selected VM no longer exists (e.g., after deletion),
		// select the first available VM and update details; clear details if list is empty.
		if !found {
			vms := a.vmList.GetVMs()
			if len(vms) > 0 {
				a.vmList.SetCurrentItem(0)
				if vmState != nil {
					vmState.SelectedIndex = 0
				}
				if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
					a.vmDetails.Update(selectedVM)
				}
			} else {
				a.vmDetails.Clear()
				if vmState != nil {
					vmState.SelectedIndex = 0
				}
			}
		}
	}

	// If user changed node selection while refresh was in-flight, preserve it.
	if !shouldRestoreNodeSelection(hasNode, nodeName, a.nodeList.GetSelectedNode()) {
		hasNode = false
		if nodeState != nil {
			nodeState.SelectedIndex = a.nodeList.GetCurrentItem()
		}
	}

	if hasNode {
		for i, node := range a.nodeList.GetNodes() {
			if node != nil && node.Name == nodeName {
				a.nodeList.SetCurrentItem(i)

				if nodeState != nil {
					nodeState.SelectedIndex = i
				}

				// Manually trigger the node changed callback to update details
				if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
					a.nodeDetails.Update(selectedNode, models.GlobalState.OriginalNodes)
				}

				break
			}
		}
	}
}

func shouldRestoreVMSelection(hasVM bool, vmID int, vmNode string, current *api.VM) bool {
	if !hasVM {
		return false
	}
	if current == nil {
		return true
	}

	return current.ID == vmID && current.Node == vmNode
}

func shouldRestoreNodeSelection(hasNode bool, nodeName string, current *api.Node) bool {
	if !hasNode {
		return false
	}
	if current == nil {
		return true
	}

	return current.Name == nodeName
}
