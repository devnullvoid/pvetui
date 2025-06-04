package ssh

import (
	"fmt"
	"os"
	"os/exec"
)

// SSHClient wraps an SSH connection context
// TODO: implement methods to connect and execute commands
type SSHClient struct {
	// client field removed as it's unused
}

// NewSSHClient establishes an SSH connection to host.
func NewSSHClient(host, user, password string) (*SSHClient, error) {
	// TODO: implement actual SSH dialing
	return &SSHClient{}, nil
}

// ExecuteNodeShell opens an interactive SSH session to a node
func ExecuteNodeShell(user, nodeIP string) error {
	sshCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", user, nodeIP))
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Execute command using the current process environment and stdin/stdout
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute SSH command: %w", err)
	}

	return nil
}

// ExecuteLXCShell opens an interactive SSH session to a node and then
// uses 'pct exec' to enter the container
func ExecuteLXCShell(user, nodeIP string, vmID int) error {
	sshArgs := []string{
		fmt.Sprintf("%s@%s", user, nodeIP),
		"-t", // Force pseudo-terminal allocation
		fmt.Sprintf("sudo pct exec %d -- /bin/bash -l", vmID),
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Execute command using the current process environment and stdin/stdout
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute LXC shell command: %w", err)
	}

	return nil
}

// ExecuteQemuShell attempts to connect to a Qemu VM using SSH directly
// This is a fallback option when we don't have a more direct method
func ExecuteQemuShell(user, vmIP string) error {
	if vmIP == "" {
		return fmt.Errorf("no IP address available for VM")
	}

	sshCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", user, vmIP))
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Execute command using the current process environment and stdin/stdout
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("failed to connect to VM via SSH: %w", err)
	}

	return nil
}

// ExecuteQemuGuestAgentShell connects to a QEMU VM using the guest agent via host node
func ExecuteQemuGuestAgentShell(user, nodeIP string, vmID int) error {
	// For QEMU VMs with guest agent, we can SSH to node, then run guest agent commands
	fmt.Println("\nNOTE: This connects to the VM through the QEMU guest agent")
	fmt.Println("You will need root permissions on the Proxmox host for this to work")
	fmt.Println("Commands will execute inside the VM with the privileges of the guest agent")

	sshArgs := []string{
		fmt.Sprintf("%s@%s", user, nodeIP),
		"-t", // Force pseudo-terminal allocation
		fmt.Sprintf("sudo qm guest exec %d bash -- -l", vmID),
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Execute command using the current process environment and stdin/stdout
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute QEMU guest shell command: %w", err)
	}

	return nil
}
