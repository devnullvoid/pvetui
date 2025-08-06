// Package bootstrap handles application initialization and startup logic.
//
// This package provides a clean separation between the main entry point
// and the actual application startup process, making the code more
// testable and maintainable.
package bootstrap

import (
	"flag"
	"fmt"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/app"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/internal/onboarding"
	"github.com/devnullvoid/proxmox-tui/internal/profile"
	"github.com/devnullvoid/proxmox-tui/internal/ui/components"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/internal/version"
)

// BootstrapOptions contains all the options for bootstrapping the application.
type BootstrapOptions struct {
	ConfigPath   string
	Profile      string
	NoCache      bool
	Version      bool
	ConfigWizard bool
	// Flag values for config overrides
	FlagAddr        string
	FlagUser        string
	FlagPassword    string
	FlagTokenID     string
	FlagTokenSecret string
	FlagRealm       string
	FlagInsecure    bool
	FlagApiPath     string
	FlagSSHUser     string
	FlagDebug       bool
	FlagCacheDir    string
}

// BootstrapResult contains the result of the bootstrap process.
type BootstrapResult struct {
	Config     *config.Config
	ConfigPath string
	Profile    string
	NoCache    bool
}

// ParseFlags parses command line flags and returns bootstrap options.
func ParseFlags() BootstrapOptions {
	var configPath, profile string
	var noCache, version, configWizard bool

	// Bootstrap flags
	flag.StringVar(&configPath, "config", "", "Path to YAML config file")
	flag.StringVar(&configPath, "c", "", "Short for --config")
	flag.StringVar(&profile, "profile", "", "Connection profile to use (overrides default_profile)")
	flag.StringVar(&profile, "p", "", "Short for --profile")
	flag.BoolVar(&noCache, "no-cache", false, "Disable caching")
	flag.BoolVar(&noCache, "n", false, "Short for --no-cache")
	flag.BoolVar(&version, "version", false, "Show version information")
	flag.BoolVar(&version, "v", false, "Short for --version")
	flag.BoolVar(&configWizard, "config-wizard", false, "Launch interactive config wizard and exit")
	flag.BoolVar(&configWizard, "w", false, "Short for --config-wizard")

	// Config flags (these will be applied to the config object later)
	var flagAddr, flagUser, flagPassword, flagTokenID, flagTokenSecret, flagRealm, flagApiPath, flagSSHUser, flagCacheDir string
	var flagInsecure, flagDebug bool

	flag.StringVar(&flagAddr, "addr", "", "Proxmox API URL (env PROXMOX_ADDR)")
	flag.StringVar(&flagAddr, "a", "", "Short for --addr")
	flag.StringVar(&flagUser, "user", "", "Proxmox username (env PROXMOX_USER)")
	flag.StringVar(&flagUser, "u", "", "Short for --user")
	flag.StringVar(&flagPassword, "password", "", "Proxmox password (env PROXMOX_PASSWORD)")
	flag.StringVar(&flagPassword, "pass", "", "Short for --password")
	flag.StringVar(&flagTokenID, "token-id", "", "Proxmox API token ID (env PROXMOX_TOKEN_ID)")
	flag.StringVar(&flagTokenID, "tid", "", "Short for --token-id")
	flag.StringVar(&flagTokenSecret, "token-secret", "", "Proxmox API token secret (env PROXMOX_TOKEN_SECRET)")
	flag.StringVar(&flagTokenSecret, "ts", "", "Short for --token-secret")
	flag.StringVar(&flagRealm, "realm", "", "Proxmox realm (env PROXMOX_REALM)")
	flag.StringVar(&flagRealm, "r", "", "Short for --realm")
	flag.BoolVar(&flagInsecure, "insecure", false, "Skip TLS verification (env PROXMOX_INSECURE)")
	flag.BoolVar(&flagInsecure, "i", false, "Short for --insecure")
	flag.StringVar(&flagApiPath, "api-path", "", "Proxmox API path (env PROXMOX_API_PATH)")
	flag.StringVar(&flagApiPath, "ap", "", "Short for --api-path")
	flag.StringVar(&flagSSHUser, "ssh-user", "", "SSH username (env PROXMOX_SSH_USER)")
	flag.StringVar(&flagSSHUser, "su", "", "Short for --ssh-user")
	flag.BoolVar(&flagDebug, "debug", false, "Enable debug logging (env PROXMOX_DEBUG)")
	flag.BoolVar(&flagDebug, "d", false, "Short for --debug")
	flag.StringVar(&flagCacheDir, "cache-dir", "", "Cache directory path (env PROXMOX_CACHE_DIR)")
	flag.StringVar(&flagCacheDir, "cd", "", "Short for --cache-dir")

	flag.Parse()

	return BootstrapOptions{
		ConfigPath:   configPath,
		Profile:      profile,
		NoCache:      noCache,
		Version:      version,
		ConfigWizard: configWizard,
		// Store flag values for later use
		FlagAddr:        flagAddr,
		FlagUser:        flagUser,
		FlagPassword:    flagPassword,
		FlagTokenID:     flagTokenID,
		FlagTokenSecret: flagTokenSecret,
		FlagRealm:       flagRealm,
		FlagInsecure:    flagInsecure,
		FlagApiPath:     flagApiPath,
		FlagSSHUser:     flagSSHUser,
		FlagDebug:       flagDebug,
		FlagCacheDir:    flagCacheDir,
	}
}

