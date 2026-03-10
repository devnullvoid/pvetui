package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api"
)

type storageSelection struct {
	Node    *api.Node
	Storage *api.Storage
}

type StorageBrowser struct {
	*tview.Flex

	tree         *tview.TreeView
	details      *tview.Table
	contentTable *tview.Table
	app          *App
	nodes        []*api.Node
	selection    storageSelection
}

var _ StorageBrowserComponent = (*StorageBrowser)(nil)

func NewStorageBrowser() *StorageBrowser {
	tree := tview.NewTreeView()
	tree.SetBorder(true)
	tree.SetTitle(" Storage ")
	tree.SetGraphics(true)

	details := tview.NewTable()
	details.SetBorders(false)
	details.SetBorder(true)
	details.SetTitle(" Storage Details ")

	contentTable := tview.NewTable()
	contentTable.SetBorder(true)
	contentTable.SetTitle(" Storage Content ")
	contentTable.SetSelectable(true, false)

	right := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(details, 11, 0, false).
		AddItem(contentTable, 0, 1, false)

	return &StorageBrowser{
		Flex: tview.NewFlex().
			AddItem(tree, 0, 1, true).
			AddItem(right, 0, 2, false),
		tree:         tree,
		details:      details,
		contentTable: contentTable,
	}
}

func (sb *StorageBrowser) SetApp(app *App) {
	sb.app = app

	treeNav := createNavigationInputCapture(app, nil, sb.contentTable)
	var pendingG bool
	sb.tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if handleVimTopBottomRune(event, &pendingG, func() {
			sb.jumpTreeTop()
		}, func() {
			sb.jumpTreeBottom()
		}) {
			return nil
		}

		return treeNav(event)
	})

	contentNav := createNavigationInputCapture(app, sb.tree, nil)
	pendingG = false
	sb.contentTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if handleVimTopBottomRune(event, &pendingG, func() {
			jumpTableTop(sb.contentTable)
		}, func() {
			jumpTableBottom(sb.contentTable)
		}) {
			return nil
		}
		return contentNav(event)
	})
}

func (sb *StorageBrowser) SetNodes(nodes []*api.Node) {
	sb.nodes = cloneAndSortStorageNodes(nodes)
	selectedKey := selectionKey(sb.selection.Node, sb.selection.Storage)

	root := tview.NewTreeNode("root").SetSelectable(false)
	for _, node := range sb.nodes {
		nodeData := &storageSelection{Node: node}
		nodeLabel := node.Name
		if node.SourceProfile != "" {
			nodeLabel += fmt.Sprintf(" [secondary](%s)[-]", node.SourceProfile)
		}

		nodeNode := tview.NewTreeNode(theme.ReplaceSemanticTags(nodeLabel)).
			SetReference(nodeData).
			SetSelectable(true)
		nodeNode.SetExpanded(true)

		storages := make([]*api.Storage, 0, len(node.Storage))
		storages = append(storages, node.Storage...)
		sort.Slice(storages, func(i, j int) bool {
			return storages[i].Name < storages[j].Name
		})

		for _, storage := range storages {
			if storage == nil {
				continue
			}

			label := storage.Name
			if storage.IsShared() {
				label += " [warning](shared)[-]"
			}
			if strings.TrimSpace(storage.Status) != "" && !strings.EqualFold(storage.Status, "active") {
				label += fmt.Sprintf(" [secondary](%s)[-]", storage.Status)
			}

			storageNode := tview.NewTreeNode(theme.ReplaceSemanticTags(label)).
				SetReference(&storageSelection{Node: node, Storage: storage}).
				SetSelectable(true)
			nodeNode.AddChild(storageNode)
		}

		root.AddChild(nodeNode)
	}

	sb.tree.SetRoot(root)
	sb.tree.SetCurrentNode(sb.restoreSelection(root, selectedKey))
	sb.tree.SetChangedFunc(func(node *tview.TreeNode) {
		sb.handleTreeSelection(node)
	})

	if current := sb.tree.GetCurrentNode(); current != nil {
		sb.handleTreeSelection(current)
	} else {
		sb.showNodeSummary(nil)
		sb.showContentMessage("Select a storage")
	}
}

func (sb *StorageBrowser) SelectNode(node *api.Node) {
	if node == nil || sb.tree.GetRoot() == nil {
		return
	}

	target := sb.findNode(sb.tree.GetRoot(), selectionKey(node, nil))
	if target == nil {
		return
	}

	sb.tree.SetCurrentNode(target)
	sb.handleTreeSelection(target)
	if sb.app != nil {
		sb.app.pages.SwitchToPage(api.PageStorage)
		sb.app.SetFocus(sb.tree)
	}
}

