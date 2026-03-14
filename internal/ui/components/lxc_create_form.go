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

type lxcCreateNodeChoice struct {
	label string
	node  *api.Node
}

type lxcCreateNodeData struct {
	nextID             int
	rootFSStorages     []string
	templateStorages   []string
	templatesByStorage map[string][]string
}

func (a *App) showLXCCreateForm(initialNode *api.Node) {
	choices := a.lxcCreateNodeChoices()
	if len(choices) == 0 {
		a.showMessageSafe("No nodes available for LXC creation")
		return
	}

	nodes := make([]*api.Node, len(choices))
	for i, choice := range choices {
		nodes[i] = choice.node
	}
	selectedIndex := initialCreateNodeIndex(initialNode, nodes)
	loadGuestCreateData(
		a,
		"Loading LXC creation options...",
		choices[selectedIndex].node,
		a.loadLXCCreateNodeData,
		func(data lxcCreateNodeData) { a.displayLXCCreateForm(choices, selectedIndex, data) },
		"Failed to load LXC creation options",
	)
}

func (a *App) displayLXCCreateForm(choices []lxcCreateNodeChoice, selectedIndex int, initialData lxcCreateNodeData) {
	if len(choices) == 0 {
		a.showMessageSafe("No nodes available for LXC creation")
		return
	}

	form := newStandardForm()
	form.SetBorder(true)
	form.SetTitle(" Create LXC ")

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
		SetLabel("CT ID").
		SetFieldWidth(12).
		SetText(strconv.Itoa(initialData.nextID))
	hostnameField := tview.NewInputField().
		SetLabel("Hostname").
		SetFieldWidth(28)
	memoryField := tview.NewInputField().
		SetLabel("Memory (MB)").
		SetFieldWidth(12).
		SetText("512")
	swapField := tview.NewInputField().
		SetLabel("Swap (MB)").
		SetFieldWidth(12).
		SetText("512")
	coresField := tview.NewInputField().
		SetLabel("Cores").
		SetFieldWidth(8).
		SetText("1")
	rootFSStorageDropdown := tview.NewDropDown().
		SetLabel("RootFS Storage").
		SetFieldWidth(24)
	rootFSSizeField := tview.NewInputField().
		SetLabel("RootFS Size (GB)").
		SetFieldWidth(12).
		SetText("8")
	templateStorageDropdown := tview.NewDropDown().
		SetLabel("Template Storage").
		SetFieldWidth(24)
	templateDropdown := tview.NewDropDown().
		SetLabel("Template").
		SetFieldWidth(48)
	bridgeField := tview.NewInputField().
		SetLabel("Bridge").
		SetFieldWidth(16).
		SetText("vmbr0")
	unprivilegedCheckbox := tview.NewCheckbox().
		SetLabel("Unprivileged").
		SetChecked(true)
	startCheckbox := tview.NewCheckbox().
		SetLabel("Start After Create").
		SetChecked(true)

	form.AddFormItem(nodeDropdown)
	form.AddFormItem(vmidField)
	form.AddFormItem(hostnameField)
	form.AddFormItem(memoryField)
	form.AddFormItem(swapField)
	form.AddFormItem(coresField)
	form.AddFormItem(rootFSStorageDropdown)
	form.AddFormItem(rootFSSizeField)
	form.AddFormItem(templateStorageDropdown)
	form.AddFormItem(templateDropdown)
	form.AddFormItem(bridgeField)
	form.AddFormItem(unprivilegedCheckbox)
	form.AddFormItem(startCheckbox)

	currentData := initialData
	updateTemplates := func(storage string) {
		templates := append([]string{vmCreateNoneOption}, currentData.templatesByStorage[storage]...)
		if storage == "" || len(currentData.templatesByStorage[storage]) == 0 {
			templates = []string{vmCreateNoneOption}
		}
		templateDropdown.SetOptions(templates, nil)
		templateDropdown.SetCurrentOption(0)
	}
	applyNodeData := func(data lxcCreateNodeData) {
		currentData = data
		vmidField.SetText(strconv.Itoa(data.nextID))

		rootFSStorages := append([]string(nil), data.rootFSStorages...)
		if len(rootFSStorages) == 0 {
			rootFSStorages = []string{"No rootfs storages"}
		}
		rootFSStorageDropdown.SetOptions(rootFSStorages, nil)
		rootFSStorageDropdown.SetCurrentOption(0)

		templateStorages := append([]string{vmCreateNoneOption}, data.templateStorages...)
		templateStorageDropdown.SetOptions(templateStorages, nil)
		if len(data.templateStorages) > 0 {
			templateStorageDropdown.SetCurrentOption(1)
			updateTemplates(data.templateStorages[0])
		} else {
			templateStorageDropdown.SetCurrentOption(0)
			updateTemplates("")
		}
	}
	applyNodeData(initialData)

	nodeDropdown.SetSelectedFunc(func(option string, index int) {
		if index < 0 || index >= len(choices) {
			return
		}

		loadGuestCreateData(
			a,
			fmt.Sprintf("Loading LXC options for %s...", choices[index].label),
			choices[index].node,
			a.loadLXCCreateNodeData,
			applyNodeData,
			"Failed to load node options",
		)
	})

	templateStorageDropdown.SetSelectedFunc(func(option string, index int) {
		if option == vmCreateNoneOption {
			updateTemplates("")
			return
		}
		updateTemplates(option)
	})

	pageName := "modal:lxcCreate"
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
			a.showMessageSafe("Enter a valid positive CT ID")
			return
		}
		hostname := strings.TrimSpace(hostnameField.GetText())
		if hostname == "" {
			a.showMessageSafe("Hostname is required")
			return
		}
		memoryMB, err := strconv.Atoi(strings.TrimSpace(memoryField.GetText()))
		if err != nil || memoryMB <= 0 {
			a.showMessageSafe("Enter a valid positive memory value")
			return
		}
		swapMB, err := strconv.Atoi(strings.TrimSpace(swapField.GetText()))
		if err != nil || swapMB < 0 {
			a.showMessageSafe("Enter a valid swap value")
			return
		}
		cores, err := strconv.Atoi(strings.TrimSpace(coresField.GetText()))
		if err != nil || cores <= 0 {
			a.showMessageSafe("Enter a valid positive core count")
			return
		}
		rootFSStorageIndex, _ := rootFSStorageDropdown.GetCurrentOption()
		if rootFSStorageIndex < 0 || rootFSStorageIndex >= len(currentData.rootFSStorages) {
			a.showMessageSafe("Select a valid rootfs storage")
			return
		}
		rootFSSizeGB, err := strconv.Atoi(strings.TrimSpace(rootFSSizeField.GetText()))
		if err != nil || rootFSSizeGB <= 0 {
			a.showMessageSafe("Enter a valid positive rootfs size")
			return
		}
		_, templateStorage := templateStorageDropdown.GetCurrentOption()
		_, templateValue := templateDropdown.GetCurrentOption()
		if templateStorage == vmCreateNoneOption || templateValue == vmCreateNoneOption || templateValue == "" {
			a.showMessageSafe("Select an OS template")
			return
		}
		bridge := strings.TrimSpace(bridgeField.GetText())
		if bridge == "" {
			a.showMessageSafe("Bridge is required")
			return
		}

		options := api.LXCCreateOptions{
			VMID:          vmid,
			Hostname:      hostname,
			MemoryMB:      memoryMB,
			SwapMB:        swapMB,
			Cores:         cores,
			RootFSStorage: currentData.rootFSStorages[rootFSStorageIndex],
			RootFSSizeGB:  rootFSSizeGB,
			OSTemplate:    templateValue,
			Bridge:        bridge,
			Unprivileged:  unprivilegedCheckbox.IsChecked(),
			Start:         startCheckbox.IsChecked(),
		}

		closeForm()
		a.enqueueLXCCreate(node, options)
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

