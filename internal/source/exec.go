package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommandTimeout bounds every external command a source runs.
const CommandTimeout = 30 * time.Second

// Runner runs a command and returns its standard output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ExecRunner runs commands directly, without a shell, under CommandTimeout.
// A failure carries the command's trimmed standard error.
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%s: timed out after %s", name, CommandTimeout)
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

// runJSON runs a command and decodes its standard output into v.
func runJSON(ctx context.Context, run Runner, v any, name string, args ...string) error {
	out, err := run(ctx, name, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("%s: decoding output: %w", name, err)
	}
	return nil
}
