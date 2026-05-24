package utils

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/devnullvoid/pvetui/internal/display"
)

var showIcons atomic.Bool

func init() {
	showIcons.Store(true)
}

// SetShowIcons controls icon prefixes used in terminal flows that suspend the TUI.
func SetShowIcons(enabled bool) {
	showIcons.Store(enabled)
}

// WaitForReturnWithCountdown waits for a specified number of seconds,
// displaying a countdown, before returning to the TUI.
func WaitForReturnWithCountdown(seconds int) {
	for i := seconds; i > 0; i-- {
		fmt.Printf("\rReturning to TUI in %d seconds... (Press Enter to return immediately)", i)
		time.Sleep(1 * time.Second)
	}

	fmt.Print("\r" + fmt.Sprintf("%*s", 80, " ") + "\r") // Clear the line
}

// WaitForEnter waits for the user to press Enter before continuing.
// This is useful for pausing execution and waiting for user acknowledgment.
func WaitForEnter() {
	if _, err := fmt.Scanln(); err != nil {
		// Ignore scan errors as they're not critical for this function
		// The user might have pressed Ctrl+C or similar
		_ = err // Explicitly ignore the error to avoid empty branch
	}

	// Add a small delay and countdown for better UX
	WaitForReturnWithCountdown(3)
}

// WaitForEnterToReturn displays a status message and waits for the user to press Enter
// before returning to the TUI. This provides a consistent experience across all
// suspend/resume operations.
func WaitForEnterToReturn(err error, successMsg, failureMsg string) {
	// Show completion status
	if err != nil {
		fmt.Printf("\n%s\n", display.IconText("❌", fmt.Sprintf("%s: %v", failureMsg, err), showIcons.Load()))
	} else {
		fmt.Printf("\n%s\n", display.IconText("✅", successMsg, showIcons.Load()))
	}

	fmt.Print("\nPress Enter to return to the TUI...")

	if _, err := fmt.Scanln(); err != nil {
		// Ignore scan errors as they're not critical for this function
		// The user might have pressed Ctrl+C or similar
		_ = err // Explicitly ignore the error to avoid empty branch
	}
}
