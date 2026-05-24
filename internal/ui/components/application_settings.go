package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
)

const (
	applicationSettingsMaxWidth  = 104
	applicationSettingsMaxHeight = 24
)

func (a *App) showApplicationSettingsDialog() {
	form := newStandardForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Application Settings ")
	form.SetTitleColor(theme.Colors.Primary)

	showIcons := a.config.ShowIcons
	debug := a.config.Debug
	cacheDir := a.config.CacheDir
	ageDir := a.config.AgeDir
	themeName := a.config.Theme.Name
	themeColorsRaw := formatStringMapYAML(a.config.Theme.Colors)
	bindings := a.config.KeyBindings

	form.AddCheckbox("Show Icons", showIcons, func(checked bool) { showIcons = checked })
	form.AddCheckbox("Debug Logging", debug, func(checked bool) { debug = checked })
	form.AddInputField("Cache Directory", cacheDir, 64, nil, func(text string) {
		cacheDir = strings.TrimSpace(text)
	})
	form.AddInputField("Age Directory", ageDir, 64, nil, func(text string) {
		ageDir = strings.TrimSpace(text)
	})
	form.AddInputField("Theme Name", themeName, 32, nil, func(text string) {
		themeName = strings.TrimSpace(text)
	})
	form.AddTextArea("Theme Colors (YAML)", themeColorsRaw, 0, 6, 0, func(text string) {
		themeColorsRaw = text
	})
	form.AddInputField("Next View Key", bindings.SwitchView, 16, nil, func(text string) { bindings.SwitchView = strings.TrimSpace(text) })
	form.AddInputField("Previous View Key", bindings.SwitchViewReverse, 16, nil, func(text string) { bindings.SwitchViewReverse = strings.TrimSpace(text) })
	form.AddInputField("Nodes Page Key", bindings.NodesPage, 16, nil, func(text string) { bindings.NodesPage = strings.TrimSpace(text) })
	form.AddInputField("Guests Page Key", bindings.GuestsPage, 16, nil, func(text string) { bindings.GuestsPage = strings.TrimSpace(text) })
	form.AddInputField("Tasks Page Key", bindings.TasksPage, 16, nil, func(text string) { bindings.TasksPage = strings.TrimSpace(text) })
	form.AddInputField("Storage Page Key", bindings.StoragePage, 16, nil, func(text string) { bindings.StoragePage = strings.TrimSpace(text) })
	form.AddInputField("Toggle Queue Key", bindings.TasksToggleQueue, 16, nil, func(text string) { bindings.TasksToggleQueue = strings.TrimSpace(text) })
	form.AddInputField("Stop Task Key", bindings.TaskStopCancel, 16, nil, func(text string) { bindings.TaskStopCancel = strings.TrimSpace(text) })
	form.AddInputField("Context Menu Key", bindings.Menu, 16, nil, func(text string) { bindings.Menu = strings.TrimSpace(text) })
	form.AddInputField("Global Menu Key", bindings.GlobalMenu, 16, nil, func(text string) { bindings.GlobalMenu = strings.TrimSpace(text) })
	form.AddInputField("Shell Key", bindings.Shell, 16, nil, func(text string) { bindings.Shell = strings.TrimSpace(text) })
	form.AddInputField("VNC Key", bindings.VNC, 16, nil, func(text string) { bindings.VNC = strings.TrimSpace(text) })
	form.AddInputField("Refresh Key", bindings.Refresh, 16, nil, func(text string) { bindings.Refresh = strings.TrimSpace(text) })
	form.AddInputField("Auto Refresh Key", bindings.AutoRefresh, 16, nil, func(text string) { bindings.AutoRefresh = strings.TrimSpace(text) })
	form.AddInputField("Search Key", bindings.Search, 16, nil, func(text string) { bindings.Search = strings.TrimSpace(text) })
	form.AddInputField("Guest Filter Key", bindings.AdvancedGuestFilter, 16, nil, func(text string) { bindings.AdvancedGuestFilter = strings.TrimSpace(text) })
	form.AddInputField("Help Key", bindings.Help, 16, nil, func(text string) { bindings.Help = strings.TrimSpace(text) })
	form.AddInputField("Quit Key", bindings.Quit, 16, nil, func(text string) { bindings.Quit = strings.TrimSpace(text) })

	closeDialog := func() {
		a.pages.RemovePage("applicationSettings")
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	form.AddButton("Save", func() {
		colors, err := parseStringMapYAML(themeColorsRaw)
		if err != nil {
			a.showMessageSafe(fmt.Sprintf("Invalid theme colors: %v", err))
			return
		}

		if err := config.ValidateKeyBindings(bindings); err != nil {
			a.showMessageSafe(err.Error())
			return
		}

		restartRequired := a.config.CacheDir != config.ExpandHomePath(cacheDir) ||
			a.config.Theme.Name != themeName ||
			formatStringMapYAML(a.config.Theme.Colors) != formatStringMapYAML(colors)

		a.config.ShowIcons = showIcons
		a.config.Debug = debug
		a.config.CacheDir = config.ExpandHomePath(cacheDir)
		a.config.AgeDir = config.ExpandHomePath(ageDir)
		a.config.Theme.Name = themeName
		a.config.Theme.Colors = colors
		a.config.KeyBindings = bindings

		if err := a.SaveConfigPreservingSOPS(); err != nil {
			a.showMessageSafe(fmt.Sprintf("Failed to save application settings: %v", err))
			return
		}

		config.DebugEnabled = debug
		logger.SetDebugEnabled(debug)
		config.SetAgeDirOverride(a.config.AgeDir)
		a.footer.UpdateKeybindings(FormatFooterText(a.config.KeyBindings))
		a.helpModal = NewHelpModal(a.config.KeyBindings)
		a.helpModal.SetApp(a)
		a.refreshVisualSettings()

		closeDialog()
		if restartRequired {
			a.header.ShowSuccess("Application settings saved. Restart to apply theme or cache changes.")
			return
		}
		a.header.ShowSuccess("Application settings saved.")
	})
	form.AddButton("Cancel", closeDialog)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event != nil && event.Key() == tcell.KeyEscape {
			closeDialog()
			return nil
		}
		return event
	})

	modal := newCenteredCappedModal(form, applicationSettingsMaxWidth, applicationSettingsMaxHeight)

	a.pages.AddPage("applicationSettings", modal, true, true)
	a.SetFocus(form)
}

