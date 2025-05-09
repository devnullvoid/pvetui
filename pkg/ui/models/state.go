package models

import (
	"github.com/rivo/tview"
)

// State holds the state of the UI components.
type State struct {
	NodeList tview.Primitive
	VMList   tview.Primitive
}

// GlobalState is the global state of the UI components.
var GlobalState State
