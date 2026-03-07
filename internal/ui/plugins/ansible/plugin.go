package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	cfgpkg "github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/logger"
	coreansible "github.com/devnullvoid/pvetui/internal/plugins/ansible"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
	"gopkg.in/yaml.v3"
)

// PluginID identifies the ansible plugin for configuration toggles.
const PluginID = "ansible"

const (
	menuPageName              = "plugin.ansible.menu"
	outputPageName            = "plugin.ansible.output"
	savePathPageName          = "plugin.ansible.save"
	playbookPageName          = "plugin.ansible.playbook"
	adhocPageName             = "plugin.ansible.adhoc"
	setupPageName             = "plugin.ansible.setup"
	runningPageName           = "plugin.ansible.running"
	settingsPageName          = "plugin.ansible.settings"
	bootstrapSettingsPageName = "plugin.ansible.bootstrap.settings"
	bootstrapRunPageName      = "plugin.ansible.bootstrap.run"
	bootstrapConfirmPageName  = "plugin.ansible.bootstrap.confirm"
	bootstrapScopeAll         = "all"
	bootstrapScopeNodes       = "nodes"
	bootstrapScopeGuests      = "guests"
	bootstrapMethodDirect     = "direct"
	bootstrapMethodAnsible    = "ansible"
	hostKindNode              = "node"
	hostKindGuest             = "guest"
	statusOK                  = "ok"
	statusChanged             = "changed"
	statusFailed              = "failed"
	statusSkipped             = "skipped"
	transportSSH              = "ssh"
	transportPCTExec          = "pct-exec"
	transportGuestAgent       = "guest-agent"
	liveOutputPageName        = "plugin.ansible.live-output"
)

// Plugin provides Ansible integration for inventory generation and playbook execution.
type Plugin struct {
	app                   *components.App
	runner                *coreansible.Runner
	runMu                 sync.Mutex
	runCancel             context.CancelFunc
	lastPlaybookFormState *playbookFormState
}