type centeredCappedModal struct {
	*tview.Box
	content             tview.Primitive
	maxWidth, maxHeight int
}

func newCenteredCappedModal(content tview.Primitive, maxWidth, maxHeight int) *centeredCappedModal {
	return &centeredCappedModal{
		Box:       tview.NewBox(),
		content:   content,
		maxWidth:  maxWidth,
		maxHeight: maxHeight,
	}
}

func (m *centeredCappedModal) Draw(screen tcell.Screen) {
	x, y, width, height := m.GetRect()
	contentWidth := cappedModalDimension(width, m.maxWidth)
	contentHeight := cappedModalDimension(height, m.maxHeight)
	if contentWidth == 0 || contentHeight == 0 {
		return
	}

	m.content.SetRect(
		x+(width-contentWidth)/2,
		y+(height-contentHeight)/2,
		contentWidth,
		contentHeight,
	)
	m.content.Draw(screen)
}

func (m *centeredCappedModal) Focus(delegate func(tview.Primitive)) {
	delegate(m.content)
}

func (m *centeredCappedModal) HasFocus() bool {
	return m.content.HasFocus()
}

func (m *centeredCappedModal) Blur() {
	m.content.Blur()
}

func (m *centeredCappedModal) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.content.InputHandler()
}

func (m *centeredCappedModal) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return m.content.MouseHandler()
}

func (m *centeredCappedModal) PasteHandler() func(string, func(tview.Primitive)) {
	return m.content.PasteHandler()
}

func cappedModalDimension(available, limit int) int {
	if available <= 0 {
		return 0
	}
	if available <= 2 {
		return available
	}
	return min(available-2, limit)
}

func (a *App) refreshVisualSettings() {
	if a.nodeList != nil {
		a.nodeList.SetNodes(a.nodeList.GetNodes())
		if node := a.nodeList.GetSelectedNode(); node != nil {
			a.nodeDetails.Update(node, a.nodeList.GetNodes())
		}
	}
	if a.vmList != nil {
		a.vmList.SetVMs(a.vmList.GetVMs())
		if vm := a.vmList.GetSelectedVM(); vm != nil {
			a.vmDetails.Update(vm)
		}
	}
}

func formatStringMapYAML(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func parseStringMapYAML(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var values map[string]string
	if err := yaml.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	return values, nil
}
