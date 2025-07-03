package ssh

import (
	"context"
	"os/exec"
)

// CommandExecutor abstracts exec.CommandContext to allow dependency injection.
type CommandExecutor interface {
	CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd
}

// defaultExecutor is the default implementation using os/exec.
type defaultExecutor struct{}

func (defaultExecutor) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// NewDefaultExecutor returns a new CommandExecutor using os/exec.
func NewDefaultExecutor() CommandExecutor { return defaultExecutor{} }
