package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/internal/ssh"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// guestOutput is the JSON-serializable view of a VM for CLI consumers.
type guestOutput struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Node          string `json:"node"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	IP            string `json:"ip,omitempty"`
	Template      bool   `json:"template,omitempty"`
	Tags          string `json:"tags,omitempty"`
	SourceProfile string `json:"source_profile,omitempty"`
}

// lifecycleOutput is returned by start/stop/shutdown/restart commands.
type lifecycleOutput struct {
	VMID      int    `json:"vmid"`
	Operation string `json:"operation"`
	UPID      string `json:"upid"`
	Node      string `json:"node"`
}

// execOutput is returned by the exec command.
type execOutput struct {
	VMID       int    `json:"vmid"`
	Command    string `json:"command"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
}

func vmToGuestOutput(vm *api.VM) guestOutput {
	return guestOutput{
		ID:            vm.ID,
		Name:          vm.Name,
		Node:          vm.Node,
		Type:          vm.Type,
		Status:        vm.Status,
		IP:            vm.IP,
		Template:      vm.Template,
		Tags:          vm.Tags,
		SourceProfile: vm.SourceProfile,
	}
}

func templateMarker(t bool) string {
	if t {
		return "[T]"
	}

	return ""
}

// newGuestsCmd returns the `guests` parent command.
func newGuestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guests",
		Short: "Manage and inspect Proxmox guests (VMs and containers)",
	}

	cmd.AddCommand(newGuestsListCmd())
	cmd.AddCommand(newGuestsShowCmd())
	cmd.AddCommand(newGuestsStartCmd())
	cmd.AddCommand(newGuestsStopCmd())
	cmd.AddCommand(newGuestsShutdownCmd())
	cmd.AddCommand(newGuestsRestartCmd())
	cmd.AddCommand(newGuestsExecCmd())
	cmd.AddCommand(newGuestsShellCmd())

	return cmd
}

// ── guests list ──────────────────────────────────────────────────────────────

func newGuestsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all guests (VMs and containers)",
		Long:  "List all VMs and LXC containers across all cluster nodes.",
		Example: `  # JSON output (default)
  pvetui guests list

  # Filter by node and status
  pvetui guests list --node pve01 --status running

  # Only QEMU VMs, table format
  pvetui guests list --type qemu --output table

  # Use a specific profile
  pvetui --profile prod guests list`,
		RunE: runGuestsList,
	}

	cmd.Flags().String("node", "", "Filter by node name")
	cmd.Flags().String("status", "", "Filter by status (running, stopped, paused)")
	cmd.Flags().String("type", "", "Filter by type (qemu, lxc)")

	return cmd
}

func runGuestsList(cmd *cobra.Command, _ []string) error {
	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	nodeFilter, _ := cmd.Flags().GetString("node")
	statusFilter, _ := cmd.Flags().GetString("status")
	typeFilter, _ := cmd.Flags().GetString("type")

	vms, err := session.getVMs(context.Background())
	if err != nil {
		return printError(fmt.Errorf("failed to fetch guests: %w", err))
	}

	var out []guestOutput

	for _, vm := range vms {
		if vm == nil {
			continue
		}

		if nodeFilter != "" && vm.Node != nodeFilter {
			continue
		}

		if statusFilter != "" && vm.Status != statusFilter {
			continue
		}

		if typeFilter != "" && vm.Type != typeFilter {
			continue
		}

		out = append(out, vmToGuestOutput(vm))
	}

	if out == nil {
		out = []guestOutput{}
	}

	if getOutputFormat(cmd) == outputTable {
		headers := []string{"ID", "NAME", "NODE", "TYPE", "STATUS", "IP", "TAGS"}
		rows := make([][]string, 0, len(out))

		for _, g := range out {
			name := g.Name + templateMarker(g.Template)
			rows = append(rows, []string{
				strconv.Itoa(g.ID),
				name,
				g.Node,
				g.Type,
				g.Status,
				g.IP,
				g.Tags,
			})
		}

		printTable(headers, rows)

		return nil
	}

	return printJSON(out)
}

// ── guests show ──────────────────────────────────────────────────────────────

func newGuestsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <vmid>",
		Short: "Show details for a specific guest",
		Long:  "Show detailed information for a guest identified by VMID.",
		Example: `  pvetui guests show 100
  pvetui --profile prod guests show 100`,
		Args:              cobra.ExactArgs(1),
		RunE:              runGuestsShow,
		ValidArgsFunction: completeVMIDs,
	}
}

