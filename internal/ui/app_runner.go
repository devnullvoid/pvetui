package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/internal/ui/plugins"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// RunApp creates and starts the application using the component-based architecture.
func RunApp(ctx context.Context, client *api.Client, cfg *config.Config, configPath string) error {
	app := components.NewApp(ctx, client, cfg, configPath)

	pluginInstances, missing := plugins.EnabledFromConfig(cfg)
	if len(missing) > 0 {
		fmt.Printf("⚠️ Unknown plugins requested: %s\n", strings.Join(missing, ", "))
	}

	if err := app.InitializePlugins(ctx, pluginInstances); err != nil {
		return err
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := app.ShutdownPlugins(shutdownCtx); err != nil {
			fmt.Printf("⚠️ Failed to shutdown plugins: %v\n", err)
		}
	}()

	return app.Run()
}