func (sb *StorageBrowser) handleTreeSelection(node *tview.TreeNode) {
	ref, _ := node.GetReference().(*storageSelection)
	if ref == nil {
		sb.selection = storageSelection{}
		sb.showNodeSummary(nil)
		sb.showContentMessage("Select a storage")
		return
	}

	sb.selection = *ref
	if ref.Storage == nil {
		sb.showNodeSummary(ref.Node)
		sb.showContentMessage("Select a storage under the node to view content")
		return
	}

	sb.showStorageSummary(ref.Node, ref.Storage, nil)
	sb.showContentMessage("Loading storage content...")
	sb.loadStorageContent(*ref)
}

func (sb *StorageBrowser) loadStorageContent(selection storageSelection) {
	if sb.app == nil || selection.Node == nil || selection.Storage == nil {
		return
	}

	go func() {
		client, err := sb.app.getClientForNode(selection.Node)
		if err != nil {
			sb.app.QueueUpdateDraw(func() {
				if selectionKey(sb.selection.Node, sb.selection.Storage) != selectionKey(selection.Node, selection.Storage) {
					return
				}
				sb.showStorageSummary(selection.Node, selection.Storage, err)
				sb.showContentMessage(fmt.Sprintf("Failed to load content: %v", err))
			})
			return
		}

		content, err := client.GetStorageContent(selection.Node.Name, selection.Storage.Name, "")
		sb.app.QueueUpdateDraw(func() {
			if selectionKey(sb.selection.Node, sb.selection.Storage) != selectionKey(selection.Node, selection.Storage) {
				return
			}
			sb.showStorageSummary(selection.Node, selection.Storage, err)
			if err != nil {
				sb.showContentMessage(fmt.Sprintf("Failed to load content: %v", err))
				return
			}
			sb.showStorageContent(content)
		})
	}()
}

func (sb *StorageBrowser) showNodeSummary(node *api.Node) {
	sb.details.Clear()
	sb.details.SetTitle(" Storage Details ")
	if node == nil {
		sb.details.SetCell(0, 0, tview.NewTableCell("Select a node or storage").SetTextColor(theme.Colors.Primary))
		return
	}

	sb.setDetailsRow(0, "Node", node.Name)
	status := "Offline"
	if node.Online {
		status = "Online"
	}
	sb.setDetailsRow(1, "Status", status)
	sb.setDetailsRow(2, "Storages", fmt.Sprintf("%d", len(node.Storage)))

	shared := 0
	contentTypes := make(map[string]struct{})
	for _, storage := range node.Storage {
		if storage == nil {
			continue
		}
		if storage.IsShared() {
			shared++
		}
		for _, content := range strings.Split(storage.Content, ",") {
			content = strings.TrimSpace(content)
			if content != "" {
				contentTypes[content] = struct{}{}
			}
		}
	}
	sb.setDetailsRow(3, "Shared Storages", fmt.Sprintf("%d", shared))
	sb.setDetailsRow(4, "Content Types", strings.Join(sortedKeys(contentTypes), ", "))
}

func (sb *StorageBrowser) showStorageSummary(node *api.Node, storage *api.Storage, loadErr error) {
	sb.details.Clear()
	sb.details.SetTitle(" Storage Details ")
	if node == nil || storage == nil {
		sb.details.SetCell(0, 0, tview.NewTableCell("Select a storage").SetTextColor(theme.Colors.Primary))
		return
	}

	sharedStatus := "Local"
	if storage.IsShared() {
		sharedStatus = "Shared"
	}

	sb.setDetailsRow(0, "Node", node.Name)
	sb.setDetailsRow(1, "Storage", storage.Name)
	sb.setDetailsRow(2, "Type", storage.Plugintype)
	sb.setDetailsRow(3, "Scope", sharedStatus)
	sb.setDetailsRow(4, "Status", storage.Status)
	sb.setDetailsRow(5, "Content", storage.Content)
	sb.setDetailsRow(6, "Usage", fmt.Sprintf("%s / %s (%.1f%%)", utils.FormatBytes(storage.Disk), utils.FormatBytes(storage.MaxDisk), storage.GetUsagePercent()))
	if loadErr != nil {
		sb.setDetailsRow(7, "Content Load", loadErr.Error())
	}
}

