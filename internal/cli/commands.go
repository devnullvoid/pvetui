package cli

import (
	"github.com/spf13/cobra"

	"github.com/devnullvoid/proxmox-tui/internal/bootstrap"
	"github.com/devnullvoid/proxmox-tui/internal/config"
)

// newConfigWizardCmd creates the config wizard command
func newConfigWizardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-wizard",
		Short: "Launch interactive config wizard",
		Long: `Launch an interactive configuration wizard to set up your Proxmox TUI configuration.

This wizard will guide you through creating a configuration file with your
Proxmox server details, authentication credentials, and other settings.`,
		RunE: runConfigWizard,
	}

	return cmd
}

// runConfigWizard executes the config wizard
func runConfigWizard(cmd *cobra.Command, args []string) error {
	// Get config path from flags
	configPath, _ := cmd.Flags().GetString("config")

	// Create a new config
	cfg := config.NewConfig()

	// Resolve config path
	resolvedPath := bootstrap.ResolveConfigPath(configPath)

	// Run the config wizard
	return bootstrap.HandleConfigWizard(cfg, resolvedPath, "")
}
