package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	core "github.com/devnullvoid/pvetui/internal/plugins/communityscripts"
)

const communityScriptsPluginID = "community-scripts"

type communityScriptOutput struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Description   string   `json:"description,omitempty"`
	Categories    []string `json:"categories,omitempty"`
	Type          string   `json:"type"`
	ScriptPath    string   `json:"script_path"`
	SourceRepo    string   `json:"source_repo"`
	ScriptURL     string   `json:"script_url"`
	Website       string   `json:"website,omitempty"`
	Documentation string   `json:"documentation,omitempty"`
	ConfigPath    string   `json:"config_path,omitempty"`
	InterfacePort int      `json:"interface_port,omitempty"`
	Updateable    bool     `json:"updateable"`
	Privileged    bool     `json:"privileged"`
	IsDev         bool     `json:"is_dev"`
	IsDisabled    bool     `json:"is_disabled"`
	IsDeleted     bool     `json:"is_deleted"`
	DateCreated   string   `json:"date_created,omitempty"`
}

type communityScriptInstallOutput struct {
	Node     string                `json:"node"`
	Host     string                `json:"host"`
	SSHUser  string                `json:"ssh_user"`
	ExitCode int                   `json:"exit_code"`
	Script   communityScriptOutput `json:"script"`
}

func newCommunityScriptsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "community-scripts",
		Aliases: []string{"scripts"},
		Short:   "Search and install Proxmox Community Scripts",
		Long: `Search, inspect, and install Proxmox Community Scripts from CLI mode.

The community-scripts plugin must be enabled in config. Install uses SSH to the
selected Proxmox node and runs the same remote installer flow as the TUI plugin.`,
	}

	cmd.AddCommand(newCommunityScriptsSearchCmd())
	cmd.AddCommand(newCommunityScriptsShowCmd())
	cmd.AddCommand(newCommunityScriptsInstallCmd())

	return cmd
}

func newCommunityScriptsSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search available community scripts",
		Example: `  pvetui community-scripts search nextcloud
  pvetui community-scripts search docker --output table`,
		Args: cobra.ExactArgs(1),
		RunE: runCommunityScriptsSearch,
	}
}

func newCommunityScriptsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug-or-name>",
		Short: "Show details for a community script",
		Example: `  pvetui community-scripts show nextcloud
  pvetui community-scripts show "Home Assistant" --output table`,
		Args:              cobra.ExactArgs(1),
		RunE:              runCommunityScriptsShow,
		ValidArgsFunction: completeCommunityScriptSlugs,
	}
}

func newCommunityScriptsInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <slug-or-name> --node <node>",
		Short: "Install a community script on a Proxmox node",
		Long: `Install a Proxmox Community Script on the selected node.

Installer output is streamed to stderr so stdout can contain the final structured
result. Many upstream installers are interactive and may prompt in the terminal.`,
		Example: `  pvetui community-scripts install nextcloud --node pve01
  pvetui --profile prod community-scripts install docker --node pve02`,
		Args:              cobra.ExactArgs(1),
		RunE:              runCommunityScriptsInstall,
		ValidArgsFunction: completeCommunityScriptSlugs,
	}

	cmd.Flags().String("node", "", "Target Proxmox node")
	cmd.Flags().Bool("skip-url-check", false, "Skip checking that the raw install script URL exists before SSH")
	_ = cmd.MarkFlagRequired("node")

	return cmd
}

func runCommunityScriptsSearch(cmd *cobra.Command, args []string) error {
	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}
	if session == nil {
		return nil
	}
	if err := ensureCommunityScriptsEnabled(session); err != nil {
		return printError(err)
	}

	scripts, err := core.FetchScripts()
	if err != nil {
		return printError(fmt.Errorf("failed to fetch community scripts: %w", err))
	}

	matches := core.SearchScripts(scripts, args[0])
	out := communityScriptsToOutput(matches)

	if getOutputFormat(cmd) == outputTable {
		rows := make([][]string, 0, len(out))
		for _, script := range out {
			state := "prod"
			if script.IsDev {
				state = "dev"
			}
			rows = append(rows, []string{script.Name, script.Slug, script.Type, state, script.Description})
		}
		printTable([]string{"NAME", "SLUG", "TYPE", "SOURCE", "DESCRIPTION"}, rows)
		return nil
	}

	return printJSON(out)
}

