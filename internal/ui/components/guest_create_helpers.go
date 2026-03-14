package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devnullvoid/pvetui/internal/taskmanager"
	"github.com/devnullvoid/pvetui/pkg/api"
)

func initialCreateNodeIndex(initialNode *api.Node, nodes []*api.Node) int {
	if initialNode == nil {
		return 0
	}

	for i, node := range nodes {
		if node != nil && node.Name == initialNode.Name && node.SourceProfile == initialNode.SourceProfile {
			return i
		}
	}

	return 0
}

func loadGuestCreateData[T any](
	a *App,
	loadingMessage string,
	node *api.Node,
	load func(*api.Node) (T, error),
	onLoaded func(T),
	onErrorMessage string,
) {
	a.header.ShowLoading(loadingMessage)
	go func() {
		data, err := load(node)
		a.QueueUpdateDraw(func() {
			a.header.ShowActiveProfile(a.header.GetCurrentProfile())
			if err != nil {
				a.showMessageSafe(fmt.Sprintf("%s: %v", onErrorMessage, err))
				return
			}
			onLoaded(data)
		})
	}()
}

func (a *App) enqueueGuestCreateTask(
	node *api.Node,
	description string,
	targetName string,
	vmid int,
	successMessage string,
	failurePrefix string,
	operation func(*api.Node) (string, error),
) {
	if node == nil {
		a.showMessageSafe("Select a node first")
		return
	}

	task := &taskmanager.Task{
		Type:        "Create",
		Description: description,
		TargetVMID:  vmid,
		TargetNode:  node.Name,
		TargetName:  targetName,
		Operation: func() (string, error) {
			return operation(node)
		},
		OnComplete: func(err error) {
			if err != nil {
				a.QueueUpdateDraw(func() {
					message := fmt.Sprintf("%s: %v", failurePrefix, err)
					a.header.ShowError(message)
					a.showMessageSafe(message)
				})
				return
			}

			a.ClearAPICache()
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(successMessage)
			})
			go a.manualRefresh()
		},
	}

	a.taskManager.Enqueue(task)
	a.header.ShowSuccess(fmt.Sprintf("Queued %s", description))
}

func (a *App) loadCreateNodeStorages(node *api.Node) (int, []*api.Storage, error) {
	if node == nil {
		return 0, nil, fmt.Errorf("node is required")
	}

	client, err := a.getClientForNode(node)
	if err != nil {
		return 0, nil, err
	}

	nextID, err := client.GetNextID(0)
	if err != nil {
		return 0, nil, err
	}

	storages, err := client.GetNodeStorages(node.Name)
	if err != nil {
		return 0, nil, err
	}

	return nextID, storages, nil
}

func (a *App) collectNodeStorageContent(
	node *api.Node,
	storages []*api.Storage,
	supports func(*api.Storage) bool,
	contentType string,
) ([]string, map[string][]string, error) {
	client, err := a.getClientForNode(node)
	if err != nil {
		return nil, nil, err
	}

	names := make([]string, 0)
	itemsByStorage := make(map[string][]string)

	for _, storage := range storages {
		if storage == nil || !supports(storage) {
			continue
		}

		names = append(names, storage.Name)
		items, listErr := client.GetStorageContent(node.Name, storage.Name, contentType)
		if listErr != nil {
			return nil, nil, listErr
		}
		for _, item := range items {
			itemsByStorage[storage.Name] = append(itemsByStorage[storage.Name], item.VolID)
		}
		sort.Strings(itemsByStorage[storage.Name])
	}

	sort.Strings(names)
	return names, itemsByStorage, nil
}

func storageSupportsToken(content string) func(*api.Storage) bool {
	return func(storage *api.Storage) bool {
		return storage != nil && strings.Contains(storage.Content, content)
	}
}
