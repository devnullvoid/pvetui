package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/internal/config"
	core "github.com/devnullvoid/pvetui/internal/plugins/communityscripts"
	"github.com/devnullvoid/pvetui/pkg/api"
)

const communityScriptsPluginID = "community-scripts"

type communityScriptOutput struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Description   string   `json:"description,omitempty"`
	Categories    []string `json:"categories,omitempty"`
	Type          string   `json:"type"`
	SourceType    string   `json:"source_type,omitempty"`
	Target        string   `json:"target,omitempty"`
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
	Node           string                `json:"node"`
	Host           string                `json:"host"`
	SSHUser        string                `json:"ssh_user"`
	ExitCode       int                   `json:"exit_code"`
	NonInteractive bool                  `json:"non_interactive"`
	Env            []core.EnvOverride    `json:"env,omitempty"`
	Script         communityScriptOutput `json:"script"`
	Guest          *guestOutput          `json:"guest,omitempty"`
	CreatedGuests  []guestOutput         `json:"created_guests,omitempty"`
	Warnings       []string              `json:"warnings,omitempty"`
}

type communityScriptPlanOutput struct {
	Node           string                `json:"node"`
	Host           string                `json:"host"`
	SSHUser        string                `json:"ssh_user"`
	NonInteractive bool                  `json:"non_interactive"`
	Env            []core.EnvOverride    `json:"env,omitempty"`
	RemoteCommand  string                `json:"remote_command"`
	Script         communityScriptOutput `json:"script"`
	Guest          *guestOutput          `json:"guest,omitempty"`
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
	cmd.AddCommand(newCommunityScriptsPlanCmd())
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
		Use:     "install <slug-or-name> (--node <node>|--guest <vmid>)",
		Aliases: []string{"deploy"},
		Short:   "Install a community script on a Proxmox node",
		Long: `Install a Proxmox Community Script on the selected node.

Installer output is streamed to stderr so stdout can contain the final structured
result. Many upstream installers are interactive and may prompt in the terminal.
Use --yes with --set var_name=value overrides for unattended deployments.`,
		Example: `  pvetui community-scripts install nextcloud --node pve01
  pvetui --profile prod community-scripts deploy grafana --node pve02 --yes --set var_hostname=grafana --set var_cpu=2 --set var_ram=2048
  pvetui community-scripts install dockge --guest 200`,
		Args:              cobra.ExactArgs(1),
		RunE:              runCommunityScriptsInstall,
		ValidArgsFunction: completeCommunityScriptSlugs,
	}

	addCommunityScriptDeployFlags(cmd)

	return cmd
}

func newCommunityScriptsPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan <slug-or-name> (--node <node>|--guest <vmid>)",
		Short: "Show the planned community script install command",
		Long: `Show the resolved node, script metadata, environment overrides, and
redacted remote command that would be used for a Community Scripts install.`,
		Example: `  pvetui community-scripts plan grafana --node pve01 --yes --set var_hostname=grafana
  pvetui community-scripts plan docker --node pve01 --set var_nesting=1 --output table
  pvetui community-scripts plan dockge --guest 200`,
		Args:              cobra.ExactArgs(1),
		RunE:              runCommunityScriptsPlan,
		ValidArgsFunction: completeCommunityScriptSlugs,
	}

	addCommunityScriptDeployFlags(cmd)

	return cmd
}

func addCommunityScriptDeployFlags(cmd *cobra.Command) {
	cmd.Flags().String("node", "", "Target Proxmox node")
	cmd.Flags().Int("guest", 0, "Target running LXC container VMID for tools/addon scripts")
	cmd.Flags().Bool("skip-url-check", false, "Skip checking that the raw install script URL exists before SSH")
	cmd.Flags().StringArray("set", nil, "Community Scripts var_* override in KEY=VALUE form (repeatable)")
	cmd.Flags().Bool("yes", false, "Run without allocating a TTY and fail instead of waiting for interactive prompts")
	cmd.Flags().Bool("non-interactive", false, "Alias for --yes")
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
			rows = append(rows, []string{script.Name, script.Slug, script.Type, script.Target, state, script.Description})
		}
		printTable([]string{"NAME", "SLUG", "TYPE", "TARGET", "SOURCE", "DESCRIPTION"}, rows)
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

