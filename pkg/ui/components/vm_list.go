package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// VMList encapsulates the VM list panel
type VMList struct {
	*tview.List
	vms       []*api.VM
	onSelect  func(*api.VM)
	onChanged func(*api.VM)
	app       *App
}

// NewVMList creates a new VM list component
func NewVMList() *VMList {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle("Guests")

	return &VMList{
		List: list,
		vms:  nil,
	}
}

// SetApp sets the parent app reference for focus management
func (vl *VMList) SetApp(app *App) {
	vl.app = app

	// Set up input capture for arrow keys
	vl.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRight:
			if vl.app != nil {
				vl.app.SetFocus(vl.app.vmDetails)
				return nil
			}
		}
		return event
	})
}

// SetVMs updates the list with the provided VMs
func (vl *VMList) SetVMs(vms []*api.VM) {
	vl.Clear()
	vl.vms = vms

	for _, vm := range vms {
		if vm != nil {
			// Format the VM name with ID and status indicator
			mainText := utils.FormatStatusIndicator(vm.Status) + fmt.Sprintf("%d - %s", vm.ID, vm.Name)

			// Store node info in secondary text (not visible but used for search functionality)
			secondaryText := fmt.Sprintf("Node: %s Type: %s", vm.Node, vm.Type)

			vl.AddItem(mainText, secondaryText, 0, nil)
		}
	}

	// If there are VMs, select the first one by default
	if len(vms) > 0 {
		vl.SetCurrentItem(0)
		if vl.onSelect != nil {
			vl.onSelect(vms[0])
		}
	}
}

// GetSelectedVM returns the currently selected VM
func (vl *VMList) GetSelectedVM() *api.VM {
	idx := vl.GetCurrentItem()
	if idx >= 0 && idx < len(vl.vms) {
		return vl.vms[idx]
	}
	return nil
}

// SetVMSelectedFunc sets the function to be called when a VM is selected
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

// SetVMChangedFunc sets the function to be called when selection changes
func (vl *VMList) SetVMChangedFunc(handler func(*api.VM)) {
	vl.onChanged = handler

	vl.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(vl.vms) {
			if vl.onChanged != nil {
				vl.onChanged(vl.vms[index])
			}
		}
	})
}
