package components

import (
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
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
		// * Check if there are any pending operations before enabling auto-refresh
		if models.GlobalState.HasPendingOperations() {
			a.showMessageSafe("Cannot enable auto-refresh while there are pending operations in progress")
			return
		}
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

	if a.autoRefreshRunning {
		return // Already running
	}

	a.autoRefreshRunning = true
	a.autoRefreshCountdown = 10
	a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
	a.autoRefreshCountdownStop = make(chan bool, 1)

	// Start countdown goroutine using a proper ticker instead of busy-wait + sleep
	go func() {
		uiLogger := models.GetUILogger()
		countdownTicker := time.NewTicker(1 * time.Second)
		defer countdownTicker.Stop()

		for {
			select {
			case <-a.autoRefreshCountdownStop:
				return
			case <-a.ctx.Done():
				return
			case <-countdownTicker.C:
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
					// Only refresh if not currently loading, no pending operations,
					// and no other refresh (manual/fast/enrichment) is in progress.
					if !a.header.IsLoading() && !models.GlobalState.HasPendingOperations() && !a.isRefreshActive() {
						uiLogger.Debug("Auto-refresh triggered by countdown")

						go a.autoRefreshDataWithFooter()
					} else {
						if a.isRefreshActive() {
							uiLogger.Debug("Auto-refresh skipped - refresh already in progress")
						} else if a.header.IsLoading() {
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

	// Spinner animation goroutine using a proper ticker
	go func() {
		spinnerTicker := time.NewTicker(100 * time.Millisecond)
		defer spinnerTicker.Stop()

		for {
			select {
			case <-a.autoRefreshCountdownStop:
				return
			case <-a.ctx.Done():
				return
			case <-spinnerTicker.C:
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
	if a.autoRefreshCountdownStop != nil {
		close(a.autoRefreshCountdownStop)
		a.autoRefreshCountdownStop = nil
	}

	a.autoRefreshRunning = false
	a.autoRefreshCountdown = 0
	a.footer.UpdateAutoRefreshCountdown(0)
}
