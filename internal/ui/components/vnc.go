package components

import (
	"context"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/vnc"
)

// GetVNCService returns the VNC service instance
func (a *App) GetVNCService() *vnc.Service {
	return a.vncService
}

// startVNCSessionMonitoring starts a background goroutine to monitor and update VNC session count
func (a *App) startVNCSessionMonitoring() {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Starting VNC session monitoring")

	go func() {
		ticker := time.NewTicker(5 * time.Second) // Reduced from 30 seconds to 5 seconds as backup
		defer ticker.Stop()

		lastSessionCount := -1 // Track last count to only log changes

		for {
			select {
			case <-ticker.C:
			case <-a.ctx.Done():
				return
			}
			// Get current session count
			sessionCount := a.vncService.GetActiveSessionCount()

			// Update footer with session count
			a.QueueUpdateDraw(func() {
				a.footer.UpdateVNCSessionCount(sessionCount)
			})

			// Only log when session count changes
			if sessionCount != lastSessionCount {
				uiLogger.Debug("VNC session count changed (polling): %d -> %d", lastSessionCount, sessionCount)
				lastSessionCount = sessionCount
			}

			// Clean up inactive sessions (older than 30 minutes) - but don't log every time
			a.vncService.CleanupInactiveSessions(30 * time.Minute)
		}
	}()
}

// registerVNCSessionCallback registers a callback for immediate VNC session count updates
func (a *App) registerVNCSessionCallback() {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Registering VNC session count callback for immediate updates")

	a.vncService.SetSessionCountCallback(func(count int) {
		uiLogger.Debug("VNC session count changed (callback): %d", count)

		// Update the UI immediately
		a.QueueUpdateDraw(func() {
			a.footer.UpdateVNCSessionCount(count)
		})
	})
}
