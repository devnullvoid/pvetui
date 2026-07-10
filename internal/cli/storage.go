package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// storageListRow is the JSON-serializable view of one (storage, node) pair.
type storageListRow struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Used    int64  `json:"used"`
	Total   int64  `json:"total"`
	Active  bool   `json:"active"`
}

// storageContentRow is the JSON-serializable view of one storage content item.
type storageContentRow struct {
	VolID string `json:"volid"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	CTime int64  `json:"ctime"`
	VMID  int    `json:"vmid"`
}

// storageTaskOutput is the JSON-serializable view of a task-producing storage operation.
type storageTaskOutput struct {
	Node       string `json:"node"`
	Storage    string `json:"storage,omitempty"`
	VolID      string `json:"volid,omitempty"`
	VMID       int    `json:"vmid,omitempty"`
	Type       string `json:"type,omitempty"`
	URL        string `json:"url,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Template   string `json:"template,omitempty"`
	Reference  string `json:"reference,omitempty"`
	UPID       string `json:"upid"`
	Status     string `json:"status"`
	ExitStatus string `json:"exit_status,omitempty"`
}

// ── storage ──────────────────────────────────────────────────────────────────

func newStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Manage Proxmox storage and content",
	}

	cmd.AddCommand(newStorageListCmd())
	cmd.AddCommand(newStorageShowCmd())
	cmd.AddCommand(newStorageContentCmd())
	cmd.AddCommand(newStorageDownloadCmd())
	cmd.AddCommand(newStorageRestoreCmd())

	return cmd
}

// ── storage list ─────────────────────────────────────────────────────────────

func newStorageListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List storages",
		Long: `List storages across the cluster.

Without --node, each (storage, node) pair is its own row — shared storages appear
once per node they are active on, matching the TUI storage browser layout.`,
		Example: `  pvetui storage list
  pvetui storage list --node pve01
  pvetui storage list --output table`,
		RunE: runStorageList,
	}

	cmd.Flags().String("node", "", "Filter to a specific node")

	return cmd
}

func runStorageList(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	nodeName, _ := cmd.Flags().GetString("node")

	var rows []storageListRow

	if nodeName != "" {
		client, err := session.clientForNode(ctx, nodeName)
		if err != nil {
			return printError(err)
		}

		rows, err = fetchStorageRows(client, nodeName)
		if err != nil {
			return printError(err)
		}
	} else {
		nodes, err := session.getNodes(ctx)
		if err != nil {
			return printError(fmt.Errorf("failed to fetch nodes: %w", err))
		}

		for _, node := range nodes {
			if node == nil {
				continue
			}

			client, err := session.clientForNode(ctx, node.Name)
			if err != nil {
				continue
			}

			nodeRows, err := fetchStorageRows(client, node.Name)
			if err != nil {
				continue
			}

			rows = append(rows, nodeRows...)
		}
	}

	format := getOutputFormat(cmd)
	if format == outputTable {
		headers := []string{"NAME", "NODE", "TYPE", "USED/TOTAL", "CONTENT"}
		tableRows := make([][]string, 0, len(rows))
		for _, r := range rows {
			usedTotal := fmt.Sprintf("%s/%s", formatBytes(r.Used), formatBytes(r.Total))
			tableRows = append(tableRows, []string{r.Name, r.Node, r.Type, usedTotal, r.Content})
		}
		printTable(headers, tableRows)
		return nil
	}

	return printJSON(rows)
}

func fetchStorageRows(client *api.Client, nodeName string) ([]storageListRow, error) {
	storages, err := client.GetNodeStorages(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get storages for node %s: %w", nodeName, err)
	}

	rows := make([]storageListRow, 0, len(storages))
	for _, s := range storages {
		if s == nil {
			continue
		}

		rows = append(rows, storageListRow{
			Name:    s.Name,
			Node:    nodeName,
			Type:    s.Plugintype,
			Content: s.Content,
			Used:    s.Disk,
			Total:   s.MaxDisk,
			Active:  true, // GetNodeStorages already filters out inactive storages
		})
	}

	return rows, nil
}

// ── storage show ──────────────────────────────────────────────────────────────

func newStorageShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show <node> <storage>",
		Short:   "Show details for a specific storage",
		Args:    cobra.ExactArgs(2),
		Example: `  pvetui storage show pve01 local-zfs`,
		RunE:    runStorageShow,
	}

	cmd.ValidArgsFunction = completeStorageNames

	return cmd
}

