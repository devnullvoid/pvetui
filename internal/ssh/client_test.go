package ssh

import (
	"context"
	"os/exec"
	"testing"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
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
	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 100, nil)
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

	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 101, vm)
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

	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 102, vm)
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

	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", 103, vm)
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
	err := ExecuteLXCShellWith(ctx, me, "testuser", "192.0.2.1", vm.ID, vm)
	require.NoError(t, err)
	require.Equal(t, 1, me.called)
	require.Equal(t, "ssh", me.lastName)
	expectedCmd := "sudo pct exec 104 -- /bin/sh -c 'if [ -f /etc/set-environment ]; then . /etc/set-environment; fi; exec bash'"
	require.Equal(t, []string{"testuser@192.0.2.1", "-t", expectedCmd}, me.lastArgs)
}
