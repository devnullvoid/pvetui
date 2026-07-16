package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
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
	cmd.AddCommand(newGuestsDeleteCmd())
	cmd.AddCommand(newGuestsExecCmd())
	cmd.AddCommand(newGuestsShellCmd())
	cmd.AddCommand(newGuestsCreateCmd())
	cmd.AddCommand(newGuestsMigrateCmd())

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
  pvetui guests list --node pve01 --node-local --status running

  # Only QEMU VMs, table format
  pvetui guests list --type qemu --output table

  # Use a specific profile
  pvetui --profile prod guests list`,
		RunE: runGuestsList,
	}

	cmd.Flags().String("node", "", "Filter by node name")
	cmd.Flags().String("status", "", "Filter by status (running, stopped, paused)")
	cmd.Flags().String("type", "", "Filter by type (qemu, lxc)")
	cmd.Flags().Bool("node-local", false, "When --node is set, query that node directly instead of scanning cluster-wide inventory")

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
	nodeLocal, _ := cmd.Flags().GetBool("node-local")

	ctx := context.Background()
	var vms []*api.VM
	if nodeLocal {
		if nodeFilter == "" {
			return printError(fmt.Errorf("--node-local requires --node"))
		}

		client, clientErr := session.clientForNode(ctx, nodeFilter)
		if clientErr != nil {
			return printError(clientErr)
		}

		vms, err = client.ListNodeGuests(nodeFilter)
	} else {
		vms, err = session.getVMs(ctx)
	}
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
	cmd := &cobra.Command{
		Use:   "start <vmid>",
		Short: "Start a guest",
		Long:  "Start a stopped VM or container.",
		Example: `  pvetui guests start 100
  pvetui --profile prod guests start 100
  pvetui --profile prod guests start 100 --node pve1 --type qemu`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("start", func(client *api.Client, vm *api.VM) (string, error) {
			return client.StartVM(vm)
		}),
	}

	addDirectGuestTargetFlags(cmd)

	return cmd
}

func newGuestsStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <vmid>",
		Short: "Force stop a guest (immediate power off)",
		Long: `Force stop a running VM or container.

This is equivalent to pulling the power cord and may cause data loss.
For a graceful shutdown, use 'guests shutdown' instead.`,
		Example: `  pvetui guests stop 100
  pvetui --profile prod guests stop 100 --node pve1 --type qemu`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("stop", func(client *api.Client, vm *api.VM) (string, error) {
			return client.StopVM(vm)
		}),
	}

	addDirectGuestTargetFlags(cmd)

	return cmd
}

func newGuestsShutdownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown <vmid>",
		Short: "Gracefully shut down a guest",
		Long:  "Request a graceful ACPI shutdown of a running VM or container.",
		Example: `  pvetui guests shutdown 100
  pvetui --profile prod guests shutdown 100 --node pve1 --type lxc`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("shutdown", func(client *api.Client, vm *api.VM) (string, error) {
			return client.ShutdownVM(vm)
		}),
	}

	addDirectGuestTargetFlags(cmd)

	return cmd
}

func newGuestsRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart <vmid>",
		Short: "Restart a guest",
		Long:  "Request a graceful restart of a running VM or container.",
		Example: `  pvetui guests restart 100
  pvetui --profile prod guests restart 100 --node pve1 --type qemu`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE: makeLifecycleCmd("restart", func(client *api.Client, vm *api.VM) (string, error) {
			return client.RestartVM(vm)
		}),
	}

	addDirectGuestTargetFlags(cmd)

	return cmd
}

func newGuestsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <vmid>",
		Short: "Permanently delete a guest",
		Long: `Permanently delete a VM or container and all its associated disks.

