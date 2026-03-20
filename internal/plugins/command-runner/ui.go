package commandrunner

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UIApp interface defines the minimal app methods needed by UIManager
type UIApp interface {
	Pages() *tview.Pages
	SetFocus(p tview.Primitive) *tview.Application
	QueueUpdateDraw(func()) *tview.Application
}

// UIManager handles UI interactions for the command runner plugin
type UIManager struct {
	app       UIApp
	executor  *Executor
	vmTargets map[string]VM
}

func isBackKey(event *tcell.EventKey) bool {
	if event == nil {
		return false
	}

	return event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2
}

// NewUIManager creates a new UI manager
func NewUIManager(app UIApp, executor *Executor) *UIManager {
	return &UIManager{
		app:       app,
		executor:  executor,
		vmTargets: make(map[string]VM),
	}
}

// ShowCommandMenu displays a list of available commands for selection
func (u *UIManager) ShowCommandMenu(targetType TargetType, target string, onClose func()) {
	commands := u.executor.GetAllowedCommands(targetType)
	u.showCommandMenu(targetType, target, commands, onClose)
}

// ShowVMCommandMenu displays a list of commands tailored to a VM's OS.
func (u *UIManager) ShowVMCommandMenu(vm VM, onClose func()) {
	target := fmt.Sprintf("%s/%d", vm.Node, vm.ID)
	u.vmTargets[target] = vm
	commands := u.executor.GetAllowedVMCommands(vm)
	u.showCommandMenu(TargetVM, target, commands, onClose)
}

func (u *UIManager) showCommandMenu(targetType TargetType, target string, commands []string, onClose func()) {
	if len(commands) == 0 {
		u.ShowErrorModal("No commands available", "No commands configured for this target type", onClose)
		return
	}

	pages := u.app.Pages()

	// Close function that removes the page and calls onClose callback
	closeMenu := func() {
		pages.RemovePage("commandMenu")
		if onClose != nil {
			onClose()
		}
	}

	// Create list of commands
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(fmt.Sprintf(" Run Command on %s (%s) ", target, targetType))

	returnToMenu := func() {
		u.app.SetFocus(list)
	}

	// addToList inserts a newly whitelisted command at index 1 (right after "Custom
	// Command...") so it appears at the top of the whitelist section immediately.
	addToList := func(cmd string) {
		desc := GetCommandDescription(cmd)
		cmdCopy := cmd
		list.InsertItem(1, cmdCopy, desc, 0, func() {
			u.handleCommandSelection(targetType, target, cmdCopy, returnToMenu)
		})
	}

	// Custom command entry first so it is always visible at the top.
	list.AddItem("Custom Command...", "Type and run any non-interactive command", '!', func() {
		u.showCustomCommandForm(targetType, target, returnToMenu, addToList)
	})

	for _, cmd := range commands {
		cmdCopy := cmd // Capture for closure
		description := GetCommandDescription(cmdCopy)
		list.AddItem(cmdCopy, description, 0, func() {
			u.handleCommandSelection(targetType, target, cmdCopy, returnToMenu)
		})
	}

	// Add cancel option
	list.AddItem("Cancel", "Press to close", 'q', closeMenu)

	// Set input handler for Esc key and vi-style navigation
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeMenu()
			return nil
		}
		// Vi-style navigation
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'j': // Down
				currentItem := list.GetCurrentItem()
				if currentItem < list.GetItemCount()-1 {
					list.SetCurrentItem(currentItem + 1)
				}
				return nil
			case 'k': // Up
				currentItem := list.GetCurrentItem()
				if currentItem > 0 {
					list.SetCurrentItem(currentItem - 1)
				}
				return nil
			}
		}
		return event
	})

	pages.AddPage("commandMenu", list, true, true)
	u.app.SetFocus(list)
}

// handleCommandSelection processes command selection and prompts for parameters if needed
func (u *UIManager) handleCommandSelection(targetType TargetType, target, command string, onResultClosed func()) {
	template := ParseTemplate(command)

	if len(template.Parameters) == 0 {
		// No parameters, execute directly
		u.executeAndShowResult(targetType, target, command, nil, onResultClosed)
	} else {
		// Has parameters, show input form
		u.showParameterForm(targetType, target, command, template, onResultClosed)
	}
}

