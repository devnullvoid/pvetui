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

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/components"
)

// HandleValidationError handles configuration validation errors by guiding users through onboarding.
func HandleValidationError(cfg *config.Config, configPath string, noCacheFlag bool, activeProfile string) error {
	fmt.Println("🔧 Configuration Setup Required")
	fmt.Println()
	target, err := resolveOnboardingTarget(configPath)
	if err != nil {
		return fmt.Errorf("resolve onboarding target: %w", err)
	}

	if target.exists {
		fmt.Println("It looks like your configuration needs attention.")
	} else {
		fmt.Println("It looks like this is your first time running pvetui.")
	}
	validationErr := cfg.Validate()
	if validationErr != nil {
		fmt.Printf("Issue: %v\n", validationErr)
	}
	fmt.Println()

	if target.exists {
		fmt.Printf("✅ Found existing configuration at '%s'.\n", target.path)
		if isKeyBindingValidationError(validationErr) {
			fmt.Printf("💡 Key binding errors must be fixed directly in '%s' under key_bindings.\n", target.path)
			fmt.Printf("💡 Please update the configuration file to resolve: %v\n", validationErr)
			fmt.Println("🚪 Exiting.")
			os.Exit(0)
		}
		if !promptYesNo("Would you like to open the interactive editor to fix it?") {
			fmt.Printf("💡 Please update the configuration file to resolve: %v\n", validationErr)
			fmt.Println("🚪 Exiting.")
			os.Exit(0)
		}

		fmt.Println()

		targetProfile := activeProfile
		if conflicts := cfg.FindGroupProfileNameConflicts(); len(conflicts) > 0 {
			targetProfile = conflicts[0]
			fmt.Printf("💡 Opening the editor for profile '%s' so you can rename it (group name conflict).\n", targetProfile)
		}

		res := launchConfigWizard(cfg, target.path, targetProfile)
		if res.SopsEncrypted {
			fmt.Printf("✅ Configuration saved and encrypted with SOPS: %s\n", target.path)
		} else if res.Saved {
			fmt.Println("✅ Configuration saved.")
		} else if res.Canceled {
			fmt.Println("ℹ️  No changes were saved.")
		}

		fmt.Println()
		fmt.Println("✅ Configuration is ready!")
		fmt.Println("🔄 Please re-run 'pvetui' to start the application with your updated configuration.")
		fmt.Println("🚪 Exiting.")
		os.Exit(0)
	}

	defaultPath := target.path
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
			fmt.Println("ℹ️  Using default configuration.")
		}
	}

	fmt.Println()
	fmt.Println("✅ Configuration is ready!")
	fmt.Println("🔄 Please re-run 'pvetui' to start the application with your new configuration.")
	fmt.Println("🚪 Exiting.")
	os.Exit(0)
	return nil
}

type onboardingTarget struct {
	path   string
	exists bool
}

func resolveOnboardingTarget(configPath string) (onboardingTarget, error) {
	if configPath == "" {
		return onboardingTarget{
			path:   config.GetDefaultConfigPath(),
			exists: false,
		}, nil
	}

	if _, err := os.Stat(configPath); err == nil {
		return onboardingTarget{
			path:   configPath,
			exists: true,
		}, nil
	} else if os.IsNotExist(err) {
		return onboardingTarget{
			path:   configPath,
			exists: false,
		}, nil
	} else {
		return onboardingTarget{}, err
	}
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

func isKeyBindingValidationError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "key binding ")
}
