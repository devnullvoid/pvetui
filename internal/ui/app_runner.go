package ui

import (
	"context"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/components"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// RunApp creates and starts the application using the component-based architecture
func RunApp(ctx context.Context, client *api.Client, cfg *config.Config) error {
	app := components.NewApp(ctx, client, cfg)
	return app.Run()
}
