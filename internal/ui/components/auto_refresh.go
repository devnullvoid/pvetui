package components

import (
	"time"

	"github.com/devnullvoid/peevetui/internal/ui/models"
)

// toggleAutoRefresh toggles the auto-refresh functionality on/off.
func (a *App) toggleAutoRefresh() {
	uiLogger := models.GetUILogger()

	if a.autoRefreshEnabled {
		// Disable auto-refresh
		a.stopAutoRefresh()
		a.autoRefreshEnabled = false
		a.footer.UpdateAutoRefreshStatus(false)
		a.header.ShowSuccess("Auto-refresh disabled")
		uiLogger.Debug("Auto-refresh disabled by user")
	} else {
		// Enable auto-refresh
		a.autoRefreshEnabled = true
		a.startAutoRefresh()
		a.footer.UpdateAutoRefreshStatus(true)
		a.header.ShowSuccess("Auto-refresh enabled (10s interval)")
		uiLogger.Debug("Auto-refresh enabled by user")
	}
}

// startAutoRefresh starts the auto-refresh timer.
func (a *App) startAutoRefresh() {
	// Don't start if auto-refresh is not enabled
	if !a.autoRefreshEnabled {
		return
	}

	if a.autoRefreshTicker != nil {
		return // Already running
	}

	a.autoRefreshStop = make(chan bool, 1)
	a.autoRefreshTicker = time.NewTicker(10 * time.Second) // 10 second interval
	a.autoRefreshCountdown = 10
	a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
	a.autoRefreshCountdownStop = make(chan bool, 1)

	// Start countdown goroutine
	go func() {
		uiLogger := models.GetUILogger()

		for {
			select {
			case <-a.autoRefreshCountdownStop:
				return
			case <-a.ctx.Done():
				return
			default:
				time.Sleep(1 * time.Second)

				if !a.autoRefreshEnabled {
					return
				}

				if a.footer.IsLoading() {
					continue // Pause countdown while loading
				}

				a.autoRefreshCountdown--
				if a.autoRefreshCountdown < 0 {
					a.autoRefreshCountdown = 0
				}

				// Trigger refresh when countdown reaches 0
				if a.autoRefreshCountdown == 0 {
					// Only refresh if not currently loading something and no pending operations
					if !a.header.IsLoading() && !models.GlobalState.HasPendingOperations() {
						uiLogger.Debug("Auto-refresh triggered by countdown")

						go a.autoRefreshDataWithFooter()
					} else {
						if a.header.IsLoading() {
							uiLogger.Debug("Auto-refresh skipped - header loading operation in progress")
						} else {
							uiLogger.Debug("Auto-refresh skipped - pending VM/node operations in progress")
						}
						// Reset countdown to try again in 10 seconds
						a.autoRefreshCountdown = 10
					}
				}

				a.QueueUpdateDraw(func() {
					a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
				})
			}
		}
	}()

	// Spinner animation goroutine
	go func() {
		for {
			select {
			case <-a.ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)

				if !a.autoRefreshEnabled {
					return
				}

				if a.footer.IsLoading() {
					a.QueueUpdateDraw(func() {
						a.footer.TickSpinner()
					})
				}
			}
		}
	}()
}

// stopAutoRefresh stops the auto-refresh timer.
func (a *App) stopAutoRefresh() {
	// Always stop and nil out the ticker, close channels, and reset countdown
	if a.autoRefreshTicker != nil {
		a.autoRefreshTicker.Stop()
		a.autoRefreshTicker = nil
	}

	if a.autoRefreshStop != nil {
		select {
		case a.autoRefreshStop <- true:
		default:
		}
		close(a.autoRefreshStop)
		a.autoRefreshStop = nil
	}

	if a.autoRefreshCountdownStop != nil {
		close(a.autoRefreshCountdownStop)
		a.autoRefreshCountdownStop = nil
	}

	a.autoRefreshCountdown = 0
	a.footer.UpdateAutoRefreshCountdown(0)
}
