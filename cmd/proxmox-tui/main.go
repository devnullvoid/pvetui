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

func launchConfigWizard(cfg *config.Config, configPath string) bool {
	tviewApp := tview.NewApplication()
	result := make(chan bool, 1)
	wizard := components.NewConfigWizardPage(tviewApp, cfg, configPath, func(c *config.Config) error {
		err := components.SaveConfigToFile(c, configPath)
		if err == nil {
			result <- true
		} else {
			result <- false
		}
		return err
	}, func() {
		result <- false
		tviewApp.Stop()
	})
	tviewApp.SetRoot(wizard, true)
	_ = tviewApp.Run()
	return <-result
}

// Add promptYesNo helper
func promptYesNo(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [Y/n] ", prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "" {
			return true
		} else if input == "n" {
			return false
		} else {
			fmt.Println("Please enter 'y' or 'n'.")
		}
	}
}

// Refactor onboardingFlow to use promptYesNo for all prompts
func onboardingFlow(cfg *config.Config, configPath string, noCacheFlag *bool) {
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
		if launchConfigWizard(newCfg, path) {
			fmt.Println("✅ Configuration saved. Exiting.")
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

	if *configWizardFlag {
		if launchConfigWizard(cfg, configPath) {
			fmt.Println("✅ Configuration saved. Exiting.")
		}
		os.Exit(0)
	}

	cfg.SetDefaults()
	config.DebugEnabled = cfg.Debug

	if err := cfg.Validate(); err != nil {
		onboardingFlow(cfg, configPath, noCacheFlag)
	}

	startMainApp(cfg, configPath, noCacheFlag)
}
