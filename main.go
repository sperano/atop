// Command atop is a top for coding agents: which one needs you, who is
// running what, who is idle.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/sperano/atop/internal/config"
	"github.com/sperano/atop/internal/hidden"
	"github.com/sperano/atop/internal/source"
	"github.com/sperano/atop/internal/ui"
)

const (
	appName = "atop"
	// fallbackWidth is the --once width when stdout is not a terminal.
	fallbackWidth = 120
)

func main() {
	cfg, err := config.Parse(appName, os.Args[1:], os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		if errors.Is(err, config.ErrInvalid) {
			fmt.Fprintln(os.Stderr, "atop:", err)
		}
		os.Exit(2)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "atop:", err)
		os.Exit(1)
	}
}

func run(cfg config.Config) error {
	hiddenFile, err := hiddenStore(cfg.HiddenFile)
	if err != nil {
		return err
	}
	hiddenRows, err := hiddenFile.Load()
	if err != nil {
		return fmt.Errorf("hidden rows: %w", err)
	}
	sources := buildSources(cfg)
	fetch := func(ctx context.Context) []source.Row { return source.FetchAll(ctx, sources) }
	stdoutTTY := term.IsTerminal(os.Stdout.Fd())
	color := stdoutTTY && os.Getenv("NO_COLOR") == ""
	if cfg.Once {
		return printOnce(fetch, hiddenRows, stdoutTTY, color)
	}
	model := ui.New(ui.Options{
		Fetch:      fetch,
		Now:        time.Now,
		Opener:     ui.ExecOpener{},
		InTmux:     os.Getenv("TMUX") != "",
		Interval:   cfg.Interval,
		Color:      color,
		Hidden:     hiddenRows,
		SaveHidden: hiddenFile.Save,
	})
	_, err = tea.NewProgram(model).Run()
	return err
}

// hiddenStore is the file of hidden rows: path, else the default one.
func hiddenStore(path string) (hidden.File, error) {
	if path != "" {
		return hidden.File{Path: path}, nil
	}
	path, err := hidden.DefaultPath()
	if err != nil {
		return hidden.File{}, fmt.Errorf("hidden rows: %w (set --hidden-file)", err)
	}
	return hidden.File{Path: path}, nil
}

func printOnce(fetch func(context.Context) []source.Row, set hidden.Set, tty, color bool) error {
	rows := fetch(context.Background())
	width := fallbackWidth
	if tty {
		if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil {
			width = w
		}
	}
	_, err := fmt.Print(ui.Snapshot(rows, set, time.Now(), width, color))
	return err
}

func buildSources(cfg config.Config) []source.Named {
	runner := source.Runner(source.ExecRunner)
	vikunjaWeb := source.VikunjaWebBase(cfg.VikunjaURL)
	kelos := source.Kelos{
		Namespace: cfg.Namespace, ConsoleURL: cfg.ConsoleURL, VikunjaWeb: vikunjaWeb,
		Run: runner, Now: time.Now,
	}
	github := source.GitHub{Owner: cfg.GitHubOwner, StaleDays: cfg.StaleDays, Run: runner, Now: time.Now}
	vikunja := source.Vikunja{
		APIURL:          cfg.VikunjaURL,
		Token:           os.Getenv("VIKUNJA_API_TOKEN"),
		Secret:          source.TokenSecret{Namespace: cfg.TokenNamespace, Name: cfg.TokenSecret, Key: cfg.TokenKey},
		Operator:        cfg.Operator,
		InProgressLabel: cfg.InProgressLabel,
		PROpenLabel:     cfg.PROpenLabel,
		StaleDays:       cfg.StaleDays,
		Client:          &http.Client{},
		Run:             runner,
		Now:             time.Now,
	}
	return []source.Named{
		{Name: source.SourceLocal, Fetch: fetchLocal},
		{Name: source.SourceKelosSession, Fetch: kelos.Sessions},
		{Name: source.SourceKelosTask, Fetch: kelos.Tasks},
		{Name: source.SourcePR, Fetch: github.Fetch},
		{Name: source.SourceVikunja, Fetch: vikunja.Fetch},
	}
}

// fetchLocal lists local sessions; a missing home directory fails only this
// source.
func fetchLocal(context.Context) ([]source.Row, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return source.LocalSessions{Home: home, Alive: source.PIDAlive}.Fetch()
}
