package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// taskOutput is the JSON-serializable view of a ClusterTask.
type taskOutput struct {
	UPID          string `json:"upid"`
	Node          string `json:"node"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	User          string `json:"user"`
	StartTime     int64  `json:"starttime"`
	EndTime       int64  `json:"endtime,omitempty"`
	SourceProfile string `json:"source_profile,omitempty"`
}

// newTasksCmd returns the `tasks` parent command.
func newTasksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Inspect Proxmox cluster tasks",
	}

	cmd.AddCommand(newTasksListCmd())

	return cmd
}

func newTasksListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent cluster tasks",
		Long:  "List recent Proxmox cluster tasks across all nodes.",
		Example: `  # JSON output (default, last 20 tasks)
  pvetui tasks list

  # Show last 50 tasks in table format
  pvetui tasks list --recent 50 --output table

  # Use a specific profile
  pvetui --profile prod tasks list`,
		RunE: runTasksList,
	}

	cmd.Flags().Int("recent", 20, "Limit output to the N most recent tasks")

	return cmd
}

func runTasksList(cmd *cobra.Command, _ []string) error {
	client, _, err := initAPIClient(cmd)
	if err != nil {
		return printError(err)
	}

	if client == nil {
		return nil
	}

	recent, _ := cmd.Flags().GetInt("recent")

	tasks, err := client.GetClusterTasks()
	if err != nil {
		return printError(fmt.Errorf("failed to fetch tasks: %w", err))
	}

	// Apply limit.
	if recent > 0 && len(tasks) > recent {
		tasks = tasks[:recent]
	}

	out := make([]taskOutput, 0, len(tasks))
	for _, t := range tasks {
		if t == nil {
			continue
		}

		out = append(out, taskOutput{
			UPID:          t.UPID,
			Node:          t.Node,
			Type:          t.Type,
			Status:        t.Status,
			User:          t.User,
			StartTime:     t.StartTime,
			EndTime:       t.EndTime,
			SourceProfile: t.SourceProfile,
		})
	}

	if getOutputFormat(cmd) == outputTable {
		headers := []string{"NODE", "TYPE", "STATUS", "USER", "STARTED", "UPID"}
		rows := make([][]string, 0, len(out))

		for _, t := range out {
			rows = append(rows, []string{
				t.Node,
				t.Type,
				t.Status,
				t.User,
				formatUnixTime(t.StartTime),
				t.UPID,
			})
		}

		printTable(headers, rows)

		return nil
	}

	return printJSON(out)
}

func formatUnixTime(ts int64) string {
	if ts == 0 {
		return ""
	}

	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}
