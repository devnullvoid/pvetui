package utils

import (
	"fmt"
	"time"
)

// WaitForReturnWithCountdown displays a countdown and automatically returns after the specified duration.
// This completely avoids the terminal input issues that occur after TUI suspension.
func WaitForReturnWithCountdown(seconds int) {
	fmt.Printf("Returning to TUI in ")
	for i := seconds; i > 0; i-- {
		fmt.Printf("%d...", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println("0")
	fmt.Println("Returning to TUI...")
}

// WaitForEnter is now just a wrapper that uses a 3-second countdown
func WaitForEnter() {
	WaitForReturnWithCountdown(3)
}
