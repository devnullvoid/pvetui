package ssh

import (
    "golang.org/x/crypto/ssh"
)

// SSHClient wraps an SSH connection context
// TODO: implement methods to connect and execute commands
type SSHClient struct {
    client *ssh.Client
}

// NewSSHClient establishes an SSH connection to host.
func NewSSHClient(host, user, password string) (*SSHClient, error) {
    // TODO: implement actual SSH dialing
    return &SSHClient{}, nil
}
