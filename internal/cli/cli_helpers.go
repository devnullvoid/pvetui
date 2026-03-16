package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/internal/bootstrap"
	"github.com/devnullvoid/pvetui/internal/cache"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// Output format constants used across all subcommand files.
const (
	outputJSON  = "json"
	outputTable = "table"
)

// initAPIClient bootstraps the application configuration and returns an
// authenticated API client. It returns (nil, nil, nil) when bootstrap handled
// an early-exit flag (--version, --list-profiles) and the caller should return
// nil without doing further work.
func initAPIClient(cmd *cobra.Command) (*api.Client, *bootstrap.BootstrapResult, error) {
	opts := getBootstrapOptions(cmd)

	result, err := bootstrap.Bootstrap(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("bootstrap failed: %w", err)
	}

	if result == nil {
		// Early-exit flag was handled (e.g. --version, --list-profiles).
		return nil, nil, nil
	}

	cfg := result.Config

	// Normalize the API URL the same way the TUI does.
	cfg.Addr = strings.TrimRight(cfg.Addr, "/") + "/" + strings.TrimPrefix(cfg.ApiPath, "/")

	// Initialize global cache unless disabled.
	if !result.NoCache {
		if cacheErr := cache.InitGlobalCache(cfg.CacheDir); cacheErr != nil {
			// Non-fatal — continue without persistent cache.
			_ = cacheErr
		}
	}

	configAdapter := adapters.NewConfigAdapter(cfg)
	loggerAdapter := adapters.NewLoggerAdapter(cfg)
	cacheAdapter := adapters.NewCacheAdapter()

	client, err := api.NewClient(
		configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(cacheAdapter),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Proxmox API: %w", err)
	}

	return client, result, nil
}

// getOutputFormat reads the --output flag value from the command. Returns
// "json" if the flag is absent or empty.
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

// printTable writes aligned tabular output to stdout. headers is the first row;
// rows are the data rows. Each cell is tab-separated and padded by tabwriter.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	_, _ = fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Separator line under headers
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

// findGuestByVMID scans all nodes in the cluster for a guest with the given
// VMID and returns it. Returns an error if no matching guest is found.
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