func runCommunityScriptsPlan(cmd *cobra.Command, args []string) error {
	plan, err := buildCommunityScriptPlan(cmd, args[0])
	if err != nil {
		return printError(err)
	}
	if plan == nil {
		return nil
	}

	if getOutputFormat(cmd) == outputTable {
		printCommunityScriptPlanTable(*plan)
		return nil
	}

	return printJSON(plan)
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
	guestID, _ := cmd.Flags().GetInt("guest")
	skipURLCheck, _ := cmd.Flags().GetBool("skip-url-check")
	env, err := parseCommunityScriptEnvFlags(cmd)
	if err != nil {
		return printError(err)
	}
	nonInteractive := communityScriptNonInteractive(cmd)

	script, err := findCommunityScript(args[0])
	if err != nil {
		return printError(err)
	}
	if err := validateCommunityScriptInstall(script); err != nil {
		return printError(err)
	}
	if guestID > 0 && !script.SupportsGuestInstall() {
		return printError(fmt.Errorf("script %q is not a tools/addon script and cannot be installed into an existing LXC", script.Slug))
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

	ctx := context.Background()
	node, guest, guestOut, err := resolveCommunityScriptTarget(ctx, session, nodeName, guestID)
	if err != nil {
		return printError(err)
	}

	var sshUser string
	var sshKeyfile string
	var jumpHost config.SSHJumpHost
	if guest != nil {
		sshUser, sshKeyfile, jumpHost = session.resolveSSHCreds(guest)
	} else {
		sshUser, jumpHost = session.resolveNodeSSHCreds(node)
		sshKeyfile = session.resolveNodeSSHKeyfile(node)
	}
	if sshUser == "" {
		return printError(fmt.Errorf("SSH user not configured; set ssh_user in config or use --ssh-user"))
	}

	host := node.IP
	if host == "" {
		host = node.Name
	}

	var warnings []string
	beforeVMs, beforeErr := session.getVMs(ctx)
	if beforeErr != nil {
		warnings = append(warnings, fmt.Sprintf("could not snapshot guests before install: %v", beforeErr))
	}

	if guest != nil {
		fmt.Fprintf(os.Stderr, "Installing %s in LXC %d on node %s (%s) as %s...\n", script.Slug, guest.ID, node.Name, host, sshUser)
	} else {
		fmt.Fprintf(os.Stderr, "Installing %s on node %s (%s) as %s...\n", script.Slug, node.Name, host, sshUser)
	}

	var installStdin io.Reader = os.Stdin
	if nonInteractive {
		installStdin = nil
	}

	installOptions := core.InstallOptions{
		User:           sshUser,
		Host:           host,
		Keyfile:        sshKeyfile,
		JumpHost:       jumpHost,
		Script:         script,
		Env:            env,
		NonInteractive: nonInteractive,
		Stdin:          installStdin,
		Stdout:         os.Stderr,
		Stderr:         os.Stderr,
	}
	var exitCode int
	if guest != nil {
		exitCode, err = core.InstallScriptInLXCWithOptions(ctx, installOptions, guest.ID)
	} else {
		exitCode, err = core.InstallScriptWithOptions(ctx, installOptions)
	}

	createdGuests := []guestOutput(nil)
	if err == nil && beforeErr == nil && guest == nil {
		if cacheErr := session.clearNodeInventoryCache(ctx, node.Name); cacheErr != nil {
			warnings = append(warnings, fmt.Sprintf("could not clear guest inventory cache after install: %v", cacheErr))
		}

		afterVMs, afterErr := session.getVMs(ctx)
		if afterErr != nil {
			warnings = append(warnings, fmt.Sprintf("could not refresh guests after install: %v", afterErr))
		} else {
			createdGuests = detectCreatedGuests(beforeVMs, afterVMs, node.Name)
		}
	}

	out := communityScriptInstallOutput{
		Node:           node.Name,
		Host:           host,
		SSHUser:        sshUser,
		ExitCode:       exitCode,
		NonInteractive: nonInteractive,
		Env:            redactCommunityScriptEnv(env),
		Script:         communityScriptToOutput(script),
		Guest:          guestOut,
		CreatedGuests:  createdGuests,
		Warnings:       warnings,
	}

	if err != nil {
		_ = printInstallResult(cmd, out)
		return printError(fmt.Errorf("community script install failed: %w", err))
	}

	return printInstallResult(cmd, out)
}

func buildCommunityScriptPlan(cmd *cobra.Command, nameOrSlug string) (*communityScriptPlanOutput, error) {
	session, err := initCLISession(cmd)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	if err := ensureCommunityScriptsEnabled(session); err != nil {
		return nil, err
	}

	nodeName, _ := cmd.Flags().GetString("node")
	guestID, _ := cmd.Flags().GetInt("guest")
	env, err := parseCommunityScriptEnvFlags(cmd)
	if err != nil {
		return nil, err
	}

	script, err := findCommunityScript(nameOrSlug)
	if err != nil {
		return nil, err
	}
	if err := validateCommunityScriptInstall(script); err != nil {
		return nil, err
	}
	if guestID > 0 && !script.SupportsGuestInstall() {
		return nil, fmt.Errorf("script %q is not a tools/addon script and cannot be installed into an existing LXC", script.Slug)
	}

	ctx := context.Background()
	node, guest, guestOut, err := resolveCommunityScriptTarget(ctx, session, nodeName, guestID)
	if err != nil {
		return nil, err
	}

	var sshUser string
	if guest != nil {
		sshUser, _, _ = session.resolveSSHCreds(guest)
	} else {
		sshUser, _ = session.resolveNodeSSHCreds(node)
	}
	if sshUser == "" {
		return nil, fmt.Errorf("SSH user not configured; set ssh_user in config or use --ssh-user")
	}

	host := node.IP
	if host == "" {
		host = node.Name
	}

	redactedEnv := redactCommunityScriptEnv(env)
	nonInteractive := communityScriptNonInteractive(cmd)
	preset := ""
	if nonInteractive {
		preset = "default"
	}

	var remoteCmd string
	if guest != nil {
		remoteCmd, err = core.BuildRemoteInstallInLXCCommandWithEnv(sshUser, guest.ID, script, redactedEnv, preset, nonInteractive)
	} else {
		remoteCmd, err = core.BuildRemoteInstallCommandWithMode(sshUser, script, redactedEnv, preset, nonInteractive)
	}
	if err != nil {
		return nil, err
	}

	return &communityScriptPlanOutput{
		Node:           node.Name,
		Host:           host,
		SSHUser:        sshUser,
		NonInteractive: nonInteractive,
		Env:            redactedEnv,
		RemoteCommand:  remoteCmd,
		Script:         communityScriptToOutput(script),
		Guest:          guestOut,
	}, nil
}

func printInstallResult(cmd *cobra.Command, out communityScriptInstallOutput) error {
	if getOutputFormat(cmd) == outputTable {
		printTable([]string{"FIELD", "VALUE"}, [][]string{
			{"Node", out.Node},
			{"Host", out.Host},
			{"SSH User", out.SSHUser},
			{"Script", out.Script.Slug},
			{"Guest", formatOptionalGuest(out.Guest)},
			{"Non-interactive", fmt.Sprintf("%t", out.NonInteractive)},
			{"Overrides", formatCommunityScriptEnv(out.Env)},
			{"Created Guests", formatGuestOutputs(out.CreatedGuests)},
			{"Warnings", strings.Join(out.Warnings, "; ")},
			{"Exit Code", fmt.Sprintf("%d", out.ExitCode)},
		})
		return nil
	}

	return printJSON(out)
}

func printCommunityScriptPlanTable(plan communityScriptPlanOutput) {
	printTable([]string{"FIELD", "VALUE"}, [][]string{
		{"Node", plan.Node},
		{"Host", plan.Host},
		{"SSH User", plan.SSHUser},
		{"Script", plan.Script.Slug},
		{"Script URL", plan.Script.ScriptURL},
		{"Guest", formatOptionalGuest(plan.Guest)},
		{"Non-interactive", fmt.Sprintf("%t", plan.NonInteractive)},
		{"Overrides", formatCommunityScriptEnv(plan.Env)},
		{"Remote Command", plan.RemoteCommand},
	})
}

func parseCommunityScriptEnvFlags(cmd *cobra.Command) ([]core.EnvOverride, error) {
	rawOverrides, _ := cmd.Flags().GetStringArray("set")
	overrides := make([]core.EnvOverride, 0, len(rawOverrides))

	seen := map[string]struct{}{}
	for _, raw := range rawOverrides {
		override, err := core.ParseEnvOverride(raw)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[override.Name]; exists {
			return nil, fmt.Errorf("duplicate community script override %q", override.Name)
		}
		seen[override.Name] = struct{}{}
		overrides = append(overrides, override)
	}

	return overrides, nil
}

func communityScriptNonInteractive(cmd *cobra.Command) bool {
	yes, _ := cmd.Flags().GetBool("yes")
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	return yes || nonInteractive
}

func resolveCommunityScriptTarget(ctx context.Context, session *cliSession, nodeName string, guestID int) (*api.Node, *api.VM, *guestOutput, error) {
	if guestID > 0 && strings.TrimSpace(nodeName) != "" {
		return nil, nil, nil, fmt.Errorf("--node and --guest are mutually exclusive")
	}

	var guest *api.VM
	var guestOut *guestOutput
	if guestID > 0 {
		vm, err := session.findVM(ctx, guestID)
		if err != nil {
			return nil, nil, nil, err
		}
		if vm.Type != api.VMTypeLXC {
			return nil, nil, nil, fmt.Errorf("guest %d is type %q; community tools can only be installed into LXC containers", guestID, vm.Type)
		}
		if vm.Status != "running" {
			return nil, nil, nil, fmt.Errorf("guest %d is not running (status: %s)", guestID, vm.Status)
		}
		guest = vm
		nodeName = vm.Node
		out := vmToGuestOutput(vm)
		guestOut = &out
	}

	if strings.TrimSpace(nodeName) == "" {
		return nil, nil, nil, fmt.Errorf("target required: pass --node <node> or --guest <vmid>")
	}

	node, err := session.findNodeByName(ctx, nodeName)
	if err != nil {
		return nil, nil, nil, err
	}
	if !node.Online {
		return nil, nil, nil, fmt.Errorf("node %q is offline", nodeName)
	}

	return node, guest, guestOut, nil
}

func formatOptionalGuest(guest *guestOutput) string {
	if guest == nil {
		return ""
	}
	return fmt.Sprintf("%d/%s/%s", guest.ID, guest.Name, guest.Type)
}

func redactCommunityScriptEnv(env []core.EnvOverride) []core.EnvOverride {
	return core.RedactEnvOverrides(env)
}

func formatCommunityScriptEnv(env []core.EnvOverride) string {
	if len(env) == 0 {
		return ""
	}

	parts := make([]string, 0, len(env))
	for _, override := range env {
		parts = append(parts, override.Name+"="+override.Value)
	}

	return strings.Join(parts, ", ")
}

func detectCreatedGuests(before, after []*api.VM, nodeName string) []guestOutput {
	beforeGuests := map[guestIdentity]struct{}{}
	for _, vm := range before {
		if vm != nil {
			beforeGuests[guestIdentityFromVM(vm)] = struct{}{}
		}
	}

	var created []guestOutput
	for _, vm := range after {
		if vm == nil || vm.Node != nodeName {
			continue
		}
		if _, existed := beforeGuests[guestIdentityFromVM(vm)]; existed {
			continue
		}
		created = append(created, vmToGuestOutput(vm))
	}

	return created
}

type guestIdentity struct {
	id            int
	node          string
	guestType     string
	sourceProfile string
}

func guestIdentityFromVM(vm *api.VM) guestIdentity {
	if vm == nil {
		return guestIdentity{}
	}

	return guestIdentity{
		id:            vm.ID,
		node:          vm.Node,
		guestType:     vm.Type,
		sourceProfile: vm.SourceProfile,
	}
}

func formatGuestOutputs(guests []guestOutput) string {
	if len(guests) == 0 {
		return ""
	}

	parts := make([]string, 0, len(guests))
	for _, guest := range guests {
		parts = append(parts, fmt.Sprintf("%d/%s/%s", guest.ID, guest.Name, guest.Type))
	}

	return strings.Join(parts, ", ")
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
		SourceType:    script.SourceType,
		Target:        script.Target,
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
		{"Source Type", script.SourceType},
		{"Target", script.Target},
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
