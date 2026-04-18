package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/internal/bootstrap"
	"github.com/devnullvoid/pvetui/internal/cache"
	"github.com/devnullvoid/pvetui/internal/config"
	commandrunner "github.com/devnullvoid/pvetui/internal/plugins/command-runner"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// Output format constants used across all subcommand files.
const (
	outputJSON  = "json"
	outputTable = "table"
)

// cliSession abstracts over single-profile and group-profile connections so
// that subcommand handlers work identically in both cases.
type cliSession struct {
	single *api.Client
	group  *api.GroupClientManager
	cfg    *config.Config
}

// getNodes returns all cluster nodes. In group mode it fans out across all
// connected profiles and merges the results.
func (s *cliSession) getNodes(ctx context.Context) ([]*api.Node, error) {
	if s.group != nil {
		return s.group.GetGroupNodes(ctx)
	}

	cluster, err := s.single.GetClusterStatus()
	if err != nil {
		return nil, err
	}

	return cluster.Nodes, nil
}

// getVMs returns all guests across all nodes. In group mode it fans out across
// all connected profiles.
func (s *cliSession) getVMs(ctx context.Context) ([]*api.VM, error) {
	if s.group != nil {
		return s.group.GetGroupVMs(ctx)
	}

	cluster, err := s.single.GetClusterStatus()
	if err != nil {
		return nil, err
	}

	var vms []*api.VM

	for _, n := range cluster.Nodes {
		if n == nil {
			continue
		}

		vms = append(vms, n.VMs...)
	}

	return vms, nil
}

// findVM locates a guest by VMID across all profiles.
func (s *cliSession) findVM(ctx context.Context, vmid int) (*api.VM, error) {
	if s.group != nil {
		vm, _, err := s.group.FindVMByIDInGroup(ctx, vmid)
		if err != nil {
			return nil, fmt.Errorf("guest %d not found", vmid)
		}

		return vm, nil
	}

	return findGuestByVMID(s.single, vmid)
}

// clientForVM returns the API client responsible for the given guest. In group
// mode it resolves via SourceProfile; in single mode it returns the single client.
func (s *cliSession) clientForVM(vm *api.VM) (*api.Client, error) {
	if s.group != nil {
		pc, exists := s.group.GetClient(vm.SourceProfile)
		if !exists || pc == nil {
			return nil, fmt.Errorf("no client for profile %q", vm.SourceProfile)
		}

		return pc.Client, nil
	}

	return s.single, nil
}

// getTasks returns recent cluster tasks across all profiles.
func (s *cliSession) getTasks(ctx context.Context) ([]*api.ClusterTask, error) {
	if s.group != nil {
		return s.group.GetGroupTasks(ctx)
	}

	return s.single.GetClusterTasks()
}

// initCLISession bootstraps configuration and returns an authenticated session.
// Returns (nil, nil) when bootstrap handled an early-exit flag (--version,
// --list-profiles) — callers should return nil without doing further work.
func initCLISession(cmd *cobra.Command) (*cliSession, error) {
	opts := getBootstrapOptions(cmd)
	opts.Quiet = true // suppress TUI startup banners for CLI subcommands

	result, err := bootstrap.Bootstrap(opts)
	if err != nil {
		return nil, fmt.Errorf("bootstrap failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	cfg := result.Config

	// Normalize the API URL the same way the TUI does.
	cfg.Addr = strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.ApiPath, "/")

	// Initialize global cache unless disabled.
	if !result.NoCache {
		if cacheErr := cache.InitGlobalCache(cfg.CacheDir); cacheErr != nil {
			_ = cacheErr // non-fatal; continue without persistent cache
		}
	}

	loggerAdapter := adapters.NewLoggerAdapter(cfg)
	cacheAdapter := adapters.NewCacheAdapter()

	// Single-profile mode.
	if result.InitialGroup == "" {
		configAdapter := adapters.NewConfigAdapter(cfg)

		client, err := api.NewClient(
			configAdapter,
			api.WithLogger(loggerAdapter),
			api.WithCache(cacheAdapter),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Proxmox API: %w", err)
		}

		return &cliSession{single: client, cfg: cfg}, nil
	}

	// Group mode: build a client per profile, fan out queries.
	manager := api.NewGroupClientManager(result.InitialGroup, loggerAdapter, cacheAdapter)

	profileNames := cfg.GetProfileNamesInGroup(result.InitialGroup)
	if len(profileNames) == 0 {
		return nil, fmt.Errorf("group %q has no member profiles", result.InitialGroup)
	}

	var profiles []api.ProfileEntry

	for _, name := range profileNames {
		p, exists := cfg.Profiles[name]
		if !exists {
			continue
		}

		profileCfg := &config.Config{
			Addr:        p.Addr,
			User:        p.User,
			Password:    p.Password,
			TokenID:     p.TokenID,
			TokenSecret: p.TokenSecret,
			Realm:       p.Realm,
			ApiPath:     p.ApiPath,
			Insecure:    p.Insecure,
			SSHUser:     p.SSHUser,
			VMSSHUser:   p.VMSSHUser,
			CacheDir:    cfg.CacheDir,
			Debug:       cfg.Debug,
		}
		// Normalize URL for each profile entry.
		profileCfg.Addr = strings.TrimRight(profileCfg.Addr, "/") + "/" + strings.TrimPrefix(profileCfg.ApiPath, "/")

		profiles = append(profiles, api.ProfileEntry{
			Name:   name,
			Config: adapters.NewConfigAdapter(profileCfg),
		})
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no valid profiles found in group %q", result.InitialGroup)
	}

	if err := manager.Initialize(context.Background(), profiles); err != nil {
		return nil, fmt.Errorf("failed to connect to group %q: %w", result.InitialGroup, err)
	}

	return &cliSession{group: manager, cfg: cfg}, nil
}

// getOutputFormat reads the --output flag value from the command.
func getOutputFormat(cmd *cobra.Command) string {
	f, _ := cmd.Flags().GetString("output")
	if f == "" {
		return outputJSON
	}

	return f
}

// printJSON marshals v as indented JSON and writes it to stdout.
func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

// printError writes a JSON error object to stderr and returns err unchanged so
// callers can `return printError(err)`.
func printError(err error) error {
	data, _ := json.Marshal(map[string]string{"error": err.Error()})
	fmt.Fprintln(os.Stderr, string(data))

	return err
}

// printTable writes aligned tabular output to stdout.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	_, _ = fmt.Fprintln(w, strings.Join(headers, "\t"))

	seps := make([]string, len(headers))
	for i, h := range headers {
		seps[i] = strings.Repeat("-", len(h))
	}

	_, _ = fmt.Fprintln(w, strings.Join(seps, "\t"))

	for _, row := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	_ = w.Flush()
}