WARNING: This operation is irreversible. The guest must be stopped before deletion
unless --force is passed.`,
		Example: `  pvetui guests delete 108
  pvetui guests delete 108 --purge
  pvetui guests delete 108 --force --no-wait`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeVMIDs,
		RunE:              runGuestsDelete,
	}

	cmd.Flags().Bool("purge", false, "Remove VMID from backup and replication jobs")
	cmd.Flags().Bool("force", false, "Force deletion even if the guest is running")
	addNoWaitFlag(cmd)

	return cmd
}

func runGuestsDelete(cmd *cobra.Command, args []string) error {
	vmid, err := parseVMID(args[0])
	if err != nil {
		return printError(err)
	}

	purge, _ := cmd.Flags().GetBool("purge")
	force, _ := cmd.Flags().GetBool("force")

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	ctx := context.Background()

	vm, err := session.findVM(ctx, vmid)
	if err != nil {
		return printError(err)
	}

	client, err := session.clientForVM(vm)
	if err != nil {
		return printError(err)
	}

	upid, err := client.DeleteVMWithOptions(vm, &api.DeleteVMOptions{
		Purge: purge,
		Force: force,
	})
	if err != nil {
		return printError(fmt.Errorf("failed to delete guest %d: %w", vmid, err))
	}

	out := struct {
		VMID       int    `json:"vmid"`
		Node       string `json:"node"`
		UPID       string `json:"upid"`
		Status     string `json:"status"`
		ExitStatus string `json:"exit_status,omitempty"`
	}{
		VMID:   vmid,
		Node:   vm.Node,
		UPID:   upid,
		Status: "running",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, vm.Node, upid, "delete guest")
	out.Status = "complete"
	out.ExitStatus = exitStatus

	if err := printJSON(out); err != nil {
		return err
	}

	if waitErr != nil {
		return waitErr
	}

	return nil
}

func addDirectGuestTargetFlags(cmd *cobra.Command) {
	cmd.Flags().String("node", "", "Target node hosting the guest; skips cluster-wide guest discovery when set")
	cmd.Flags().String("type", string(api.VMTypeQemu), "Guest type for direct targeting: qemu or lxc")
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

		var (
			client *api.Client
			vm     *api.VM
		)

		nodeName, _ := cmd.Flags().GetString("node")
		if nodeName != "" {
			guestType, _ := cmd.Flags().GetString("type")
			if guestType != string(api.VMTypeQemu) && guestType != string(api.VMTypeLXC) {
				return printError(fmt.Errorf("invalid guest type %q; expected qemu or lxc", guestType))
			}

			vm = &api.VM{
				ID:   vmid,
				Node: nodeName,
				Type: guestType,
			}

			var err error
			client, err = session.clientForNode(ctx, nodeName)
			if err != nil {
				return printError(err)
			}
		} else {
			var err error
			vm, err = session.findVM(ctx, vmid)
			if err != nil {
				return printError(err)
			}

			if vm.Template {
				return printError(fmt.Errorf("guest %d is a template; lifecycle operations are not supported", vmid))
			}

			client, err = session.clientForVM(vm)
			if err != nil {
				return printError(err)
			}
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

// ── guests create ────────────────────────────────────────────────────────────

func newGuestsCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new guest (VM or LXC container)",
	}
	cmd.AddCommand(newGuestsCreateVMCmd())
	cmd.AddCommand(newGuestsCreateLXCCmd())
	return cmd
}

type guestCreateOutput struct {
	VMID       int    `json:"vmid"`
	Name       string `json:"name"`
	Node       string `json:"node"`
	Type       string `json:"type"`
	UPID       string `json:"upid"`
	Status     string `json:"status"`
	ExitStatus string `json:"exit_status,omitempty"`
}

func newGuestsCreateVMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vm",
		Short: "Create a new QEMU VM",
		Long:  "Create a new QEMU VM on a Proxmox node.",
		Example: `  # Minimal VM with a new disk
  pvetui guests create vm --node pve --name myvm --disk-storage local-zfs --disk-size 32

  # Import an existing volume
  pvetui guests create vm --node pve --name myvm --disk-storage local-zfs --import-from local:import/disk.img

  # With ISO, custom resources, start after create
  pvetui guests create vm --node pve --name myvm --disk-storage local-zfs --disk-size 32 \
    --iso local:iso/debian-12.iso --memory 4096 --cores 4 --start

  # Return UPID immediately without waiting
  pvetui guests create vm --node pve --name myvm --disk-storage local-zfs --disk-size 32 --no-wait`,
		RunE: runGuestsCreateVM,
	}

	cmd.Flags().String("node", "", "Target node name (required)")
	cmd.Flags().String("name", "", "VM name (required)")
	cmd.Flags().Int("vmid", 0, "VM ID (auto-assigned if omitted)")
	cmd.Flags().Int("memory", 2048, "Memory in MB")
	cmd.Flags().Int("cores", 2, "Number of CPU cores")
	cmd.Flags().Int("sockets", 1, "Number of CPU sockets")
	cmd.Flags().String("disk-storage", "", "Storage name for the primary disk (required)")
	cmd.Flags().Int("disk-size", 0, "Primary disk size in GB (required unless --import-from)")
	cmd.Flags().String("import-from", "", "Volume to import as primary disk (replaces --disk-size)")
	cmd.Flags().String("iso", "", "ISO volume for CD-ROM (e.g. local:iso/debian-12.iso)")
	cmd.Flags().String("bridge", "vmbr0", "Network bridge")
	cmd.Flags().Bool("start", false, "Start VM after creation")
	addNoWaitFlag(cmd)

	_ = cmd.MarkFlagRequired("node")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("disk-storage")

	cmd.ValidArgsFunction = cobra.NoFileCompletions

	return cmd
}

func runGuestsCreateVM(cmd *cobra.Command, _ []string) error {
	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}
	if session == nil {
		return nil
	}

	nodeName, _ := cmd.Flags().GetString("node")
	name, _ := cmd.Flags().GetString("name")
	vmid, _ := cmd.Flags().GetInt("vmid")
	memoryMB, _ := cmd.Flags().GetInt("memory")
	cores, _ := cmd.Flags().GetInt("cores")
	sockets, _ := cmd.Flags().GetInt("sockets")
	diskStorage, _ := cmd.Flags().GetString("disk-storage")
	diskSize, _ := cmd.Flags().GetInt("disk-size")
	importFrom, _ := cmd.Flags().GetString("import-from")
	iso, _ := cmd.Flags().GetString("iso")
	bridge, _ := cmd.Flags().GetString("bridge")
	start, _ := cmd.Flags().GetBool("start")

	if importFrom == "" && diskSize <= 0 {
		return printError(fmt.Errorf("--disk-size is required unless --import-from is specified"))
	}

	ctx := context.Background()

	node, err := session.findNodeByName(ctx, nodeName)
	if err != nil {
		return printError(err)
	}
	if !node.Online {
		return printError(fmt.Errorf("node %q is offline", nodeName))
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	if vmid == 0 {
		vmid, err = client.GetNextID(0)
		if err != nil {
			return printError(fmt.Errorf("failed to get next VMID: %w", err))
		}
	}

	options := api.VMCreateOptions{
		VMID:        vmid,
		Name:        name,
		MemoryMB:    memoryMB,
		Cores:       cores,
		Sockets:     sockets,
		DiskStorage: diskStorage,
		DiskSizeGB:  diskSize,
		ImportFrom:  importFrom,
		ISOVolume:   iso,
		Bridge:      bridge,
		Start:       start,
	}

	upid, err := client.CreateVM(nodeName, options)
	if err != nil {
		return printError(fmt.Errorf("failed to create VM: %w", err))
	}

	out := guestCreateOutput{
		VMID:   vmid,
		Name:   name,
		Node:   nodeName,
		Type:   "qemu",
		UPID:   upid,
		Status: "queued",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, nodeName, upid, "create VM")
	out.Status = "completed"
	out.ExitStatus = exitStatus
	if waitErr != nil {
		out.Status = "failed"
		_ = printJSON(out)
		return printError(waitErr)
	}

	return printJSON(out)
}

func newGuestsCreateLXCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lxc",
		Short: "Create a new LXC container",
		Long:  "Create a new LXC container on a Proxmox node.",
		Example: `  # Basic container
  pvetui guests create lxc --node pve --hostname myct --rootfs-storage local-zfs \
    --template local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst

  # Using package name (auto-resolves to latest version)
  pvetui guests create lxc --node pve --hostname myct --rootfs-storage local-zfs \
    --template debian-12-standard

  # Custom resources, privileged, start after create
  pvetui guests create lxc --node pve --hostname myct --rootfs-storage local-zfs \
    --template debian-12-standard --memory 2048 --cores 2 --unprivileged=false --start`,
		RunE: runGuestsCreateLXC,
	}

	cmd.Flags().String("node", "", "Target node name (required)")
	cmd.Flags().String("hostname", "", "Container hostname (required)")
	cmd.Flags().Int("vmid", 0, "Container ID (auto-assigned if omitted)")
	cmd.Flags().Int("memory", 512, "Memory in MB")
	cmd.Flags().Int("swap", 512, "Swap in MB (0 to disable)")
	cmd.Flags().Int("cores", 1, "Number of CPU cores")
	cmd.Flags().String("rootfs-storage", "", "Storage for root filesystem (required)")
	cmd.Flags().Int("rootfs-size", 8, "Root filesystem size in GB")
	cmd.Flags().String("template", "", "OS template volid or package name (required)")
	cmd.Flags().String("bridge", "vmbr0", "Network bridge")
	cmd.Flags().Bool("unprivileged", true, "Run as unprivileged container")
	cmd.Flags().Bool("nesting", true, "Enable nesting (allows Docker inside container)")
	cmd.Flags().Bool("start", false, "Start container after creation")
	addNoWaitFlag(cmd)

	_ = cmd.MarkFlagRequired("node")
	_ = cmd.MarkFlagRequired("hostname")
	_ = cmd.MarkFlagRequired("rootfs-storage")
	_ = cmd.MarkFlagRequired("template")

	cmd.ValidArgsFunction = cobra.NoFileCompletions

	return cmd
}

func runGuestsCreateLXC(cmd *cobra.Command, _ []string) error {
	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}
	if session == nil {
		return nil
	}

	nodeName, _ := cmd.Flags().GetString("node")
	hostname, _ := cmd.Flags().GetString("hostname")
	vmid, _ := cmd.Flags().GetInt("vmid")
	memoryMB, _ := cmd.Flags().GetInt("memory")
	swapMB, _ := cmd.Flags().GetInt("swap")
	cores, _ := cmd.Flags().GetInt("cores")
	rootfsStorage, _ := cmd.Flags().GetString("rootfs-storage")
	rootfsSize, _ := cmd.Flags().GetInt("rootfs-size")
	templateArg, _ := cmd.Flags().GetString("template")
	bridge, _ := cmd.Flags().GetString("bridge")
	unprivileged, _ := cmd.Flags().GetBool("unprivileged")
	nesting, _ := cmd.Flags().GetBool("nesting")
	start, _ := cmd.Flags().GetBool("start")

	if swapMB < 0 {
		return printError(fmt.Errorf("--swap must be >= 0"))
	}

	ctx := context.Background()

	node, err := session.findNodeByName(ctx, nodeName)
	if err != nil {
		return printError(err)
	}
	if !node.Online {
		return printError(fmt.Errorf("node %q is offline", nodeName))
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	// Resolve template to a full storage volid.
	// Accepted forms (in priority order):
	//   "local:vztmpl/debian-13-standard_13.1-2_amd64.tar.zst"  → full volid, use as-is
	//   "debian-13-standard_13.1-2_amd64.tar.zst"               → filename, prepend storage:vztmpl/
	//   "debian-13-standard"                                     → package name, search storage content
	template := templateArg
	switch {
	case strings.Contains(templateArg, ":"):
		// Already a full volid — use as-is.
	case strings.Contains(templateArg, "."):
		// Bare filename — prepend the rootfs storage prefix.
		template = rootfsStorage + ":vztmpl/" + templateArg
	default:
		// Package name — search vztmpl content already on the node's storages.
		template, err = resolveLXCTemplateFromStorage(client, nodeName, templateArg)
		if err != nil {
			return printError(err)
		}
	}

	if vmid == 0 {
		vmid, err = client.GetNextID(0)
		if err != nil {
			return printError(fmt.Errorf("failed to get next VMID: %w", err))
		}
	}

	options := api.LXCCreateOptions{
		VMID:          vmid,
		Hostname:      hostname,
		MemoryMB:      memoryMB,
		SwapMB:        swapMB,
		Cores:         cores,
		RootFSStorage: rootfsStorage,
		RootFSSizeGB:  rootfsSize,
		OSTemplate:    template,
		Bridge:        bridge,
		Unprivileged:  unprivileged,
		Nesting:       nesting,
		Start:         start,
	}

	upid, err := client.CreateLXC(nodeName, options)
	if err != nil {
		return printError(fmt.Errorf("failed to create LXC: %w", err))
	}

	out := guestCreateOutput{
		VMID:   vmid,
		Name:   hostname,
		Node:   nodeName,
		Type:   "lxc",
		UPID:   upid,
		Status: "queued",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, nodeName, upid, "create LXC")
	out.Status = "completed"
	out.ExitStatus = exitStatus
	if waitErr != nil {
		out.Status = "failed"
		_ = printJSON(out)
		return printError(waitErr)
	}

	return printJSON(out)
}

// resolveLXCTemplateFromStorage resolves a package name to a storage volid by
// scanning the vztmpl content already downloaded on the node. It searches all
// storages on the node for a file whose name starts with "{packageName}_" or
// equals "{packageName}.*", then returns the alphabetically last (latest) volid.
func resolveLXCTemplateFromStorage(client *api.Client, nodeName, packageName string) (string, error) {
	storages, err := client.GetNodeStorages(nodeName)
	if err != nil {
		return "", fmt.Errorf("failed to list storages: %w", err)
	}

	var matches []string

	for _, s := range storages {
		if s == nil || !strings.Contains(s.Content, "vztmpl") {
			continue
		}

		items, err := client.GetStorageContent(nodeName, s.Name, "vztmpl")
		if err != nil {
			continue
		}

		for _, item := range items {
			// Extract filename after the last '/'.
			filename := item.VolID
			if idx := strings.LastIndex(filename, "/"); idx >= 0 {
				filename = filename[idx+1:]
			}

			if strings.HasPrefix(filename, packageName+"_") || strings.HasPrefix(filename, packageName+".") {
				matches = append(matches, item.VolID)
			}
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no downloaded template found matching %q — download it first with `pvetui storage download template`", packageName)
	}

	// Pick the alphabetically last volid (latest version).
	sort.Strings(matches)
	resolved := matches[len(matches)-1]
	fmt.Fprintf(os.Stderr, "resolved template %q → %q\n", packageName, resolved)

	return resolved, nil
}

// resolveTemplateName resolves a package name (e.g. "debian-12-standard") to a
// full template filename by querying aplinfo. If section is non-empty, only
// templates in that section are considered. Returns the alphabetically last
// (latest) match.
func resolveTemplateName(client *api.Client, nodeName, packageName, section string) (string, error) {
	templates, err := client.GetAvailableTemplates(nodeName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch template list: %w", err)
	}

	var matches []string
	for _, t := range templates {
		if t.Package != packageName {
			continue
		}
		if section != "" && !strings.EqualFold(t.Section, section) {
			continue
		}
		matches = append(matches, t.Filename)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no template found matching package name %q", packageName)
	}

	// Sort and pick the last (latest) version.
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j] > matches[i] {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
	resolved := matches[0]
	fmt.Fprintf(os.Stderr, "resolved template %q → %q\n", packageName, resolved)
	return resolved, nil
}

// ── guests migrate ───────────────────────────────────────────────────────────

type guestMigrateOutput struct {
	VMID       int    `json:"vmid"`
	Name       string `json:"name"`
	SourceNode string `json:"source_node"`
	TargetNode string `json:"target_node"`
	Mode       string `json:"mode"`
	UPID       string `json:"upid"`
	Status     string `json:"status"`
	ExitStatus string `json:"exit_status,omitempty"`
}

func newGuestsMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate <vmid> <target-node>",
		Short: "Migrate a guest to another node",
		Long: `Migrate a VM or LXC container to another node in the same cluster.

