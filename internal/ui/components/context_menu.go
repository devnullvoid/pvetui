package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// ContextMenu represents a popup menu with actions for a selected item
type ContextMenu struct {
	list      *tview.List
	app       *App
	onAction  func(index int, action string)
	menuItems []string
	title     string
}

// NewContextMenu creates a new context menu component
func NewContextMenu(title string, actions []string, onAction func(index int, action string)) *ContextMenu {
	return &ContextMenu{
		menuItems: actions,
		title:     title,
		onAction:  onAction,
	}
}

// SetApp sets the parent app reference
func (cm *ContextMenu) SetApp(app *App) {
	cm.app = app
}

// Show displays the context menu as a modal
func (cm *ContextMenu) Show() *tview.List {
	// Create the list with proper type
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(cm.title)

	// Add actions to the list
	for i, action := range cm.menuItems {
		list.AddItem(action, "", rune('a'+i), nil)
	}

	// Set list highlight color
	list.SetHighlightFullLine(true)
	// list.SetSelectedBackgroundColor(tcell.ColorBlue)
	// list.SetSelectedTextColor(tcell.ColorGray)

	// Set up action handler
	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if cm.app != nil {
			cm.app.CloseContextMenu()
		}
		if cm.onAction != nil {
			cm.onAction(index, mainText)
		}
	})

	// Setup input capture to close on escape and handle VI-like navigation
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape && cm.app != nil {
			cm.app.CloseContextMenu()
			return nil
		} else if event.Key() == tcell.KeyRune {
			// Handle VI-like navigation (hjkl)
			switch event.Rune() {
			case 'j': // VI-like down navigation
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k': // VI-like up navigation
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'h': // VI-like left navigation - close menu
				if cm.app != nil {
					cm.app.CloseContextMenu()
				}
				return nil
			case 'l': // VI-like right navigation - select item (same as Enter)
				index := list.GetCurrentItem()
				if index >= 0 && index < len(cm.menuItems) {
					if cm.app != nil {
						cm.app.CloseContextMenu()
					}
					if cm.onAction != nil {
						cm.onAction(index, cm.menuItems[index])
					}
				}
				return nil
			}
		}
		return event
	})

	cm.list = list
	return list
}

// ShowNodeContextMenu displays the context menu for node actions
func (a *App) ShowNodeContextMenu() {
	node := a.nodeList.GetSelectedNode()
	if node == nil {
		return
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on node state
	menuItems := []string{
		"Open Shell",
		"Open VNC Shell",
		// "View Logs",
		"Install Community Script",
		"Refresh",
	}

	// Create and show context menu
	menu := NewContextMenu(" Node Actions ", menuItems, func(index int, action string) {
		switch action {
		case "Open Shell":
			a.openNodeShell()
		case "Open VNC Shell":
			a.openNodeVNC()
		// case "View Logs":
		// 	a.showMessage("Viewing logs for node: " + node.Name)
		case "Install Community Script":
			a.openScriptSelector(node, nil)
		case "Refresh":
			a.refreshNodeData(node)
		}
	})
	menu.SetApp(a)

	// Display the menu
	menuList := menu.Show()
	a.contextMenu = menuList
	a.isMenuOpen = true

	// Create a centered modal layout
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// ShowVMContextMenu displays the context menu for VM actions
func (a *App) ShowVMContextMenu() {
	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		return
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on VM state
	menuItems := []string{
		"Open Shell",
		"Edit Configuration",
		"Refresh",
	}

	// Add VNC option for QEMU VMs and LXC containers that are running
	if (vm.Type == api.VMTypeQemu || vm.Type == api.VMTypeLXC) && vm.Status == api.VMStatusRunning {
		menuItems = append([]string{"Open VNC Console"}, menuItems...)
	}

	// Add state-dependent actions
	if vm.Status == api.VMStatusRunning {
		menuItems = append(menuItems, "Shutdown", "Restart")
	} else if vm.Status == api.VMStatusStopped {
		menuItems = append(menuItems, "Start")
	}

	// Add migrate option (always available)
	menuItems = append(menuItems, "Migrate")

	// Add delete option (always available)
	menuItems = append(menuItems, "Delete")

	// Note: Removed "Install Community Script" as it's only applicable to nodes

	// Create and show context menu
	menu := NewContextMenu(" Guest Actions ", menuItems, func(index int, action string) {
		switch action {
		case "Open VNC Console":
			a.openVMVNC()
		case "Open Shell":
			a.openVMShell()
		case "Edit Configuration":
			// Load config and show config page
			go func() {
				cfg, err := a.client.GetVMConfig(vm)
				a.QueueUpdateDraw(func() {
					if err != nil {
						a.showMessage(fmt.Sprintf("Failed to load config: %v", err))
						return
					}
					page := NewVMConfigPage(a, vm, cfg, func(newCfg *api.VMConfig) error {
						return a.client.UpdateVMConfig(vm, newCfg)
					})
					a.pages.AddPage("vmConfig", page, true, true)
					a.SetFocus(page)
				})
			}()
		case "Refresh":
			a.refreshVMData(vm)
		case "Start":
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to start VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.StartVM, "Starting")
				},
			)
		case "Shutdown":
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to shutdown VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.StopVM, "Shutting down")
				},
			)
		case "Restart":
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to restart VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.RestartVM, "Restarting")
				},
			)
		case "Migrate":
			a.showMigrationDialog(vm)
		case "Delete":
			// Check if VM is running and provide appropriate options
			if vm.Status == api.VMStatusRunning {
				a.showDeleteRunningVMDialog(vm)
			} else {
				a.showConfirmationDialog(
					fmt.Sprintf("⚠️  DANGER: Are you sure you want to permanently DELETE VM '%s' (ID: %d)?\n\nThis action is IRREVERSIBLE and will destroy all VM data including disks!", vm.Name, vm.ID),
					func() {
						a.performVMDeleteOperation(vm, false) // false = not forced
					},
				)
			}
		}
	})
	menu.SetApp(a)

	// Display the menu
	menuList := menu.Show()
	a.contextMenu = menuList
	a.isMenuOpen = true

	// Create a centered modal layout
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// CloseContextMenu closes the context menu and restores the previous focus
func (a *App) CloseContextMenu() {
	if a.isMenuOpen {
		a.pages.RemovePage("contextMenu")
		a.isMenuOpen = false
		a.contextMenu = nil
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}
}

