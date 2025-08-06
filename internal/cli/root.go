package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/devnullvoid/proxmox-tui/internal/bootstrap"
	"github.com/devnullvoid/proxmox-tui/internal/version"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "proxmox-tui",
	Short: "A terminal user interface for Proxmox VE",
	Long: `Proxmox TUI is a terminal user interface for managing Proxmox VE clusters.

It provides an interactive interface for managing virtual machines, containers,
nodes, and other Proxmox resources directly from the terminal.`,
	Version: version.GetVersionString(),
	RunE:    runMainApplication,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// init initializes the root command and sets up flags
func init() {
	// Disable cobra's completion command for now
	RootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add persistent flags
	addPersistentFlags(RootCmd)

	// Add commands
	RootCmd.AddCommand(newConfigWizardCmd())
}

// runMainApplication runs the main application
func runMainApplication(cmd *cobra.Command, args []string) error {
	opts := getBootstrapOptions(cmd)

	// Bootstrap the application
	result, err := bootstrap.Bootstrap(opts)
	if err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// If result is nil, the application should exit (e.g., version flag)
	if result == nil {
		return nil
	}

	// Start the main application
	// Handle application runtime errors differently from CLI usage errors
	if err := bootstrap.StartApplication(result); err != nil {
		// Application runtime error - exit directly without showing usage
		os.Exit(1)
	}

	return nil
}

// getBootstrapOptions converts cobra flags to BootstrapOptions
func getBootstrapOptions(cmd *cobra.Command) bootstrap.BootstrapOptions {
	configPath, _ := cmd.Flags().GetString("config")
	profile, _ := cmd.Flags().GetString("profile")
	noCache, _ := cmd.Flags().GetBool("no-cache")
	version, _ := cmd.Flags().GetBool("version")
	configWizard, _ := cmd.Flags().GetBool("config-wizard")

	// Get config values from viper (which handles env vars)
	addr := viper.GetString("addr")
	user := viper.GetString("user")
	password := viper.GetString("password")
	tokenID := viper.GetString("token_id")
	tokenSecret := viper.GetString("token_secret")
	realm := viper.GetString("realm")
	insecure := viper.GetBool("insecure")
	apiPath := viper.GetString("api_path")
	sshUser := viper.GetString("ssh_user")
	debug := viper.GetBool("debug")
	cacheDir := viper.GetString("cache_dir")

	return bootstrap.BootstrapOptions{
		ConfigPath:      configPath,
		Profile:         profile,
		NoCache:         noCache,
		Version:         version,
		ConfigWizard:    configWizard,
		FlagAddr:        addr,
		FlagUser:        user,
		FlagPassword:    password,
		FlagTokenID:     tokenID,
		FlagTokenSecret: tokenSecret,
		FlagRealm:       realm,
		FlagInsecure:    insecure,
		FlagApiPath:     apiPath,
		FlagSSHUser:     sshUser,
		FlagDebug:       debug,
		FlagCacheDir:    cacheDir,
	}
}

// addPersistentFlags adds all the persistent flags to the root command
func addPersistentFlags(cmd *cobra.Command) {
	// Bootstrap flags
	cmd.PersistentFlags().StringP("config", "c", "", "Path to YAML config file")
	cmd.PersistentFlags().StringP("profile", "p", "", "Connection profile to use (overrides default_profile)")
	cmd.PersistentFlags().BoolP("no-cache", "n", false, "Disable caching")
	cmd.PersistentFlags().BoolP("version", "v", false, "Show version information")
	cmd.PersistentFlags().BoolP("config-wizard", "w", false, "Launch interactive config wizard and exit")

	// Config flags
	cmd.PersistentFlags().String("addr", "", "Proxmox API URL")
	cmd.PersistentFlags().String("user", "", "Proxmox username")
	cmd.PersistentFlags().String("password", "", "Proxmox password")
	cmd.PersistentFlags().String("token-id", "", "Proxmox API token ID")
	cmd.PersistentFlags().String("token-secret", "", "Proxmox API token secret")
	cmd.PersistentFlags().String("realm", "", "Proxmox realm")
	cmd.PersistentFlags().Bool("insecure", false, "Skip TLS verification")
	cmd.PersistentFlags().String("api-path", "", "Proxmox API path")
	cmd.PersistentFlags().String("ssh-user", "", "SSH username")
	cmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	cmd.PersistentFlags().String("cache-dir", "", "Cache directory path")

	// Bind flags to environment variables
	viper.SetEnvPrefix("PROXMOX")
	viper.AutomaticEnv()

	// Bind each flag to its corresponding environment variable
	if err := viper.BindPFlag("addr", cmd.PersistentFlags().Lookup("addr")); err != nil {
		panic(fmt.Sprintf("failed to bind addr flag: %v", err))
	}
	if err := viper.BindPFlag("user", cmd.PersistentFlags().Lookup("user")); err != nil {
		panic(fmt.Sprintf("failed to bind user flag: %v", err))
	}
	if err := viper.BindPFlag("password", cmd.PersistentFlags().Lookup("password")); err != nil {
		panic(fmt.Sprintf("failed to bind password flag: %v", err))
	}
	if err := viper.BindPFlag("token_id", cmd.PersistentFlags().Lookup("token-id")); err != nil {
		panic(fmt.Sprintf("failed to bind token_id flag: %v", err))
	}
	if err := viper.BindPFlag("token_secret", cmd.PersistentFlags().Lookup("token-secret")); err != nil {
		panic(fmt.Sprintf("failed to bind token_secret flag: %v", err))
	}
	if err := viper.BindPFlag("realm", cmd.PersistentFlags().Lookup("realm")); err != nil {
		panic(fmt.Sprintf("failed to bind realm flag: %v", err))
	}
	if err := viper.BindPFlag("insecure", cmd.PersistentFlags().Lookup("insecure")); err != nil {
		panic(fmt.Sprintf("failed to bind insecure flag: %v", err))
	}
	if err := viper.BindPFlag("api_path", cmd.PersistentFlags().Lookup("api-path")); err != nil {
		panic(fmt.Sprintf("failed to bind api_path flag: %v", err))
	}
	if err := viper.BindPFlag("ssh_user", cmd.PersistentFlags().Lookup("ssh-user")); err != nil {
		panic(fmt.Sprintf("failed to bind ssh_user flag: %v", err))
	}
	if err := viper.BindPFlag("debug", cmd.PersistentFlags().Lookup("debug")); err != nil {
		panic(fmt.Sprintf("failed to bind debug flag: %v", err))
	}
	if err := viper.BindPFlag("cache_dir", cmd.PersistentFlags().Lookup("cache-dir")); err != nil {
		panic(fmt.Sprintf("failed to bind cache_dir flag: %v", err))
	}
}
