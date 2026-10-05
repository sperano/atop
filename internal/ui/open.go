package ui

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// openTimeout bounds the open and tmux commands.
const openTimeout = 10 * time.Second

// Opener performs what enter does for a row.
type Opener interface {
	OpenURL(url string) error
	SwitchTmux(pane string) error
}

// ExecOpener opens URLs with the platform opener and switches tmux panes,
// running each command directly, without a shell.
type ExecOpener struct{}

// URLCommand is the program that opens a URL on this platform.
func URLCommand(goos string) string {
	if goos == "darwin" {
		return "open"
	}
	return "xdg-open"
}

// OpenURL opens an http(s) URL in the default browser. Targets come from
// remote data, so other schemes, and anything `open` could read as an
// option, are refused.
func (ExecOpener) OpenURL(raw string) error {
	if err := checkURL(raw); err != nil {
		return err
	}
	return run(URLCommand(runtime.GOOS), raw)
}

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// checkURL accepts only absolute http and https URLs with a host.
func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS) || u.Host == "" {
		return fmt.Errorf("refusing to open %q", raw)
	}
	return nil
}

// SwitchTmux switches the current tmux client to pane.
func (ExecOpener) SwitchTmux(pane string) error {
	return run("tmux", "switch-client", "-t", pane)
}

func run(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