Migration mode is selected automatically:
  - QEMU running  → online migration
  - QEMU stopped  → offline migration
  - LXC           → restart migration`,
		Example: `  # Migrate guest 100 to pve02 (auto mode)
  pvetui guests migrate 100 pve02

  # Force offline migration for a running QEMU VM
  pvetui guests migrate 100 pve02 --offline

  # Offline migrate local disks to storage available on the target node
  pvetui guests migrate 100 pve02 --offline --target-storage shared-ssd
  pvetui guests migrate 200 pve02 --target-storage shared-ssd

  # Return UPID immediately
  pvetui guests migrate 100 pve02 --no-wait

  # Wait longer for large disk copies
  pvetui guests migrate 100 pve02 --offline --target-storage shared-ssd --wait-timeout 2h`,
		Args:              cobra.ExactArgs(2),
		RunE:              runGuestsMigrate,
		ValidArgsFunction: completeVMIDs,
	}

	cmd.Flags().Bool("online", false, "Force online migration (QEMU only)")
	cmd.Flags().Bool("offline", false, "Force offline migration (QEMU only)")
	cmd.Flags().String("target-storage", "", "Target storage for migrated disks or LXC rootfs")
	cmd.Flags().Duration("wait-timeout", 10*time.Minute, "Maximum time to wait for migration task completion")
	addNoWaitFlag(cmd)

	return cmd
}

func runGuestsMigrate(cmd *cobra.Command, args []string) error {
	vmid, err := parseVMID(args[0])
	if err != nil {
		return printError(err)
	}
	targetNode := args[1]

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}
	if session == nil {
		return nil
	}

	ctx := context.Background()

	vm, err := session.findVM(ctx, vmid)
	if err != nil {
		return printError(err)
	}

	// Validate target node exists and is online.
	target, err := session.findNodeByName(ctx, targetNode)
	if err != nil {
		return printError(err)
	}
	if !target.Online {
		return printError(fmt.Errorf("target node %q is offline", targetNode))
	}
	if target.Name == vm.Node {
		return printError(fmt.Errorf("guest %d is already on node %q", vmid, targetNode))
	}

	forceOnline, _ := cmd.Flags().GetBool("online")
	forceOffline, _ := cmd.Flags().GetBool("offline")
	targetStorage, _ := cmd.Flags().GetString("target-storage")
	waitTimeout, _ := cmd.Flags().GetDuration("wait-timeout")

	if vm.Type == api.VMTypeLXC && (forceOnline || forceOffline) {
		return printError(fmt.Errorf("--online/--offline are not applicable to LXC containers"))
	}
	if forceOnline && forceOffline {
		return printError(fmt.Errorf("--online and --offline are mutually exclusive"))
	}
	if waitTimeout <= 0 {
		return printError(fmt.Errorf("--wait-timeout must be greater than zero"))
	}

	// Build migration options with auto mode selection.
	options := &api.MigrationOptions{Target: targetNode}
	mode := "offline"
	switch vm.Type {
	case api.VMTypeLXC:
		mode = "restart"
	case api.VMTypeQemu:
		online := vm.Status == api.VMStatusRunning
		if forceOnline {
			online = true
		}
		if forceOffline {
			online = false
		}
		options.Online = &online
		if online {
			mode = "online"
		}
	}
	if targetStorage != "" {
		if options.Online != nil && *options.Online {
			return printError(fmt.Errorf("--target-storage requires an offline migration; pass --offline"))
		}
		options.TargetStorage = targetStorage
	}

	client, err := session.clientForVM(vm)
	if err != nil {
		return printError(err)
	}

	upid, err := client.MigrateVM(vm, options)
	if err != nil {
		return printError(fmt.Errorf("migration failed: %w", err))
	}

	out := guestMigrateOutput{
		VMID:       vm.ID,
		Name:       vm.Name,
		SourceNode: vm.Node,
		TargetNode: targetNode,
		Mode:       mode,
		UPID:       upid,
		Status:     "queued",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTaskWithTimeout(ctx, client, vm.Node, upid, "migrate", waitTimeout)
	out.Status = "completed"
	out.ExitStatus = exitStatus
	if waitErr != nil {
		out.Status = "failed"
		_ = printJSON(out)
		return printError(waitErr)
	}

	return printJSON(out)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func parseVMID(s string) (int, error) {
	vmid, err := strconv.Atoi(s)
	if err != nil || vmid <= 0 {
		return 0, fmt.Errorf("invalid VMID %q: must be a positive integer", s)
	}

	return vmid, nil
}