func runStorageShow(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	nodeName := args[0]
	storageName := args[1]

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	storages, err := client.GetNodeStorages(nodeName)
	if err != nil {
		return printError(fmt.Errorf("failed to get storages for node %s: %w", nodeName, err))
	}

	for _, s := range storages {
		if s == nil || s.Name != storageName {
			continue
		}

		row := storageListRow{
			Name:    s.Name,
			Node:    nodeName,
			Type:    s.Plugintype,
			Content: s.Content,
			Used:    s.Disk,
			Total:   s.MaxDisk,
			Active:  true,
		}

		format := getOutputFormat(cmd)
		if format == outputTable {
			printTable([]string{"FIELD", "VALUE"}, [][]string{
				{"name", row.Name},
				{"node", row.Node},
				{"type", row.Type},
				{"content", row.Content},
				{"used", formatBytes(row.Used)},
				{"total", formatBytes(row.Total)},
				{"active", strconv.FormatBool(row.Active)},
			})
			return nil
		}

		return printJSON(row)
	}

	return printError(fmt.Errorf("storage %q not found on node %s", storageName, nodeName))
}

// ── storage content ───────────────────────────────────────────────────────────

func newStorageContentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "content",
		Short: "Manage storage content",
	}

	cmd.AddCommand(newStorageContentListCmd())
	cmd.AddCommand(newStorageContentDeleteCmd())

	return cmd
}

func newStorageContentListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <node> <storage>",
		Short: "List content in a storage",
		Args:  cobra.ExactArgs(2),
		Example: `  pvetui storage content list pve01 local
  pvetui storage content list pve01 local --type iso
  pvetui storage content list pve01 local --output table`,
		RunE: runStorageContentList,
	}

	cmd.Flags().String("type", "", "Filter by content type (iso, vztmpl, backup, snippets, images)")

	cmd.ValidArgsFunction = completeStorageNames

	return cmd
}

func runStorageContentList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	nodeName := args[0]
	storageName := args[1]

	contentType, _ := cmd.Flags().GetString("type")

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	items, err := client.GetStorageContent(nodeName, storageName, contentType)
	if err != nil {
		return printError(fmt.Errorf("failed to list content: %w", err))
	}

	rows := make([]storageContentRow, 0, len(items))
	for _, item := range items {
		name := item.VolID
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}

		rows = append(rows, storageContentRow{
			VolID: item.VolID,
			Name:  name,
			Type:  item.Content,
			Size:  item.Size,
			CTime: item.CreatedAt.Unix(),
			VMID:  item.VMID,
		})
	}

	format := getOutputFormat(cmd)
	if format == outputTable {
		headers := []string{"VOLID", "TYPE", "SIZE", "DATE"}
		tableRows := make([][]string, 0, len(rows))
		for _, r := range rows {
			date := ""
			if r.CTime > 0 {
				date = time.Unix(r.CTime, 0).Format("2006-01-02")
			}
			tableRows = append(tableRows, []string{r.VolID, r.Type, formatBytes(r.Size), date})
		}
		printTable(headers, tableRows)
		return nil
	}

	return printJSON(rows)
}

func newStorageContentDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <node> <storage> <volid>",
		Short: "Delete a storage content item",
		Args:  cobra.ExactArgs(3),
		Example: `  pvetui storage content delete pve01 local local:iso/debian-12.iso
  pvetui storage content delete pve01 local local:iso/debian-12.iso --no-wait`,
		RunE: runStorageContentDelete,
	}

	addNoWaitFlag(cmd)

	cmd.ValidArgsFunction = completeStorageNames

	return cmd
}

func runStorageContentDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	nodeName := args[0]
	storageName := args[1]
	volID := args[2]

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	upid, err := client.DeleteStorageContent(nodeName, storageName, volID)
	if err != nil {
		return printError(fmt.Errorf("failed to delete content: %w", err))
	}

	out := storageTaskOutput{
		Node:    nodeName,
		Storage: storageName,
		VolID:   volID,
		UPID:    upid,
		Status:  "running",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, nodeName, upid, "delete content")
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

// ── storage download ──────────────────────────────────────────────────────────

func newStorageDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download content into storage",
	}

	cmd.AddCommand(newStorageDownloadURLCmd())
	cmd.AddCommand(newStorageDownloadTemplateCmd())
	cmd.AddCommand(newStorageDownloadOCICmd())

	return cmd
}

func newStorageDownloadURLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "url <node> <storage> <url>",
		Short: "Download content from a URL",
		Args:  cobra.ExactArgs(3),
		Example: `  pvetui storage download url pve01 local https://example.com/debian-12.iso
  pvetui storage download url pve01 local https://example.com/debian-12.iso --filename debian-12.iso
  pvetui storage download url pve01 local https://example.com/debian-12.iso --no-wait`,
		RunE: runStorageDownloadURL,
	}

	cmd.Flags().String("filename", "", "Override the destination filename")
	cmd.Flags().String("content-type", "", "Content type override (iso, vztmpl, import); inferred from URL extension when omitted")
	addNoWaitFlag(cmd)

	cmd.ValidArgsFunction = completeStorageNames

	return cmd
}