func runGuestsShow(cmd *cobra.Command, args []string) error {
	vmid, err := parseVMID(args[0])
	if err != nil {
		return printError(err)
	}

	session, initErr := initCLISession(cmd)
	if initErr != nil {
		return printError(initErr)
	}

	if session == nil {
		return nil
	}

	vm, err := session.findVM(context.Background(), vmid)
	if err != nil {
		return printError(err)
	}

	out := vmToGuestOutput(vm)

	if getOutputFormat(cmd) == outputTable {
		printTable(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"ID", strconv.Itoa(out.ID)},
				{"Name", out.Name},
				{"Node", out.Node},
				{"Type", out.Type},
				{"Status", out.Status},
				{"IP", out.IP},
				{"Template", strconv.FormatBool(out.Template)},
				{"Tags", out.Tags},
			},
		)

		return nil
	}

	return printJSON(out)
}

// ── guests lifecycle ─────────────────────────────────────────────────────────

func newGuestsStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <vmid>",
		Short: "Start a guest",
		Long:  "Start a stopped VM or container.",
		Example: `  pvetui guests start 100
  pvetui --profile prod guests start 100`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("start", func(client *api.Client, vm *api.VM) (string, error) {
			return client.StartVM(vm)
		}),
	}
}

func newGuestsStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <vmid>",
		Short: "Force stop a guest (immediate power off)",
		Long: `Force stop a running VM or container.

This is equivalent to pulling the power cord and may cause data loss.
For a graceful shutdown, use 'guests shutdown' instead.`,
		Example:           `  pvetui guests stop 100`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("stop", func(client *api.Client, vm *api.VM) (string, error) {
			return client.StopVM(vm)
		}),
	}
}

func newGuestsShutdownCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "shutdown <vmid>",
		Short:             "Gracefully shut down a guest",
		Long:              "Request a graceful ACPI shutdown of a running VM or container.",
		Example:           `  pvetui guests shutdown 100`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("shutdown", func(client *api.Client, vm *api.VM) (string, error) {
			return client.ShutdownVM(vm)
		}),
	}
}

func newGuestsRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "restart <vmid>",
		Short:             "Restart a guest",
		Long:              "Request a graceful restart of a running VM or container.",
		Example:           `  pvetui guests restart 100`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("restart", func(client *api.Client, vm *api.VM) (string, error) {
			return client.RestartVM(vm)
		}),
	}
}

// makeLifecycleCmd returns a RunE handler for start/stop/shutdown/restart.
func makeLifecycleCmd(
	operation string,
	fn func(*api.Client, *api.VM) (string, error),
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		vmid, err := parseVMID(args[0])
		if err != nil {
			return printError(err)
		}

		session, initErr := initCLISession(cmd)
		if initErr != nil {
			return printError(initErr)
		}

		if session == nil {
			return nil
		}

		ctx := context.Background()

		vm, err := session.findVM(ctx, vmid)
		if err != nil {
			return printError(err)
		}

		if vm.Template {
			return printError(fmt.Errorf("guest %d is a template; lifecycle operations are not supported", vmid))
		}

		client, err := session.clientForVM(vm)
		if err != nil {
			return printError(err)
		}

		upid, err := fn(client, vm)
		if err != nil {
			return printError(fmt.Errorf("failed to %s guest %d: %w", operation, vmid, err))
		}

		out := lifecycleOutput{
			VMID:      vmid,
			Operation: operation,
			UPID:      upid,
			Node:      vm.Node,
		}

		if getOutputFormat(cmd) == outputTable {
			printTable(
				[]string{"FIELD", "VALUE"},
				[][]string{
					{"VMID", strconv.Itoa(out.VMID)},
					{"Operation", out.Operation},
					{"Node", out.Node},
					{"UPID", out.UPID},
				},
			)

			return nil
		}

		return printJSON(out)
	}
}

// ── guests exec ──────────────────────────────────────────────────────────────

func newGuestsExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <vmid> <command>",
		Short: "Execute a command inside a guest via QEMU guest agent",
		Long: `Execute a shell command inside a running guest.

For QEMU VMs the command runs via the QEMU guest agent (no SSH to the guest
required). The VM must have the guest agent enabled and running.
On Linux guests the command runs via /bin/sh -c.
On Windows guests it runs via PowerShell.

For LXC containers the command runs via pct exec over SSH to the Proxmox node
(ssh_user must be configured). The command always runs via /bin/sh -c.

Note: unlike the command-runner plugin, exec imposes no command whitelist.
The caller is responsible for what they run. Security is enforced by the
Proxmox API token permissions and SSH access granted to this client.`,
		Example: `  pvetui guests exec 100 "uptime"
  pvetui guests exec 200 "df -h"
  pvetui --profile prod guests exec 100 "systemctl status nginx"`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeVMIDs,
		RunE:              runGuestsExec,
	}

	cmd.Flags().Duration("timeout", 30*time.Second, "Command execution timeout")

	return cmd
}