type playbookFormState struct {
	PlaybookPath string
	Scope        string
	Limit        string
	ExtraArgsRaw string
	CheckMode    bool
	TimeoutRaw   string
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
		settingsPageName,
		bootstrapSettingsPageName,
		bootstrapRunPageName,
		bootstrapConfirmPageName,
		liveOutputPageName,
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
		title := fmt.Sprintf("Generated Inventory (%s)", strings.ToUpper(inventory.Format))
		p.showOutput(title, inventory.Text, p.showMainMenu)
	})
	list.AddItem("Save Inventory", "Write generated inventory to a file", 0, func() {
		p.showSaveInventoryForm(inventory, p.showMainMenu)
	})
	list.AddItem("Run Ping", "Run ansible ping module against this inventory", 0, func() {
		defaultLimit := p.defaultLimitForSelection(selectedNode, selectedGuest, inventory)
		p.showAdhocForm(defaultLimit, inventory, p.showMainMenu)
	})
	list.AddItem("Run Playbook", "Execute ansible-playbook on generated inventory", 0, func() {
		defaultLimit := p.defaultLimitForSelection(selectedNode, selectedGuest, inventory)
		p.showPlaybookForm(defaultLimit, inventory, p.showMainMenu)
	})
	list.AddItem("Bootstrap Access", "Bulk-prepare hosts for Ansible access", 0, func() {
		p.showBootstrapRunForm(inventory, p.showMainMenu)
	})
	list.AddItem("SSH Setup Guide", "Show commands to prepare key-based SSH access", 0, func() {
		p.showSetupAssistant(inventory, p.showMainMenu)
	})
	list.AddItem("General Settings", "Configure inventory/run defaults", 0, func() {
		p.showSettingsForm(p.showMainMenu)
	})
	list.AddItem("Bootstrap Settings", "Configure bootstrap access defaults", 0, func() {
		p.showBootstrapSettingsForm(p.showMainMenu)
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

	pages.AddPage(menuPageName, p.centerModal(list, 70, 18), true, true)
	p.app.SetFocus(list)
}

func (p *Plugin) showSettingsForm(onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	cfg := p.app.Config()
	if cfg == nil {
		p.app.ShowMessageSafe("Configuration unavailable.")
		if onDone != nil {
			onDone()
		}
		return
	}

	ansibleCfg := cfg.Plugins.Ansible
	form := components.NewStandardForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Ansible Settings ")
	form.SetTitleColor(theme.Colors.Primary)

	defaultUser := strings.TrimSpace(ansibleCfg.DefaultUser)
	defaultPassword := strings.TrimSpace(ansibleCfg.DefaultPassword)
	sshPrivateKeyFile := strings.TrimSpace(ansibleCfg.SSHPrivateKeyFile)
	extraArgs := strings.Join(ansibleCfg.ExtraArgs, " ")
	inventoryVarsYAML := formatInventoryVarsYAML(ansibleCfg.InventoryVars)
	inventoryFormat := coreansible.NormalizeInventoryFormat(ansibleCfg.InventoryFormat)
	inventoryStyle := coreansible.NormalizeInventoryStyle(ansibleCfg.InventoryStyle)
	defaultLimitMode := strings.TrimSpace(ansibleCfg.DefaultLimitMode)
	if defaultLimitMode == "" {
		defaultLimitMode = "selection"
	}
	askPass := ansibleCfg.AskPass
	askBecomePass := ansibleCfg.AskBecomePass

	form.AddDropDown(
		"Inventory Format",
		[]string{coreansible.InventoryFormatYAML, coreansible.InventoryFormatINI},
		map[string]int{coreansible.InventoryFormatYAML: 0, coreansible.InventoryFormatINI: 1}[inventoryFormat],
		func(option string, _ int) {
			inventoryFormat = option
		},
	)
	form.AddDropDown(
		"Inventory Style",
		[]string{coreansible.InventoryStyleCompact, coreansible.InventoryStyleExpanded},
		map[string]int{coreansible.InventoryStyleCompact: 0, coreansible.InventoryStyleExpanded: 1}[inventoryStyle],
		func(option string, _ int) {
			inventoryStyle = option
		},
	)
	form.AddDropDown(
		"Default Limit Mode",
		[]string{"selection", "all", "none"},
		map[string]int{"selection": 0, "all": 1, "none": 2}[defaultLimitMode],
		func(option string, _ int) {
			defaultLimitMode = option
		},
	)
	form.AddInputField("Default User", defaultUser, 40, nil, func(text string) { defaultUser = strings.TrimSpace(text) })
	form.AddPasswordField("Default Password", defaultPassword, 64, '*', func(text string) {
		defaultPassword = strings.TrimSpace(text)
	})
	form.AddInputField("SSH Private Key", sshPrivateKeyFile, 80, nil, func(text string) {
		sshPrivateKeyFile = strings.TrimSpace(text)
	})
	form.AddTextArea("Inventory Vars (YAML)", inventoryVarsYAML, 0, 5, 0, func(text string) {
		inventoryVarsYAML = text
	})
	form.AddCheckbox("Ask Pass", askPass, func(checked bool) { askPass = checked })
	form.AddCheckbox("Ask Become Pass", askBecomePass, func(checked bool) { askBecomePass = checked })
	form.AddInputField("Extra Args", extraArgs, 80, nil, func(text string) {
		extraArgs = strings.TrimSpace(text)
	})

	closeForm := func() {
		pages.RemovePage(settingsPageName)
		if onDone != nil {
			onDone()
		}
	}

	form.AddButton("Save", func() {
		cfg.Plugins.Ansible.InventoryFormat = coreansible.NormalizeInventoryFormat(inventoryFormat)
		cfg.Plugins.Ansible.InventoryStyle = coreansible.NormalizeInventoryStyle(inventoryStyle)
		cfg.Plugins.Ansible.DefaultLimitMode = strings.TrimSpace(defaultLimitMode)
		cfg.Plugins.Ansible.DefaultUser = strings.TrimSpace(defaultUser)
		cfg.Plugins.Ansible.DefaultPassword = strings.TrimSpace(defaultPassword)
		cfg.Plugins.Ansible.SSHPrivateKeyFile = cfgpkg.ExpandHomePath(strings.TrimSpace(sshPrivateKeyFile))
		inventoryVars, err := parseInventoryVarsYAML(inventoryVarsYAML)
		if err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Invalid inventory vars: %v", err))
			return
		}
		cfg.Plugins.Ansible.InventoryVars = inventoryVars
		cfg.Plugins.Ansible.AskPass = askPass
		cfg.Plugins.Ansible.AskBecomePass = askBecomePass
		cfg.Plugins.Ansible.ExtraArgs = strings.Fields(extraArgs)

		if err := p.app.SaveConfigPreservingSOPS(); err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Failed to save settings: %v", err))
			return
		}

		closeForm()
		p.app.ShowMessageSafe("Ansible settings saved.")
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event != nil && event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(settingsPageName, p.centerModal(form, 100, 28), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) showBootstrapSettingsForm(onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	cfg := p.app.Config()
	if cfg == nil {
		p.app.ShowMessageSafe("Configuration unavailable.")
		if onDone != nil {
			onDone()
		}
		return
	}

	bootstrap := cfg.Plugins.Ansible.Bootstrap
	form := components.NewStandardForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Bootstrap Settings ")
	form.SetTitleColor(theme.Colors.Primary)

	enabled := bootstrap.Enabled
	username := strings.TrimSpace(bootstrap.Username)
	shell := strings.TrimSpace(bootstrap.Shell)
	createHome := bootstrap.CreateHome
	excludeWindowsGuests := bootstrap.ExcludeWindowsGuests
	sshPublicKeyFile := strings.TrimSpace(bootstrap.SSHPublicKeyFile)
	installAuthorizedKey := bootstrap.InstallAuthorizedKey
	setPassword := bootstrap.SetPassword
	password := strings.TrimSpace(bootstrap.Password)
	grantSudo := bootstrap.GrantSudoNOPASSWD
	sudoersMode := strings.TrimSpace(bootstrap.SudoersFileMode)
	dryRunDefault := bootstrap.DryRunDefault
	parallelismRaw := strconv.Itoa(bootstrap.Parallelism)
	timeoutRaw := strings.TrimSpace(bootstrap.Timeout)
	failFast := bootstrap.FailFast

	form.AddCheckbox("Enabled", enabled, func(checked bool) { enabled = checked })
	form.AddInputField("Username", username, 32, nil, func(text string) { username = strings.TrimSpace(text) })
	form.AddInputField("Shell", shell, 32, nil, func(text string) { shell = strings.TrimSpace(text) })
	form.AddCheckbox("Create Home", createHome, func(checked bool) { createHome = checked })
	form.AddCheckbox("Exclude Windows Guests", excludeWindowsGuests, func(checked bool) {
		excludeWindowsGuests = checked
	})
	form.AddInputField("SSH Public Key File", sshPublicKeyFile, 80, nil, func(text string) {
		sshPublicKeyFile = strings.TrimSpace(text)
	})
	form.AddCheckbox("Install Authorized Key", installAuthorizedKey, func(checked bool) { installAuthorizedKey = checked })
	form.AddCheckbox("Set Password", setPassword, func(checked bool) { setPassword = checked })
	form.AddPasswordField("Password", password, 64, '*', func(text string) { password = strings.TrimSpace(text) })
	form.AddCheckbox("Grant Sudo NOPASSWD", grantSudo, func(checked bool) { grantSudo = checked })
	form.AddInputField("Sudoers File Mode", sudoersMode, 16, nil, func(text string) { sudoersMode = strings.TrimSpace(text) })
	form.AddCheckbox("Dry Run Default", dryRunDefault, func(checked bool) { dryRunDefault = checked })
	form.AddInputField("Parallelism", parallelismRaw, 8, nil, func(text string) { parallelismRaw = strings.TrimSpace(text) })
	form.AddInputField("Timeout", timeoutRaw, 16, nil, func(text string) { timeoutRaw = strings.TrimSpace(text) })
	form.AddCheckbox("Fail Fast", failFast, func(checked bool) { failFast = checked })

	closeForm := func() {
		pages.RemovePage(bootstrapSettingsPageName)
		if onDone != nil {
			onDone()
		}
	}

	form.AddButton("Save", func() {
		if username == "" {
			p.app.ShowMessageSafe("Bootstrap username is required.")
			return
		}
		parallelism, err := strconv.Atoi(strings.TrimSpace(parallelismRaw))
		if err != nil || parallelism <= 0 {
			p.app.ShowMessageSafe("Parallelism must be a positive integer.")
			return
		}
		if _, err := parseDuration(timeoutRaw, 2*time.Minute); err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Invalid timeout: %v", err))
			return
		}
		if setPassword && strings.TrimSpace(password) == "" {
			p.app.ShowMessageSafe("Password is required when Set Password is enabled.")
			return
		}

		cfg.Plugins.Ansible.Bootstrap.Enabled = enabled
		cfg.Plugins.Ansible.Bootstrap.Username = username
		cfg.Plugins.Ansible.Bootstrap.Shell = shell
		cfg.Plugins.Ansible.Bootstrap.CreateHome = createHome
		cfg.Plugins.Ansible.Bootstrap.ExcludeWindowsGuests = excludeWindowsGuests
		cfg.Plugins.Ansible.Bootstrap.SSHPublicKeyFile = cfgpkg.ExpandHomePath(sshPublicKeyFile)
		cfg.Plugins.Ansible.Bootstrap.InstallAuthorizedKey = installAuthorizedKey
		cfg.Plugins.Ansible.Bootstrap.SetPassword = setPassword
		cfg.Plugins.Ansible.Bootstrap.Password = password
		cfg.Plugins.Ansible.Bootstrap.GrantSudoNOPASSWD = grantSudo
		cfg.Plugins.Ansible.Bootstrap.SudoersFileMode = sudoersMode
		cfg.Plugins.Ansible.Bootstrap.DryRunDefault = dryRunDefault
		cfg.Plugins.Ansible.Bootstrap.Parallelism = parallelism
		cfg.Plugins.Ansible.Bootstrap.Timeout = timeoutRaw
		cfg.Plugins.Ansible.Bootstrap.FailFast = failFast

		if err := p.app.SaveConfigPreservingSOPS(); err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Failed to save bootstrap settings: %v", err))
			return
		}

		closeForm()
		p.app.ShowMessageSafe("Bootstrap settings saved.")
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event != nil && event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(bootstrapSettingsPageName, p.centerModal(form, 104, 31), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) showBootstrapRunForm(inventory coreansible.InventoryResult, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	cfg := p.ansiblePluginConfig()
	bootstrap := cfg.Bootstrap

	form := components.NewStandardForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Bootstrap Access ")
	form.SetTitleColor(theme.Colors.Primary)

	scopeOptions := []string{bootstrapScopeAll, bootstrapScopeNodes, bootstrapScopeGuests}
	methodOptions := []string{bootstrapMethodDirect, bootstrapMethodAnsible}
	method := bootstrapMethodDirect
	scope := bootstrapScopeAll
	limit := ""
	limitPickerLabels, limitPickerValues := buildLimitPickerOptions(inventory)
	dryRun := bootstrap.DryRunDefault
	timeoutRaw := bootstrap.Timeout
	extraArgsRaw := ""
	var limitInput *tview.InputField

	form.AddDropDown("Method", methodOptions, 0, func(option string, _ int) {
		method = option
	})
	form.AddDropDown("Scope", scopeOptions, 0, func(option string, _ int) {
		scope = option
		resolved := resolveLimitForScope(scope, "")
		limit = resolved
		if limitInput != nil {
			limitInput.SetText(resolved)
		}
	})
	form.AddInputField("Limit", limit, 50, nil, func(text string) { limit = strings.TrimSpace(text) })
	if input, ok := form.GetFormItem(form.GetFormItemCount() - 1).(*tview.InputField); ok {
		limitInput = input
	}
	form.AddDropDown("Target", limitPickerLabels, 0, func(_ string, index int) {
		if index < 0 || index >= len(limitPickerValues) {
			return
		}
		val := strings.TrimSpace(limitPickerValues[index])
		if val == "" {
			return
		}
		limit = val
		if limitInput != nil {
			limitInput.SetText(val)
		}
	})
	form.AddCheckbox("Dry Run (--check)", dryRun, func(checked bool) { dryRun = checked })
	form.AddInputField("Timeout", timeoutRaw, 16, nil, func(text string) { timeoutRaw = strings.TrimSpace(text) })
	form.AddInputField("Extra Args", extraArgsRaw, 80, nil, func(text string) { extraArgsRaw = strings.TrimSpace(text) })

	closeForm := func() {
		pages.RemovePage(bootstrapRunPageName)
		if onDone != nil {
			onDone()
		}
	}

	form.AddButton("Run", func() {
		if !bootstrap.Enabled {
			p.app.ShowMessageSafe("Bootstrap is disabled in Bootstrap Settings.")
			return
		}
		timeout, err := parseDuration(timeoutRaw, 2*time.Minute)
		if err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Invalid timeout: %v", err))
			return
		}

		currentLimit := strings.TrimSpace(limit)
		if limitInput != nil {
			currentLimit = strings.TrimSpace(limitInput.GetText())
		}
		resolvedLimit := resolveLimitForScope(scope, currentLimit)

		run := func() {
			closeForm()
			if method == bootstrapMethodAnsible {
				p.runBootstrapAccess(inventory, resolvedLimit, dryRun, strings.Fields(extraArgsRaw), timeout)
				return
			}
			p.runBootstrapDirect(inventory, resolvedLimit, dryRun, timeout)
		}
		if !dryRun && (bootstrap.SetPassword || bootstrap.GrantSudoNOPASSWD) {
			pages.AddPage(bootstrapConfirmPageName,
				p.centerModal(
					tview.NewModal().
						SetText("This will apply privileged bootstrap changes (password and/or NOPASSWD sudo). Continue?").
						AddButtons([]string{"Continue", "Cancel"}).
						SetDoneFunc(func(_ int, label string) {
							pages.RemovePage(bootstrapConfirmPageName)
							if label == "Continue" {
								run()
							}
						}),
					72,
					9,
				),
				true,
				true,
			)
			return
		}

		run()
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event != nil && event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(bootstrapRunPageName, p.centerModal(form, 98, 18), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) showAdhocForm(defaultLimit string, inventory coreansible.InventoryResult, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	form := components.NewStandardForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Run Ping ")
	form.SetTitleColor(theme.Colors.Primary)

	limit := defaultLimit
	scope := detectScopeFromLimit(defaultLimit)
	limitPickerLabels, limitPickerValues := buildLimitPickerOptions(inventory)
	extra := ""
	timeout := "5m"
	var limitInput *tview.InputField

	form.AddDropDown("Scope", []string{bootstrapScopeAll, bootstrapScopeNodes, bootstrapScopeGuests}, scopeToIndex(scope), func(option string, _ int) {
		scope = option
		resolved := resolveLimitForScope(scope, "")
		limit = resolved
		if limitInput != nil {
			limitInput.SetText(resolved)
		}
	})
	form.AddInputField("Limit", defaultLimit, 50, nil, func(text string) { limit = strings.TrimSpace(text) })
	if input, ok := form.GetFormItem(form.GetFormItemCount() - 1).(*tview.InputField); ok {
		limitInput = input
	}
	form.AddDropDown("Target", limitPickerLabels, 0, func(_ string, index int) {
		if index < 0 || index >= len(limitPickerValues) {
			return
		}
		val := strings.TrimSpace(limitPickerValues[index])
		if val == "" {
			return
		}
		limit = val
		if limitInput != nil {
			limitInput.SetText(val)
		}
	})
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
		currentLimit := strings.TrimSpace(limit)
		if limitInput != nil {
			currentLimit = strings.TrimSpace(limitInput.GetText())
		}
		resolvedLimit := resolveLimitForScope(scope, currentLimit)
		closeForm()
		p.runPing(inventory, resolvedLimit, strings.Fields(extra), timeoutDuration)
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event != nil && event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(adhocPageName, p.centerModal(form, 86, 14), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) showPlaybookForm(defaultLimit string, inventory coreansible.InventoryResult, onDone func()) {
	p.showPlaybookFormWithState(defaultLimit, inventory, onDone, p.lastPlaybookFormState)
}

func (p *Plugin) showPlaybookFormWithState(
	defaultLimit string,
	inventory coreansible.InventoryResult,
	onDone func(),
	state *playbookFormState,
) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	form := components.NewStandardForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Run Playbook ")
	form.SetTitleColor(theme.Colors.Primary)

	playbookPath := ""
	scope := detectScopeFromLimit(defaultLimit)
	limit := defaultLimit
	limitPickerLabels, limitPickerValues := buildLimitPickerOptions(inventory)
	extra := ""
	checkMode := false
	timeout := "20m"
	var limitInput *tview.InputField
	if state != nil {
		playbookPath = state.PlaybookPath
		if strings.TrimSpace(state.Scope) != "" {
			scope = strings.TrimSpace(state.Scope)
		}
		limit = state.Limit
		extra = state.ExtraArgsRaw
		checkMode = state.CheckMode
		if strings.TrimSpace(state.TimeoutRaw) != "" {
			timeout = state.TimeoutRaw
		}
	}

	form.AddInputField("Playbook", playbookPath, 60, nil, func(text string) { playbookPath = text })
	form.AddDropDown("Scope", []string{bootstrapScopeAll, bootstrapScopeNodes, bootstrapScopeGuests}, scopeToIndex(scope), func(option string, _ int) {
		scope = option
		resolved := resolveLimitForScope(scope, "")
		limit = resolved
		if limitInput != nil {
			limitInput.SetText(resolved)
		}
	})
	form.AddInputField("Limit", limit, 40, nil, func(text string) { limit = strings.TrimSpace(text) })
	if input, ok := form.GetFormItem(form.GetFormItemCount() - 1).(*tview.InputField); ok {
		limitInput = input
	}
	form.AddDropDown("Target", limitPickerLabels, 0, func(_ string, index int) {
		if index < 0 || index >= len(limitPickerValues) {
			return
		}
		val := strings.TrimSpace(limitPickerValues[index])
		if val == "" {
			return
		}
		limit = val
		if limitInput != nil {
			limitInput.SetText(val)
		}
	})
	form.AddInputField("Extra Args", extra, 60, nil, func(text string) { extra = text })
	form.AddCheckbox("Check Mode", checkMode, func(checked bool) { checkMode = checked })
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

		submitted := &playbookFormState{
			PlaybookPath: strings.TrimSpace(playbookPath),
			Scope:        strings.TrimSpace(scope),
			Limit: func() string {
				currentLimit := strings.TrimSpace(limit)
				if limitInput != nil {
					currentLimit = strings.TrimSpace(limitInput.GetText())
				}
				return resolveLimitForScope(scope, currentLimit)
			}(),
			ExtraArgsRaw: strings.TrimSpace(extra),
			CheckMode:    checkMode,
			TimeoutRaw:   strings.TrimSpace(timeout),
		}
		p.lastPlaybookFormState = submitted

		closeForm()
		p.runPlaybook(inventory, coreansible.PlaybookOptions{
			PlaybookPath: submitted.PlaybookPath,
			Limit:        submitted.Limit,
			ExtraArgs:    strings.Fields(submitted.ExtraArgsRaw),
			CheckMode:    submitted.CheckMode,
		}, timeoutDuration, func() {
			p.showPlaybookFormWithState(defaultLimit, inventory, p.showMainMenu, submitted)
		})
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event != nil && event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(playbookPageName, p.centerModal(form, 92, 19), true, true)
	p.app.SetFocus(form)
}

func resolveLimitForScope(scope, limit string) string {
	manual := strings.TrimSpace(limit)
	if manual != "" {
		return manual
	}

	switch strings.ToLower(strings.TrimSpace(scope)) {
	case bootstrapScopeNodes:
		return "proxmox_nodes"
	case bootstrapScopeGuests:
		return "proxmox_guests"
	default:
		return bootstrapScopeAll
	}
}

func detectScopeFromLimit(limit string) string {
	switch strings.TrimSpace(limit) {
	case "proxmox_nodes":
		return bootstrapScopeNodes
	case "proxmox_guests":
		return bootstrapScopeGuests
	default:
		return bootstrapScopeAll
	}
}

func scopeToIndex(scope string) int {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case bootstrapScopeNodes:
		return 1
	case bootstrapScopeGuests:
		return 2
	default:
		return 0
	}
}

func buildLimitPickerOptions(inventory coreansible.InventoryResult) ([]string, []string) {
	groupSet := make(map[string]struct{})
	hosts := make([]string, 0, len(inventory.Hosts))
	for _, host := range inventory.Hosts {
		if host.Alias != "" {
			hosts = append(hosts, host.Alias)
		}
		for _, group := range host.GroupNames {
			if strings.TrimSpace(group) == "" {
				continue
			}
			groupSet[group] = struct{}{}
		}
	}
	sort.Strings(hosts)

	groups := make([]string, 0, len(groupSet))
	for g := range groupSet {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	labels := []string{"(manual/custom)"}
	values := []string{""}

	labels = append(labels, "Scope: all")
	values = append(values, bootstrapScopeAll)
	labels = append(labels, "Scope: nodes")
	values = append(values, "proxmox_nodes")
	labels = append(labels, "Scope: guests")
	values = append(values, "proxmox_guests")

	for _, group := range groups {
		labels = append(labels, "Group: "+group)
		values = append(values, group)
	}
	for _, host := range hosts {
		labels = append(labels, "Host: "+host)
		values = append(values, host)
	}

	return labels, values
}

func (p *Plugin) runPing(inventory coreansible.InventoryResult, limit string, extraArgs []string, timeout time.Duration) {
	if err := p.runner.CheckAvailability(); err != nil {
		p.app.ShowMessageSafe(fmt.Sprintf("Ansible is not available: %v", err))
		return
	}

	appendLiveLine, closeLive := p.showLiveOutputModal("Running ansible ping...")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	p.setRunningCancel(cancel)

	go func() {
		defer cancel()
		defer p.clearRunningCancel()

		result := p.runner.RunPingStream(
			ctx,
			inventory.Text,
			inventory.Format,
			limit,
			p.mergeConfiguredAnsibleArgs(extraArgs),
			func(line string) {
				appendLiveLine(line)
				ansibleLogger().Debug("ansible ping stream: %s", line)
			},
		)
		p.app.QueueUpdateDraw(func() {
			closeLive()
			title := "Ping Result"
			body := formatCommandResult(result)
			if result.Err != nil {
				title = "Ping Failed"
			}
			p.showOutput(title, body, p.showMainMenu)
		})
	}()
}

func (p *Plugin) runPlaybook(
	inventory coreansible.InventoryResult,
	opts coreansible.PlaybookOptions,
	timeout time.Duration,
	onResultBack func(),
) {
	if err := p.runner.CheckAvailability(); err != nil {
		p.app.ShowMessageSafe(fmt.Sprintf("Ansible is not available: %v", err))
		return
	}

	appendLiveLine, closeLive := p.showLiveOutputModal("Running ansible-playbook...")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	p.setRunningCancel(cancel)

	go func() {
		defer cancel()
		defer p.clearRunningCancel()

		opts.ExtraArgs = p.mergeConfiguredAnsibleArgs(opts.ExtraArgs)
		result := p.runner.RunPlaybookStream(
			ctx,
			inventory.Text,
			inventory.Format,
			opts,
			func(line string) {
				appendLiveLine(line)
				ansibleLogger().Debug("ansible playbook stream: %s", line)
			},
		)
		p.app.QueueUpdateDraw(func() {
			closeLive()
			title := "Playbook Result"
			body := formatCommandResult(result)
			if result.Err != nil {
				title = "Playbook Failed"
			}
			back := p.showMainMenu
			if onResultBack != nil {
				back = onResultBack
			}
			p.showOutput(title, body, back)
		})
	}()
}

func (p *Plugin) runBootstrapAccess(
	inventory coreansible.InventoryResult,
	limit string,
	dryRun bool,
	extraArgs []string,
	timeout time.Duration,
) {
	if err := p.runner.CheckAvailability(); err != nil {
		p.app.ShowMessageSafe(fmt.Sprintf("Ansible is not available: %v", err))
		return
	}

	bootstrap := p.ansiblePluginConfig().Bootstrap
	playbookContent, err := p.buildBootstrapPlaybook(bootstrap)
	if err != nil {
		p.app.ShowMessageSafe(fmt.Sprintf("Bootstrap setup error: %v", err))
		return
	}

	playbookPath, cleanup, err := writeTempPlaybook(playbookContent)
	if err != nil {
		p.app.ShowMessageSafe(fmt.Sprintf("Failed to create bootstrap playbook: %v", err))
		return
	}

	appendLiveLine, closeLive := p.showLiveOutputModal("Running bootstrap access workflow...")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	p.setRunningCancel(cancel)

	go func() {
		defer cancel()
		defer p.clearRunningCancel()
		defer cleanup()

		opts := coreansible.PlaybookOptions{
			PlaybookPath: playbookPath,
			Limit:        strings.TrimSpace(limit),
			CheckMode:    dryRun,
			ExtraArgs:    append([]string{}, extraArgs...),
		}

		if bootstrap.Parallelism > 0 {
			opts.ExtraArgs = append(opts.ExtraArgs, "--forks", strconv.Itoa(bootstrap.Parallelism))
		}

		opts.ExtraArgs = p.mergeConfiguredAnsibleArgs(opts.ExtraArgs)
		result := p.runner.RunPlaybookStream(
			ctx,
			inventory.Text,
			inventory.Format,
			opts,
			func(line string) {
				appendLiveLine(line)
				ansibleLogger().Debug("ansible bootstrap stream: %s", line)
			},
		)

		p.app.QueueUpdateDraw(func() {
			closeLive()
			title := "Bootstrap Result"
			if dryRun {
				title = "Bootstrap Dry-Run Result"
			}
			if result.Err != nil {
				title = "Bootstrap Failed"
			}
			p.showOutput(title, formatCommandResult(result), p.showMainMenu)
		})
	}()
}

type directBootstrapResult struct {
	Alias     string
	Target    string
	Status    string
	Changed   bool
	Message   string
	Output    string
	Transport string
}

func (p *Plugin) runBootstrapDirect(
	inventory coreansible.InventoryResult,
	limit string,
	dryRun bool,
	timeout time.Duration,
) {
	bootstrap := p.ansiblePluginConfig().Bootstrap
	targets := selectDirectBootstrapTargets(inventory.Hosts, limit)
	if len(targets) == 0 {
		p.app.ShowMessageSafe("No matching bootstrap targets for the selected scope/limit.")
		return
	}

	var keyContent string
	if bootstrap.InstallAuthorizedKey {
		keyPath := strings.TrimSpace(bootstrap.SSHPublicKeyFile)
		if keyPath == "" {
			p.app.ShowMessageSafe("Bootstrap ssh_public_key_file is required when install_authorized_key is enabled.")
			return
		}
		// #nosec G304 -- path is a local, user-configured key file for bootstrap.
		data, err := os.ReadFile(keyPath)
		if err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Failed to read SSH public key file: %v", err))
			return
		}
		keyContent = strings.TrimSpace(string(data))
		if keyContent == "" {
			p.app.ShowMessageSafe("SSH public key file is empty.")
			return
		}
	}

	appendLiveLine, closeLive := p.showLiveOutputModal("Running direct bootstrap workflow...")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	p.setRunningCancel(cancel)

	go func() {
		defer cancel()
		defer p.clearRunningCancel()

		results := make([]directBootstrapResult, len(targets))
		parallelism := bootstrap.Parallelism
		if parallelism <= 0 {
			parallelism = 1
		}
		sem := make(chan struct{}, parallelism)
		var wg sync.WaitGroup

		nodeByName := map[string]coreansible.InventoryHost{}
		for _, host := range inventory.Hosts {
			if host.Vars["pvetui_kind"] == hostKindNode {
				nodeByName[host.Vars["pvetui_node"]] = host
			}
		}
		guestByNodeID := map[string]*api.VM{}
		for _, vm := range p.app.VMList().GetVMs() {
			if vm == nil {
				continue
			}
			key := fmt.Sprintf("%s:%d", vm.Node, vm.ID)
			guestByNodeID[key] = vm
		}

		for idx, host := range targets {
			idx, host := idx, host
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					results[idx] = directBootstrapResult{
						Alias:   host.Alias,
						Target:  host.Vars["ansible_host"],
						Status:  statusSkipped,
						Message: "cancelled before execution",
					}
					return
				}

				results[idx] = p.runDirectBootstrapForHost(
					ctx,
					host,
					bootstrap,
					keyContent,
					dryRun,
					nodeByName,
					guestByNodeID,
					func(line string) {
						appendLiveLine(fmt.Sprintf("[%s] %s", host.Alias, line))
						ansibleLogger().Debug("direct bootstrap stream %s: %s", host.Alias, line)
					},
				)
			}()
		}

		wg.Wait()
		p.app.QueueUpdateDraw(func() {
			closeLive()
			title := "Bootstrap Result (Direct)"
			if dryRun {
				title = "Bootstrap Dry-Run Result (Direct)"
			}
			p.showOutput(title, formatDirectBootstrapReport(results, dryRun), p.showMainMenu)
		})
	}()
}

func (p *Plugin) runDirectBootstrapForHost(
	ctx context.Context,
	host coreansible.InventoryHost,
	cfg cfgpkg.AnsibleBootstrapConfig,
	keyContent string,
	dryRun bool,
	nodeByName map[string]coreansible.InventoryHost,
	guestByNodeID map[string]*api.VM,
	stream func(string),
) directBootstrapResult {
	hostKind := strings.TrimSpace(host.Vars["pvetui_kind"])
	guestType := strings.TrimSpace(host.Vars["pvetui_guest_type"])
	target := strings.TrimSpace(host.Vars["ansible_host"])
	user := ""
	switch hostKind {
	case hostKindNode:
		user = strings.TrimSpace(p.resolveNodeUser())
	case hostKindGuest:
		user = strings.TrimSpace(p.resolveVMUser())
	}
	password := ""
	keyPath := ""

	result := directBootstrapResult{
		Alias:  host.Alias,
		Target: target,
		Status: statusOK,
	}
	ansibleLogger().Debug(
		"direct bootstrap host start alias=%s kind=%s guest_type=%s target=%s",
		host.Alias,
		hostKind,
		guestType,
		target,
	)

	if hostKind == hostKindNode && !strings.EqualFold(host.Vars["pvetui_online"], "true") {
		result.Status = statusSkipped
		result.Message = "node is offline"
		return result
	}
	if hostKind == hostKindGuest && !strings.EqualFold(host.Vars["pvetui_status"], api.VMStatusRunning) {
		result.Status = statusSkipped
		result.Message = "guest is not running"
		return result
	}
	if hostKind == hostKindGuest && cfg.ExcludeWindowsGuests && isWindowsGuestHost(host) {
		result.Status = statusSkipped
		result.Message = "windows guest excluded (winrm bootstrap not implemented)"
		return result
	}

	// Nodes only support SSH transport; require a routable IP target.
	if hostKind == hostKindNode {
		if target == "" || user == "" {
			result.Status = statusFailed
			result.Message = "missing ansible_host or SSH user"
			return result
		}
		if net.ParseIP(target) == nil {
			result.Status = statusSkipped
			result.Message = "node target is not an IP address"
			return result
		}
	}

	script := buildDirectBootstrapScript(cfg, keyContent, dryRun)
	targetIsIP := net.ParseIP(target) != nil
	attemptTimeout := directBootstrapAttemptTimeout(dryRun)
	canAttemptSSH := targetIsIP && strings.TrimSpace(user) != ""

	if hostKind == hostKindGuest {
		primaryTransport := ""
		switch guestType {
		case api.VMTypeLXC:
			primaryTransport = transportPCTExec
		case api.VMTypeQemu:
			primaryTransport = transportGuestAgent
		}

		if primaryTransport != "" {
			fallbackCtx, cancelFallback := context.WithTimeout(ctx, attemptTimeout)
			start := time.Now()
			output, transport, err := p.tryGuestBootstrapFallback(
				fallbackCtx,
				host,
				guestType,
				nodeByName,
				guestByNodeID,
				p.resolveNodeUser(),
				script,
				attemptTimeout,
				stream,
			)
			cancelFallback()
			ansibleLogger().Debug(
				"direct bootstrap host=%s transport=%s duration=%s err=%v",
				host.Alias,
				transport,
				time.Since(start),
				err,
			)
			if err == nil {
				result.Output = output
				result.Transport = transport
				return finalizeDirectBootstrapResult(result, output, dryRun, transport)
			}

			if !canAttemptSSH {
				result.Status = statusFailed
				result.Transport = transport
				result.Output = output
				result.Message = err.Error()
				return result
			}

			sshCtx, cancelSSH := context.WithTimeout(ctx, attemptTimeout)
			sshStart := time.Now()
			sshOutput, sshErr := executeRemoteBootstrapScript(sshCtx, user, target, password, keyPath, script, stream)
			cancelSSH()
			ansibleLogger().Debug(
				"direct bootstrap host=%s transport=ssh duration=%s err=%v (after %s failed)",
				host.Alias,
				time.Since(sshStart),
				sshErr,
				primaryTransport,
			)
			result.Output = sshOutput
			result.Transport = transportSSH
			if sshErr != nil {
				result.Status = statusFailed
				result.Message = fmt.Sprintf("%s failed: %v; ssh failed: %v", primaryTransport, err, sshErr)
				return result
			}
			return finalizeDirectBootstrapResult(result, sshOutput, dryRun, transportSSH)
		}
	}

	if !canAttemptSSH {
		result.Status = statusSkipped
		result.Message = "missing routable ansible_host or SSH user for SSH transport"
		return result
	}
	sshCtx, cancelSSH := context.WithTimeout(ctx, attemptTimeout)
	sshStart := time.Now()
	output, err := executeRemoteBootstrapScript(sshCtx, user, target, password, keyPath, script, stream)
	cancelSSH()
	ansibleLogger().Debug("direct bootstrap host=%s transport=ssh duration=%s err=%v", host.Alias, time.Since(sshStart), err)

	result.Output = output
	if result.Transport == "" {
		result.Transport = transportSSH
	}
	if err != nil {
		result.Status = statusFailed
		result.Message = err.Error()
		return result
	}

	return finalizeDirectBootstrapResult(result, output, dryRun, result.Transport)
}

func directBootstrapAttemptTimeout(dryRun bool) time.Duration {
	if dryRun {
		return 15 * time.Second
	}

	return 45 * time.Second
}

func isWindowsGuestHost(host coreansible.InventoryHost) bool {
	guestType := strings.ToLower(strings.TrimSpace(host.Vars["pvetui_guest_type"]))
	if guestType != strings.ToLower(api.VMTypeQemu) {
		return false
	}

	osType := strings.ToLower(strings.TrimSpace(host.Vars["pvetui_guest_os"]))
	if strings.HasPrefix(osType, "win") || strings.Contains(osType, "windows") {
		return true
	}

	name := strings.ToLower(strings.TrimSpace(host.Vars["pvetui_guest_name"]))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(host.Alias))
	}

	// Conservative heuristic: "win" token patterns commonly seen in Windows guest names.
	return strings.Contains(name, "windows") ||
		strings.Contains(name, "win2k") ||
		strings.Contains(name, "win11") ||
		strings.Contains(name, "win10")
}

