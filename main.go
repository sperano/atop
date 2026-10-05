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
	"github.com/sperano/atop/internal/source"
	"github.com/sperano/atop/internal/ui"
)

// fallbackWidth is the --once width when stdout is not a terminal.
const fallbackWidth = 120

func main() {
	cfg, err := config.Parse(os.Args[0], os.Args[1:], os.Getenv)
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
	sources, err := buildSources(cfg)
	if err != nil {
		return err
	}
	fetch := func(ctx context.Context) []source.Row { return source.FetchAll(ctx, sources) }
	stdoutTTY := term.IsTerminal(os.Stdout.Fd())
	color := stdoutTTY && os.Getenv("NO_COLOR") == ""
	if cfg.Once {
		return printOnce(fetch, stdoutTTY, color)
	}
	model := ui.New(ui.Options{
		Fetch:    fetch,
		Now:      time.Now,
		Opener:   ui.ExecOpener{},
		InTmux:   os.Getenv("TMUX") != "",
		Interval: cfg.Interval,
		Color:    color,
	})
	_, err = tea.NewProgram(model).Run()
	return err
}

func printOnce(fetch func(context.Context) []source.Row, tty, color bool) error {
	rows := fetch(context.Background())
	ui.SortRows(rows)
	width := fallbackWidth
	if tty {
		if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil {
			width = w
		}
	}
	_, err := fmt.Print(ui.Snapshot(rows, time.Now(), width, color))
	return err
}

func buildSources(cfg config.Config) ([]source.Named, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	run := source.Runner(source.ExecRunner)
	vikunjaWeb := source.VikunjaWebBase(cfg.VikunjaURL)
	kelos := source.Kelos{
		Namespace: cfg.Namespace, ConsoleURL: cfg.ConsoleURL, VikunjaWeb: vikunjaWeb,
		Run: run, Now: time.Now,
	}
	github := source.GitHub{Owner: cfg.GitHubOwner, StaleDays: cfg.StaleDays, Run: run, Now: time.Now}
	vikunja := source.Vikunja{
		APIURL:          cfg.VikunjaURL,
		Token:           os.Getenv("VIKUNJA_API_TOKEN"),
		Secret:          source.TokenSecret{Namespace: cfg.TokenNamespace, Name: cfg.TokenSecret, Key: cfg.TokenKey},
		Operator:        cfg.Operator,
		InProgressLabel: cfg.InProgressLabel,
		StaleDays:       cfg.StaleDays,
		Client:          &http.Client{},
		Run:             run,
		Now:             time.Now,
	}
	local := source.LocalSessions{Home: home, Alive: source.PIDAlive}
	return []source.Named{
		{Name: source.SourceLocal, Fetch: func(context.Context) ([]source.Row, error) { return local.Fetch() }},
		{Name: source.SourceKelosSession, Fetch: kelos.Sessions},
		{Name: source.SourceKelosTask, Fetch: kelos.Tasks},
		{Name: source.SourcePR, Fetch: github.Fetch},
		{Name: source.SourceVikunja, Fetch: vikunja.Fetch},
	}, nil
}
