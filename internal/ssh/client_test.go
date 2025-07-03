package ssh

import (
	"context"
	"os/exec"
	"testing"

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