func runGuestsExec(cmd *cobra.Command, args []string) error {
	vmid, err := parseVMID(args[0])
	if err != nil {
		return printError(err)
	}

	command := args[1]
	timeout, _ := cmd.Flags().GetDuration("timeout")

	session, initErr := initCLISession(cmd)
	if initErr != nil {
		return printError(initErr)
	}

	if session == nil {
		return nil
	}

	ctx := context.Background()

	vm, err := session.findVM(ctx, vmid)
	if err != nil {
		return printError(err)
	}

	if vm.Type != api.VMTypeQemu && vm.Type != api.VMTypeLXC {
		return printError(fmt.Errorf("guest %d is type %q; exec is only supported for QEMU VMs and LXC containers", vmid, vm.Type))
	}

	if vm.Status != api.VMStatusRunning {
		return printError(fmt.Errorf("guest %d is not running (status: %s)", vmid, vm.Status))
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	var stdout, stderr string
	var exitCode int

	if vm.Type == api.VMTypeLXC {
		stdout, stderr, exitCode, err = session.execLXC(execCtx, vm, []string{"/bin/sh", "-c", command}, timeout)
		if err != nil {
			return printError(fmt.Errorf("exec failed on LXC %d: %w", vmid, err))
		}
	} else {
		if !vm.AgentEnabled || !vm.AgentRunning {
			return printError(fmt.Errorf("guest agent is not available on VM %d (enabled: %v, running: %v)", vmid, vm.AgentEnabled, vm.AgentRunning))
		}

		client, clientErr := session.clientForVM(vm)
		if clientErr != nil {
			return printError(clientErr)
		}

		cmdParts := buildExecCommand(vm.OSType, command)
		stdout, stderr, exitCode, err = client.ExecuteGuestAgentCommand(execCtx, vm, cmdParts, timeout)
		if err != nil {
			return printError(fmt.Errorf("exec failed on VM %d: %w", vmid, err))
		}
	}

	elapsed := time.Since(start)

	out := execOutput{
		VMID:       vmid,
		Command:    command,
		Stdout:     stdout,
		Stderr:     stderr,
		ExitCode:   exitCode,
		DurationMS: elapsed.Milliseconds(),
	}

	if getOutputFormat(cmd) == outputTable {
		printTable(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"VMID", strconv.Itoa(out.VMID)},
				{"Command", out.Command},
				{"Exit Code", strconv.Itoa(out.ExitCode)},
				{"Duration", fmt.Sprintf("%dms", out.DurationMS)},
				{"Stdout", strings.TrimRight(out.Stdout, "\n")},
				{"Stderr", strings.TrimRight(out.Stderr, "\n")},
			},
		)

		return nil
	}

	return printJSON(out)
}

// buildExecCommand wraps a shell command in the appropriate interpreter for
// the guest OS. Mirrors the logic in
// internal/plugins/command-runner/executor.go:buildGuestAgentCommand.
func buildExecCommand(osType, command string) []string {
	if isWindowsOSType(osType) {
		return []string{
			"powershell.exe",
			"-NoProfile",
			"-NonInteractive",
			"-ExecutionPolicy", "Bypass",
			"-Command", command,
		}
	}

	return []string{"/bin/sh", "-c", command}
}

// isWindowsOSType returns true when the Proxmox ostype string indicates a
// Windows guest. Mirrors detectOSFamily in
// internal/plugins/command-runner/os_detection.go.
func isWindowsOSType(osType string) bool {
	return strings.HasPrefix(strings.ToLower(osType), "win")
}

// ── guests shell ─────────────────────────────────────────────────────────────

func newGuestsShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <vmid>",
		Short: "Open an interactive shell inside a guest",
		Long: `Open an interactive shell inside a running guest.

For LXC containers: SSHes to the Proxmox node and runs 'pct enter <vmid>'
(NixOS containers are detected automatically and use 'pct exec' instead).
Requires ssh_user to be configured.

For QEMU VMs: opens a direct SSH connection to the VM's IP address.
Requires the VM to have an IP address visible to pvetui (e.g. via QEMU guest
agent) and vm_ssh_user (or ssh_user as fallback) to be configured.

Authentication follows the standard SSH priority: agent > configured keyfile
> ~/.ssh defaults.`,
		Example: `  pvetui guests shell 100
  pvetui --profile prod guests shell 200
  pvetui --ssh-user root guests shell 100`,
		Args:              cobra.ExactArgs(1),
		RunE:              runGuestsShell,
		ValidArgsFunction: completeVMIDs,
	}
}

