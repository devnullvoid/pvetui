package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/internal/ui/models"
)

// Run starts the application.
func (a *App) Run() error {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Starting application")

	a.startAutoRefresh()

	defer func() {
		a.stopAutoRefresh()
		// Stop cluster health checks on exit
		if a.clusterClient != nil {
			a.clusterClient.Close()
		}
		a.cancel()
	}()

	if err := a.Application.Run(); err != nil {
		uiLogger.Error("Application run failed: %v", err)

		return err
	}

	uiLogger.Debug("Application stopped normally")
	// Clean up VNC sessions on exit
	uiLogger.Debug("Cleaning up VNC sessions on application exit")

	if closeErr := a.vncService.CloseAllSessions(); closeErr != nil {
		uiLogger.Error("Failed to close VNC sessions on exit: %v", closeErr)
	}

	return nil
}

// updateHeaderWithActiveProfile updates the header to show the current active profile or group.
func (a *App) updateHeaderWithActiveProfile() {
	if a.isClusterMode && a.clusterClient != nil {
		// In cluster mode, show "Cluster: <name> (via <activeProfile>)"
		activeProfile := a.clusterClient.GetActiveProfileName()
		a.header.ShowActiveProfile(fmt.Sprintf("Cluster: %s (via %s)", a.groupName, activeProfile))
	} else if a.isGroupMode {
		// In aggregate group mode, show "Group: <name>"
		a.header.ShowActiveProfile(fmt.Sprintf("Group: %s", a.groupName))
	} else {
		profileName := a.config.GetActiveProfile()
		if profileName == "" {
			a.header.ShowActiveProfile("")
		} else {
			a.header.ShowActiveProfile(profileName)
		}
	}
}

// showQuitConfirmation displays a confirmation dialog before quitting the app.
func (a *App) showQuitConfirmation() {
	sessionCount := a.vncService.GetActiveSessionCount()
	if sessionCount > 0 {
		var message string
		if sessionCount == 1 {
			message = "There is 1 active VNC session that will be disconnected.\n\nAre you sure you want to quit?"
		} else {
			message = fmt.Sprintf("There are %d active VNC sessions that will be disconnected.\n\nAre you sure you want to quit?", sessionCount)
		}
		confirm := CreateConfirmDialog("Quit Application", message, func() {
			a.pages.RemovePage("confirmation")
			a.Application.Stop()
		}, func() {
			a.pages.RemovePage("confirmation")
		})
		a.pages.AddPage("confirmation", confirm, false, true)
		a.SetFocus(confirm)
	} else {
		confirm := CreateConfirmDialog("Quit Application", "Are you sure you want to quit?", func() {
			a.pages.RemovePage("confirmation")
			a.Application.Stop()
		}, func() {
			a.pages.RemovePage("confirmation")
		})
		a.pages.AddPage("confirmation", confirm, false, true)
		a.SetFocus(confirm)
	}
}
