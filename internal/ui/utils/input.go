package utils

import (
	"bufio"
	"os"
)

// WaitForEnter waits for the user to press Enter in a way that works after TUI suspension.
// This is more reliable than fmt.Scanln() after a TUI application has been suspended
// because the terminal state gets modified by the TUI framework.
func WaitForEnter() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan() // Wait for any input (typically Enter)
}