func runGuestsShell(cmd *cobra.Command, args []string) error {
	vmid, err := parseVMID(args[0])
	if err != nil {
		return printError(err)
	}

	session, initErr := initCLISession(cmd)
	if initErr != nil {
		return printError(initErr)
	}

	if session == nil {
		return nil
	}

	ctx := context.Background()

	vm, err := session.findVM(ctx, vmid)
	if err != nil {
		return printError(err)
	}

	if vm.Status != api.VMStatusRunning {
		return printError(fmt.Errorf("guest %d is not running (status: %s)", vmid, vm.Status))
	}

	if vm.Template {
		return printError(fmt.Errorf("guest %d is a template; shell is not supported", vmid))
	}

	switch vm.Type {
	case api.VMTypeLXC:
		return runLXCShell(cmd, session, vm)
	case api.VMTypeQemu:
		return runQEMUShell(cmd, session, vm)
	default:
		return printError(fmt.Errorf("unsupported guest type %q for guest %d", vm.Type, vmid))
	}
}

func runLXCShell(_ *cobra.Command, session *cliSession, vm *api.VM) error {
	ctx := context.Background()

	nodeIP, err := session.findNodeIP(ctx, vm.Node)
	if err != nil {
		return printError(fmt.Errorf("cannot resolve node for guest %d: %w", vm.ID, err))
	}

	sshUser, jumpHost := session.resolveNodeSSHCreds(&api.Node{
		Name:          vm.Node,
		SourceProfile: vm.SourceProfile,
	})
	if sshUser == "" {
		return printError(fmt.Errorf("SSH user not configured; set ssh_user in config or use --ssh-user"))
	}

	keyfile := session.resolveNodeSSHKeyfile(&api.Node{
		Name:          vm.Node,
		SourceProfile: vm.SourceProfile,
	})

	isNixOS := vm.OSType == "nixos" || vm.OSType == "nix"

	var pctCmd string
	if isNixOS {
		pctCmd = fmt.Sprintf("pct exec %d -- /bin/sh -c 'if [ -f /etc/set-environment ]; then . /etc/set-environment; fi; exec bash'", vm.ID)
	} else {
		pctCmd = fmt.Sprintf("pct enter %d", vm.ID)
	}

	if !strings.EqualFold(sshUser, "root") {
		pctCmd = "sudo " + pctCmd
	}

	sshArgs := ssh.BuildSSHArgs(sshUser, nodeIP, jumpHost)
	sshArgs = append(sshArgs, "-t", pctCmd)

	containerType := "LXC"
	if isNixOS {
		containerType = "NixOS LXC"
	}

	fmt.Fprintf(os.Stderr, "Connecting to %s container %s (ID: %d) on node %s (%s)...\n",
		containerType, vm.Name, vm.ID, vm.Node, nodeIP)

	if err := execInteractiveShell(sshArgs, keyfile); err != nil {
		return printError(fmt.Errorf("shell session ended with error: %w", err))
	}

	return nil
}

func runQEMUShell(_ *cobra.Command, session *cliSession, vm *api.VM) error {
	if vm.IP == "" {
		return printError(fmt.Errorf(
			"no IP address available for VM %d (%s); ensure QEMU guest agent is running and the VM has network connectivity",
			vm.ID, vm.Name,
		))
	}

	vmSSHUser := session.resolveVMSSHUser(vm)
	if vmSSHUser == "" {
		return printError(fmt.Errorf("VM SSH user not configured; set vm_ssh_user (or ssh_user) in config or use --vm-ssh-user"))
	}

	keyfile := session.resolveVMSSHKeyfile(vm)

	_, jumpHost := session.resolveNodeSSHCreds(&api.Node{
		Name:          vm.Node,
		SourceProfile: vm.SourceProfile,
	})

	sshArgs := ssh.BuildSSHArgs(vmSSHUser, vm.IP, jumpHost)

	fmt.Fprintf(os.Stderr, "Connecting to QEMU VM %s (ID: %d) at %s as %s...\n",
		vm.Name, vm.ID, vm.IP, vmSSHUser)

	if err := execInteractiveShell(sshArgs, keyfile); err != nil {
		return printError(fmt.Errorf("SSH session ended with error: %w", err))
	}

	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func parseVMID(s string) (int, error) {
	vmid, err := strconv.Atoi(s)
	if err != nil || vmid <= 0 {
		return 0, fmt.Errorf("invalid VMID %q: must be a positive integer", s)
	}

	return vmid, nil
}
