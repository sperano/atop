package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sperano/atop/internal/source"
)

const testWidth = 120

var testNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func fixedNow() time.Time { return testNow }

// fakeOpener records what it was asked to open.
type fakeOpener struct {
	urls  []string
	panes []string
	err   error
}

func (f *fakeOpener) OpenURL(url string) error {
	f.urls = append(f.urls, url)
	return f.err
}

func (f *fakeOpener) SwitchTmux(pane string) error {
	f.panes = append(f.panes, pane)
	return f.err
}

func newTestModel(op Opener, inTmux bool, rows ...source.Row) Model {
	m := New(Options{
		Fetch:    func(_ context.Context) []source.Row { return rows },
		Now:      fixedNow,
		Opener:   op,
		InTmux:   inTmux,
		Interval: time.Minute,
	})
	return send(m, rowsMsg{rows: rows})
}

func send(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func named(names ...string) []source.Row {
	rows := make([]source.Row, len(names))
	for i, n := range names {
		rows[i] = source.Row{Source: "pr", Name: n, State: "review"}
	}
	return rows
}