// Bootstrap handles the complete application bootstrap process.
func Bootstrap(opts BootstrapOptions) (*BootstrapResult, error) {
	// Handle version flag
	if opts.Version {
		printVersion()
		return nil, nil
	}

	// Initialize configuration
	cfg := config.NewConfig()

	// Resolve configuration path
	configPath := ResolveConfigPath(opts.ConfigPath)

	// Handle config wizard BEFORE config loading and profile resolution
	// This allows the wizard to work even when no config file exists
	if opts.ConfigWizard {
		// Try to load existing config if it exists, but don't fail if it doesn't
		if configPath != "" {
			_ = cfg.MergeWithFile(configPath) // Ignore errors for config wizard
		}

		// For config wizard, we don't need profile resolution
		if err := HandleConfigWizard(cfg, configPath, opts.Profile); err != nil {
			return nil, fmt.Errorf("config wizard failed: %w", err)
		}
		return nil, nil
	}

	// Regular application flow: load config and resolve profiles
	if configPath != "" {
		if err := cfg.MergeWithFile(configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Handle profile selection
	selectedProfile, err := profile.ResolveProfile(opts.Profile, cfg)
	if err != nil {
		return nil, fmt.Errorf("profile resolution failed: %w", err)
	}

	// Apply selected profile
	if selectedProfile != "" {
		if err := cfg.ApplyProfile(selectedProfile); err != nil {
			return nil, fmt.Errorf("could not select profile '%s': %w", selectedProfile, err)
		}
	}

	// Apply command line flags to config (overrides env vars, config file, and profile)
	applyFlagsToConfig(cfg, opts)

	// Update the active profile with flag values so GetAddr() returns the correct values
	if selectedProfile != "" && len(cfg.Profiles) > 0 {
		if profile, exists := cfg.Profiles[selectedProfile]; exists {
			if opts.FlagAddr != "" {
				profile.Addr = opts.FlagAddr
			}
			if opts.FlagUser != "" {
				profile.User = opts.FlagUser
			}
			if opts.FlagPassword != "" {
				profile.Password = opts.FlagPassword
			}
			if opts.FlagTokenID != "" {
				profile.TokenID = opts.FlagTokenID
			}
			if opts.FlagTokenSecret != "" {
				profile.TokenSecret = opts.FlagTokenSecret
			}
			if opts.FlagRealm != "" {
				profile.Realm = opts.FlagRealm
			}
			if opts.FlagInsecure {
				profile.Insecure = true
			}
			if opts.FlagApiPath != "" {
				profile.ApiPath = opts.FlagApiPath
			}
			if opts.FlagSSHUser != "" {
				profile.SSHUser = opts.FlagSSHUser
			}
			cfg.Profiles[selectedProfile] = profile
		}
	}

	// Set defaults and validate
	cfg.SetDefaults()
	config.DebugEnabled = cfg.Debug
	logger.SetDebugEnabled(cfg.Debug)

	// Handle validation errors with onboarding
	if err := cfg.Validate(); err != nil {
		if err := onboarding.HandleValidationError(cfg, configPath, opts.NoCache, selectedProfile); err != nil {
			return nil, fmt.Errorf("onboarding failed: %w", err)
		}
		return nil, nil
	}

	return &BootstrapResult{
		Config:     cfg,
		ConfigPath: configPath,
		Profile:    selectedProfile,
		NoCache:    opts.NoCache,
	}, nil
}

// applyFlagsToConfig applies command line flags to the config object
func applyFlagsToConfig(cfg *config.Config, opts BootstrapOptions) {
	// Apply flag values to config if they were set
	if opts.FlagAddr != "" {
		cfg.Addr = opts.FlagAddr
	}
	if opts.FlagUser != "" {
		cfg.User = opts.FlagUser
	}
	if opts.FlagPassword != "" {
		cfg.Password = opts.FlagPassword
	}
	if opts.FlagTokenID != "" {
		cfg.TokenID = opts.FlagTokenID
	}
	if opts.FlagTokenSecret != "" {
		cfg.TokenSecret = opts.FlagTokenSecret
	}
	if opts.FlagRealm != "" {
		cfg.Realm = opts.FlagRealm
	}
	if opts.FlagInsecure {
		cfg.Insecure = true
	}
	if opts.FlagApiPath != "" {
		cfg.ApiPath = opts.FlagApiPath
	}
	if opts.FlagSSHUser != "" {
		cfg.SSHUser = opts.FlagSSHUser
	}
	if opts.FlagDebug {
		cfg.Debug = true
	}
	if opts.FlagCacheDir != "" {
		cfg.CacheDir = opts.FlagCacheDir
	}
}

// StartApplication starts the main application with the given configuration.
func StartApplication(result *BootstrapResult) error {
	if result == nil {
		return fmt.Errorf("bootstrap result is nil")
	}

	fmt.Println("\n🚀 Starting Proxmox TUI...")

	if result.ConfigPath != "" {
		fmt.Printf("✅ Configuration loaded from %s\n", result.ConfigPath)
	} else {
		fmt.Println("✅ Configuration loaded from environment variables")
	}

	// Apply theme configuration
	theme.ApplyCustomTheme(&result.Config.Theme)
	theme.ApplyToTview()

	appOpts := app.Options{NoCache: result.NoCache}
	if err := app.RunWithStartupVerification(result.Config, result.ConfigPath, appOpts); err != nil {
		return handleStartupError(err, result.Config)
	}

	fmt.Println("🚪 Exiting.")
	return nil
}

// ResolveConfigPath resolves the configuration file path.
func ResolveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}

	if path, found := config.FindDefaultConfigPath(); found {
		return path
	}

	return ""
}