func executeRemoteBootstrapScript(
	ctx context.Context,
	user, target, password, keyPath, script string,
	stream func(string),
) (string, error) {
	remoteCmd := "sh -s"
	if !strings.EqualFold(strings.TrimSpace(user), "root") {
		remoteCmd = "sudo -n sh -s"
	}

	return executeSSHScript(ctx, user, target, password, keyPath, remoteCmd, script, stream)
}

func executeSSHScript(
	ctx context.Context,
	user, target, password, keyPath, remoteCmd, script string,
	stream func(string),
) (string, error) {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		"-o", "LogLevel=ERROR",
	}
	if strings.TrimSpace(keyPath) != "" {
		sshArgs = append(sshArgs, "-i", strings.TrimSpace(keyPath))
	}
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", user, target))
	sshArgs = append(sshArgs, remoteCmd)

	var cmd *exec.Cmd
	var logCmd []string
	if strings.TrimSpace(password) != "" {
		if _, err := exec.LookPath("sshpass"); err != nil {
			return "", fmt.Errorf("sshpass not found but ansible_password is set for %s", target)
		}
		args := append([]string{"-p", password, "ssh"}, sshArgs...)
		logCmd = append([]string{"sshpass", "-p", "<redacted>", "ssh"}, sshArgs...)
		// #nosec G204 -- command and args are constructed from validated settings.
		cmd = exec.CommandContext(ctx, "sshpass", args...)
	} else {
		logCmd = append([]string{"ssh"}, sshArgs...)
		// #nosec G204 -- command and args are constructed from validated settings.
		cmd = exec.CommandContext(ctx, "ssh", sshArgs...)
	}
	ansibleLogger().Debug("direct bootstrap exec: %s", strings.Join(logCmd, " "))
	cmd.Stdin = strings.NewReader(script)
	output, err := runCommandWithStreaming(cmd, stream)
	if err != nil {
		return strings.TrimSpace(output), fmt.Errorf("remote bootstrap failed: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func executeLXCBootstrapViaPCT(
	ctx context.Context,
	host coreansible.InventoryHost,
	nodeByName map[string]coreansible.InventoryHost,
	nodeSSHUser string,
	script string,
	stream func(string),
) (string, error) {
	vmidRaw := strings.TrimSpace(host.Vars["pvetui_guest_id"])
	nodeName := strings.TrimSpace(host.Vars["pvetui_node"])
	if vmidRaw == "" || nodeName == "" {
		return "", fmt.Errorf("missing guest VMID/node metadata for pct exec fallback")
	}
	vmid, err := strconv.Atoi(vmidRaw)
	if err != nil || vmid <= 0 {
		return "", fmt.Errorf("invalid guest VMID %q for pct exec fallback", vmidRaw)
	}

	nodeHost, ok := nodeByName[nodeName]
	if !ok {
		return "", fmt.Errorf("node %q not found in inventory for pct exec fallback", nodeName)
	}
	nodeTarget := strings.TrimSpace(nodeHost.Vars["ansible_host"])
	nodeUser := strings.TrimSpace(nodeSSHUser)
	nodePassword := ""
	nodeKeyPath := ""
	if nodeTarget == "" || nodeUser == "" {
		return "", fmt.Errorf("missing ansible_host or SSH user on node %q for pct exec fallback", nodeName)
	}
	if net.ParseIP(nodeTarget) == nil {
		return "", fmt.Errorf("node %q target %q is not an IP address for pct exec fallback", nodeName, nodeTarget)
	}

	pctCmd := fmt.Sprintf("pct exec %d -- sh -s", vmid)
	if !strings.EqualFold(nodeUser, "root") {
		pctCmd = "sudo -n " + pctCmd
	}
	ansibleLogger().Debug("direct bootstrap pct fallback command: %s", pctCmd)
	return executeSSHScript(ctx, nodeUser, nodeTarget, nodePassword, nodeKeyPath, pctCmd, script, stream)
}

func (p *Plugin) executeQEMUBootstrapViaGuestAgent(
	ctx context.Context,
	host coreansible.InventoryHost,
	guestByNodeID map[string]*api.VM,
	script string,
	timeout time.Duration,
	stream func(string),
) (string, error) {
	vmidRaw := strings.TrimSpace(host.Vars["pvetui_guest_id"])
	nodeName := strings.TrimSpace(host.Vars["pvetui_node"])
	if vmidRaw == "" || nodeName == "" {
		return "", fmt.Errorf("missing guest VMID/node metadata for guest-agent fallback")
	}
	vmid, err := strconv.Atoi(vmidRaw)
	if err != nil || vmid <= 0 {
		return "", fmt.Errorf("invalid guest VMID %q for guest-agent fallback", vmidRaw)
	}

	key := fmt.Sprintf("%s:%d", nodeName, vmid)
	vm := guestByNodeID[key]
	if vm == nil {
		return "", fmt.Errorf("guest %s not found in VM list for guest-agent fallback", key)
	}

	client := p.app.Client()
	if client == nil {
		return "", fmt.Errorf("api client unavailable for guest-agent fallback")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	// Execute a non-interactive shell snippet through guest agent.
	command := []string{"/bin/sh", "-c", script}
	ansibleLogger().Debug(
		"direct bootstrap guest-agent command: vm=%s node=%s vmid=%d argv=%q script_bytes=%d",
		vm.Name,
		vm.Node,
		vm.ID,
		command[:2],
		len(script),
	)
	stdout, stderr, exitCode, execErr := client.ExecuteGuestAgentCommand(ctx, vm, command, timeout)
	if strings.TrimSpace(stdout) != "" && stream != nil {
		for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
			stream(line)
		}
	}
	if strings.TrimSpace(stderr) != "" && stream != nil {
		for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
			stream(line)
		}
	}
	combined := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(stdout), strings.TrimSpace(stderr)}, "\n"))
	if execErr != nil {
		if combined == "" {
			return combined, fmt.Errorf("guest-agent execution failed: %w", execErr)
		}
		return combined, fmt.Errorf("guest-agent execution failed: %w", execErr)
	}
	if exitCode != 0 {
		if combined == "" {
			return combined, fmt.Errorf("guest-agent command exited with code %d", exitCode)
		}
		return combined, fmt.Errorf("guest-agent command exited with code %d: %s", exitCode, firstLine(combined))
	}

	return combined, nil
}

