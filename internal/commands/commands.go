package commands

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ssh"
)

// TODO: implement commands for listing nodes, managing VMs/LXCs, and opening shells.

// ListNodes retrieves and processes cluster nodes.
func ListNodes(client *api.Client) error {
	// TODO: call client.ListNodes() and update UI tree
	return nil
}

// StartVM starts a VM or LXC by ID.
func StartVM(client *api.Client, id string) error {
	// TODO: call client.StartVM(id)
	return nil
}

// ShellNode opens an SSH shell to the given node.
func ShellNode(sshClient *ssh.SSHClient) error {
	// TODO: implement interactive shell
	return nil
}
