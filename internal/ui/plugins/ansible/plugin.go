package ansible

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	coreansible "github.com/devnullvoid/pvetui/internal/plugins/ansible"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// PluginID identifies the ansible plugin for configuration toggles.
const PluginID = "ansible"

const (
	menuPageName     = "plugin.ansible.menu"
	outputPageName   = "plugin.ansible.output"
	savePathPageName = "plugin.ansible.save"
	playbookPageName = "plugin.ansible.playbook"
	adhocPageName    = "plugin.ansible.adhoc"
	setupPageName    = "plugin.ansible.setup"
	runningPageName  = "plugin.ansible.running"
)

// Plugin provides Ansible integration for inventory generation and playbook execution.
type Plugin struct {
	app       *components.App
	runner    *coreansible.Runner
	runMu     sync.Mutex
	runCancel context.CancelFunc
}

// New creates a fresh plugin instance.
func New() *Plugin {
	return &Plugin{runner: coreansible.NewRunner()}
}

// ID returns the stable identifier for configuration wiring.
func (p *Plugin) ID() string {
	return PluginID
}

// Name returns a human-friendly plugin name.
func (p *Plugin) Name() string {
	return "Ansible Toolkit"
}

// Description summarises the plugin's behaviour.
func (p *Plugin) Description() string {
	return "Generate inventory from nodes/guests, run Ansible commands, and guide SSH access setup from the global menu."
}

// Initialize wires plugin dependencies.
func (p *Plugin) Initialize(ctx context.Context, app *components.App, registrar components.PluginRegistrar) error {
	p.app = app
	_ = ctx
	_ = registrar

	return nil
}

// Shutdown releases resources associated with the plugin.
func (p *Plugin) Shutdown(ctx context.Context) error {
	p.cancelRunningCommand()
	p.app = nil
	return nil
}

// ModalPageNames returns modal page names registered by this plugin.
func (p *Plugin) ModalPageNames() []string {
	return []string{
		menuPageName,
		outputPageName,
		savePathPageName,
		playbookPageName,
		adhocPageName,
		setupPageName,
		runningPageName,
	}
}

// OpenGlobal opens the toolkit from the global menu context.
func (p *Plugin) OpenGlobal(ctx context.Context, app *components.App) error {
	if app == nil {
		return fmt.Errorf("application context unavailable")
	}

	p.showMainMenu()
	return nil
}

func (p *Plugin) showMainMenu() {
	pages := p.app.Pages()
	previousFocus := p.app.GetFocus()

	closeMenu := func() {
		pages.RemovePage(menuPageName)
		if previousFocus != nil {
			p.app.SetFocus(previousFocus)
		}
	}

	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true)
	list.SetBorderColor(theme.Colors.Border)
	list.SetTitle(" Ansible Toolkit ")
	list.SetTitleColor(theme.Colors.Primary)
	list.SetMainTextColor(theme.Colors.Primary)
	list.SetSecondaryTextColor(theme.Colors.Secondary)

	inventory := p.currentInventory()
	selectedNode, selectedGuest := p.currentSelectionForLimit()

	list.AddItem("Preview Inventory", "Render inventory from current nodes and guests", 0, func() {
		p.showOutput("Generated Inventory", inventory.Text, p.showMainMenu)
	})
	list.AddItem("Save Inventory", "Write generated inventory to a file", 0, func() {
		p.showSaveInventoryForm(inventory.Text, p.showMainMenu)
	})
	list.AddItem("Run Ping", "Run ansible ping module against this inventory", 0, func() {
		defaultLimit := p.defaultLimitForSelection(selectedNode, selectedGuest, inventory)
		p.showAdhocForm(defaultLimit, inventory.Text, p.showMainMenu)
	})
	list.AddItem("Run Playbook", "Execute ansible-playbook on generated inventory", 0, func() {
		defaultLimit := p.defaultLimitForSelection(selectedNode, selectedGuest, inventory)
		p.showPlaybookForm(defaultLimit, inventory.Text, p.showMainMenu)
	})
	list.AddItem("SSH Setup Assistant", "Show commands to prepare key-based SSH access", 0, func() {
		p.showSetupAssistant(inventory, p.showMainMenu)
	})
	list.AddItem("Close", "Return", 'q', closeMenu)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeMenu()
			return nil
		}

		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'j':
				current := list.GetCurrentItem()
				if current < list.GetItemCount()-1 {
					list.SetCurrentItem(current + 1)
				}
				return nil
			case 'k':
				current := list.GetCurrentItem()
				if current > 0 {
					list.SetCurrentItem(current - 1)
				}
				return nil
			}
		}

		return event
	})

	pages.AddPage(menuPageName, centerModal(list, 70, 18), true, true)
	p.app.SetFocus(list)
}