func (p *Plugin) tryGuestBootstrapFallback(
	ctx context.Context,
	host coreansible.InventoryHost,
	guestType string,
	nodeByName map[string]coreansible.InventoryHost,
	guestByNodeID map[string]*api.VM,
	nodeSSHUser string,
	script string,
	timeout time.Duration,
	stream func(string),
) (output string, transport string, err error) {
	switch guestType {
	case api.VMTypeLXC:
		out, pctErr := executeLXCBootstrapViaPCT(ctx, host, nodeByName, nodeSSHUser, script, stream)
		if pctErr != nil {
			return out, "pct-exec", pctErr
		}
		return out, transportPCTExec, nil
	case api.VMTypeQemu:
		out, gaErr := p.executeQEMUBootstrapViaGuestAgent(ctx, host, guestByNodeID, script, timeout, stream)
		if gaErr != nil {
			return out, transportGuestAgent, gaErr
		}
		return out, transportGuestAgent, nil
	default:
		return "", "", fmt.Errorf("guest type %q has no supported fallback transport", guestType)
	}
}

func finalizeDirectBootstrapResult(
	result directBootstrapResult,
	output string,
	dryRun bool,
	transport string,
) directBootstrapResult {
	result.Output = output
	if transport != "" {
		result.Transport = transport
	}
	if strings.Contains(output, "changed=1") {
		result.Status = statusChanged
		result.Changed = true
		if result.Transport != "" {
			result.Message = fmt.Sprintf("changes applied via %s", result.Transport)
		} else {
			result.Message = "changes applied"
		}
		return result
	}
	if dryRun {
		if result.Transport != "" {
			result.Message = fmt.Sprintf("dry-run completed via %s", result.Transport)
		} else {
			result.Message = "dry-run completed"
		}
	} else {
		if result.Transport != "" {
			result.Message = fmt.Sprintf("already in desired state via %s", result.Transport)
		} else {
			result.Message = "already in desired state"
		}
	}

	return result
}