func (sb *StorageBrowser) setDetailsRow(row int, label, value string) {
	sb.details.SetCell(row, 0, tview.NewTableCell(label).SetTextColor(theme.Colors.HeaderText))
	sb.details.SetCell(row, 1, tview.NewTableCell(value).SetTextColor(theme.Colors.Primary))
}

func (sb *StorageBrowser) showContentMessage(message string) {
	sb.contentTable.Clear()
	sb.contentTable.SetCell(0, 0, tview.NewTableCell(message).SetTextColor(theme.Colors.Primary))
}

func (sb *StorageBrowser) showStorageContent(items []api.StorageContentItem) {
	sb.contentTable.Clear()
	headers := []string{"Type", "Volume", "Size", "Created", "VMID", "Format"}
	for col, header := range headers {
		sb.contentTable.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(theme.Colors.HeaderText).
			SetSelectable(false))
	}

	if len(items) == 0 {
		sb.contentTable.SetCell(1, 0, tview.NewTableCell("No content found").SetTextColor(theme.Colors.Primary))
		return
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Content == items[j].Content {
			return items[i].VolID < items[j].VolID
		}
		return items[i].Content < items[j].Content
	})

	for i, item := range items {
		row := i + 1
		created := "-"
		if !item.CreatedAt.IsZero() {
			created = item.CreatedAt.Local().Format("2006-01-02 15:04")
		}

		vmid := "-"
		if item.VMID > 0 {
			vmid = fmt.Sprintf("%d", item.VMID)
		}

		sb.contentTable.SetCell(row, 0, tview.NewTableCell(item.Content).SetTextColor(theme.Colors.Primary))
		sb.contentTable.SetCell(row, 1, tview.NewTableCell(item.VolID).SetTextColor(theme.Colors.Primary))
		sb.contentTable.SetCell(row, 2, tview.NewTableCell(utils.FormatBytes(item.Size)).SetTextColor(theme.Colors.Primary))
		sb.contentTable.SetCell(row, 3, tview.NewTableCell(created).SetTextColor(theme.Colors.Primary))
		sb.contentTable.SetCell(row, 4, tview.NewTableCell(vmid).SetTextColor(theme.Colors.Primary))
		sb.contentTable.SetCell(row, 5, tview.NewTableCell(item.Format).SetTextColor(theme.Colors.Primary))
	}

	sb.contentTable.Select(1, 0)
}

func (sb *StorageBrowser) restoreSelection(root *tview.TreeNode, key string) *tview.TreeNode {
	if key != "" {
		if node := sb.findNode(root, key); node != nil {
			return node
		}
	}

	children := root.GetChildren()
	if len(children) == 0 {
		return root
	}
	if len(children[0].GetChildren()) > 0 {
		return children[0].GetChildren()[0]
	}
	return children[0]
}

func (sb *StorageBrowser) findNode(root *tview.TreeNode, key string) *tview.TreeNode {
	if root == nil || key == "" {
		return nil
	}
	var found *tview.TreeNode
	root.Walk(func(node, parent *tview.TreeNode) bool {
		ref, _ := node.GetReference().(*storageSelection)
		if ref != nil && selectionKey(ref.Node, ref.Storage) == key {
			found = node
			return false
		}
		return true
	})
	return found
}

func (sb *StorageBrowser) jumpTreeTop() {
	if root := sb.tree.GetRoot(); root != nil {
		children := root.GetChildren()
		if len(children) > 0 {
			sb.tree.SetCurrentNode(children[0])
			sb.handleTreeSelection(children[0])
		}
	}
}

func (sb *StorageBrowser) jumpTreeBottom() {
	root := sb.tree.GetRoot()
	if root == nil {
		return
	}
	children := root.GetChildren()
	if len(children) == 0 {
		return
	}
	last := children[len(children)-1]
	grandChildren := last.GetChildren()
	if len(grandChildren) > 0 {
		last = grandChildren[len(grandChildren)-1]
	}
	sb.tree.SetCurrentNode(last)
	sb.handleTreeSelection(last)
}

func cloneAndSortStorageNodes(nodes []*api.Node) []*api.Node {
	cloned := make([]*api.Node, 0, len(nodes))
	cloned = append(cloned, nodes...)
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].Name == cloned[j].Name {
			return cloned[i].SourceProfile < cloned[j].SourceProfile
		}
		return cloned[i].Name < cloned[j].Name
	})
	return cloned
}

func selectionKey(node *api.Node, storage *api.Storage) string {
	if node == nil {
		return ""
	}
	key := node.Name + "|" + node.SourceProfile
	if storage != nil {
		key += "|" + storage.Name
	}
	return key
}

func sortedKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
