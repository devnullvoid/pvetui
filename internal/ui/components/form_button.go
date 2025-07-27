package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// FormButton is a reusable FormItem that acts like a button and can be placed anywhere in a tview.Form.
type FormButton struct {
	*tview.Box

	label    string
	selected func()
	focused  bool
	disabled bool
	// Add doneFunc for Tab/Backtab navigation
	doneFunc func(key tcell.Key)
}

// NewFormButton creates a new FormButton with the given label and callback.
func NewFormButton(label string, selected func()) *FormButton {
	return &FormButton{
		Box:      tview.NewBox(),
		label:    label,
		selected: selected,
	}
}

// Draw renders the button.
func (b *FormButton) Draw(screen tcell.Screen) {
	b.Box.DrawForSubclass(screen, b)
	// Draw the label centered with tview button style
	x, y, w, h := b.GetInnerRect()
	label := b.label
	style := tcell.StyleDefault
	bg := tview.Styles.PrimitiveBackgroundColor
	fg := tview.Styles.PrimaryTextColor

	if b.focused {
		bg = tview.Styles.ContrastBackgroundColor
		fg = tview.Styles.PrimaryTextColor
		style = style.Background(bg).Foreground(fg).Bold(true)
	} else {
		style = style.Background(bg).Foreground(fg)
	}
	// Add a space padding around the label to match tview button look
	paddedLabel := " " + label + " "
	labelWidth := tview.TaggedStringWidth(paddedLabel)

	labelX := x + (w-labelWidth)/2
	if labelX < x {
		labelX = x
	}

	for i, r := range paddedLabel {
		screen.SetContent(labelX+i, y+h/2, r, nil, style)
	}
}

// GetLabel returns the button label.
func (b *FormButton) GetLabel() string {
	return b.label
}

// SetLabel sets the button label.
func (b *FormButton) SetLabel(label string) tview.FormItem {
	b.label = label

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
	b.Box.Focus(delegate)
}

// Blur removes focus from the button.
func (b *FormButton) Blur() {
	b.focused = false
	b.Box.Blur()
}

// HasFocus returns true if the button is focused.
func (b *FormButton) HasFocus() bool {
	return b.focused
}

// InputHandler handles key events for the button.
func (b *FormButton) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if event.Key() == tcell.KeyEnter || (event.Key() == tcell.KeyRune && event.Rune() == ' ') {
			if b.selected != nil {
				b.selected()
			}

			return
		}

		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
			if b.doneFunc != nil {
				b.doneFunc(event.Key())
			}

			return
		}
		// Do not handle Escape/Backspace; let the form handle it at the form level.
	}
}

// SetSelectedFunc sets the callback for when the button is pressed.
func (b *FormButton) SetSelectedFunc(handler func()) *FormButton {
	b.selected = handler

	return b
}

// SetDisabled sets whether the button is disabled.
func (b *FormButton) SetDisabled(disabled bool) tview.FormItem {
	b.disabled = disabled

	return b
}

// IsDisabled returns true if the button is disabled.
func (b *FormButton) IsDisabled() bool {
	return b.disabled
}

// SetFinishedFunc sets the doneFunc for Tab/Backtab navigation.
func (b *FormButton) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	b.doneFunc = handler

	return b
}