func buildDirectBootstrapScript(cfg cfgpkg.AnsibleBootstrapConfig, keyContent string, dryRun bool) string {
	userQuoted := shellSingleQuote(strings.TrimSpace(cfg.Username))
	shellQuoted := shellSingleQuote(strings.TrimSpace(cfg.Shell))
	passQuoted := shellSingleQuote(strings.TrimSpace(cfg.Password))
	keyQuoted := shellSingleQuote(strings.TrimSpace(keyContent))
	modeQuoted := shellSingleQuote(strings.TrimSpace(cfg.SudoersFileMode))

	return fmt.Sprintf(`#!/bin/sh
set -eu

BOOTSTRAP_USER=%s
BOOTSTRAP_SHELL=%s
BOOTSTRAP_PASS=%s
BOOTSTRAP_KEY=%s
SUDOERS_MODE=%s
DRY_RUN=%t
CREATE_HOME=%t
INSTALL_KEY=%t
SET_PASSWORD=%t
GRANT_SUDO=%t

changed=0
applied_create_user=0
applied_update_shell=0
applied_install_authorized_key=0
applied_set_password=0
applied_create_sudoers=0

if [ "$DRY_RUN" = "true" ]; then
  would_change=0

  if ! id -u "$BOOTSTRAP_USER" >/dev/null 2>&1; then
    echo "would_create_user=1"
    echo "plan_useradd=useradd -s $BOOTSTRAP_SHELL $BOOTSTRAP_USER"
    would_change=$((would_change+1))
  else
    echo "would_create_user=0"
  fi

  if [ "$INSTALL_KEY" = "true" ] && [ -n "$BOOTSTRAP_KEY" ]; then
    if id -u "$BOOTSTRAP_USER" >/dev/null 2>&1; then
      HOME_DIR="$(getent passwd "$BOOTSTRAP_USER" | cut -d: -f6)"
      AUTH_KEYS="$HOME_DIR/.ssh/authorized_keys"
      if [ ! -f "$AUTH_KEYS" ] || ! grep -qxF "$BOOTSTRAP_KEY" "$AUTH_KEYS"; then
        echo "would_install_authorized_key=1"
        echo "plan_authorized_key=install key to $AUTH_KEYS"
        would_change=$((would_change+1))
      else
        echo "would_install_authorized_key=0"
      fi
    else
      echo "would_install_authorized_key=1"
      echo "plan_authorized_key=install key to /home/$BOOTSTRAP_USER/.ssh/authorized_keys"
      would_change=$((would_change+1))
    fi
  fi

  if [ "$SET_PASSWORD" = "true" ] && [ -n "$BOOTSTRAP_PASS" ]; then
    echo "would_set_password=1"
    echo "plan_set_password=chpasswd for $BOOTSTRAP_USER"
    would_change=$((would_change+1))
  fi

  if [ "$GRANT_SUDO" = "true" ]; then
    if [ ! -f "/etc/sudoers.d/$BOOTSTRAP_USER" ]; then
      echo "would_create_sudoers=1"
      echo "plan_create_sudoers=/etc/sudoers.d/$BOOTSTRAP_USER mode=$SUDOERS_MODE"
      would_change=$((would_change+1))
    else
      echo "would_create_sudoers=0"
    fi
  fi
  echo "would_change_total=$would_change"
  echo "changed=0"
  exit 0
fi

if ! id -u "$BOOTSTRAP_USER" >/dev/null 2>&1; then
  if [ "$CREATE_HOME" = "true" ]; then
    useradd -m -s "$BOOTSTRAP_SHELL" "$BOOTSTRAP_USER"
  else
    useradd -M -s "$BOOTSTRAP_SHELL" "$BOOTSTRAP_USER"
  fi
  changed=1
  applied_create_user=1
fi

# Update shell only if needed.
CURRENT_SHELL="$(getent passwd "$BOOTSTRAP_USER" | cut -d: -f7 || true)"
if [ "$CURRENT_SHELL" != "$BOOTSTRAP_SHELL" ]; then
  usermod -s "$BOOTSTRAP_SHELL" "$BOOTSTRAP_USER"
  changed=1
  applied_update_shell=1
fi
HOME_DIR="$(getent passwd "$BOOTSTRAP_USER" | cut -d: -f6)"

if [ "$INSTALL_KEY" = "true" ] && [ -n "$BOOTSTRAP_KEY" ]; then
  mkdir -p "$HOME_DIR/.ssh"
  chmod 700 "$HOME_DIR/.ssh"
  AUTH_KEYS="$HOME_DIR/.ssh/authorized_keys"
  touch "$AUTH_KEYS"
  chmod 600 "$AUTH_KEYS"
  if ! grep -qxF "$BOOTSTRAP_KEY" "$AUTH_KEYS"; then
    printf "%%s\n" "$BOOTSTRAP_KEY" >> "$AUTH_KEYS"
    changed=1
    applied_install_authorized_key=1
  fi
  chown -R "$BOOTSTRAP_USER:$BOOTSTRAP_USER" "$HOME_DIR/.ssh"
fi

if [ "$SET_PASSWORD" = "true" ] && [ -n "$BOOTSTRAP_PASS" ]; then
  printf "%%s:%%s\n" "$BOOTSTRAP_USER" "$BOOTSTRAP_PASS" | chpasswd
  changed=1
  applied_set_password=1
fi

if [ "$GRANT_SUDO" = "true" ]; then
  SUDOERS_FILE="/etc/sudoers.d/$BOOTSTRAP_USER"
  if [ ! -f "$SUDOERS_FILE" ]; then
    printf "%%s ALL=(ALL) NOPASSWD:ALL\n" "$BOOTSTRAP_USER" > "$SUDOERS_FILE"
    chmod "$SUDOERS_MODE" "$SUDOERS_FILE"
    if command -v visudo >/dev/null 2>&1; then
      visudo -cf "$SUDOERS_FILE"
    fi
    changed=1
    applied_create_sudoers=1
  fi
fi

echo "applied_create_user=$applied_create_user"
echo "applied_update_shell=$applied_update_shell"
echo "applied_install_authorized_key=$applied_install_authorized_key"
echo "applied_set_password=$applied_set_password"
echo "applied_create_sudoers=$applied_create_sudoers"
echo "changed=$changed"
`, userQuoted, shellQuoted, passQuoted, keyQuoted, modeQuoted, dryRun, cfg.CreateHome, cfg.InstallAuthorizedKey, cfg.SetPassword, cfg.GrantSudoNOPASSWD)
}

