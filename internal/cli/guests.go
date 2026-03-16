package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

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
	client, _, err := initAPIClient(cmd)
	if err != nil {
		return printError(err)
	}

	if client == nil {
		return nil
	}

	nodeFilter, _ := cmd.Flags().GetString("node")
	statusFilter, _ := cmd.Flags().GetString("status")
	typeFilter, _ := cmd.Flags().GetString("type")

	cluster, err := client.GetClusterStatus()
	if err != nil {
		return printError(fmt.Errorf("failed to fetch guests: %w", err))
	}

	var out []guestOutput

	for _, n := range cluster.Nodes {
		if n == nil {
			continue
		}

		if nodeFilter != "" && n.Name != nodeFilter {
			continue
		}

		for _, vm := range n.VMs {
			if vm == nil {
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
		Args: cobra.ExactArgs(1),
		RunE: runGuestsShow,
	}
}

func runGuestsShow(cmd *cobra.Command, args []string) error {
	vmid, err := parseVMID(args[0])
	if err != nil {
		return printError(err)
	}

	client, _, initErr := initAPIClient(cmd)
	if initErr != nil {
		return printError(initErr)
	}

	if client == nil {
		return nil
	}

	vm, err := findGuestByVMID(client, vmid)
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
		Args: cobra.ExactArgs(1),
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
		Example: `  pvetui guests stop 100`,
		Args:    cobra.ExactArgs(1),
		RunE: makeLifecycleCmd("stop", func(client *api.Client, vm *api.VM) (string, error) {
			return client.StopVM(vm)
		}),
	}
}

func newGuestsShutdownCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "shutdown <vmid>",
		Short:   "Gracefully shut down a guest",
		Long:    "Request a graceful ACPI shutdown of a running VM or container.",
		Example: `  pvetui guests shutdown 100`,
		Args:    cobra.ExactArgs(1),
		RunE: makeLifecycleCmd("shutdown", func(client *api.Client, vm *api.VM) (string, error) {
			return client.ShutdownVM(vm)
		}),
	}
}

func newGuestsRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "restart <vmid>",
		Short:   "Restart a guest",
		Long:    "Request a graceful restart of a running VM or container.",
		Example: `  pvetui guests restart 100`,
		Args:    cobra.ExactArgs(1),
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

		client, _, initErr := initAPIClient(cmd)
		if initErr != nil {
			return printError(initErr)
		}

		if client == nil {
			return nil
		}

		vm, err := findGuestByVMID(client, vmid)
		if err != nil {
			return printError(err)
		}

		if vm.Template {
			return printError(fmt.Errorf("guest %d is a template; lifecycle operations are not supported", vmid))
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
		Long: `Execute a shell command inside a running QEMU VM via the guest agent.

The guest must be running and have the QEMU guest agent enabled and responding.
This command does NOT require SSH access to the guest.

On Linux guests the command runs via /bin/sh -c.
On Windows guests it runs via PowerShell.

Note: unlike the command-runner plugin, exec imposes no command whitelist.
The caller is responsible for what they run. Security is enforced by the
Proxmox API token permissions granted to this client.`,
		Example: `  pvetui guests exec 100 "uptime"
  pvetui guests exec 100 "df -h"
  pvetui --profile prod guests exec 100 "systemctl status nginx"`,
		Args: cobra.ExactArgs(2),
		RunE: runGuestsExec,
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

	client, _, initErr := initAPIClient(cmd)
	if initErr != nil {
		return printError(initErr)
	}

	if client == nil {
		return nil
	}

	vm, err := findGuestByVMID(client, vmid)
	if err != nil {
		return printError(err)
	}

	if vm.Type != api.VMTypeQemu {
		return printError(fmt.Errorf("guest %d is type %q; exec is only supported for QEMU VMs via guest agent", vmid, vm.Type))
	}

	if vm.Status != api.VMStatusRunning {
		return printError(fmt.Errorf("guest %d is not running (status: %s)", vmid, vm.Status))
	}

	if !vm.AgentEnabled || !vm.AgentRunning {
		return printError(fmt.Errorf("guest agent is not available on VM %d (enabled: %v, running: %v)", vmid, vm.AgentEnabled, vm.AgentRunning))
	}

	cmdParts := buildExecCommand(vm.OSType, command)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()

	stdout, stderr, exitCode, err := client.ExecuteGuestAgentCommand(ctx, vm, cmdParts, timeout)
	elapsed := time.Since(start)

	if err != nil {
		return printError(fmt.Errorf("exec failed on VM %d: %w", vmid, err))
	}

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

// ── helpers ──────────────────────────────────────────────────────────────────

func parseVMID(s string) (int, error) {
	vmid, err := strconv.Atoi(s)
	if err != nil || vmid <= 0 {
		return 0, fmt.Errorf("invalid VMID %q: must be a positive integer", s)
	}

	return vmid, nil
}
