package components

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type vmCreateNodeChoice struct {
	label string
	node  *api.Node
}

type vmCreateNodeData struct {
	nextID       int
	diskStorages []string
	isoStorages  []string
	isoByStorage map[string][]string
}

const vmCreateNoneOption = "None"

const (
	vmCreateSourceBlank = "Blank Disk"
	vmCreateSourceISO   = "ISO Installer"
)

func (a *App) showVMCreateForm(initialNode *api.Node) {
	choices := a.vmCreateNodeChoices()
	if len(choices) == 0 {
		a.showMessageSafe("No nodes available for VM creation")
		return
	}

	nodes := make([]*api.Node, len(choices))
	for i, choice := range choices {
		nodes[i] = choice.node
	}
	selectedIndex := initialCreateNodeIndex(initialNode, nodes)
	loadGuestCreateData(
		a,
		"Loading VM creation options...",
		choices[selectedIndex].node,
		a.loadVMCreateNodeData,
		func(data vmCreateNodeData) { a.displayVMCreateForm(choices, selectedIndex, data) },
		"Failed to load VM creation options",
	)
}

func (a *App) displayVMCreateForm(choices []vmCreateNodeChoice, selectedIndex int, initialData vmCreateNodeData) {
	if len(choices) == 0 {
		a.showMessageSafe("No nodes available for VM creation")
		return
	}

	form := newStandardForm()
	form.SetBorder(true)
	form.SetTitle(" Create VM ")

	nodeOptions := make([]string, len(choices))
	for i, choice := range choices {
		nodeOptions[i] = choice.label
	}

	nodeDropdown := tview.NewDropDown().
		SetLabel("Node").
		SetOptions(nodeOptions, nil).
		SetCurrentOption(selectedIndex).
		SetFieldWidth(28)

	vmidField := tview.NewInputField().
		SetLabel("VM ID").
		SetFieldWidth(12).
		SetText(strconv.Itoa(initialData.nextID))
	nameField := tview.NewInputField().
		SetLabel("Name").
		SetFieldWidth(28)
	memoryField := tview.NewInputField().
		SetLabel("Memory (MB)").
		SetFieldWidth(12).
		SetText("2048")
	coresField := tview.NewInputField().
		SetLabel("Cores").
		SetFieldWidth(8).
		SetText("2")
	sourceDropdown := tview.NewDropDown().
		SetLabel("Source").
		SetOptions([]string{vmCreateSourceBlank, vmCreateSourceISO}, nil).
		SetCurrentOption(1).
		SetFieldWidth(18)
	diskStorageDropdown := tview.NewDropDown().
		SetLabel("Disk Storage").
		SetFieldWidth(24)
	diskSizeField := tview.NewInputField().
		SetLabel("Disk Size (GB)").
		SetFieldWidth(12).
		SetText("20")
	isoStorageDropdown := tview.NewDropDown().
		SetLabel("ISO Storage").
		SetFieldWidth(24)
	isoVolumeDropdown := tview.NewDropDown().
		SetLabel("ISO Image").
		SetFieldWidth(40)
	bridgeField := tview.NewInputField().
		SetLabel("Bridge").
		SetFieldWidth(16).
		SetText("vmbr0")
	startCheckbox := tview.NewCheckbox().
		SetLabel("Start After Create").
		SetChecked(true)

	form.AddFormItem(nodeDropdown)
	form.AddFormItem(vmidField)
	form.AddFormItem(nameField)
	form.AddFormItem(memoryField)
	form.AddFormItem(coresField)
	form.AddFormItem(sourceDropdown)
	form.AddFormItem(diskStorageDropdown)
	form.AddFormItem(diskSizeField)
	form.AddFormItem(isoStorageDropdown)
	form.AddFormItem(isoVolumeDropdown)
	form.AddFormItem(bridgeField)
	form.AddFormItem(startCheckbox)

	currentData := initialData
	updateISOImages := func(storage string) {
		volumes := append([]string{vmCreateNoneOption}, currentData.isoByStorage[storage]...)
		if storage == "" || len(currentData.isoByStorage[storage]) == 0 {
			volumes = []string{vmCreateNoneOption}
		}
		isoVolumeDropdown.SetOptions(volumes, nil)
		isoVolumeDropdown.SetCurrentOption(0)
	}
	applyNodeData := func(data vmCreateNodeData) {
		currentData = data
		vmidField.SetText(strconv.Itoa(data.nextID))

		diskStorages := append([]string(nil), data.diskStorages...)
		if len(diskStorages) == 0 {
			diskStorages = []string{"No disk storages"}
		}
		diskStorageDropdown.SetOptions(diskStorages, nil)
		diskStorageDropdown.SetCurrentOption(0)

		isoStorages := append([]string{vmCreateNoneOption}, data.isoStorages...)
		isoStorageDropdown.SetOptions(isoStorages, nil)
		if len(data.isoStorages) > 0 {
			isoStorageDropdown.SetCurrentOption(1)
			updateISOImages(data.isoStorages[0])
		} else {
			isoStorageDropdown.SetCurrentOption(0)
			updateISOImages("")
		}

	}
	applyNodeData(initialData)

	nodeDropdown.SetSelectedFunc(func(option string, index int) {
		if index < 0 || index >= len(choices) {
			return
		}

		loadGuestCreateData(
			a,
			fmt.Sprintf("Loading VM options for %s...", choices[index].label),
			choices[index].node,
			a.loadVMCreateNodeData,
			applyNodeData,
			"Failed to load node options",
		)
	})

	isoStorageDropdown.SetSelectedFunc(func(option string, index int) {
		if option == vmCreateNoneOption {
			updateISOImages("")
			return
		}
		updateISOImages(option)
	})
	pageName := "modal:vmCreate"
	closeForm := func() {
		a.removePageIfPresent(pageName)
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	form.AddButton("Create", func() {
		nodeIndex, _ := nodeDropdown.GetCurrentOption()
		if nodeIndex < 0 || nodeIndex >= len(choices) {
			a.showMessageSafe("Select a node")
			return
		}
		node := choices[nodeIndex].node

		vmid, err := strconv.Atoi(strings.TrimSpace(vmidField.GetText()))
		if err != nil || vmid <= 0 {
			a.showMessageSafe("Enter a valid positive VM ID")
			return
		}
		name := strings.TrimSpace(nameField.GetText())
		if name == "" {
			a.showMessageSafe("VM name is required")
			return
		}
		memoryMB, err := strconv.Atoi(strings.TrimSpace(memoryField.GetText()))
		if err != nil || memoryMB <= 0 {
			a.showMessageSafe("Enter a valid positive memory value")
			return
		}
		cores, err := strconv.Atoi(strings.TrimSpace(coresField.GetText()))
		if err != nil || cores <= 0 {
			a.showMessageSafe("Enter a valid positive core count")
			return
		}
		diskStorageIndex, _ := diskStorageDropdown.GetCurrentOption()
		if diskStorageIndex < 0 || diskStorageIndex >= len(currentData.diskStorages) {
			a.showMessageSafe("Select a valid disk storage")
			return
		}
		_, sourceType := sourceDropdown.GetCurrentOption()
		diskSizeGB, err := strconv.Atoi(strings.TrimSpace(diskSizeField.GetText()))
		if err != nil || diskSizeGB <= 0 {
			a.showMessageSafe("Enter a valid positive disk size")
			return
		}
		isoStorageIndex, isoStorage := isoStorageDropdown.GetCurrentOption()
		_ = isoStorageIndex
		isoVolumeIndex, isoVolume := isoVolumeDropdown.GetCurrentOption()
		_ = isoVolumeIndex
		if sourceType != vmCreateSourceISO || isoStorage == vmCreateNoneOption || isoVolume == vmCreateNoneOption {
			isoVolume = ""
		}
		bridge := strings.TrimSpace(bridgeField.GetText())
		if bridge == "" {
			a.showMessageSafe("Bridge is required")
			return
		}

		options := api.VMCreateOptions{
			VMID:        vmid,
			Name:        name,
			MemoryMB:    memoryMB,
			Cores:       cores,
			Sockets:     1,
			DiskStorage: currentData.diskStorages[diskStorageIndex],
			DiskSizeGB:  diskSizeGB,
			ISOVolume:   isoVolume,
			Bridge:      bridge,
			Start:       startCheckbox.IsChecked(),
		}

		closeForm()
		a.enqueueVMCreate(node, options)
	})
	form.AddButton("Cancel", closeForm)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeForm()
			return nil
		}
		return event
	})

	a.pages.AddPage(pageName, form, true, true)
	a.SetFocus(form)
}

