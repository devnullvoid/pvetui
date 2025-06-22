package utils

import (
	"bufio"
	"os"

	"golang.org/x/term"
)

// WaitForEnter waits for the user to press Enter in a way that works after TUI suspension.
// This handles the terminal state properly to ensure input works correctly after
// a TUI application has been suspended and resumed.
func WaitForEnter() {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Not a terminal, use simple scanner
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		return
	}

	// We're in a terminal, handle raw mode properly
	// Put terminal into raw mode to handle input correctly
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// Fall back to simple scanner if raw mode fails
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		return
	}

	// Ensure we restore the terminal state
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Read a single character (typically Enter)
	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
		// Check for Enter key (carriage return or newline)
		if buf[0] == '\r' || buf[0] == '\n' {
			break
		}
	}
}