// performVMOperation performs an asynchronous VM operation and shows status message
func (a *App) performVMOperation(vm *api.VM, operation func(*api.VM) error, operationName string) {
	// Set pending state immediately for visual feedback
	models.GlobalState.SetVMPending(vm, operationName)

	// Show visual feedback with small delay to avoid UI deadlock
	go func() {
		time.Sleep(50 * time.Millisecond) // Slightly longer delay for stability
		a.QueueUpdateDraw(func() {
			a.refreshVMList()
		})
	}()

	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("%s %s", operationName, vm.Name))

	// For restart, record the original uptime
	var originalUptime int64 = -1
	if strings.ToLower(operationName) == "restarting" {
		freshVM, err := a.client.RefreshVMData(vm, nil)
		if err == nil {
			originalUptime = freshVM.Uptime
		}
	}

	// Run operation in goroutine to avoid blocking UI
	go func() {
		defer func() {
			// Always clear pending state when we're completely done
			models.GlobalState.ClearVMPending(vm)
			// Final UI refresh
			a.QueueUpdateDraw(func() {
				a.refreshVMList()
			})
		}()

		if err := operation(vm); err != nil {
			// Update message with error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error %s %s: %v", strings.ToLower(operationName), vm.Name, err))
			})
			return // Exit early on error, defer will clear pending state
		}

		// API call succeeded, but operation is still in progress
		a.QueueUpdateDraw(func() {
			a.header.ShowLoading(fmt.Sprintf("Waiting for %s %s to complete...", strings.ToLower(operationName), vm.Name))
		})

		// Clear API cache to ensure fresh data
		a.client.ClearAPICache()

		// Wait for the actual operation to complete by monitoring VM state
		if strings.ToLower(operationName) == "restarting" {
			a.waitForVMRestartCompletion(vm, originalUptime)
		} else {
			a.waitForVMOperationCompletion(vm, operationName)
		}

		// Show final success message
		a.QueueUpdateDraw(func() {
			a.header.ShowSuccess(fmt.Sprintf("%s %s completed successfully", operationName, vm.Name))
		})

		// Do a final refresh to show the updated state
		time.Sleep(2 * time.Second) // Brief pause before final refresh
		a.QueueUpdateDraw(func() {
			a.refreshVMData(vm)
		})
	}()
}

