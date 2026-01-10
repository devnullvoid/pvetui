package cli

import (
	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/internal/bootstrap"
	"github.com/devnullvoid/pvetui/internal/config"
)

// newConfigWizardCmd creates the config wizard command
func newConfigWizardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-wizard",
		Short: "Launch interactive config wizard",
		Long: `Launch an interactive configuration wizard to set up your pvetui configuration.

This wizard will guide you through creating a configuration file with your
Proxmox server details, authentication credentials, and other settings.

You can add or edit a specific profile using the --profile flag:
  pvetui config-wizard --profile my-profile`,
		RunE: runConfigWizard,
	}

	cmd.Flags().StringP("profile", "p", "", "Profile to add or edit")

	return cmd
}

// runConfigWizard executes the config wizard
func runConfigWizard(cmd *cobra.Command, args []string) error {
	// Get config path from flags
	configPath, _ := cmd.Flags().GetString("config")
	profileName, _ := cmd.Flags().GetString("profile")

	// Create a new config
	cfg := config.NewConfig()

	// Resolve config path
	resolvedPath := bootstrap.ResolveConfigPathForWizard(configPath)

	// Load existing config if it exists
	if resolvedPath != "" {
		_ = cfg.MergeWithFile(resolvedPath) // Ignore errors for config wizard
	}

	// Set defaults for config wizard
	cfg.SetDefaults()

	// If a specific profile is requested for the wizard, set it as default
	// This signals the wizard to edit/create this specific profile
	if profileName != "" {
		cfg.DefaultProfile = profileName
	}

	// Run the config wizard
	// We pass profileName as activeProfile, though the wizard largely relies on cfg.DefaultProfile
	return bootstrap.HandleConfigWizard(cfg, resolvedPath, profileName)
}