func runStorageDownloadURL(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	nodeName := args[0]
	storageName := args[1]
	rawURL := args[2]

	filename, _ := cmd.Flags().GetString("filename")
	contentType, _ := cmd.Flags().GetString("content-type")

	if contentType == "" {
		contentType = inferContentTypeFromURL(rawURL)
	}

	if contentType == "" {
		return printError(fmt.Errorf("cannot infer content type from URL %q — use --content-type iso|vztmpl|import", rawURL))
	}

	if filename == "" {
		filename = rawURL
		if idx := strings.LastIndex(filename, "/"); idx >= 0 {
			filename = filename[idx+1:]
		}
		if idx := strings.Index(filename, "?"); idx >= 0 {
			filename = filename[:idx]
		}
	}

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	upid, err := client.DownloadStorageContentFromURL(nodeName, storageName, api.StorageDownloadURLOptions{
		URL:      rawURL,
		Content:  contentType,
		Filename: filename,
	})
	if err != nil {
		return printError(fmt.Errorf("failed to start download: %w", err))
	}

	out := storageTaskOutput{
		Node:     nodeName,
		Storage:  storageName,
		URL:      rawURL,
		Filename: filename,
		UPID:     upid,
		Status:   "running",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, nodeName, upid, "download url")
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

func newStorageDownloadTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template <node> <storage> <template>",
		Short: "Download an appliance template",
		Long: `Download an appliance template from the Proxmox catalog.

<template> may be a full filename (e.g. debian-12-standard_12.7-1_amd64.tar.zst) or
a package name (e.g. debian-12-standard). When a package name is given, the latest
version is resolved automatically and the resolved filename is printed to stderr.`,
		Args: cobra.ExactArgs(3),
		Example: `  pvetui storage download template pve01 local debian-12-standard
  pvetui storage download template pve01 local debian-12-standard --section system
  pvetui storage download template pve01 local debian-12-standard_12.7-1_amd64.tar.zst`,
		RunE: runStorageDownloadTemplate,
	}

	cmd.Flags().String("section", "", "Narrow template resolution by section (system, mail, turnkeylinux)")
	addNoWaitFlag(cmd)

	cmd.ValidArgsFunction = completeTemplateNames

	return cmd
}

func runStorageDownloadTemplate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	nodeName := args[0]
	storageName := args[1]
	templateArg := args[2]

	section, _ := cmd.Flags().GetString("section")

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	resolvedFilename := templateArg
	if !strings.Contains(templateArg, ".") {
		resolvedFilename, err = resolveTemplateName(client, nodeName, templateArg, section)
		if err != nil {
			return printError(err)
		}
	}

	upid, err := client.DownloadApplianceTemplate(nodeName, storageName, resolvedFilename)
	if err != nil {
		return printError(fmt.Errorf("failed to start template download: %w", err))
	}

	out := storageTaskOutput{
		Node:     nodeName,
		Storage:  storageName,
		Template: resolvedFilename,
		UPID:     upid,
		Status:   "running",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, nodeName, upid, "download template")
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

func newStorageDownloadOCICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oci <node> <storage> <reference>",
		Short: "Pull an OCI image into storage",
		Args:  cobra.ExactArgs(3),
		Example: `  pvetui storage download oci pve01 local registry.example.com/myimage:latest
  pvetui storage download oci pve01 local registry.example.com/myimage:latest --no-wait`,
		RunE: runStorageDownloadOCI,
	}

	addNoWaitFlag(cmd)

	cmd.ValidArgsFunction = completeStorageNames

	return cmd
}