// showParameterForm displays a form to collect parameter values
func (u *UIManager) showParameterForm(targetType TargetType, target, command string, template CommandTemplate, onReturn func()) {
	pages := u.app.Pages()

	// Close function that removes the form page
	closeForm := func() {
		pages.RemovePage("parameterForm")
		if onReturn != nil {
			onReturn()
		}
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" Parameters for: %s ", command))

	params := make(map[string]string)

	// Add input field for each parameter
	for _, param := range template.Parameters {
		paramCopy := param // Capture for closure
		form.AddInputField(paramCopy, "", 30, nil, func(text string) {
			params[paramCopy] = text
		})
	}

	// Add buttons
	form.AddButton("Execute", func() {
		pages.RemovePage("parameterForm")
		u.executeAndShowResult(targetType, target, command, params, onReturn)
	})

	form.AddButton("Cancel", closeForm)

	// Only intercept ESC — backspace must remain available for text editing in input fields.
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage("parameterForm", form, true, true)
}

// executeAndShowResult executes the command and displays the result
func (u *UIManager) executeAndShowResult(targetType TargetType, target, command string, params map[string]string, onClose func()) {
	// Show "executing" modal
	u.showExecutingModal(command)

	// Execute command in goroutine to keep UI responsive
	go func() {
		var result ExecutionResult
		ctx := context.Background() // TODO: Get context from app if available

		if params != nil {
			// Has template parameters
			switch targetType {
			case TargetHost:
				result = u.executor.ExecuteTemplatedCommand(ctx, targetType, target, command, params)
			case TargetContainer:
				// Parse target format: "node/vmid"
				node, containerID, err := parseContainerTarget(target)
				if err != nil {
					result = ExecutionResult{
						Command: command,
						Error:   fmt.Errorf("invalid target format: %w", err),
					}
				} else {
					result = u.executor.ExecuteTemplatedContainerCommand(ctx, node, containerID, command, params)
				}
			case TargetVM:
				// Parse target format: "node/vmid"
				vm, err := u.vmFromTarget(target)
				if err != nil {
					result = ExecutionResult{
						Command: command,
						Error:   fmt.Errorf("invalid target format: %w", err),
					}
				} else {
					result = u.executor.ExecuteTemplatedVMCommand(ctx, vm, command, params)
				}
			default:
				result = ExecutionResult{
					Command: command,
					Error:   fmt.Errorf("unsupported target type: %s", targetType),
				}
			}
		} else {
			// No template parameters, execute directly based on target type
			switch targetType {
			case TargetHost:
				result = u.executor.ExecuteHostCommand(ctx, target, command)
			case TargetContainer:
				// Parse target format: "node/vmid"
				node, containerID, err := parseContainerTarget(target)
				if err != nil {
					result = ExecutionResult{
						Command: command,
						Error:   fmt.Errorf("invalid target format: %w", err),
					}
				} else {
					result = u.executor.ExecuteContainerCommand(ctx, node, containerID, command)
				}
			case TargetVM:
				// Parse target format: "node/vmid"
				vm, err := u.vmFromTarget(target)
				if err != nil {
					result = ExecutionResult{
						Command: command,
						Error:   fmt.Errorf("invalid target format: %w", err),
					}
				} else {
					result = u.executor.ExecuteVMCommand(ctx, vm, command)
				}
			default:
				result = ExecutionResult{
					Command: command,
					Error:   fmt.Errorf("unsupported target type: %s", targetType),
				}
			}
		}

		// Update UI with result
		u.app.QueueUpdateDraw(func() {
			u.ShowResultModal(result, onClose)
		})
	}()
}

// showExecutingModal displays a modal indicating command is running
func (u *UIManager) showExecutingModal(command string) {
	pages := u.app.Pages()

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Executing:\n\n%s\n\nPlease wait...", command)).
		AddButtons([]string{"Running"})

	pages.AddPage("executingCommand", modal, true, true)
}

