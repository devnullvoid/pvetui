package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/app"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/components"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/rivo/tview"
)

var (
	version    = "dev"
	buildDate  = "unknown"
	commitHash = "unknown"
)

func printVersion() {
	fmt.Printf("proxmox-tui version %s\n", version)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Commit: %s\n", commitHash)
}

func resolveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}

	if path, found := config.FindDefaultConfigPath(); found {
		return path
	}

	return ""
}

func launchConfigWizard(cfg *config.Config, configPath string, activeProfile string) components.WizardResult {
	tviewApp := tview.NewApplication()
	resultChan := make(chan components.WizardResult, 1)
	wizard := components.NewConfigWizardPage(tviewApp, cfg, configPath, func(c *config.Config) error {
		// Minimal fix: if editing a profile, update the profile map with the edited legacy fields
		if activeProfile != "" && c.Profiles != nil {
			c.Profiles[activeProfile] = config.ProfileConfig{
				Addr:        c.Addr,
				User:        c.User,
				Password:    c.Password,
				TokenID:     c.TokenID,
				TokenSecret: c.TokenSecret,
				Realm:       c.Realm,
				ApiPath:     c.ApiPath,
				Insecure:    c.Insecure,
				SSHUser:     c.SSHUser,
			}
		}

		return components.SaveConfigToFile(c, configPath)
	}, func() {
		tviewApp.Stop()
	}, resultChan)
	tviewApp.SetRoot(wizard, true)
	_ = tviewApp.Run()

	return <-resultChan
}

// Add promptYesNo helper.
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

// Refactor onboardingFlow to use promptYesNo for all prompts.
func onboardingFlow(cfg *config.Config, configPath string, noCacheFlag *bool, activeProfile string) {
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
			startMainApp(cfg, path, noCacheFlag)
		}
	}

	fmt.Println("🚪 Exiting.")
	os.Exit(0)
}

func startMainApp(cfg *config.Config, configPath string, noCacheFlag *bool) {
	fmt.Println("\n🚀 Starting Proxmox TUI...")

	if configPath != "" {
		fmt.Printf("✅ Configuration loaded from %s\n", configPath)
	} else {
		fmt.Println("✅ Configuration loaded from environment variables")
	}

	theme.ApplyCustomTheme(&cfg.Theme)
	theme.ApplyToTview()

	if err := app.RunWithStartupVerification(cfg, app.Options{NoCache: *noCacheFlag}); err != nil {
		fmt.Printf("❌ %v\n", err)
		fmt.Println()

		if strings.Contains(err.Error(), "authentication failed") {
			fmt.Println("💡 Please check your credentials in the config file:")

			if configPath != "" {
				fmt.Printf("   %s\n", configPath)
			} else {
				fmt.Printf("   %s\n", config.GetDefaultConfigPath())
			}
		} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
			fmt.Println("💡 Please check your Proxmox server address and network connectivity:")
			fmt.Printf("   Current address: %s\n", cfg.Addr)
		}

		os.Exit(1)
	}

	fmt.Println("🚪 Exiting.")
	os.Exit(0)
}

func main() {
	cfg := config.NewConfig()
	cfg.ParseFlags()

	configPathFlag := flag.String("config", "", "Path to YAML config file")
	profileFlag := flag.String("profile", "", "Connection profile to use (overrides default_profile)")
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching")
	versionFlag := flag.Bool("version", false, "Show version information")
	configWizardFlag := flag.Bool("config-wizard", false, "Launch interactive config wizard and exit")
	flag.Parse()

	if *versionFlag {
		printVersion()

		return
	}

	configPath := resolveConfigPath(*configPathFlag)

	if configPath != "" {
		_ = cfg.MergeWithFile(configPath)
	}

	// Profile selection logic
	selectedProfile := *profileFlag
	if selectedProfile == "" {
		selectedProfile = os.Getenv("PROXMOX_TUI_PROFILE")
	}

	if selectedProfile == "" {
		selectedProfile = cfg.DefaultProfile
	}

	if selectedProfile == "" && len(cfg.Profiles) > 0 {
		selectedProfile = "default"
	}

	if selectedProfile != "" {
		err := cfg.ApplyProfile(selectedProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Could not select profile '%s': %v\n", selectedProfile, err)
			os.Exit(1)
		}
	}

	if *configWizardFlag {
		res := launchConfigWizard(cfg, configPath, selectedProfile)
		if res.SopsEncrypted {
			fmt.Printf("✅ Configuration saved and encrypted with SOPS: %s\n", configPath)
		} else if res.Saved {
			fmt.Println("✅ Configuration saved.")
		} else if res.Canceled {
			fmt.Println("🚪 Exiting.")
		}

		os.Exit(0)
	}

	cfg.SetDefaults()
	config.DebugEnabled = cfg.Debug

	if err := cfg.Validate(); err != nil {
		onboardingFlow(cfg, configPath, noCacheFlag, selectedProfile)
	}

	startMainApp(cfg, configPath, noCacheFlag)
}
