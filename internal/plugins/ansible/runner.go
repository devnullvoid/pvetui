package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CommandResult stores process execution details.
type CommandResult struct {
	Command  string
	Output   string
	Duration time.Duration
	Err      error
}

// OutputLineHandler receives a single line of command output.
type OutputLineHandler func(line string)

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
func (r *Runner) RunPing(ctx context.Context, inventoryText, inventoryFormat, limit string, extraArgs []string) CommandResult {
	return r.RunPingStream(ctx, inventoryText, inventoryFormat, limit, extraArgs, nil)
}

// RunPingStream executes `ansible -m ping` and streams output lines when handler is provided.
func (r *Runner) RunPingStream(
	ctx context.Context,
	inventoryText,
	inventoryFormat,
	limit string,
	extraArgs []string,
	handler OutputLineHandler,
) CommandResult {
	inventoryPath, cleanup, err := writeTempInventory(inventoryText, inventoryFormat)
	if err != nil {
		return CommandResult{Err: fmt.Errorf("create temp inventory: %w", err)}
	}
	defer cleanup()

	args := []string{"-i", inventoryPath, "all", "-m", "ping"}
	if strings.TrimSpace(limit) != "" {
		args = append(args, "--limit", strings.TrimSpace(limit))
	}
	args = append(args, extraArgs...)

	return runAnsibleCommand(ctx, args, handler)
}

// PlaybookOptions describes ansible-playbook invocation options.
type PlaybookOptions struct {
	PlaybookPath string
	Limit        string
	ExtraArgs    []string
	CheckMode    bool
}

// RunPlaybook executes ansible-playbook with a generated temporary inventory.
func (r *Runner) RunPlaybook(ctx context.Context, inventoryText, inventoryFormat string, opts PlaybookOptions) CommandResult {
	return r.RunPlaybookStream(ctx, inventoryText, inventoryFormat, opts, nil)
}

// RunPlaybookStream executes ansible-playbook and streams output lines when handler is provided.
func (r *Runner) RunPlaybookStream(
	ctx context.Context,
	inventoryText,
	inventoryFormat string,
	opts PlaybookOptions,
	handler OutputLineHandler,
) CommandResult {
	playbookPath := strings.TrimSpace(opts.PlaybookPath)
	if playbookPath == "" {
		return CommandResult{Err: fmt.Errorf("playbook path is required")}
	}

	inventoryPath, cleanup, err := writeTempInventory(inventoryText, inventoryFormat)
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

	return runAnsiblePlaybookCommand(ctx, args, handler)
}

func runAnsibleCommand(ctx context.Context, args []string, handler OutputLineHandler) CommandResult {
	started := time.Now()
	// #nosec G204 -- binary path is fixed to ansible; args are passed without shell interpolation.
	cmd := exec.CommandContext(ctx, "ansible", args...)
	return runCommandStreaming(cmd, "ansible "+strings.Join(args, " "), started, handler)
}

func runAnsiblePlaybookCommand(ctx context.Context, args []string, handler OutputLineHandler) CommandResult {
	started := time.Now()
	// #nosec G204 -- binary path is fixed to ansible-playbook; args are passed without shell interpolation.
	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	return runCommandStreaming(cmd, "ansible-playbook "+strings.Join(args, " "), started, handler)
}

func runCommandStreaming(cmd *exec.Cmd, command string, started time.Time, handler OutputLineHandler) CommandResult {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return CommandResult{Command: strings.TrimSpace(command), Err: fmt.Errorf("stdout pipe: %w", err)}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return CommandResult{Command: strings.TrimSpace(command), Err: fmt.Errorf("stderr pipe: %w", err)}
	}

	if err := cmd.Start(); err != nil {
		return CommandResult{Command: strings.TrimSpace(command), Err: err}
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
		b  bytes.Buffer
	)
	appendLine := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	streamReader := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		const maxScanToken = 1024 * 1024
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, maxScanToken)
		for scanner.Scan() {
			line := scanner.Text()
			appendLine(line)
			if handler != nil {
				handler(line)
			}
		}
	}

	wg.Add(2)
	go streamReader(stdoutPipe)
	go streamReader(stderrPipe)

	waitErr := cmd.Wait()
	wg.Wait()
	duration := time.Since(started)

	output := strings.TrimSpace(b.String())
	result := CommandResult{
		Command:  strings.TrimSpace(command),
		Output:   output,
		Duration: duration,
		Err:      waitErr,
	}
	if result.Output == "" {
		result.Output = "(no output)"
	}

	return result
}

func writeTempInventory(inventoryText, inventoryFormat string) (path string, cleanup func(), err error) {
	ext := ".ini"
	if NormalizeInventoryFormat(inventoryFormat) == InventoryFormatYAML {
		ext = ".yml"
	}

	tmpFile, err := os.CreateTemp("", "pvetui-ansible-inventory-*"+ext)
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
