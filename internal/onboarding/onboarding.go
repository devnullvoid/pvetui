// Package onboarding handles the first-time setup and configuration wizard flow.
//
// This package provides a clean separation of concerns for handling
// configuration validation errors and guiding users through the initial setup.
package onboarding

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/components"
)

// HandleValidationError handles configuration validation errors by guiding users through onboarding.
func HandleValidationError(cfg *config.Config, configPath string, noCacheFlag bool, activeProfile string) error {
	fmt.Println("🔧 Configuration Setup Required")
	fmt.Println()
	fmt.Printf("It looks like this is your first time running proxmox-tui, or your configuration needs attention.\n")
	fmt.Printf("Missing: %v\n", cfg.Validate())
	fmt.Println()

	defaultPath := config.GetDefaultConfigPath()
	if !promptYesNo(fmt.Sprintf("Would you like to create a default configuration file at '%s'?", defaultPath)) {
		fmt.Println("❌ Configuration setup canceled. You can configure via flags or environment variables instead.")
		fmt.Println("🚪 Exiting.")
		os.Exit(0)
	}

	fmt.Println()

	path, createErr := config.CreateDefaultConfigFile()
	if createErr != nil {
		log.Fatalf("❌ Error creating config file: %v", createErr)
	}

	fmt.Printf("✅ Success! Configuration file created at %s\n", path)
	fmt.Println()

	if promptYesNo("Would you like to edit the new config in the interactive editor?") {
		newCfg := config.NewConfig()
		_ = newCfg.MergeWithFile(path)

		res := launchConfigWizard(newCfg, path, activeProfile)
		if res.SopsEncrypted {
			fmt.Printf("✅ Configuration saved and encrypted with SOPS: %s\n", path)
		} else if res.Saved {
			fmt.Println("✅ Configuration saved.")
		} else if res.Canceled {
			fmt.Println("🚪 Exiting.")
		}

		if promptYesNo("Would you like to proceed with main application startup?") {
			*cfg = *config.NewConfig()
			_ = cfg.MergeWithFile(path)
			cfg.SetDefaults()
			config.DebugEnabled = cfg.Debug
			return nil // Signal to continue with main app
		}
	}

	fmt.Println("🚪 Exiting.")
	os.Exit(0)
	return nil
}

// promptYesNo is a helper function for yes/no prompts.
func promptYesNo(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [Y/n] ", prompt)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "y", "":
			return true
		case "n":
			return false
		default:
			fmt.Println("Please enter 'y' or 'n'.")
		}
	}
}

// launchConfigWizard launches the configuration wizard.
func launchConfigWizard(cfg *config.Config, configPath string, activeProfile string) components.WizardResult {
	return components.LaunchConfigWizard(cfg, configPath, activeProfile)
}