func selectDirectBootstrapTargets(hosts []coreansible.InventoryHost, limit string) []coreansible.InventoryHost {
	out := make([]coreansible.InventoryHost, 0, len(hosts))
	for _, host := range hosts {
		if matchesBootstrapLimit(host, limit) {
			out = append(out, host)
		}
	}
	return out
}

func matchesBootstrapLimit(host coreansible.InventoryHost, rawLimit string) bool {
	limit := strings.TrimSpace(rawLimit)
	if limit == "" || strings.EqualFold(limit, bootstrapScopeAll) {
		return true
	}

	parts := strings.Split(limit, ",")
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if token == host.Alias {
			return true
		}
		for _, group := range host.GroupNames {
			if token == group {
				return true
			}
		}
		if strings.ContainsAny(token, "*?[]") {
			if ok, _ := filepath.Match(token, host.Alias); ok {
				return true
			}
		}
	}

	return false
}

func formatDirectBootstrapReport(results []directBootstrapResult, dryRun bool) string {
	var b strings.Builder
	title := "Direct bootstrap summary"
	if dryRun {
		title = "Direct bootstrap dry-run summary"
	}
	b.WriteString(title + "\n\n")

	totalChanged := 0
	totalOK := 0
	totalFailed := 0
	totalSkipped := 0

	for _, r := range results {
		switch r.Status {
		case statusChanged:
			totalChanged++
		case statusFailed:
			totalFailed++
		case statusSkipped:
			totalSkipped++
		default:
			totalOK++
		}
		_, _ = fmt.Fprintf(&b, "- %s (%s) [%s]: %s\n", r.Alias, r.Target, r.Status, r.Message)
		if dryRun {
			plan := extractDryRunPlan(r.Output)
			if plan != "" {
				_, _ = fmt.Fprintf(&b, "  plan: %s\n", plan)
				continue
			}
		}
		applied := extractAppliedPlan(r.Output)
		if applied != "" {
			_, _ = fmt.Fprintf(&b, "  applied: %s\n", applied)
			continue
		}
		if strings.TrimSpace(r.Output) != "" {
			_, _ = fmt.Fprintf(&b, "  output: %s\n", firstLine(r.Output))
		}
	}

	_, _ = fmt.Fprintf(&b, "\nTotals: ok=%d changed=%d failed=%d skipped=%d\n", totalOK, totalChanged, totalFailed, totalSkipped)
	return b.String()
}

func firstLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func extractDryRunPlan(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	plans := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "would_") || strings.HasPrefix(trimmed, "plan_") || strings.HasPrefix(trimmed, "applied_") {
			plans = append(plans, trimmed)
		}
	}
	return strings.Join(plans, ", ")
}

func extractAppliedPlan(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	applied := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "applied_") {
			applied = append(applied, trimmed)
		}
	}
	return strings.Join(applied, ", ")
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func (p *Plugin) buildBootstrapPlaybook(cfg cfgpkg.AnsibleBootstrapConfig) (string, error) {
	username := strings.TrimSpace(cfg.Username)
	if username == "" {
		return "", fmt.Errorf("bootstrap username is required")
	}

	var keyContent string
	if cfg.InstallAuthorizedKey {
		keyPath := strings.TrimSpace(cfg.SSHPublicKeyFile)
		if keyPath == "" {
			return "", fmt.Errorf("ssh public key file is required when install_authorized_key is enabled")
		}
		// #nosec G304 -- path is a local, user-configured key file for bootstrap.
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return "", fmt.Errorf("read ssh public key file: %w", err)
		}
		keyContent = strings.TrimSpace(string(data))
		if keyContent == "" {
			return "", fmt.Errorf("ssh public key file is empty")
		}
	}

	if cfg.SetPassword && strings.TrimSpace(cfg.Password) == "" {
		return "", fmt.Errorf("password is required when set_password is enabled")
	}

	quotedKey := strconv.Quote(keyContent)
	quotedPassword := strconv.Quote(strings.TrimSpace(cfg.Password))
	quotedUsername := strconv.Quote(username)
	quotedShell := strconv.Quote(strings.TrimSpace(cfg.Shell))
	quotedMode := strconv.Quote(strings.TrimSpace(cfg.SudoersFileMode))

	playbook := fmt.Sprintf(`---
- name: pvetui bootstrap ansible access
  hosts: all
  gather_facts: false
  become: true
  any_errors_fatal: %t
  vars:
    bootstrap_username: %s
    bootstrap_shell: %s
    bootstrap_create_home: %t
    bootstrap_install_authorized_key: %t
    bootstrap_authorized_key: %s
    bootstrap_set_password: %t
    bootstrap_password: %s
    bootstrap_grant_sudo_nopasswd: %t
    bootstrap_sudoers_file_mode: %s
    bootstrap_sudoers_path: "/etc/sudoers.d/{{ bootstrap_username }}"

  tasks:
    - name: Ensure bootstrap user exists
      ansible.builtin.user:
        name: "{{ bootstrap_username }}"
        shell: "{{ bootstrap_shell }}"
        create_home: "{{ bootstrap_create_home }}"
        state: present

    - name: Ensure ~/.ssh exists for bootstrap user
      ansible.builtin.file:
        path: "/home/{{ bootstrap_username }}/.ssh"
        state: directory
        owner: "{{ bootstrap_username }}"
        group: "{{ bootstrap_username }}"
        mode: "0700"
      when: bootstrap_install_authorized_key

    - name: Install authorized key for bootstrap user
      ansible.posix.authorized_key:
        user: "{{ bootstrap_username }}"
        key: "{{ bootstrap_authorized_key }}"
        state: present
      when:
        - bootstrap_install_authorized_key
        - bootstrap_authorized_key | length > 0

    - name: Set bootstrap user password
      ansible.builtin.user:
        name: "{{ bootstrap_username }}"
        password: "{{ bootstrap_password | password_hash('sha512', bootstrap_username) }}"
        update_password: always
      when:
        - bootstrap_set_password
        - bootstrap_password | length > 0

    - name: Check if bootstrap sudoers file exists
      ansible.builtin.stat:
        path: "{{ bootstrap_sudoers_path }}"
      register: bootstrap_sudoers_stat
      when: bootstrap_grant_sudo_nopasswd

    - name: Create bootstrap sudoers file if absent
      ansible.builtin.copy:
        dest: "{{ bootstrap_sudoers_path }}"
        content: "{{ bootstrap_username }} ALL=(ALL) NOPASSWD:ALL\n"
        owner: root
        group: root
        mode: "{{ bootstrap_sudoers_file_mode }}"
        validate: "/usr/sbin/visudo -cf %%s"
        force: false
      when:
        - bootstrap_grant_sudo_nopasswd
        - not bootstrap_sudoers_stat.stat.exists
`, cfg.FailFast, quotedUsername, quotedShell, cfg.CreateHome, cfg.InstallAuthorizedKey, quotedKey, cfg.SetPassword, quotedPassword, cfg.GrantSudoNOPASSWD, quotedMode)

	return playbook, nil
}

func (p *Plugin) showSaveInventoryForm(inventory coreansible.InventoryResult, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	form := components.NewStandardForm()
	form.SetBorder(true)
	form.SetBorderColor(theme.Colors.Border)
	form.SetTitle(" Save Inventory ")
	form.SetTitleColor(theme.Colors.Primary)

	defaultPath := filepath.Join(defaultHomeDir(), "ansible", defaultInventoryFilename(inventory.Format))
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

		if err := coreansible.SaveInventory(targetPath, inventory.Text); err != nil {
			p.app.ShowMessageSafe(fmt.Sprintf("Failed to save inventory: %v", err))
			return
		}

		closeForm()
		p.app.ShowMessageSafe(fmt.Sprintf("Inventory saved to %s", targetPath))
	})
	form.AddButton("Cancel", closeForm)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event != nil && event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage(savePathPageName, p.centerModal(form, 96, 9), true, true)
	p.app.SetFocus(form)
}

