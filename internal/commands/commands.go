package commands

import (
	"fmt"
	"strconv"

	"github.com/devnullvoid/proxmox-tui/internal/ssh"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// TODO: implement commands for listing nodes, managing VMs/LXCs, and opening shells.

// ListNodes retrieves and processes cluster nodes.
func ListNodes(client *api.Client) ([]api.Node, error) {
	if client == nil {
		return nil, fmt.Errorf("nil api client")
	}

	return client.ListNodes()
}

// StartVM starts a VM or LXC by ID.
func StartVM(client *api.Client, id string) error {
	if client == nil {
		return fmt.Errorf("nil api client")
	}

	vmID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid id %s: %w", id, err)
	}

	if client.Cluster == nil {
		if _, err := client.GetClusterStatus(); err != nil {
			return err
		}
	}

	for _, node := range client.Cluster.Nodes {
		if node == nil {
			continue
		}

		for _, vm := range node.VMs {
			if vm != nil && vm.ID == vmID {
				return client.StartVM(vm)
			}
		}
	}

	return fmt.Errorf("vm %d not found", vmID)
}

// ShellNode opens an SSH shell to the given node.
func ShellNode(sshClient *ssh.SSHClient) error {
	if sshClient == nil {
		return fmt.Errorf("nil ssh client")
	}

	return sshClient.Shell()
}