func runCommunityScriptsShow(cmd *cobra.Command, args []string) error {
	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}
	if session == nil {
		return nil
	}
	if err := ensureCommunityScriptsEnabled(session); err != nil {
		return printError(err)
	}

	script, err := findCommunityScript(args[0])
	if err != nil {
		return printError(err)
	}

	out := communityScriptToOutput(script)
	if getOutputFormat(cmd) == outputTable {
		printCommunityScriptDetailsTable(out)
		return nil
	}

	return printJSON(out)
}

func runCommunityScriptsInstall(cmd *cobra.Command, args []string) error {
	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}
	if session == nil {
		return nil
	}
	if err := ensureCommunityScriptsEnabled(session); err != nil {
		return printError(err)
	}

	nodeName, _ := cmd.Flags().GetString("node")
	skipURLCheck, _ := cmd.Flags().GetBool("skip-url-check")

	ctx := context.Background()
	node, err := session.findNodeByName(ctx, nodeName)
	if err != nil {
		return printError(err)
	}
	if !node.Online {
		return printError(fmt.Errorf("node %q is offline", nodeName))
	}

	script, err := findCommunityScript(args[0])
	if err != nil {
		return printError(err)
	}
	if err := validateCommunityScriptInstall(script); err != nil {
		return printError(err)
	}
	if !skipURLCheck {
		ok, err := core.ScriptURLExists(script)
		if err != nil {
			return printError(fmt.Errorf("failed to verify script URL: %w", err))
		}
		if !ok {
			return printError(fmt.Errorf("script URL not found: %s", core.RawScriptURL(script)))
		}
	}

	sshUser, jumpHost := session.resolveNodeSSHCreds(node)
	if sshUser == "" {
		return printError(fmt.Errorf("SSH user not configured; set ssh_user in config or use --ssh-user"))
	}

	host := node.IP
	if host == "" {
		host = node.Name
	}

	fmt.Fprintf(os.Stderr, "Installing %s on node %s (%s) as %s...\n", script.Slug, node.Name, host, sshUser)

	exitCode, err := core.InstallScriptWithOptions(ctx, core.InstallOptions{
		User:     sshUser,
		Host:     host,
		Keyfile:  session.resolveNodeSSHKeyfile(node),
		JumpHost: jumpHost,
		Script:   script,
		Stdin:    os.Stdin,
		Stdout:   os.Stderr,
		Stderr:   os.Stderr,
	})

	out := communityScriptInstallOutput{
		Node:     node.Name,
		Host:     host,
		SSHUser:  sshUser,
		ExitCode: exitCode,
		Script:   communityScriptToOutput(script),
	}

	if err != nil {
		_ = printInstallResult(cmd, out)
		return printError(fmt.Errorf("community script install failed: %w", err))
	}

	return printInstallResult(cmd, out)
}

func printInstallResult(cmd *cobra.Command, out communityScriptInstallOutput) error {
	if getOutputFormat(cmd) == outputTable {
		printTable([]string{"FIELD", "VALUE"}, [][]string{
			{"Node", out.Node},
			{"Host", out.Host},
			{"SSH User", out.SSHUser},
			{"Script", out.Script.Slug},
			{"Exit Code", fmt.Sprintf("%d", out.ExitCode)},
		})
		return nil
	}

	return printJSON(out)
}

