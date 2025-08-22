package components

import "github.com/devnullvoid/peevetui/internal/ui/models"

// restoreSelection restores node and VM selections after a refresh.
func (a *App) restoreSelection(hasVM bool, vmID int, vmNode string, vmState *models.SearchState,
	hasNode bool, nodeName string, nodeState *models.SearchState,
) {
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

	if hasNode {
		for i, node := range a.nodeList.GetNodes() {
			if node != nil && node.Name == nodeName {
				a.nodeList.SetCurrentItem(i)

				if nodeState != nil {
					nodeState.SelectedIndex = i
				}

				// Manually trigger the node changed callback to update details
				if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
					a.nodeDetails.Update(selectedNode, a.client.Cluster.Nodes)
				}

				break
			}
		}
	}
}
