package utils

import (
	"fmt"
	"time"
)

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
		fmt.Printf("\n❌ %s: %v\n", failureMsg, err)
	} else {
		fmt.Printf("\n✅ %s\n", successMsg)
	}

	fmt.Print("\nPress Enter to return to the TUI...")

	if _, err := fmt.Scanln(); err != nil {
		// Ignore scan errors as they're not critical for this function
		// The user might have pressed Ctrl+C or similar
		_ = err // Explicitly ignore the error
	}
}
