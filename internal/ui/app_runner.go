package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/display"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/internal/ui/plugins"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// RunApp creates and starts the application using the component-based architecture.
func RunApp(ctx context.Context, client *api.Client, cfg *config.Config, configPath string, initialGroup string) error {
	app := components.NewApp(ctx, client, cfg, configPath, initialGroup)

	metadata := plugins.AvailableMetadata()
	catalog := make([]components.PluginInfo, len(metadata))
	for i, entry := range metadata {
		catalog[i] = components.PluginInfo{
			ID:          entry.ID,
			Name:        entry.Name,
			Description: entry.Description,
		}
	}
	app.SetPluginCatalog(catalog)

	pluginInstances, missing := plugins.EnabledFromConfig(cfg)
	if len(missing) > 0 {
		fmt.Println(display.IconText("⚠️", fmt.Sprintf("Unknown plugins requested: %s", strings.Join(missing, ", ")), cfg.ShowIcons))
	}

	if err := app.InitializePlugins(ctx, pluginInstances); err != nil {
		return err
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := app.ShutdownPlugins(shutdownCtx); err != nil {
			fmt.Println(display.IconText("⚠️", fmt.Sprintf("Failed to shutdown plugins: %v", err), cfg.ShowIcons))
		}
	}()

	return app.Run()
}