func (p *Plugin) showAdhocForm(defaultLimit, inventory string, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Run Ping ")
	form.SetTitleColor(theme.Colors.Primary)

	limit := defaultLimit
	extra := ""
	timeout := "5m"

	form.AddInputField("Limit", defaultLimit, 50, nil, func(text string) { limit = text })
	form.AddInputField("Extra Args", "", 50, nil, func(text string) { extra = text })
	form.AddInputField("Timeout", timeout, 10, nil, func(text string) { timeout = text })

	closeForm := func() {
		pages.RemovePage(adhocPageName)
		if onDone != nil {
			onDone()
		}
	}

	form.AddButton("Run", func() {
		timeoutDuration, err := parseDuration(timeout, 5*time.Minute)
		if err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Invalid timeout: %v", err))
			return
		}
		closeForm()
		p.runPing(inventory, strings.TrimSpace(limit), strings.Fields(extra), timeoutDuration)
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(adhocPageName, centerModal(form, 86, 12), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) showPlaybookForm(defaultLimit, inventory string, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Run Playbook ")
	form.SetTitleColor(theme.Colors.Primary)

	playbookPath := ""
	limit := defaultLimit
	extra := ""
	checkMode := false
	timeout := "20m"

	form.AddInputField("Playbook", "", 60, nil, func(text string) { playbookPath = text })
	form.AddInputField("Limit", defaultLimit, 40, nil, func(text string) { limit = text })
	form.AddInputField("Extra Args", "", 60, nil, func(text string) { extra = text })
	form.AddCheckbox("Check Mode", false, func(checked bool) { checkMode = checked })
	form.AddInputField("Timeout", timeout, 10, nil, func(text string) { timeout = text })

	closeForm := func() {
		pages.RemovePage(playbookPageName)
		if onDone != nil {
			onDone()
		}
	}

	form.AddButton("Run", func() {
		if strings.TrimSpace(playbookPath) == "" {
			p.app.ShowMessageSafe("Playbook path is required.")
			return
		}
		if _, err := os.Stat(strings.TrimSpace(playbookPath)); err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Playbook not found: %v", err))
			return
		}

		timeoutDuration, err := parseDuration(timeout, 20*time.Minute)
		if err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Invalid timeout: %v", err))
			return
		}

		closeForm()
		p.runPlaybook(inventory, coreansible.PlaybookOptions{
			PlaybookPath: strings.TrimSpace(playbookPath),
			Limit:        strings.TrimSpace(limit),
			ExtraArgs:    strings.Fields(extra),
			CheckMode:    checkMode,
		}, timeoutDuration)
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(playbookPageName, centerModal(form, 92, 16), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) runPing(inventory, limit string, extraArgs []string, timeout time.Duration) {
	if err := p.runner.CheckAvailability(); err != nil {
		p.app.ShowMessageSafe(fmt.Sprintf("Ansible is not available: %v", err))
		return
	}

	p.showRunningModal("Running ansible ping...")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	p.setRunningCancel(cancel)

	go func() {
		defer cancel()
		defer p.clearRunningCancel()

		result := p.runner.RunPing(ctx, inventory, limit, extraArgs)
		p.app.QueueUpdateDraw(func() {
			p.app.Pages().RemovePage(runningPageName)
			title := "Ping Result"
			body := formatCommandResult(result)
			if result.Err != nil {
				title = "Ping Failed"
			}
			p.showOutput(title, body, nil)
		})
	}()
}

func (p *Plugin) runPlaybook(inventory string, opts coreansible.PlaybookOptions, timeout time.Duration) {
	if err := p.runner.CheckAvailability(); err != nil {
		p.app.ShowMessageSafe(fmt.Sprintf("Ansible is not available: %v", err))
		return
	}

	p.showRunningModal("Running ansible-playbook...")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	p.setRunningCancel(cancel)

	go func() {
		defer cancel()
		defer p.clearRunningCancel()

		result := p.runner.RunPlaybook(ctx, inventory, opts)
		p.app.QueueUpdateDraw(func() {
			p.app.Pages().RemovePage(runningPageName)
			title := "Playbook Result"
			body := formatCommandResult(result)
			if result.Err != nil {
				title = "Playbook Failed"
			}
			p.showOutput(title, body, nil)
		})
	}()
}

func (p *Plugin) showSaveInventoryForm(inventory string, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Save Inventory ")
	form.SetTitleColor(theme.Colors.Primary)

	defaultPath := filepath.Join(defaultHomeDir(), "ansible", "pvetui-inventory.ini")
	targetPath := defaultPath

	form.AddInputField("Path", defaultPath, 80, nil, func(text string) {
		targetPath = strings.TrimSpace(text)
	})

	closeForm := func() {
		pages.RemovePage(savePathPageName)
		if onDone != nil {
			onDone()
		}
	}

	form.AddButton("Save", func() {
		if strings.TrimSpace(targetPath) == "" {
			p.app.ShowMessageSafe("Path is required.")
			return
		}

		if err := coreansible.SaveInventory(targetPath, inventory); err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Failed to save inventory: %v", err))
			return
		}

		closeForm()
		p.app.ShowMessageSafe(fmt.Sprintf("Inventory saved to %s", targetPath))
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(savePathPageName, centerModal(form, 96, 9), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) showSetupAssistant(inventory coreansible.InventoryResult, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	text := tview.NewTextView()
	text.SetBorder(true)
	text.SetBorderColor(theme.Colors.Border)
	text.SetTitle(" SSH Setup Assistant ")
	text.SetTitleColor(theme.Colors.Primary)
	text.SetDynamicColors(true)
	text.SetWrap(true)
	text.SetWordWrap(true)
	text.SetScrollable(true)
	text.SetText(buildSetupGuide(inventory))

	closeView := func() {
		pages.RemovePage(setupPageName)
		if onDone != nil {
			onDone()
		}
	}

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) || (event.Key() == tcell.KeyRune && (event.Rune() == 'q' || event.Rune() == 'Q')) {
			closeView()
			return nil
		}
		return event
	})

	pages.AddPage(setupPageName, centerModal(text, 110, 28), true, true)
	p.app.SetFocus(text)
}

func (p *Plugin) showRunningModal(message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Cancel"})
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			p.cancelRunningCommand()
			p.app.Pages().RemovePage(runningPageName)
			return nil
		}
		return event
	})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		p.cancelRunningCommand()
		p.app.Pages().RemovePage(runningPageName)
	})

	p.app.Pages().AddPage(runningPageName, centerModal(modal, 50, 7), true, true)
	p.app.SetFocus(modal)
}