// ShowResultModal displays the command execution result in a modal
func (u *UIManager) ShowResultModal(result ExecutionResult, onClose func()) {
	pages := u.app.Pages()

	// Remove the "executing" modal first
	pages.RemovePage("executingCommand")

	// Close function that removes the result page
	closeResult := func() {
		pages.RemovePage("commandResult")
		if onClose != nil {
			onClose()
		}
	}

	var text strings.Builder

	fmt.Fprintf(&text, "Command: %s\n", result.Command)
	fmt.Fprintf(&text, "Duration: %v\n\n", result.Duration)

	if result.Error != nil {
		fmt.Fprintf(&text, "Error: %v\n\n", result.Error)
		if result.Output != "" {
			text.WriteString("Output:\n")
			text.WriteString(result.Output)
		}
	} else {
		text.WriteString("Output:\n")
		text.WriteString(result.Output)

		if result.Truncated {
			text.WriteString("\n\n[Output truncated]")
		}
	}

	// Create text view for scrollable output
	textView := tview.NewTextView().
		SetText(text.String()).
		SetDynamicColors(true).
		SetScrollable(true)

	textView.SetBorder(true)
	textView.SetTitle(" Command Result ")

	// Set input handler on the text view (which will have focus)
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeResult()
			return nil // Consume the event to prevent bubbling
		}
		return event
	})

	// Add button bar at bottom with color tags
	buttons := tview.NewTextView().
		SetText(" [primary]ESC/Backspace[-] Close | [primary]↑/↓[-] Scroll ").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Create flex layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true).
		AddItem(buttons, 1, 0, false)

	pages.AddPage("commandResult", flex, true, true)
}

// showCustomCommandForm displays an input form for the user to type an arbitrary command.
// Commands run non-interactively: sudo requiring a password and interactive programs
// (vim, top, etc.) will fail with an error from the remote end.
func (u *UIManager) showCustomCommandForm(targetType TargetType, target string, onReturn func(), onAddToWhitelist func(string)) {
	pages := u.app.Pages()

	closeForm := func() {
		pages.RemovePage("customCommandForm")
		if onReturn != nil {
			onReturn()
		}
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" Custom Command on %s (%s) — non-interactive only ", target, targetType))

	var command string
	form.AddInputField("Command", "", 60, nil, func(text string) {
		command = text
	})

	form.AddButton("Execute", func() {
		cmd := strings.TrimSpace(command)
		if cmd == "" {
			return
		}
		pages.RemovePage("customCommandForm")
		u.executeCustomAndShowResult(targetType, target, cmd, onReturn, onAddToWhitelist)
	})

	form.AddButton("Cancel", closeForm)

	// Only intercept ESC — backspace must remain available for text editing in input fields.
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeForm()
			return nil
		}
		return event
	})

	pages.AddPage("customCommandForm", form, true, true)
}

// executeCustomAndShowResult runs a custom command (no whitelist check) and shows the result.
// It mirrors executeAndShowResult but calls the custom execution paths.
func (u *UIManager) executeCustomAndShowResult(targetType TargetType, target, command string, onClose func(), onAddToWhitelist func(string)) {
	u.showExecutingModal(command)

	go func() {
		var result ExecutionResult
		ctx := context.Background()

		switch targetType {
		case TargetHost:
			result = u.executor.ExecuteCustomHostCommand(ctx, target, command)
		case TargetContainer:
			node, containerID, err := parseContainerTarget(target)
			if err != nil {
				result = ExecutionResult{
					Command: command,
					Error:   fmt.Errorf("invalid target format: %w", err),
				}
			} else {
				result = u.executor.ExecuteCustomContainerCommand(ctx, node, containerID, command)
			}
		case TargetVM:
			vm, err := u.vmFromTarget(target)
			if err != nil {
				result = ExecutionResult{
					Command: command,
					Error:   fmt.Errorf("invalid target format: %w", err),
				}
			} else {
				result = u.executor.ExecuteCustomVMCommand(ctx, vm, command)
			}
		default:
			result = ExecutionResult{
				Command: command,
				Error:   fmt.Errorf("unsupported target type: %s", targetType),
			}
		}

		u.app.QueueUpdateDraw(func() {
			u.showCustomResultModal(result, targetType, onClose, onAddToWhitelist)
		})
	}()
}