func runStorageDownloadOCI(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	nodeName := args[0]
	storageName := args[1]
	reference := args[2]

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	upid, err := client.PullStorageOCIImage(nodeName, storageName, api.StorageOCIPullOptions{
		Reference: reference,
	})
	if err != nil {
		return printError(fmt.Errorf("failed to pull OCI image: %w", err))
	}

	out := storageTaskOutput{
		Node:      nodeName,
		Storage:   storageName,
		Reference: reference,
		UPID:      upid,
		Status:    "running",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, nodeName, upid, "pull OCI image")
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

// ── storage restore ───────────────────────────────────────────────────────────

func newStorageRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <node> <storage> <volid> <vmid>",
		Short: "Restore a guest from a backup",
		Long: `Restore a guest from a vzdump backup volume.

WARNING: This overwrites the target VMID's configuration and disk(s).

Without --confirm, prints what would be restored and exits without making changes.
The guest type is inferred from the volid prefix (vzdump-qemu-* or vzdump-lxc-*);
use --type to override when inference fails.`,
		Args: cobra.ExactArgs(4),
		Example: `  # Dry-run preview (no changes made)
  pvetui storage restore pve01 local local:backup/vzdump-qemu-100-2024.tar.zst 100

  # Actually restore
  pvetui storage restore pve01 local local:backup/vzdump-qemu-100-2024.tar.zst 100 --confirm
  pvetui storage restore pve01 local local:backup/vzdump-lxc-101-2024.tar.zst 101 --confirm --type lxc`,
		RunE: runStorageRestore,
	}

	cmd.Flags().Bool("confirm", false, "Actually perform the restore (required; without it a dry-run summary is printed)")
	cmd.Flags().String("type", "", "Guest type override: qemu or lxc (inferred from volid when omitted)")
	addNoWaitFlag(cmd)

	cmd.ValidArgsFunction = completeStorageNames

	return cmd
}

func runStorageRestore(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	nodeName := args[0]
	storageName := args[1]
	volID := args[2]

	vmidStr := args[3]
	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		return printError(fmt.Errorf("invalid vmid %q: must be an integer", vmidStr))
	}

	guestType, _ := cmd.Flags().GetString("type")
	if guestType == "" {
		guestType, err = inferGuestTypeFromVolID(volID)
		if err != nil {
			return printError(err)
		}
	}

	if guestType != "qemu" && guestType != "lxc" {
		return printError(fmt.Errorf("invalid guest type %q: must be qemu or lxc", guestType))
	}

	confirmed, _ := cmd.Flags().GetBool("confirm")
	if !confirmed {
		fmt.Printf("Dry-run: would restore %s (type: %s) to VMID %d on node %s (storage: %s)\n",
			volID, guestType, vmid, nodeName, storageName)
		fmt.Println("Pass --confirm to actually perform the restore.")
		return nil
	}

	session, err := initCLISession(cmd)
	if err != nil {
		return printError(err)
	}

	if session == nil {
		return nil
	}

	client, err := session.clientForNode(ctx, nodeName)
	if err != nil {
		return printError(err)
	}

	upid, err := client.RestoreGuestFromBackup(nodeName, guestType, vmid, volID, false)
	if err != nil {
		return printError(fmt.Errorf("failed to start restore: %w", err))
	}

	out := storageTaskOutput{
		Node:    nodeName,
		Storage: storageName,
		VolID:   volID,
		VMID:    vmid,
		Type:    guestType,
		UPID:    upid,
		Status:  "running",
	}

	if getNoWait(cmd) {
		return printJSON(out)
	}

	exitStatus, waitErr := waitForTask(ctx, client, nodeName, upid, "restore guest")
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

// ── shell completions ─────────────────────────────────────────────────────────

// completeStorageNames is a ValidArgsFunction for <storage> positional args.
// It requires the <node> arg to already be present (index 0).
func completeStorageNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) < 1 {
		return completeNodeNames(cmd, args, toComplete)
	}

	nodeName := args[0]

	session, err := initCLISession(cmd)
	if err != nil || session == nil {
		return nil, cobra.ShellCompDirectiveError
	}

	client, err := session.clientForNode(context.Background(), nodeName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	storages, err := client.GetNodeStorages(nodeName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, s := range storages {
		if s != nil && strings.HasPrefix(s.Name, toComplete) {
			names = append(names, s.Name)
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeTemplateNames is a ValidArgsFunction for <template> in storage download template.
// It requires node (args[0]) and storage (args[1]) to already be present.
func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) < 1 {
		return completeNodeNames(cmd, args, toComplete)
	}

	if len(args) < 2 {
		return completeStorageNames(cmd, args, toComplete)
	}

	nodeName := args[0]

	session, err := initCLISession(cmd)
	if err != nil || session == nil {
		return nil, cobra.ShellCompDirectiveError
	}

	client, err := session.clientForNode(context.Background(), nodeName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	templates, err := client.GetAvailableTemplates(nodeName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	seen := make(map[string]bool)
	var names []string

	for _, t := range templates {
		if strings.HasPrefix(t.Filename, toComplete) && !seen[t.Filename] {
			seen[t.Filename] = true
			names = append(names, t.Filename)
		}
	}

	sort.Strings(names)

	return names, cobra.ShellCompDirectiveNoFileComp
}