func (a *App) enqueueLXCCreate(node *api.Node, options api.LXCCreateOptions) {
	a.enqueueGuestCreateTask(
		node,
		fmt.Sprintf("Create LXC %s (%d)", options.Hostname, options.VMID),
		options.Hostname,
		options.VMID,
		fmt.Sprintf("Created LXC %s", options.Hostname),
		"LXC create failed",
		func(node *api.Node) (string, error) {
			client, err := a.getClientForNode(node)
			if err != nil {
				return "", err
			}
			return client.CreateLXC(node.Name, options)
		},
	)
}

func (a *App) lxcCreateNodeChoices() []lxcCreateNodeChoice {
	nodes := append([]*api.Node(nil), models.GlobalState.OriginalNodes...)
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name == nodes[j].Name {
			return nodes[i].SourceProfile < nodes[j].SourceProfile
		}
		return nodes[i].Name < nodes[j].Name
	})

	choices := make([]lxcCreateNodeChoice, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || !node.Online {
			continue
		}
		label := node.Name
		if node.SourceProfile != "" {
			label += fmt.Sprintf(" (%s)", node.SourceProfile)
		}
		choices = append(choices, lxcCreateNodeChoice{label: label, node: node})
	}
	return choices
}

func (a *App) loadLXCCreateNodeData(node *api.Node) (lxcCreateNodeData, error) {
	nextID, storages, err := a.loadCreateNodeStorages(node)
	if err != nil {
		return lxcCreateNodeData{}, err
	}

	data := lxcCreateNodeData{
		nextID:             nextID,
		rootFSStorages:     make([]string, 0),
		templateStorages:   make([]string, 0),
		templatesByStorage: make(map[string][]string),
	}

	for _, storage := range storages {
		if storage == nil {
			continue
		}
		if strings.Contains(storage.Content, "rootdir") {
			data.rootFSStorages = append(data.rootFSStorages, storage.Name)
		}
	}

	sort.Strings(data.rootFSStorages)
	data.templateStorages, data.templatesByStorage, err = a.collectNodeStorageContent(node, storages, storageSupportsToken("vztmpl"), "vztmpl")
	if err != nil {
		return lxcCreateNodeData{}, err
	}
	return data, nil
}