func (a *App) enqueueVMCreate(node *api.Node, options api.VMCreateOptions) {
	a.enqueueGuestCreateTask(
		node,
		fmt.Sprintf("Create VM %s (%d)", options.Name, options.VMID),
		options.Name,
		options.VMID,
		fmt.Sprintf("Created VM %s", options.Name),
		"VM create failed",
		func(node *api.Node) (string, error) {
			client, err := a.getClientForNode(node)
			if err != nil {
				return "", err
			}
			return client.CreateVM(node.Name, options)
		},
	)
}

func (a *App) vmCreateNodeChoices() []vmCreateNodeChoice {
	nodes := append([]*api.Node(nil), models.GlobalState.OriginalNodes...)
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name == nodes[j].Name {
			return nodes[i].SourceProfile < nodes[j].SourceProfile
		}
		return nodes[i].Name < nodes[j].Name
	})

	choices := make([]vmCreateNodeChoice, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || !node.Online {
			continue
		}
		label := node.Name
		if node.SourceProfile != "" {
			label += fmt.Sprintf(" (%s)", node.SourceProfile)
		}
		choices = append(choices, vmCreateNodeChoice{label: label, node: node})
	}
	return choices
}

func (a *App) loadVMCreateNodeData(node *api.Node) (vmCreateNodeData, error) {
	nextID, storages, err := a.loadCreateNodeStorages(node)
	if err != nil {
		return vmCreateNodeData{}, err
	}

	data := vmCreateNodeData{
		nextID:       nextID,
		diskStorages: make([]string, 0),
		isoStorages:  make([]string, 0),
		isoByStorage: make(map[string][]string),
	}

	for _, storage := range storages {
		if storage == nil {
			continue
		}
		if strings.Contains(storage.Content, "images") {
			data.diskStorages = append(data.diskStorages, storage.Name)
		}
	}

	sort.Strings(data.diskStorages)
	data.isoStorages, data.isoByStorage, err = a.collectNodeStorageContent(node, storages, storageSupportsToken("iso"), "iso")
	if err != nil {
		return vmCreateNodeData{}, err
	}
	return data, nil
}