// formatBytes formats a byte count as a human-readable string (e.g. 1.5 GiB).
func formatBytes(b int64) string {
	const unit = 1024

	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0

	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatUptime formats seconds into a human-readable duration string.
func formatUptime(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}

	d := seconds / 86400
	h := (seconds % 86400) / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60

	switch {
	case d > 0:
		return fmt.Sprintf("%dd%dh%dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// findNodeIP returns the IP address of the named node.
func (s *cliSession) findNodeIP(ctx context.Context, nodeName string) (string, error) {
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch nodes: %w", err)
	}

	for _, n := range nodes {
		if n != nil && n.Name == nodeName {
			return n.IP, nil
		}
	}

	return "", fmt.Errorf("node %q not found", nodeName)
}

// resolveSSHCreds returns the SSH username, keyfile path, and jump-host config
// to use for the node that hosts vm, preferring the guest's source profile when available.
func (s *cliSession) resolveSSHCreds(vm *api.VM) (sshUser string, sshKeyfile string, jumpHost config.SSHJumpHost) {
	if vm != nil && vm.SourceProfile != "" {
		if p, ok := s.cfg.Profiles[vm.SourceProfile]; ok {
			if p.SSHUser != "" {
				sshUser = p.SSHUser
			}
			if p.SSHKeyfile != "" {
				sshKeyfile = p.SSHKeyfile
			}
			if p.SSHJumpHost.Addr != "" {
				jumpHost = p.SSHJumpHost
			}
		}
	}

	if sshUser == "" {
		sshUser = s.cfg.SSHUser
	}
	if sshKeyfile == "" {
		sshKeyfile = s.cfg.SSHKeyfile
	}
	if jumpHost.Addr == "" {
		jumpHost = s.cfg.SSHJumpHost
	}

	return sshUser, sshKeyfile, jumpHost
}

// execLXC executes cmdParts inside an LXC container via SSH to its host node
// using pct exec. Returns stdout, stderr, exit code, and any transport error.
func (s *cliSession) execLXC(ctx context.Context, vm *api.VM, cmdParts []string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	nodeIP, err := s.findNodeIP(ctx, vm.Node)
	if err != nil {
		return "", "", 0, fmt.Errorf("cannot resolve node for guest %d: %w", vm.ID, err)
	}

	sshUser, sshKeyfile, jumpHost := s.resolveSSHCreds(vm)

	sshClient := commandrunner.NewSSHClient(commandrunner.SSHClientConfig{
		Username: sshUser,
		KeyPath:  sshKeyfile,
		Timeout:  timeout,
		Port:     22,
		JumpHost: commandrunner.JumpHostConfig{
			Addr:    jumpHost.Addr,
			User:    jumpHost.User,
			KeyPath: jumpHost.Keyfile,
			Port:    jumpHost.Port,
		},
	})

	return sshClient.ExecuteContainerCommandDetailed(ctx, nodeIP, vm.ID, cmdParts)
}

// findNodeByName returns the node with the given name, or an error if not found.
func (s *cliSession) findNodeByName(ctx context.Context, name string) (*api.Node, error) {
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}

	for _, n := range nodes {
		if n != nil && n.Name == name {
			return n, nil
		}
	}

	return nil, fmt.Errorf("node %q not found", name)
}

// resolveNodeSSHCreds returns the SSH username and jump-host config for a node,
// preferring the node's source profile when available.
func (s *cliSession) resolveNodeSSHCreds(node *api.Node) (sshUser string, jumpHost config.SSHJumpHost) {
	if node != nil && node.SourceProfile != "" {
		if p, ok := s.cfg.Profiles[node.SourceProfile]; ok {
			if p.SSHUser != "" {
				sshUser = p.SSHUser
			}
			if p.SSHJumpHost.Addr != "" {
				jumpHost = p.SSHJumpHost
			}
		}
	}

	if sshUser == "" {
		sshUser = s.cfg.SSHUser
	}

	if jumpHost.Addr == "" {
		jumpHost = s.cfg.SSHJumpHost
	}

	return sshUser, jumpHost
}

// resolveNodeSSHKeyfile returns the SSH keyfile to use when connecting to a node.
func (s *cliSession) resolveNodeSSHKeyfile(node *api.Node) string {
	if node != nil && node.SourceProfile != "" {
		if p, ok := s.cfg.Profiles[node.SourceProfile]; ok && p.SSHKeyfile != "" {
			return p.SSHKeyfile
		}
	}

	return s.cfg.SSHKeyfile
}

// resolveVMSSHUser returns the SSH username to use when connecting directly to
// a QEMU VM, falling back to the host SSH user if vm_ssh_user is not set.
func (s *cliSession) resolveVMSSHUser(vm *api.VM) string {
	var vmSSHUser string

	if vm != nil && vm.SourceProfile != "" {
		if p, ok := s.cfg.Profiles[vm.SourceProfile]; ok && p.VMSSHUser != "" {
			vmSSHUser = p.VMSSHUser
		}
	}

	if vmSSHUser == "" {
		vmSSHUser = s.cfg.VMSSHUser
	}

	if vmSSHUser == "" {
		// Fall back to host SSH user.
		hostUser, _, _ := s.resolveSSHCreds(vm)
		vmSSHUser = hostUser
	}

	return vmSSHUser
}

// resolveVMSSHKeyfile returns the SSH keyfile to use when connecting directly
// to a QEMU VM. Prefers vm_ssh_keyfile, falls back to ssh_keyfile.
func (s *cliSession) resolveVMSSHKeyfile(vm *api.VM) string {
	if vm != nil && vm.SourceProfile != "" {
		if p, ok := s.cfg.Profiles[vm.SourceProfile]; ok {
			if p.VMSSHKeyfile != "" {
				return p.VMSSHKeyfile
			}
			if p.SSHKeyfile != "" {
				return p.SSHKeyfile
			}
		}
	}

	if s.cfg.VMSSHKeyfile != "" {
		return s.cfg.VMSSHKeyfile
	}

	return s.cfg.SSHKeyfile
}

// execInteractiveShell runs the ssh binary interactively with the given
// argument list, inheriting stdin/stdout/stderr. Unlike the TUI's ssh helpers
// it does not display a "Press Enter to return to TUI" prompt on exit.
func execInteractiveShell(sshArgs []string, keyfile string) error {
	args := make([]string, 0, len(sshArgs)+2)
	if keyfile != "" {
		args = append(args, "-i", keyfile)
	}

	args = append(args, sshArgs...)

	// #nosec G204 -- args are built from vetted internal call sites.
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	return cmd.Run()
}

// completeNodeNames is a Cobra ValidArgsFunction that returns node names for
// tab completion. It requires no positional args already provided.
func completeNodeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	session, err := initCLISession(cmd)
	if err != nil || session == nil {
		return nil, cobra.ShellCompDirectiveError
	}

	nodes, err := session.getNodes(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string

	for _, n := range nodes {
		if n != nil && strings.HasPrefix(n.Name, toComplete) {
			names = append(names, n.Name)
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeVMIDs is a Cobra ValidArgsFunction that returns VMID completions
// with the guest name and type as description. Requires no positional args
// already provided.
func completeVMIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	session, err := initCLISession(cmd)
	if err != nil || session == nil {
		return nil, cobra.ShellCompDirectiveError
	}

	vms, err := session.getVMs(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string

	for _, vm := range vms {
		if vm == nil {
			continue
		}

		id := fmt.Sprintf("%d", vm.ID)
		if strings.HasPrefix(id, toComplete) {
			completions = append(completions, fmt.Sprintf("%s\t%s (%s)", id, vm.Name, vm.Type))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// findGuestByVMID scans all nodes for a guest with the given VMID.
func findGuestByVMID(client *api.Client, vmid int) (*api.VM, error) {
	cluster, err := client.GetClusterStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster status: %w", err)
	}

	for _, node := range cluster.Nodes {
		if node == nil {
			continue
		}

		for _, vm := range node.VMs {
			if vm != nil && vm.ID == vmid {
				return vm, nil
			}
		}
	}

	return nil, fmt.Errorf("guest %d not found", vmid)
}