func (p *Plugin) showSetupAssistant(inventory coreansible.InventoryResult, onDone func()) {
	pages := p.app.Pages()
	pages.RemovePage(menuPageName)

	text := tview.NewTextView()
	text.SetBorder(true)
	text.SetBorderColor(theme.Colors.Border)
	text.SetTitle(" SSH Setup Guide ")
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

	pages.AddPage(setupPageName, p.centerModal(text, 110, 28), true, true)
	p.app.SetFocus(text)
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

	pages.AddPage(outputPageName, p.centerModal(output, 110, 30), true, true)
	p.app.SetFocus(output)
}

func (p *Plugin) currentInventory() coreansible.InventoryResult {
	nodes := p.app.NodeList().GetNodes()
	guests := p.app.VMList().GetVMs()
	ansibleCfg := p.ansiblePluginConfig()

	defaults := coreansible.InventoryDefaults{
		NodeSSHUser:       p.resolveNodeUser(),
		VMSSHUser:         p.resolveVMUser(),
		SSHPrivateKeyFile: strings.TrimSpace(ansibleCfg.SSHPrivateKeyFile),
		DefaultPassword:   strings.TrimSpace(ansibleCfg.DefaultPassword),
		InventoryVars:     cloneStringMap(ansibleCfg.InventoryVars),
		Style:             coreansible.NormalizeInventoryStyle(ansibleCfg.InventoryStyle),
	}
	if user := strings.TrimSpace(ansibleCfg.DefaultUser); user != "" {
		defaults.NodeSSHUser = user
		defaults.VMSSHUser = user
	}

	return coreansible.BuildInventoryWithFormat(nodes, guests, defaults, ansibleCfg.InventoryFormat)
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
	switch strings.ToLower(strings.TrimSpace(p.ansiblePluginConfig().DefaultLimitMode)) {
	case "none":
		return ""
	case "all":
		return "all"
	}

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

func (p *Plugin) mergeConfiguredAnsibleArgs(userArgs []string) []string {
	cfg := p.ansiblePluginConfig()
	merged := make([]string, 0, len(cfg.ExtraArgs)+len(userArgs)+2)
	merged = append(merged, cfg.ExtraArgs...)
	if cfg.AskPass {
		merged = append(merged, "--ask-pass")
	}
	if cfg.AskBecomePass {
		merged = append(merged, "--ask-become-pass")
	}
	merged = append(merged, userArgs...)

	return merged
}

func (p *Plugin) ansiblePluginConfig() cfgpkg.AnsiblePluginConfig {
	cfg := p.app.Config()
	if cfg == nil {
		return cfgpkg.AnsiblePluginConfig{
			InventoryFormat:  coreansible.InventoryFormatYAML,
			InventoryStyle:   coreansible.InventoryStyleCompact,
			DefaultLimitMode: "selection",
			Bootstrap: cfgpkg.AnsibleBootstrapConfig{
				Username:             "ansible",
				Shell:                "/bin/bash",
				CreateHome:           true,
				ExcludeWindowsGuests: true,
				InstallAuthorizedKey: true,
				SudoersFileMode:      "0440",
				DryRunDefault:        true,
				Parallelism:          10,
				Timeout:              "2m",
			},
		}
	}

	return cfg.Plugins.Ansible
}

func defaultInventoryFilename(format string) string {
	if coreansible.NormalizeInventoryFormat(format) == coreansible.InventoryFormatYAML {
		return "pvetui-inventory.yml"
	}

	return "pvetui-inventory.ini"
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func formatInventoryVarsYAML(vars map[string]string) string {
	if len(vars) == 0 {
		return ""
	}

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(map[string]string, len(vars))
	for _, k := range keys {
		ordered[k] = vars[k]
	}

	data, err := yaml.Marshal(ordered)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

func parseInventoryVarsYAML(raw string) (map[string]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	parsed := map[string]string{}
	if err := yaml.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("must be valid YAML key/value map: %w", err)
	}

	clean := make(map[string]string, len(parsed))
	for key, value := range parsed {
		k := strings.TrimSpace(key)
		if k == "" {
			return nil, fmt.Errorf("empty key is not allowed")
		}
		clean[k] = strings.TrimSpace(value)
	}

	return clean, nil
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
	inventoryFile := defaultInventoryFilename(inventory.Format)

	b.WriteString("[primary]Ansible SSH Setup Guide[-]\n\n")
	b.WriteString("1) Generate a dedicated SSH key (optional):\n")
	b.WriteString("   ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_pvetui -C \"pvetui-ansible\"\n\n")

	b.WriteString("2) Copy your key to a target host (example):\n")
	exampleUser := "ansible"
	exampleHost := "host.example.local"
	for _, host := range inventory.Hosts {
		user := strings.TrimSpace(host.Vars["ansible_user"])
		target := strings.TrimSpace(host.Vars["ansible_host"])
		if user == "" || target == "" {
			continue
		}
		exampleUser = user
		exampleHost = target
		break
	}
	_, _ = fmt.Fprintf(&b, "   ssh-copy-id %s@%s\n", exampleUser, exampleHost)
	b.WriteString("\n")

	b.WriteString("3) Optional ansible.cfg defaults:\n")
	b.WriteString("   [defaults]\n")
	b.WriteString("   host_key_checking = True\n")
	b.WriteString("   timeout = 30\n")
	b.WriteString("\n")

	b.WriteString("4) Validate connectivity:\n")
	_, _ = fmt.Fprintf(&b, "   ansible -i ./%s all -m ping\n\n", inventoryFile)

	b.WriteString("5) Example run:\n")
	_, _ = fmt.Fprintf(&b, "   ansible-playbook -i ./%s site.yml\n\n", inventoryFile)

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

func (p *Plugin) centerModal(content tview.Primitive, width, height int) tview.Primitive {
	screenW, screenH := 120, 40
	if p != nil && p.app != nil && p.app.Pages() != nil {
		_, _, w, h := p.app.Pages().GetRect()
		if w > 0 && h > 0 {
			screenW, screenH = w, h
		}
	}

	maxWidth := maxInt(24, screenW-2)
	maxHeight := maxInt(6, screenH-2)
	width = clampInt(width, 24, maxWidth)
	height = clampInt(height, 6, maxHeight)

	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(content, height, 0, true).
				AddItem(nil, 0, 1, false),
			width,
			0,
			true,
		).
		AddItem(nil, 0, 1, false)
}

func clampInt(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func writeTempPlaybook(content string) (path string, cleanup func(), err error) {
	tmpFile, err := os.CreateTemp("", "pvetui-ansible-bootstrap-*.yml")
	if err != nil {
		return "", nil, err
	}

	defer func() {
		if err != nil {
			_ = tmpFile.Close()
			// #nosec G703 -- temp path is created by os.CreateTemp.
			_ = os.Remove(tmpFile.Name())
		}
	}()

	if _, err = tmpFile.WriteString(content); err != nil {
		return "", nil, err
	}
	if err = tmpFile.Close(); err != nil {
		return "", nil, err
	}
	// #nosec G703 -- temp path is created by os.CreateTemp.
	if err = os.Chmod(tmpFile.Name(), 0o600); err != nil {
		return "", nil, err
	}

	cleanup = func() {
		// #nosec G703 -- temp path is created by os.CreateTemp.
		_ = os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup, nil
}

func (p *Plugin) showLiveOutputModal(title string) (appendLine func(string), closeModalFn func()) {
	pages := p.app.Pages()
	pages.RemovePage(liveOutputPageName)

	text := tview.NewTextView()
	text.SetBorder(true)
	text.SetBorderColor(theme.Colors.Border)
	text.SetTitle(" " + title + " ")
	text.SetTitleColor(theme.Colors.Primary)
	text.SetDynamicColors(false)
	text.SetWrap(true)
	text.SetWordWrap(true)
	text.SetScrollable(true)
	text.SetText("(streaming output...)\n")

	closeModal := func() {
		pages.RemovePage(liveOutputPageName)
	}

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) || (event.Key() == tcell.KeyRune && (event.Rune() == 'q' || event.Rune() == 'Q')) {
			p.cancelRunningCommand()
			closeModal()
			return nil
		}
		return event
	})

	pages.AddPage(liveOutputPageName, p.centerModal(text, 120, 30), true, true)
	p.app.SetFocus(text)

	appendFn := func(line string) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return
		}
		p.app.QueueUpdateDraw(func() {
			_, _ = fmt.Fprintln(text, trimmed)
			text.ScrollToEnd()
		})
	}

	return appendFn, closeModal
}

func runCommandWithStreaming(cmd *exec.Cmd, stream func(string)) (string, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
		b  bytes.Buffer
	)
	appendLine := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
		if stream != nil {
			stream(line)
		}
	}
	readPipe := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		const maxScanToken = 1024 * 1024
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, maxScanToken)
		for scanner.Scan() {
			appendLine(scanner.Text())
		}
	}

	wg.Add(2)
	go readPipe(stdoutPipe)
	go readPipe(stderrPipe)

	waitErr := cmd.Wait()
	wg.Wait()

	return b.String(), waitErr
}

func ansibleLogger() interface {
	Debug(format string, args ...interface{})
} {
	return logger.GetGlobalLogger()
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