func ensureCommunityScriptsEnabled(session *cliSession) error {
	if session == nil || session.cfg == nil {
		return fmt.Errorf("configuration unavailable")
	}
	for _, id := range session.cfg.Plugins.Enabled {
		if id == communityScriptsPluginID {
			return nil
		}
	}

	return fmt.Errorf("community-scripts plugin is not enabled; add %q to plugins.enabled", communityScriptsPluginID)
}

func findCommunityScript(nameOrSlug string) (core.Script, error) {
	scripts, err := core.FetchScripts()
	if err != nil {
		return core.Script{}, fmt.Errorf("failed to fetch community scripts: %w", err)
	}

	return core.FindScript(scripts, nameOrSlug)
}

func validateCommunityScriptInstall(script core.Script) error {
	switch {
	case script.ScriptPath == "":
		return fmt.Errorf("script %q does not have a supported install script path", script.Slug)
	case script.IsDisabled:
		return fmt.Errorf("script %q is disabled upstream", script.Slug)
	case script.IsDeleted:
		return fmt.Errorf("script %q is deleted upstream", script.Slug)
	default:
		return nil
	}
}

func communityScriptsToOutput(scripts []core.Script) []communityScriptOutput {
	out := make([]communityScriptOutput, 0, len(scripts))
	for _, script := range scripts {
		out = append(out, communityScriptToOutput(script))
	}

	return out
}

func communityScriptToOutput(script core.Script) communityScriptOutput {
	sourceRepo := "community-scripts/ProxmoxVE"
	if script.IsDev {
		sourceRepo = "community-scripts/ProxmoxVED"
	}

	return communityScriptOutput{
		Name:          script.Name,
		Slug:          script.Slug,
		Description:   script.Description,
		Categories:    append([]string(nil), script.Categories...),
		Type:          script.Type,
		ScriptPath:    script.ScriptPath,
		SourceRepo:    sourceRepo,
		ScriptURL:     core.RawScriptURL(script),
		Website:       script.Website,
		Documentation: script.Documentation,
		ConfigPath:    script.ConfigPath,
		InterfacePort: script.InterfacePort,
		Updateable:    script.Updateable,
		Privileged:    script.Privileged,
		IsDev:         script.IsDev,
		IsDisabled:    script.IsDisabled,
		IsDeleted:     script.IsDeleted,
		DateCreated:   script.DateCreated,
	}
}

func printCommunityScriptDetailsTable(script communityScriptOutput) {
	printTable([]string{"FIELD", "VALUE"}, [][]string{
		{"Name", script.Name},
		{"Slug", script.Slug},
		{"Type", script.Type},
		{"Description", script.Description},
		{"Categories", strings.Join(script.Categories, ", ")},
		{"Source Repo", script.SourceRepo},
		{"Script Path", script.ScriptPath},
		{"Script URL", script.ScriptURL},
		{"Website", script.Website},
		{"Documentation", script.Documentation},
		{"Config Path", script.ConfigPath},
		{"Interface Port", fmt.Sprintf("%d", script.InterfacePort)},
		{"Updateable", fmt.Sprintf("%t", script.Updateable)},
		{"Privileged", fmt.Sprintf("%t", script.Privileged)},
		{"Development", fmt.Sprintf("%t", script.IsDev)},
		{"Disabled", fmt.Sprintf("%t", script.IsDisabled)},
		{"Deleted", fmt.Sprintf("%t", script.IsDeleted)},
	})
}

func completeCommunityScriptSlugs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	session, err := initCLISession(cmd)
	if err != nil || session == nil || ensureCommunityScriptsEnabled(session) != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	scripts, err := core.FetchScripts()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	prefix := strings.ToLower(toComplete)
	completions := make([]string, 0, len(scripts))
	for _, script := range scripts {
		if prefix == "" || strings.HasPrefix(strings.ToLower(script.Slug), prefix) {
			completions = append(completions, fmt.Sprintf("%s\t%s", script.Slug, script.Name))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