// performVMDeleteOperation performs an asynchronous VM delete operation and refreshes the VM list
func (a *App) performVMDeleteOperation(vm *api.VM, forced bool) {
	// Set pending state immediately for visual feedback
	models.GlobalState.SetVMPending(vm, "Deleting")

	// Show visual feedback with small delay to avoid UI deadlock
	go func() {
		time.Sleep(50 * time.Millisecond) // Slightly longer delay for stability
		a.QueueUpdateDraw(func() {
			a.refreshVMList()
		})
	}()

	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Deleting %s", vm.Name))

	// Run operation in goroutine to avoid blocking UI
	go func() {
		defer func() {
			// Always clear pending state when operation completes
			models.GlobalState.ClearVMPending(vm)
			// Refresh UI once at the end
			a.QueueUpdateDraw(func() {
				a.refreshVMList()
			})
		}()

		var err error
		if forced {
			// Use force delete for running VMs
			options := &api.DeleteVMOptions{
				Force:                    true,
				DestroyUnreferencedDisks: true,
				Purge:                    true,
			}
			err = a.client.DeleteVMWithOptions(vm, options)
		} else {
			// Regular delete
			err = a.client.DeleteVM(vm)
		}

		if err != nil {
			// Update message with error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error deleting %s: %v", vm.Name, err))
			})
		} else {
			// Update message with success on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Successfully deleted %s", vm.Name))
			})

			// Clear API cache to ensure deleted VM is removed from the list
			a.client.ClearAPICache()

			// Wait a few seconds for the Proxmox server to fully process the deletion
			// before refreshing the VM list
			go func() {
				time.Sleep(5 * time.Second)
				a.QueueUpdateDraw(func() {
					a.manualRefresh()
				})
			}()
		}
	}()
}

// showDeleteRunningVMDialog shows a dialog with options for deleting a running VM
func (a *App) showDeleteRunningVMDialog(vm *api.VM) {
	message := fmt.Sprintf("⚠️  VM '%s' (ID: %d) is currently RUNNING\n\nProxmox can force delete running VMs.\n\nAre you sure you want to FORCE DELETE this running VM?\n\nThis will IMMEDIATELY DESTROY the VM and ALL its data!", vm.Name, vm.ID)

	a.showConfirmationDialog(message, func() {
		// User chose to force delete the running VM
		a.performVMDeleteOperation(vm, true) // true = forced
	})
}

// refreshVMList refreshes the VM list display without fetching new data
func (a *App) refreshVMList() {
	// Get current search state to maintain filtering
	vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

	if vmSearchState != nil && vmSearchState.Filter != "" {
		// Apply existing filter
		models.FilterVMs(vmSearchState.Filter)
		a.vmList.SetVMs(models.GlobalState.FilteredVMs)
	} else {
		// No filter, use original data
		a.vmList.SetVMs(models.GlobalState.OriginalVMs)
	}
}

// restoreNodeSelection restores node selection by name after SetNodes
func (a *App) restoreNodeSelection(nodeName string) {
	nodeList := a.nodeList.GetNodes()
	for i, node := range nodeList {
		if node != nil && node.Name == nodeName {
			a.nodeList.SetCurrentItem(i)
			// Optionally update search state
			if state := models.GlobalState.GetSearchState(api.PageNodes); state != nil {
				state.SelectedIndex = i
			}
			break
		}
	}
}

// refreshNodeList refreshes the node list display without fetching new data
func (a *App) refreshNodeList() {
	// Get current search state to maintain filtering
	nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)

	// Record the currently selected node's name
	selectedNodeName := ""
	if selected := a.nodeList.GetSelectedNode(); selected != nil {
		selectedNodeName = selected.Name
		if nodeSearchState != nil {
			// nodeSearchState.Filter = nodeSearchState.Filter // not needed
			nodeSearchState.SelectedIndex = 0 // not used for node selection anymore
		}
	}

	if nodeSearchState != nil && nodeSearchState.Filter != "" {
		// Apply existing filter
		models.FilterNodes(nodeSearchState.Filter)
		a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
	} else {
		// No filter, use original data
		a.nodeList.SetNodes(models.GlobalState.OriginalNodes)
	}

	// Restore selection by name if possible
	if selectedNodeName != "" {
		a.restoreNodeSelection(selectedNodeName)
	}
}

