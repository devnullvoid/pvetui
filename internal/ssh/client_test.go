package ssh

import (
	"context"
	"os/exec"
	"testing"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

type mockExecutor struct {
	lastName string
	lastArgs []string
	called   int
}

func (m *mockExecutor) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	m.lastName = name

	m.lastArgs = append([]string(nil), args...)
	m.called++

	return exec.CommandContext(ctx, "true")
}

func TestSSHClient_WithExecutor(t *testing.T) {
	me := &mockExecutor{}
	client, err := NewSSHClient("192.0.2.1", "testuser", "", WithExecutor(me))
	require.NoError(t, err)

	err = client.Shell()
	require.NoError(t, err)
	require.Equal(t, 1, me.called)
	require.Equal(t, "ssh", me.lastName)
	require.Equal(t, []string{"testuser@192.0.2.1"}, me.lastArgs)
}

func TestExecuteLXCShellWith_StandardContainer(t *testing.T) {
	me := &mockExecutor{}
	ctx := context.Background()

	// Test standard LXC container (no VM info)
	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 100, nil, config.SSHJumpHost{})
	require.NoError(t, err)
	require.Equal(t, 1, me.called)
	require.Equal(t, "ssh", me.lastName)
	require.Equal(t, []string{"testuser@192.0.2.1", "-t", "sudo pct enter 100"}, me.lastArgs)
}

func TestExecuteLXCShellWith_NonNixOSContainer(t *testing.T) {
	me := &mockExecutor{}
	ctx := context.Background()

	// Test non-NixOS container
	vm := &api.VM{
		ID:     101,
		OSType: "ubuntu",
	}

	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 101, vm, config.SSHJumpHost{})
	require.NoError(t, err)
	require.Equal(t, 1, me.called)
	require.Equal(t, "ssh", me.lastName)
	require.Equal(t, []string{"testuser@192.0.2.1", "-t", "sudo pct enter 101"}, me.lastArgs)
}

func TestExecuteLXCShellWith_NixOSContainer(t *testing.T) {
	me := &mockExecutor{}
	ctx := context.Background()

	// Test NixOS container with "nixos" OSType
	vm := &api.VM{
		ID:     102,
		OSType: "nixos",
	}

	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 102, vm, config.SSHJumpHost{})
	require.NoError(t, err)
	require.Equal(t, 1, me.called)
	require.Equal(t, "ssh", me.lastName)

	expectedCmd := "sudo pct exec 102 -- /bin/sh -c 'if [ -f /etc/set-environment ]; then . /etc/set-environment; fi; exec bash'"
	require.Equal(t, []string{"testuser@192.0.2.1", "-t", expectedCmd}, me.lastArgs)
}

func TestExecuteLXCShellWith_NixContainer(t *testing.T) {
	me := &mockExecutor{}
	ctx := context.Background()

	// Test NixOS container with "nix" OSType
	vm := &api.VM{
		ID:     103,
		OSType: "nix",
	}

	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 103, vm, config.SSHJumpHost{})
	require.NoError(t, err)
	require.Equal(t, 1, me.called)
	require.Equal(t, "ssh", me.lastName)

	expectedCmd := "sudo pct exec 103 -- /bin/sh -c 'if [ -f /etc/set-environment ]; then . /etc/set-environment; fi; exec bash'"
	require.Equal(t, []string{"testuser@192.0.2.1", "-t", expectedCmd}, me.lastArgs)
}

func TestExecuteLXCShellWithVM(t *testing.T) {
	// Test the convenience function with mock executor
	me := &mockExecutor{}

	vm := &api.VM{
		ID:     104,
		OSType: "nixos",
	}

	// Test using the lower-level function with mock executor
	ctx := context.Background()
	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", vm.ID, vm, config.SSHJumpHost{})
	require.NoError(t, err)
	require.Equal(t, 1, me.called)
	require.Equal(t, "ssh", me.lastName)

	expectedCmd := "sudo pct exec 104 -- /bin/sh -c 'if [ -f /etc/set-environment ]; then . /etc/set-environment; fi; exec bash'"
	require.Equal(t, []string{"testuser@192.0.2.1", "-t", expectedCmd}, me.lastArgs)
}

func TestExecuteLXCShellWith_RootUserSkipsSudo(t *testing.T) {
	me := &mockExecutor{}
	ctx := context.Background()

	err := ExecuteLXCShellWith(ctx, me, "root", "192.0.2.1", 200, nil, config.SSHJumpHost{})
	require.NoError(t, err)
	require.Equal(t, []string{"root@192.0.2.1", "-t", "pct enter 200"}, me.lastArgs)

	me = &mockExecutor{}
	vm := &api.VM{ID: 201, OSType: "nixos"}
	err = ExecuteLXCShellWith(ctx, me, "root", "192.0.2.1", vm.ID, vm, config.SSHJumpHost{})
	require.NoError(t, err)
	expectedCmd := "pct exec 201 -- /bin/sh -c 'if [ -f /etc/set-environment ]; then . /etc/set-environment; fi; exec bash'"
	require.Equal(t, []string{"root@192.0.2.1", "-t", expectedCmd}, me.lastArgs)
}

func TestExecuteNodeShellWith_JumpHostProxyCommand(t *testing.T) {
	me := &mockExecutor{}
	ctx := context.Background()

	jumpHost := config.SSHJumpHost{
		Addr:    "jump.example.com",
		User:    "jumpuser",
		Keyfile: "/home/test/.ssh/jump key",
	}

	err := ExecuteNodeShellWith(ctx, me, "testuser", "192.0.2.1", jumpHost)
	require.NoError(t, err)
	require.Equal(t, "ssh", me.lastName)

	expectedProxy := "ProxyCommand=ssh -W %h:%p -i '/home/test/.ssh/jump key' -l 'jumpuser' 'jump.example.com'"
	require.Equal(t, []string{"-o", expectedProxy, "testuser@192.0.2.1"}, me.lastArgs)
}

func TestExecuteNodeShellWith_JumpHostProxyJump(t *testing.T) {
	me := &mockExecutor{}
	ctx := context.Background()

	jumpHost := config.SSHJumpHost{
		Addr: "jump.example.com",
		User: "jumpuser",
	}

	err := ExecuteNodeShellWith(ctx, me, "testuser", "192.0.2.1", jumpHost)
	require.NoError(t, err)
	require.Equal(t, "ssh", me.lastName)
	require.Equal(t, []string{"-J", "jumpuser@jump.example.com", "testuser@192.0.2.1"}, me.lastArgs)
}
