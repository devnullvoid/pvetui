package components

import (
	"os"
	"slices"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
)

// showManagePluginsDialog displays a dialog for toggling plugins on or off.
func (a *App) showManagePluginsDialog() {
	a.lastFocus = a.GetFocus()

	infos := a.pluginCatalogSnapshot()
	if len(infos) == 0 {
		a.header.ShowError("No plugins available.")
		return
	}

	type pluginEntry struct {
		info    PluginInfo
		enabled bool
	}

	entries := make([]pluginEntry, len(infos))
	enabledSet := make(map[string]struct{}, len(a.config.Plugins.Enabled))
	for _, id := range a.config.Plugins.Enabled {
		if id == "" {
			continue
		}
		enabledSet[id] = struct{}{}
	}

	for i, info := range infos {
		_, enabled := enabledSet[info.ID]
		entries[i] = pluginEntry{
			info:    info,
			enabled: enabled,
		}
	}

	originalEnabled := append([]string(nil), a.config.Plugins.Enabled...)
	sort.Strings(originalEnabled)

	list := tview.NewList().
		ShowSecondaryText(true)
	list.SetBorder(false)
	list.SetHighlightFullLine(true)
	list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary).Attributes(tcell.AttrReverse))

	updateList := func() {
		current := list.GetCurrentItem()
		list.Clear()
		checkedMarker := tview.Escape("[x]")
		uncheckedMarker := tview.Escape("[ ]")
		for _, entry := range entries {
			marker := uncheckedMarker
			if entry.enabled {
				marker = checkedMarker
			}
			list.AddItem(marker+" "+entry.info.Name, entry.info.Description, 0, nil)
		}
		if current >= 0 && current < len(entries) {
			list.SetCurrentItem(current)
		}
	}

	toggleAt := func(index int) {
		if index < 0 || index >= len(entries) {
			return
		}
		entries[index].enabled = !entries[index].enabled
		updateList()
	}

	updateList()

	closeDialog := func() {
		a.pages.RemovePage("pluginsManager")
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	var saveButton, cancelButton *tview.Button

	saveChanges := func() {
		enabledIDs := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.enabled {
				enabledIDs = append(enabledIDs, entry.info.ID)
			}
		}

		sort.Strings(enabledIDs)
		changed := !slices.Equal(originalEnabled, enabledIDs)

		a.config.Plugins.Enabled = enabledIDs

		configPath, found := config.FindDefaultConfigPath()
		if !found {
			configPath = config.GetDefaultConfigPath()
		}

		wasSOPS := false
		if data, err := os.ReadFile(configPath); err == nil {
			wasSOPS = config.IsSOPSEncrypted(configPath, data)
		}

		if err := SaveConfigToFile(&a.config, configPath); err != nil {
			a.header.ShowError("Failed to save plugin configuration: " + err.Error())
			return
		}

		if wasSOPS {
			if err := a.reEncryptConfigIfNeeded(configPath); err != nil {
				a.header.ShowError("Failed to re-encrypt config: " + err.Error())
				return
			}
		}

		closeDialog()

		if changed {
			a.header.ShowSuccess("Plugin configuration saved. Restart required for changes to take effect.")
		} else {
			a.header.ShowSuccess("Plugin configuration unchanged.")
		}
	}

	list.SetSelectedFunc(func(index int, _ string, _ string, _ rune) {
		toggleAt(index)
	})

	navigationCapture := createNavigationInputCapture(a, nil, nil)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if navigationCapture != nil {
			event = navigationCapture(event)
			if event == nil {
				return nil
			}
		}

		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case ' ':
				toggleAt(list.GetCurrentItem())
				return nil
			case 's', 'S':
				saveChanges()
				return nil
			case 'c', 'C':
				closeDialog()
				return nil
			}
		case tcell.KeyTab:
			a.SetFocus(saveButton)
			return nil
		case tcell.KeyBacktab:
			a.SetFocus(cancelButton)
			return nil
		case tcell.KeyEscape:
			closeDialog()
			return nil
		}

		return event
	})

	helpText := tview.NewTextView()
	helpText.SetText("space: toggle  s:save  c:cancel")
	helpText.SetTextAlign(tview.AlignCenter)
	helpText.SetDynamicColors(true)
	helpText.SetTextColor(theme.Colors.Secondary)

	saveButton = tview.NewButton("Save").
		SetSelectedFunc(saveChanges)
	cancelButton = tview.NewButton("Cancel").
		SetSelectedFunc(closeDialog)

	saveButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeDialog()
			return nil
		case tcell.KeyTab:
			a.SetFocus(cancelButton)
			return nil
		case tcell.KeyBacktab:
			a.SetFocus(list)
			return nil
		}

		return event
	})

	cancelButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeDialog()
			return nil
		case tcell.KeyTab:
			a.SetFocus(list)
			return nil
		case tcell.KeyBacktab:
			a.SetFocus(saveButton)
			return nil
		}

		return event
	})

	buttonBar := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(saveButton, 0, 1, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(cancelButton, 0, 1, false).
		AddItem(nil, 0, 1, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, len(entries)*2+2, 1, true).
		AddItem(helpText, 1, 0, false).
		AddItem(buttonBar, 1, 0, false)

	frame := tview.NewFrame(layout)
	frame.SetBorder(true)
	frame.SetTitle(" Manage Plugins ")
	frame.SetBorderColor(theme.Colors.Border)
	frame.SetTitleColor(theme.Colors.Primary)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, len(entries)*2+7, 0, true).
			AddItem(nil, 0, 1, false), 0, 8, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("pluginsManager", modal, true, true)
	a.SetFocus(list)
}
