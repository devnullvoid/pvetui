package ansible

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CommandResult stores process execution details.
type CommandResult struct {
	Command  string
	Output   string
	Duration time.Duration
	Err      error
}

// Runner executes local ansible commands.
type Runner struct{}

// NewRunner creates a new command runner.
func NewRunner() *Runner {
	return &Runner{}
}

// CheckAvailability verifies that required ansible binaries are available in PATH.
func (r *Runner) CheckAvailability() error {
	missing := make([]string, 0, 2)
	for _, bin := range []string{"ansible", "ansible-playbook"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required binaries in PATH: %s", strings.Join(missing, ", "))
	}

	return nil
}

// RunPing executes `ansible -m ping` using the generated inventory.
func (r *Runner) RunPing(ctx context.Context, inventoryText, limit string, extraArgs []string) CommandResult {
	inventoryPath, cleanup, err := writeTempInventory(inventoryText)
	if err != nil {
		return CommandResult{Err: fmt.Errorf("create temp inventory: %w", err)}
	}
	defer cleanup()

	args := []string{"-i", inventoryPath, "all", "-m", "ping"}
	if strings.TrimSpace(limit) != "" {
		args = append(args, "--limit", strings.TrimSpace(limit))
	}
	args = append(args, extraArgs...)

	return runAnsibleCommand(ctx, args)
}

// PlaybookOptions describes ansible-playbook invocation options.
type PlaybookOptions struct {
	PlaybookPath string
	Limit        string
	ExtraArgs    []string
	CheckMode    bool
}

// RunPlaybook executes ansible-playbook with a generated temporary inventory.
func (r *Runner) RunPlaybook(ctx context.Context, inventoryText string, opts PlaybookOptions) CommandResult {
	playbookPath := strings.TrimSpace(opts.PlaybookPath)
	if playbookPath == "" {
		return CommandResult{Err: fmt.Errorf("playbook path is required")}
	}

	inventoryPath, cleanup, err := writeTempInventory(inventoryText)
	if err != nil {
		return CommandResult{Err: fmt.Errorf("create temp inventory: %w", err)}
	}
	defer cleanup()

	args := []string{"-i", inventoryPath, playbookPath}
	if strings.TrimSpace(opts.Limit) != "" {
		args = append(args, "--limit", strings.TrimSpace(opts.Limit))
	}
	if opts.CheckMode {
		args = append(args, "--check")
	}
	args = append(args, opts.ExtraArgs...)

	return runAnsiblePlaybookCommand(ctx, args)
}

func runAnsibleCommand(ctx context.Context, args []string) CommandResult {
	started := time.Now()
	// #nosec G204 -- binary path is fixed to ansible; args are passed without shell interpolation.
	cmd := exec.CommandContext(ctx, "ansible", args...)
	output, err := cmd.CombinedOutput()
	duration := time.Since(started)

	result := CommandResult{
		Command:  strings.TrimSpace("ansible " + strings.Join(args, " ")),
		Output:   strings.TrimSpace(string(output)),
		Duration: duration,
		Err:      err,
	}
	if result.Output == "" {
		result.Output = "(no output)"
	}

	return result
}

func runAnsiblePlaybookCommand(ctx context.Context, args []string) CommandResult {
	started := time.Now()
	// #nosec G204 -- binary path is fixed to ansible-playbook; args are passed without shell interpolation.
	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	output, err := cmd.CombinedOutput()
	duration := time.Since(started)

	result := CommandResult{
		Command:  strings.TrimSpace("ansible-playbook " + strings.Join(args, " ")),
		Output:   strings.TrimSpace(string(output)),
		Duration: duration,
		Err:      err,
	}
	if result.Output == "" {
		result.Output = "(no output)"
	}

	return result
}

func writeTempInventory(inventoryText string) (path string, cleanup func(), err error) {
	tmpFile, err := os.CreateTemp("", "pvetui-ansible-inventory-*.ini")
	if err != nil {
		return "", nil, err
	}

	defer func() {
		if err != nil {
			_ = tmpFile.Close()
			// #nosec G703 -- tmpFile path is created by os.CreateTemp in system temp dir.
			_ = os.Remove(tmpFile.Name())
		}
	}()

	if _, err = tmpFile.WriteString(inventoryText); err != nil {
		return "", nil, err
	}
	if err = tmpFile.Close(); err != nil {
		return "", nil, err
	}
	// #nosec G703 -- tmpFile path is created by os.CreateTemp in system temp dir.
	if err = os.Chmod(tmpFile.Name(), 0o600); err != nil {
		return "", nil, err
	}

	cleanup = func() {
		// #nosec G703 -- tmpFile path is created by os.CreateTemp in system temp dir.
		_ = os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup, nil
}

// SaveInventory writes inventory text to a user-selected path.
func SaveInventory(path, content string) error {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" {
		return fmt.Errorf("path is required")
	}

	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create directory %q: %w", dir, err)
	}

	if err := os.WriteFile(cleanPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write inventory %q: %w", cleanPath, err)
	}

	return nil
}