func (p *Plugin) showOutput(title, content string, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	output := tview.NewTextView()
	output.SetBorder(true)
	output.SetBorderColor(theme.Colors.Border)
	output.SetTitle(" " + title + " ")
	output.SetTitleColor(theme.Colors.Primary)
	output.SetDynamicColors(true)
	output.SetScrollable(true)
	output.SetWrap(true)
	output.SetWordWrap(true)
	output.SetText(content + "\n\n[secondary]esc/backspace/q: close[-]")

	closeOutput := func() {
		pages.RemovePage(outputPageName)
		if onDone != nil {
			onDone()
		}
	}

	output.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeOutput()
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'q' || event.Rune() == 'Q') {
			closeOutput()
			return nil
		}
		return event
	})

	pages.AddPage(outputPageName, centerModal(output, 110, 30), true, true)
	p.app.SetFocus(output)
}

func (p *Plugin) currentInventory() coreansible.InventoryResult {
	nodes := p.app.NodeList().GetNodes()
	guests := p.app.VMList().GetVMs()

	defaults := coreansible.InventoryDefaults{
		NodeSSHUser: p.resolveNodeUser(),
		VMSSHUser:   p.resolveVMUser(),
	}

	return coreansible.BuildInventory(nodes, guests, defaults)
}

func (p *Plugin) currentSelectionForLimit() (*api.Node, *api.VM) {
	currentPage, _ := p.app.Pages().GetFrontPage()
	switch currentPage {
	case api.PageGuests:
		if vm := p.app.VMList().GetSelectedVM(); vm != nil {
			return nil, vm
		}
	case api.PageNodes:
		if node := p.app.NodeList().GetSelectedNode(); node != nil {
			return node, nil
		}
	}

	// Fallback for modal-driven paths where front page may be plugin-owned.
	if vm := p.app.VMList().GetSelectedVM(); vm != nil {
		return nil, vm
	}
	if node := p.app.NodeList().GetSelectedNode(); node != nil {
		return node, nil
	}

	return nil, nil
}

func (p *Plugin) resolveNodeUser() string {
	cfg := p.app.Config()
	if cfg == nil {
		return "root"
	}

	if cfg.ActiveProfile != "" {
		if profile, ok := cfg.Profiles[cfg.ActiveProfile]; ok && strings.TrimSpace(profile.SSHUser) != "" {
			return strings.TrimSpace(profile.SSHUser)
		}
	}
	if strings.TrimSpace(cfg.SSHUser) != "" {
		return strings.TrimSpace(cfg.SSHUser)
	}
	if strings.TrimSpace(cfg.GetUser()) != "" {
		return strings.TrimSpace(cfg.GetUser())
	}

	return "root"
}

