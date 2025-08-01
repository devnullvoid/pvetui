package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ButtonAlignment defines how the button should be positioned within its container.
type ButtonAlignment int

const (
	// AlignCenter centers the button horizontally (default)
	AlignCenter ButtonAlignment = iota
	// AlignLeft aligns the button to the left
	AlignLeft
	// AlignRight aligns the button to the right
	AlignRight
	// AlignCustom uses custom positioning (x, y coordinates)
	AlignCustom
)

// FormButton is a reusable FormItem that acts like a button and can be placed anywhere in a tview.Form.
type FormButton struct {
	*tview.Box
	button *tview.Button

	label     string
	selected  func()
	focused   bool
	disabled  bool
	alignment ButtonAlignment
	customX   int // Used when alignment is AlignCustom
	customY   int // Used when alignment is AlignCustom
	// Add doneFunc for Tab/Backtab navigation
	doneFunc func(key tcell.Key)
}

// NewFormButton creates a new FormButton with the given label and callback.
func NewFormButton(label string, selected func()) *FormButton {
	button := tview.NewButton(label)
	button.SetSelectedFunc(selected)

	return &FormButton{
		Box:       tview.NewBox(),
		button:    button,
		label:     label,
		selected:  selected,
		alignment: AlignCenter, // Default to centered
	}
}

// SetAlignment sets the button's alignment within its container.
func (b *FormButton) SetAlignment(alignment ButtonAlignment) *FormButton {
	b.alignment = alignment
	return b
}

// SetCustomPosition sets custom positioning for the button (only used when alignment is AlignCustom).
func (b *FormButton) SetCustomPosition(x, y int) *FormButton {
	b.customX = x
	b.customY = y
	return b
}

// Draw renders the button by delegating to the embedded tview.Button.
func (b *FormButton) Draw(screen tcell.Screen) {
	// Get our box dimensions
	x, y, width, height := b.GetRect()

	// Calculate button width based on label length plus padding
	buttonWidth := len(b.label) + 4 // Add padding for button appearance
	if buttonWidth > width {
		buttonWidth = width
	}

	// Calculate button position based on alignment
	var buttonX, buttonY int
	switch b.alignment {
	case AlignLeft:
		buttonX = x
		buttonY = y
	case AlignRight:
		buttonX = x + width - buttonWidth
		buttonY = y
	case AlignCustom:
		buttonX = b.customX
		buttonY = b.customY
	default: // AlignCenter
		buttonX = x + (width-buttonWidth)/2
		buttonY = y
	}

	// Ensure button doesn't go outside bounds
	if buttonX < x {
		buttonX = x
	}
	if buttonX+buttonWidth > x+width {
		buttonX = x + width - buttonWidth
	}

	// Set the button's position based on alignment
	b.button.SetRect(buttonX, buttonY, buttonWidth, height)
	b.button.Draw(screen)
}

// GetLabel returns the button label.
func (b *FormButton) GetLabel() string {
	return b.label
}

// SetLabel sets the button label.
func (b *FormButton) SetLabel(label string) tview.FormItem {
	b.label = label
	b.button.SetLabel(label)
	return b
}

// GetFieldWidth returns the width of the button (label length).
func (b *FormButton) GetFieldWidth() int {
	return len(b.label)
}

// SetFieldWidth is a no-op for FormButton.
func (b *FormButton) SetFieldWidth(width int) tview.FormItem {
	return b
}

// GetFieldHeight returns the height of the button (always 1).
func (b *FormButton) GetFieldHeight() int {
	return 1
}

// SetFormAttributes is a no-op for FormButton (to satisfy tview.FormItem interface).
func (b *FormButton) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	return b
}

// Focus sets the button as focused.
func (b *FormButton) Focus(delegate func(p tview.Primitive)) {
	b.focused = true
	b.button.Focus(delegate)
}

// Blur removes focus from the button.
func (b *FormButton) Blur() {
	b.focused = false
	b.button.Blur()
}

// HasFocus returns true if the button is focused.
func (b *FormButton) HasFocus() bool {
	return b.button.HasFocus()
}

// InputHandler handles key events for the button by delegating to the embedded button.
func (b *FormButton) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return b.button.InputHandler()
}

// SetSelectedFunc sets the callback for when the button is pressed.
func (b *FormButton) SetSelectedFunc(handler func()) *FormButton {
	b.selected = handler
	b.button.SetSelectedFunc(handler)
	return b
}

// SetDisabled sets whether the button is disabled.
func (b *FormButton) SetDisabled(disabled bool) tview.FormItem {
	b.disabled = disabled
	b.button.SetDisabled(disabled)
	return b
}

// IsDisabled returns true if the button is disabled.
func (b *FormButton) IsDisabled() bool {
	return b.button.IsDisabled()
}

// SetFinishedFunc sets the doneFunc for Tab/Backtab navigation.
func (b *FormButton) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	b.doneFunc = handler
	b.button.SetExitFunc(handler)
	return b
}
