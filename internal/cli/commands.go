package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/internal/bootstrap"
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/profile"
)

// newConfigWizardCmd creates the config wizard command
func newConfigWizardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-wizard",
		Short: "Launch interactive config wizard",
		Long: `Launch an interactive configuration wizard to set up your pvetui configuration.

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

	// Load existing config if it exists
	if resolvedPath != "" {
		_ = cfg.MergeWithFile(resolvedPath) // Ignore errors for config wizard
	}

	// Set defaults for config wizard
	cfg.SetDefaults()

	// Handle profile selection for config wizard (same as bootstrap)
	selectedProfile, err := profile.ResolveProfile("", cfg) // No profile specified for subcommand
	if err != nil {
		return fmt.Errorf("profile resolution failed: %w", err)
	}

	// Apply selected profile for config wizard
	if selectedProfile != "" {
		if err := cfg.ApplyProfile(selectedProfile); err != nil {
			return fmt.Errorf("could not select profile '%s': %w", selectedProfile, err)
		}
	}

	// Run the config wizard
	return bootstrap.HandleConfigWizard(cfg, resolvedPath, selectedProfile)
}
