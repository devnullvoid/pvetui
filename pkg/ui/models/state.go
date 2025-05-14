package models

import (
	"github.com/rivo/tview"
)

// State holds all UI state components
type State struct {
	NodeList       tview.Primitive
	VMList         tview.Primitive
	LastSearchText string
}

// GlobalState is the singleton instance for UI state
var GlobalState = State{}
