package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// CreateHeader returns the application header bar
func CreateHeader() *tview.TextView {
	// Make the header more visible with a background color and make it interactive
	header := tview.NewTextView()
	header.SetTextAlign(tview.AlignCenter)
	header.SetText("Proxmox CLI UI")
	header.SetDynamicColors(true)
	header.SetBackgroundColor(tcell.ColorBlue)
	header.SetTextColor(tcell.ColorWhite)
	return header
}

// CreateFooter returns the application footer with key bindings
func CreateFooter() *tview.TextView {
	return tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText("[yellow]F1:[white]Nodes  [yellow]F2:[white]Guests  [yellow]F3:[white]Storage  [yellow]F4:[white]Network  [yellow]F5:[white]Tasks/Logs  [yellow]S:[white]Shell  [yellow]Tab:[white]Next Tab  [yellow]Q/Esc:[white]Quit")
}

// CreateVMList creates a list of VMs with their IDs and names
func CreateVMList(vms []api.VM) *tview.List {
	vmList := tview.NewList().ShowSecondaryText(false)
	vmList.SetBorder(true).SetTitle("Guests")

	for _, vm := range vms {
		text := fmt.Sprintf("%d - %s", vm.ID, vm.Name)
		vmList.AddItem(text, "", 0, nil)
	}

	return vmList
}
