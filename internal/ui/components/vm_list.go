package components

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// VMList encapsulates the VM list panel.
type VMList struct {
	*tview.List

	vms       []*api.VM
	onSelect  func(*api.VM)
	onChanged func(*api.VM)
	app       *App
	// suppressCallbacks prevents onChanged from firing during programmatic updates
	suppressCallbacks bool
}

var _ VMListComponent = (*VMList)(nil)

// NewVMList creates a new VM list component.
func NewVMList() *VMList {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Guests ")
	list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary))

	return &VMList{
		List: list,
		vms:  nil,
	}
}

// SetCurrentItem wraps the list method to match the interface.
func (vl *VMList) SetCurrentItem(index int) *tview.List {
	return vl.List.SetCurrentItem(index)
}

// SetApp sets the parent app reference for focus management.
func (vl *VMList) SetApp(app *App) {
	vl.app = app

	// Set up input capture for arrow keys and VI-like navigation (hjkl)
	vl.SetInputCapture(createNavigationInputCapture(vl.app, nil, vl.app.vmDetails))
}

// SetVMs updates the list with the provided VMs.
func (vl *VMList) SetVMs(vms []*api.VM) {
	// Preserve previously selected VM to restore selection after rebuilding
	var prevID int
	var prevNode string
	if sel := vl.GetSelectedVM(); sel != nil {
		prevID = sel.ID
		prevNode = sel.Node
	} else {
		prevID = -1
		prevNode = ""
	}

	vl.suppressCallbacks = true
	vl.Clear()
	vl.vms = vms

	// Sort VMs: running VMs first, then stopped VMs
	sortedVMs := make([]*api.VM, len(vms))
	copy(sortedVMs, vms)

	sort.Slice(sortedVMs, func(i, j int) bool {
		// Running VMs come first
		if sortedVMs[i].Status == api.VMStatusRunning && sortedVMs[j].Status != api.VMStatusRunning {
			return true
		}

		if sortedVMs[i].Status != api.VMStatusRunning && sortedVMs[j].Status == api.VMStatusRunning {
			return false
		}

		// Within the same status group, sort by ID
		return sortedVMs[i].ID < sortedVMs[j].ID
	})

	// Update the internal vms slice to match the sorted order
	vl.vms = sortedVMs

	for _, vm := range sortedVMs {
		if vm != nil {
			// Check if this VM has a pending operation
			isPending, operation := models.GlobalState.IsVMPending(vm)

			// Get the status indicator with pending state awareness
			statusIndicator := utils.FormatPendingStatusIndicator(vm.Status, isPending, operation)

			// Format the VM name with ID
			vmText := fmt.Sprintf("%d - %s", vm.ID, vm.Name)

			// Apply color formatting and pending state
			var mainText string
			if isPending {
				// For pending VMs, apply a dimmed effect to the entire item
				mainText = statusIndicator + fmt.Sprintf("[secondary]%s[-]", vmText)
			} else if vm.Status != api.VMStatusRunning {
				// For stopped VMs, use gray color for the VM text part only
				// Keep the red status indicator but make the text gray
				mainText = statusIndicator + fmt.Sprintf("[secondary]%s[-]", vmText)
			} else {
				// For running VMs, use normal formatting
				mainText = statusIndicator + vmText
			}

			// Store node info in secondary text (not visible but used for search functionality)
			secondaryText := fmt.Sprintf("Node: %s Type: %s", vm.Node, vm.Type)

			vl.AddItem(mainText, secondaryText, 0, nil)
		}
	}

	// Restore selection to previously selected VM if present
	restoreIdx := -1
	if prevID >= 0 {
		for i, vm := range sortedVMs {
			if vm != nil && vm.ID == prevID && vm.Node == prevNode {
				restoreIdx = i
				break
			}
		}
	}
	if restoreIdx == -1 && len(sortedVMs) > 0 {
		restoreIdx = 0
	}
	if restoreIdx >= 0 {
		vl.List.SetCurrentItem(restoreIdx)
	}
	vl.suppressCallbacks = false
}

// GetSelectedVM returns the currently selected VM.
func (vl *VMList) GetSelectedVM() *api.VM {
	idx := vl.GetCurrentItem()
	if idx >= 0 && idx < len(vl.vms) {
		return vl.vms[idx]
	}

	return nil
}

// GetVMs returns the internal sorted VMs slice.
func (vl *VMList) GetVMs() []*api.VM {
	return vl.vms
}

// SetVMSelectedFunc sets the function to be called when a VM is selected.
func (vl *VMList) SetVMSelectedFunc(handler func(*api.VM)) {
	vl.onSelect = handler

	vl.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(vl.vms) {
			if vl.onSelect != nil {
				vl.onSelect(vl.vms[index])
			}
		}
	})
}

// SetVMChangedFunc sets the function to be called when selection changes.
func (vl *VMList) SetVMChangedFunc(handler func(*api.VM)) {
	vl.onChanged = handler

	vl.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if vl.suppressCallbacks {
			return
		}
		if index >= 0 && index < len(vl.vms) {
			if vl.onChanged != nil {
				vl.onChanged(vl.vms[index])
			}
		}
	})
}