// showCustomResultModal is like ShowResultModal but adds a "Save to Whitelist" button
// so the user can promote a successful custom command into the session whitelist.
func (u *UIManager) showCustomResultModal(result ExecutionResult, targetType TargetType, onClose func(), onAddToWhitelist func(string)) {
	pages := u.app.Pages()
	pages.RemovePage("executingCommand")

	closeResult := func() {
		pages.RemovePage("commandResult")
		if onClose != nil {
			onClose()
		}
	}

	var text strings.Builder
	fmt.Fprintf(&text, "Command: %s\n", result.Command)
	fmt.Fprintf(&text, "Duration: %v\n\n", result.Duration)

	if result.Error != nil {
		fmt.Fprintf(&text, "Error: %v\n\n", result.Error)
		if result.Output != "" {
			text.WriteString("Output:\n")
			text.WriteString(result.Output)
		}
	} else {
		text.WriteString("Output:\n")
		text.WriteString(result.Output)
		if result.Truncated {
			text.WriteString("\n\n[Output truncated]")
		}
	}

	textView := tview.NewTextView().
		SetText(text.String()).
		SetDynamicColors(true).
		SetScrollable(true)
	textView.SetBorder(true)
	textView.SetTitle(" Custom Command Result ")

	// Build hint line; include whitelist shortcut only when the command succeeded.
	hintText := " [primary]ESC/Backspace[-] Close | [primary]↑/↓[-] Scroll"
	if result.Error == nil {
		hintText += fmt.Sprintf(" | [primary]w[-] Save to %s Whitelist (session)", targetType)
	}
	hintText += " "

	buttons := tview.NewTextView().
		SetText(hintText).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true).
		AddItem(buttons, 1, 0, false)

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeResult()
			return nil
		}
		if result.Error == nil && event.Key() == tcell.KeyRune && event.Rune() == 'w' {
			u.executor.AddToWhitelist(targetType, result.Command)
			if onAddToWhitelist != nil {
				onAddToWhitelist(result.Command)
			}
			// Update the hint to confirm the save (disable 'w' to prevent duplicates).
			buttons.SetText(fmt.Sprintf(" [green]Saved to %s whitelist (session only)[-] | [primary]ESC/Backspace[-] Close | [primary]↑/↓[-] Scroll ", targetType))
			return nil
		}
		return event
	})

	pages.AddPage("commandResult", flex, true, true)
}

// ShowErrorModal displays an error message in a modal
func (u *UIManager) ShowErrorModal(title, message string, onClose func()) {
	pages := u.app.Pages()

	closeError := func() {
		pages.RemovePage("commandError")
		if onClose != nil {
			onClose()
		}
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("%s\n\n%s", title, message)).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			closeError()
		})
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isBackKey(event) {
			closeError()
			return nil
		}

		return event
	})

	pages.AddPage("commandError", modal, true, true)
}

// parseContainerTarget parses a container target string in the format "node/vmid"
// and returns the node name and container ID.
func parseContainerTarget(target string) (node string, containerID int, err error) {
	parts := strings.Split(target, "/")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("expected format 'node/vmid', got: %s", target)
	}

	var vmid int
	if _, err := fmt.Sscanf(parts[1], "%d", &vmid); err != nil {
		return "", 0, fmt.Errorf("invalid container ID '%s': %w", parts[1], err)
	}

	return parts[0], vmid, nil
}

// parseVMTarget parses a VM target string in the format "node/vmid"
// and returns a VM struct for guest agent execution.
func parseVMTarget(target string) (VM, error) {
	parts := strings.Split(target, "/")
	if len(parts) != 2 {
		return VM{}, fmt.Errorf("expected format 'node/vmid', got: %s", target)
	}

	var vmid int
	if _, err := fmt.Sscanf(parts[1], "%d", &vmid); err != nil {
		return VM{}, fmt.Errorf("invalid VM ID '%s': %w", parts[1], err)
	}

	// Return minimal VM struct (additional fields should be set by caller if needed)
	return VM{
		ID:           vmid,
		Node:         parts[0],
		Type:         "qemu",    // Assume QEMU for VM targets
		Status:       "running", // Assume running since we're executing commands
		AgentEnabled: true,      // Must be enabled to execute commands
		AgentRunning: true,      // Assume running
	}, nil
}

func (u *UIManager) vmFromTarget(target string) (VM, error) {
	if vm, ok := u.vmTargets[target]; ok {
		return vm, nil
	}
	return parseVMTarget(target)
}
