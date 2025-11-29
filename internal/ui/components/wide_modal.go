package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WideModal is a slightly modified version of tview.Modal that allows
// controlling the minimum width and the fraction of screen width used for
// layout. This prevents long lines (e.g., URLs) from wrapping unexpectedly.
type WideModal struct {
	*tview.Box

	frame *tview.Frame
	form  *tview.Form

	text      string
	textColor tcell.Color
	done      func(buttonIndex int, buttonLabel string)
	buttons   []string

	widthRatio float64
	minWidth   int
}

// NewWideModal creates a modal that uses the provided widthRatio (0 < ratio <= 1)
// of the available screen width and enforces a minimum inner width.
func NewWideModal(widthRatio float64, minWidth int) *WideModal {
	if widthRatio <= 0 || widthRatio > 1 {
		widthRatio = 0.5
	}
	if minWidth < 0 {
		minWidth = 0
	}

	m := &WideModal{
		Box:        tview.NewBox().SetBorder(true).SetBackgroundColor(tview.Styles.ContrastBackgroundColor),
		textColor:  tview.Styles.PrimaryTextColor,
		widthRatio: widthRatio,
		minWidth:   minWidth,
	}

	m.form = tview.NewForm().
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tview.Styles.PrimitiveBackgroundColor).
		SetButtonTextColor(tview.Styles.PrimaryTextColor)

	m.form.SetBackgroundColor(tview.Styles.ContrastBackgroundColor).SetBorderPadding(0, 0, 0, 0)
	m.form.SetCancelFunc(func() {
		if m.done != nil {
			m.done(-1, "")
		}
	})

	m.frame = tview.NewFrame(m.form).SetBorders(0, 0, 1, 0, 0, 0)
	m.frame.SetBackgroundColor(tview.Styles.ContrastBackgroundColor).
		SetBorderPadding(1, 1, 1, 1)

	return m
}

// SetBackgroundColor sets the color of the modal frame background.
func (m *WideModal) SetBackgroundColor(color tcell.Color) *WideModal {
	m.form.SetBackgroundColor(color)
	m.frame.SetBackgroundColor(color)
	return m
}

// SetTextColor sets the color of the message text.
func (m *WideModal) SetTextColor(color tcell.Color) *WideModal {
	m.textColor = color
	return m
}

// SetButtonBackgroundColor sets the background color of the buttons.
func (m *WideModal) SetButtonBackgroundColor(color tcell.Color) *WideModal {
	m.form.SetButtonBackgroundColor(color)
	return m
}

// SetButtonTextColor sets the color of the button texts.
func (m *WideModal) SetButtonTextColor(color tcell.Color) *WideModal {
	m.form.SetButtonTextColor(color)
	return m
}

// SetButtonStyle sets the style of the buttons when they are not focused.
func (m *WideModal) SetButtonStyle(style tcell.Style) *WideModal {
	m.form.SetButtonStyle(style)
	return m
}

// SetButtonActivatedStyle sets the style of the buttons when they are focused.
func (m *WideModal) SetButtonActivatedStyle(style tcell.Style) *WideModal {
	m.form.SetButtonActivatedStyle(style)
	return m
}

// SetDoneFunc sets a handler which is called when one of the buttons was pressed.
func (m *WideModal) SetDoneFunc(handler func(buttonIndex int, buttonLabel string)) *WideModal {
	m.done = handler
	return m
}

// SetText sets the message text of the window.
func (m *WideModal) SetText(text string) *WideModal {
	m.text = text
	return m
}

// AddButtons adds buttons to the window.
func (m *WideModal) AddButtons(labels []string) *WideModal {
	for index, label := range labels {
		m.buttons = append(m.buttons, label)
		func(i int, l string) {
			m.form.AddButton(label, func() {
				if m.done != nil {
					m.done(i, l)
				}
			})
			button := m.form.GetButton(m.form.GetButtonCount() - 1)
			button.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyDown, tcell.KeyRight:
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				case tcell.KeyUp, tcell.KeyLeft:
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				}
				return event
			})
		}(index, label)
	}
	return m
}

// ClearButtons removes all buttons from the window.
func (m *WideModal) ClearButtons() *WideModal {
	m.form.ClearButtons()
	m.buttons = nil
	return m
}

// SetFocus shifts the focus to the button with the given index.
func (m *WideModal) SetFocus(index int) *WideModal {
	m.form.SetFocus(index)
	return m
}

// Focus is called when this primitive receives focus.
func (m *WideModal) Focus(delegate func(p tview.Primitive)) {
	delegate(m.form)
}

// HasFocus returns whether or not this primitive has focus.
func (m *WideModal) HasFocus() bool {
	return m.form.HasFocus()
}

// InputHandler forwards key events to the embedded form so buttons work.
func (m *WideModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return m.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if handler := m.form.InputHandler(); handler != nil {
			handler(event, setFocus)
		}
	})
}

// Draw draws this primitive onto the screen.
func (m *WideModal) Draw(screen tcell.Screen) {
	// Calculate desired width.
	screenWidth, screenHeight := screen.Size()
	width := int(float64(screenWidth) * m.widthRatio)
	if width < m.minWidth {
		width = m.minWidth
	}

	// Account for buttons.
	buttonsWidth := 0
	for _, label := range m.buttons {
		buttonsWidth += tview.TaggedStringWidth(label) + 4 + 2
	}
	if buttonsWidth > 0 {
		buttonsWidth -= 2
	}
	if width < buttonsWidth {
		width = buttonsWidth
	}

	// Work without borders inside.
	innerWidth := width

	m.frame.Clear()
	lines := tview.WordWrap(m.text, innerWidth)
	for _, line := range lines {
		m.frame.AddText(line, true, tview.AlignCenter, m.textColor)
	}

	height := len(lines) + 6
	widthWithBorders := innerWidth + 4
	if widthWithBorders > screenWidth {
		widthWithBorders = screenWidth
	}
	x := (screenWidth - widthWithBorders) / 2
	y := (screenHeight - height) / 2
	m.SetRect(x, y, widthWithBorders, height)

	// Draw frame.
	m.Box.DrawForSubclass(screen, m)
	x, y, innerWidth, height = m.GetInnerRect()
	m.frame.SetRect(x, y, innerWidth, height)
	m.frame.Draw(screen)
}

// MouseHandler returns the mouse handler for this primitive.
func (m *WideModal) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return m.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		consumed, capture = m.form.MouseHandler()(action, event, setFocus)
		if !consumed && action == tview.MouseLeftDown && m.InRect(event.Position()) {
			setFocus(m)
			consumed = true
		}
		return
	})
}