// refreshNodeData refreshes the data for a specific node with pending state support
func (a *App) refreshNodeData(node *api.Node) {
	// Set pending state immediately for visual feedback
	models.GlobalState.SetNodePending(node, "Refreshing")

	// Show visual feedback with small delay to avoid UI deadlock
	go func() {
		time.Sleep(50 * time.Millisecond) // Slightly longer delay for stability
		a.QueueUpdateDraw(func() {
			a.refreshNodeList()
		})
	}()

	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Refreshing node %s", node.Name))

	// Run refresh in goroutine to avoid blocking UI
	go func() {
		defer func() {
			// Always clear pending state when operation completes
			models.GlobalState.ClearNodePending(node)
			// Refresh UI once at the end
			a.QueueUpdateDraw(func() {
				a.refreshNodeList()
			})
		}()

		// Fetch fresh node data
		freshNode, err := a.client.RefreshNodeData(node.Name)
		if err != nil {
			// Update message with error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error refreshing node %s: %v", node.Name, err))
			})
			return
		}

		// Update UI with fresh data on main thread
		a.QueueUpdateDraw(func() {
			// Find the node in the global state and update it
			for i, originalNode := range models.GlobalState.OriginalNodes {
				if originalNode != nil && originalNode.Name == node.Name {
					// Update the node data while preserving VMs
					freshNode.VMs = originalNode.VMs
					models.GlobalState.OriginalNodes[i] = freshNode
					break
				}
			}

			// Update filtered nodes if they exist
			for i, filteredNode := range models.GlobalState.FilteredNodes {
				if filteredNode != nil && filteredNode.Name == node.Name {
					// Update the node data while preserving VMs
					freshNode.VMs = filteredNode.VMs
					models.GlobalState.FilteredNodes[i] = freshNode
					break
				}
			}

			// Update the node list display
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)

			// Restore selection by name
			a.restoreNodeSelection(node.Name)

			// Update node details if this node is currently selected
			selectedNode := a.nodeList.GetSelectedNode()
			if selectedNode != nil && selectedNode.Name == node.Name {
				a.nodeDetails.Update(freshNode, models.GlobalState.OriginalNodes)
			}

			// Show success message
			a.header.ShowSuccess(fmt.Sprintf("Node %s refreshed successfully", node.Name))
		})
	}()
}

// waitForVMOperationCompletion waits for a VM operation to actually complete by monitoring status
func (a *App) waitForVMOperationCompletion(vm *api.VM, operationName string) {
	// Define expected final states for each operation
	var targetStatus string
	var intermediateCheck func(*api.VM) bool

	switch strings.ToLower(operationName) {
	case "starting":
		targetStatus = api.VMStatusRunning
	case "shutting down":
		targetStatus = api.VMStatusStopped
	case "restarting":
		// For restart, we need to see it go to stopped then back to running
		targetStatus = api.VMStatusRunning
		intermediateCheck = func(currentVM *api.VM) bool {
			// We've seen it stop, now wait for it to start
			return currentVM.Status == api.VMStatusStopped
		}
	default:
		// For unknown operations, just wait a bit
		time.Sleep(10 * time.Second)
		return
	}

	maxWaitTime := 5 * time.Minute   // Maximum time to wait
	checkInterval := 3 * time.Second // How often to check
	startTime := time.Now()
	hasSeenIntermediate := false

	for time.Since(startTime) < maxWaitTime {
		// Get fresh VM data
		freshVM, err := a.client.RefreshVMData(vm, nil)
		if err != nil {
			// If we can't get VM data, wait a bit longer
			time.Sleep(checkInterval)
			continue
		}

		// For restart operations, check intermediate state first
		if intermediateCheck != nil && !hasSeenIntermediate {
			if intermediateCheck(freshVM) {
				hasSeenIntermediate = true
				// Update UI to show we've seen the intermediate state
				a.QueueUpdateDraw(func() {
					a.header.ShowLoading(fmt.Sprintf("VM %s stopped, waiting for startup...", vm.Name))
				})
			}
		}

		// Check if we've reached the target state
		if freshVM.Status == targetStatus {
			// For restart, make sure we've seen the intermediate state
			if intermediateCheck == nil || hasSeenIntermediate {
				return // Operation completed!
			}
		}

		// Wait before next check
		time.Sleep(checkInterval)
	}

	// If we reach here, operation timed out - but that's okay, just proceed
}

// waitForVMRestartCompletion waits for a VM restart by monitoring uptime reset
func (a *App) waitForVMRestartCompletion(vm *api.VM, originalUptime int64) {
	const fudgeSeconds int64 = 10
	maxWaitTime := 2 * time.Minute
	checkInterval := 3 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		freshVM, err := a.client.RefreshVMData(vm, nil)
		if err != nil {
			time.Sleep(checkInterval)
			continue
		}
		if freshVM.Status == api.VMStatusRunning {
			if originalUptime > 0 && freshVM.Uptime > 0 && freshVM.Uptime < (originalUptime-fudgeSeconds) {
				// Uptime has reset, restart complete
				return
			}
		}
		time.Sleep(checkInterval)
	}
	// If we reach here, just proceed (timeout)
}
