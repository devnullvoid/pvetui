package components

import (
	"time"
)

// startLoadingAnimation starts the loading animation.
func (s *ScriptSelector) startLoadingAnimation() {
	// Create a ticker for animation updates
	s.animationTicker = time.NewTicker(100 * time.Millisecond) // Match header timing

	// Start animation in a goroutine
	go func() {
		// Use the same spinner as the header
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		index := 0

		for range s.animationTicker.C {
			// Update the loading text on the main thread
			s.app.QueueUpdateDraw(func() {
				if s.loadingText != nil {
					spinnerChar := spinner[index]
					s.loadingText.SetText(spinnerChar + " Fetching scripts from GitHub, please wait...")
					index = (index + 1) % len(spinner)
				}
			})
		}
	}()
}

// stopLoadingAnimation stops the loading animation.
func (s *ScriptSelector) stopLoadingAnimation() {
	if s.animationTicker != nil {
		s.animationTicker.Stop()
		s.animationTicker = nil
	}
}
