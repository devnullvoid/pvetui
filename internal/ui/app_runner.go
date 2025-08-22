package ui

import (
	"context"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// RunApp creates and starts the application using the component-based architecture.
func RunApp(ctx context.Context, client *api.Client, cfg *config.Config, configPath string) error {
	app := components.NewApp(ctx, client, cfg, configPath)

	return app.Run()
}
