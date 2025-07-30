package components

import (
	"time"
)

// startLoadingAnimation starts the loading animation.
func (s *ScriptSelector) startLoadingAnimation() {
	// Create a ticker for animation updates
	s.animationTicker = time.NewTicker(500 * time.Millisecond)

	// Start animation in a goroutine
	go func() {
		animationFrames := []string{
			"Loading scripts...",
			"Loading scripts..",
			"Loading scripts.",
			"Loading scripts",
		}
		frameIndex := 0

		for range s.animationTicker.C {
			// Update the loading text on the main thread
			s.app.QueueUpdateDraw(func() {
				if s.loadingText != nil {
					s.loadingText.SetText(animationFrames[frameIndex])
					frameIndex = (frameIndex + 1) % len(animationFrames)
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