// HandleConfigWizard launches the configuration wizard.
func HandleConfigWizard(cfg *config.Config, configPath string, activeProfile string) error {
	res := components.LaunchConfigWizard(cfg, configPath, activeProfile)

	switch {
	case res.SopsEncrypted:
		fmt.Printf("✅ Configuration saved and encrypted with SOPS: %s\n", configPath)
	case res.Saved:
		fmt.Println("✅ Configuration saved.")
	case res.Canceled:
		fmt.Println("🚪 Exiting.")
	}

	return nil
}

// handleStartupError provides user-friendly error messages for startup failures.
func handleStartupError(err error, cfg *config.Config) error {
	fmt.Printf("❌ %v\n", err)
	fmt.Println()

	if strings.Contains(err.Error(), "authentication failed") {
		fmt.Println("💡 Please check your credentials in the config file:")
		fmt.Printf("   %s\n", config.GetDefaultConfigPath())
	} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
		fmt.Println("💡 Please check your Proxmox server address and network connectivity:")
		fmt.Printf("   Current address: %s\n", cfg.Addr)
	}

	return err
}

// printVersion prints version information.
func printVersion() {
	info := version.GetBuildInfo()
	fmt.Printf("proxmox-tui version %s\n", info.Version)
	fmt.Printf("Build date: %s\n", info.BuildDate)
	fmt.Printf("Commit: %s\n", info.Commit)
	fmt.Printf("Go version: %s\n", info.GoVersion)
	fmt.Printf("OS/Arch: %s/%s\n", info.OS, info.Arch)
}
