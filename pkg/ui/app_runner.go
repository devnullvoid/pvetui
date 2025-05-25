package ui

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/components"
)

// RunApp creates and starts the application using the component-based architecture
func RunApp(client *api.Client, cfg *config.Config) error {
	app := components.NewApp(client, cfg)
	return app.Run()
}