func (p *Plugin) resolveVMUser() string {
	cfg := p.app.Config()
	if cfg == nil {
		return p.resolveNodeUser()
	}

	if cfg.ActiveProfile != "" {
		if profile, ok := cfg.Profiles[cfg.ActiveProfile]; ok {
			if strings.TrimSpace(profile.VMSSHUser) != "" {
				return strings.TrimSpace(profile.VMSSHUser)
			}
			if strings.TrimSpace(profile.SSHUser) != "" {
				return strings.TrimSpace(profile.SSHUser)
			}
		}
	}

	if strings.TrimSpace(cfg.VMSSHUser) != "" {
		return strings.TrimSpace(cfg.VMSSHUser)
	}

	return p.resolveNodeUser()
}

func (p *Plugin) defaultLimitForSelection(node *api.Node, guest *api.VM, inventory coreansible.InventoryResult) string {
	if guest != nil {
		for _, host := range inventory.Hosts {
			if host.Vars["pvetui_kind"] != "guest" {
				continue
			}
			if host.Vars["pvetui_guest_id"] == fmt.Sprintf("%d", guest.ID) && host.Vars["pvetui_node"] == guest.Node {
				return host.Alias
			}
		}
	}

	if node != nil {
		for _, host := range inventory.Hosts {
			if host.Vars["pvetui_kind"] == "node" && host.Vars["pvetui_node"] == node.Name {
				return host.Alias
			}
		}
	}

	return ""
}

func parseDuration(value string, fallback time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}

	return d, nil
}

func buildSetupGuide(inventory coreansible.InventoryResult) string {
	var b strings.Builder

	b.WriteString("[primary]Ansible SSH Access Setup[-]\n\n")
	b.WriteString("1) Generate a dedicated SSH key (optional):\n")
	b.WriteString("   ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_pvetui -C \"pvetui-ansible\"\n\n")

	b.WriteString("2) Copy your key to each target host:\n")
	if len(inventory.Hosts) == 0 {
		b.WriteString("   No hosts currently visible in the inventory.\n")
	} else {
		for _, host := range inventory.Hosts {
			user := host.Vars["ansible_user"]
			target := host.Vars["ansible_host"]
			if strings.TrimSpace(user) == "" || strings.TrimSpace(target) == "" {
				continue
			}
			_, _ = fmt.Fprintf(&b, "   ssh-copy-id %s@%s\n", user, target)
		}
	}
	b.WriteString("\n")

	b.WriteString("3) Optional ansible.cfg defaults:\n")
	b.WriteString("   [defaults]\n")
	b.WriteString("   host_key_checking = True\n")
	b.WriteString("   timeout = 30\n")
	b.WriteString("\n")

	b.WriteString("4) Validate connectivity:\n")
	b.WriteString("   ansible -i ./pvetui-inventory.ini all -m ping\n\n")

	b.WriteString("5) Example run:\n")
	b.WriteString("   ansible-playbook -i ./pvetui-inventory.ini site.yml\n\n")

	b.WriteString("[secondary]Press esc/backspace/q to close[-]")

	return b.String()
}

func formatCommandResult(result coreansible.CommandResult) string {
	var b strings.Builder

	b.WriteString("Command:\n")
	b.WriteString(result.Command)
	b.WriteString("\n\nDuration: ")
	b.WriteString(result.Duration.String())
	if result.Err != nil {
		b.WriteString("\nError: ")
		b.WriteString(result.Err.Error())
	}
	b.WriteString("\n\nOutput:\n")
	b.WriteString(result.Output)

	return b.String()
}

func centerModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 0, true).
				AddItem(nil, 0, 1, false),
			width,
			0,
			true,
		).
		AddItem(nil, 0, 1, false)
}

func isBackKey(event *tcell.EventKey) bool {
	if event == nil {
		return false
	}

	return event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2
}

func defaultHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	return home
}

func (p *Plugin) setRunningCancel(cancel context.CancelFunc) {
	p.runMu.Lock()
	defer p.runMu.Unlock()
	p.runCancel = cancel
}

func (p *Plugin) clearRunningCancel() {
	p.runMu.Lock()
	defer p.runMu.Unlock()
	p.runCancel = nil
}

func (p *Plugin) cancelRunningCommand() {
	p.runMu.Lock()
	cancel := p.runCancel
	p.runCancel = nil
	p.runMu.Unlock()

	if cancel != nil {
		cancel()
	}
}
