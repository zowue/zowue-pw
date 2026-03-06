package webhook

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// CommandRunner executes shell commands with timeout
type CommandRunner struct {
	defaultTimeout time.Duration
}

// NewCommandRunner creates command runner with default timeout
func NewCommandRunner() *CommandRunner {
	return &CommandRunner{
		defaultTimeout: 5 * time.Minute,
	}
}

// Run executes command with timeout
func (r *CommandRunner) Run(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) (string, error) {
	if timeout == 0 {
		timeout = r.defaultTimeout
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("command timeout after %v: %s", timeout, name)
		}
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}
