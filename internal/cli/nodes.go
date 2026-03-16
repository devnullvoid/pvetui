package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// nodeOutput is the JSON-serializable view of a Node. Node.Online is tagged
// json:"-" in the API type, so we project the fields we want explicitly.
type nodeOutput struct {
	Name          string  `json:"name"`
	IP            string  `json:"ip"`
	Online        bool    `json:"online"`
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsed    float64 `json:"memory_used"`
	MemoryTotal   float64 `json:"memory_total"`
	Uptime        int64   `json:"uptime"`
	Version       string  `json:"version,omitempty"`
	KernelVersion string  `json:"kernel_version,omitempty"`
	SourceProfile string  `json:"source_profile,omitempty"`
}

func nodeToOutput(n *api.Node) nodeOutput {
	return nodeOutput{
		Name:          n.Name,
		IP:            n.IP,
		Online:        n.Online,
		CPUUsage:      n.CPUUsage,
		MemoryUsed:    n.MemoryUsed,
		MemoryTotal:   n.MemoryTotal,
		Uptime:        n.Uptime,
		Version:       n.Version,
		KernelVersion: n.KernelVersion,
		SourceProfile: n.SourceProfile,
	}
}

func onlineStr(online bool) string {
	if online {
		return "online"
	}

	return "offline"
}

// newNodesCmd returns the `nodes` parent command.
func newNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage and inspect Proxmox nodes",
	}

	cmd.AddCommand(newNodesListCmd())
	cmd.AddCommand(newNodesShowCmd())

	return cmd
}

func newNodesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all cluster nodes",
		Long:  "List all Proxmox nodes in the cluster with their status and resource usage.",
		Example: `  # JSON output (default)
  pvetui nodes list

  # Human-readable table
  pvetui nodes list --output table

  # Use a specific profile
  pvetui --profile prod nodes list`,
		RunE: runNodesList,
	}
}

func runNodesList(cmd *cobra.Command, _ []string) error {
	client, _, err := initAPIClient(cmd)
	if err != nil {
		return printError(err)
	}

	if client == nil {
		return nil
	}

	cluster, err := client.GetClusterStatus()
	if err != nil {
		return printError(fmt.Errorf("failed to fetch nodes: %w", err))
	}

	out := make([]nodeOutput, 0, len(cluster.Nodes))
	for _, n := range cluster.Nodes {
		if n != nil {
			out = append(out, nodeToOutput(n))
		}
	}

	if getOutputFormat(cmd) == outputTable {
		headers := []string{"NAME", "IP", "STATUS", "CPU%", "MEM USED", "MEM TOTAL", "UPTIME"}
		rows := make([][]string, 0, len(out))

		for _, n := range out {
			rows = append(rows, []string{
				n.Name,
				n.IP,
				onlineStr(n.Online),
				fmt.Sprintf("%.1f%%", n.CPUUsage*100),
				formatBytes(int64(n.MemoryUsed)),
				formatBytes(int64(n.MemoryTotal)),
				formatUptime(n.Uptime),
			})
		}

		printTable(headers, rows)

		return nil
	}

	return printJSON(out)
}

func newNodesShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <node>",
		Short: "Show details for a specific node",
		Long:  "Show detailed information for a named Proxmox node.",
		Example: `  pvetui nodes show pve01
  pvetui --profile prod nodes show pve01`,
		Args: cobra.ExactArgs(1),
		RunE: runNodesShow,
	}
}

func runNodesShow(cmd *cobra.Command, args []string) error {
	client, _, err := initAPIClient(cmd)
	if err != nil {
		return printError(err)
	}

	if client == nil {
		return nil
	}

	nodeName := args[0]

	cluster, err := client.GetClusterStatus()
	if err != nil {
		return printError(fmt.Errorf("failed to fetch nodes: %w", err))
	}

	for _, n := range cluster.Nodes {
		if n != nil && n.Name == nodeName {
			out := nodeToOutput(n)

			if getOutputFormat(cmd) == outputTable {
				printTable(
					[]string{"FIELD", "VALUE"},
					[][]string{
						{"Name", out.Name},
						{"IP", out.IP},
						{"Status", onlineStr(out.Online)},
						{"CPU Usage", fmt.Sprintf("%.1f%%", out.CPUUsage*100)},
						{"Memory Used", formatBytes(int64(out.MemoryUsed))},
						{"Memory Total", formatBytes(int64(out.MemoryTotal))},
						{"Uptime", formatUptime(out.Uptime)},
						{"Version", out.Version},
						{"Kernel", out.KernelVersion},
					},
				)

				return nil
			}

			return printJSON(out)
		}
	}

	return printError(fmt.Errorf("node %q not found", nodeName))
}
