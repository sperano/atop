package ui

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

const (
	goldenDir      = "testdata"
	goldenNow      = "<now>"
	goldenFileMode = 0o644
	goldenNarrow   = 40
	goldenHeight   = 20
	goldenTaskRef  = "vikunja:12"
	goldenPRRef    = "pr:https://github.com/o/vigie/pull/3"
	goldenOtherRef = "vikunja:13"
)

// goldenRows covers every source, a three-level tree with sibling guides,
// hidden rows (one with a child), a source error, a stale PR, a multi-line
// detail, an unknown origin and an unknown age.
func goldenRows() []source.Row {
	at := func(d time.Duration) time.Time { return testNow.Add(-d) }
	return []source.Row{
		{Source: source.SourceVikunja, Origin: source.OriginVikunja, Name: "#12", State: "reply from bob", Since: at(3 * time.Hour),
			Detail: "[eric] board package", NeedsYou: true, Ref: goldenTaskRef, Target: source.Target{URL: "https://v.test/tasks/12"}},
		{Source: source.SourcePR, Origin: source.OriginGitHub, Name: "vigie#3", State: source.StateChecksFailing, Since: at(2 * time.Hour),
			Detail: "[claude-opus] Move the board", NeedsYou: true, Ref: goldenPRRef, ParentRefs: []string{goldenTaskRef},
			Target: source.Target{URL: "https://github.com/o/vigie/pull/3"}},
		{Source: source.SourceKelosTask, Origin: source.OriginKelos, Name: "vikunja-vigie-12-fix", State: "cubic fix", Since: at(5 * time.Minute),
			Detail: "running\nsecond line", Busy: true, ParentRefs: []string{goldenPRRef, goldenTaskRef}},
		{Source: source.SourceKelosTask, Origin: source.OriginKelos, Name: "vikunja-vigie-12", State: source.StateDone, Since: at(40 * time.Minute),
			ParentRefs: []string{goldenTaskRef}},
		{Source: source.SourceVikunja, Origin: source.OriginVikunja, Name: "#13", State: source.StateIdle, Since: at(4 * source.Day),
			Detail: "[eric] hidden parent", Ref: goldenOtherRef},
		{Source: source.SourcePR, Origin: source.OriginGitHub, Name: "vigie#4", State: "review", Since: at(time.Hour),
			Detail: "child of a hidden task", NeedsYou: true, ParentRefs: []string{goldenOtherRef}},
		{Source: source.SourcePR, Name: "old#1", State: source.StateStale, Since: at(40 * source.Day), Detail: "日本語 abandoned"},
		{Source: source.SourcePR, Origin: source.OriginGitHub, Name: "vigie#5", State: "draft", Since: at(59 * time.Second), Detail: "hidden draft"},
		{Source: source.SourceLocal, Origin: source.OriginLocal, Name: "laptop", State: source.StateBusy, Since: at(30 * time.Second),
			Busy: true, Target: source.Target{TmuxPane: "h:@5.%6"}},
		{Source: source.SourceKelosSession, Origin: source.OriginKelos, Name: "gordon", State: source.StateIdle},
		source.ErrorRow(source.SourceKelosTask, errors.New("tasks.kelos.dev is forbidden")),
	}
}

func goldenHidden() hidden.Set {
	return hidden.New(
		hidden.Entry{Source: source.SourceVikunja, Name: "#13"},
		hidden.Entry{Source: source.SourcePR, Name: "vigie#5"},
	)
}

func goldenModel(color bool) Model {
	rows := goldenRows()
	set := goldenHidden()
	m := New(Options{
		Now:          fixedNow,
		Opener:       &fakeOpener{},
		Interval:     time.Minute,
		Color:        color,
		UpdateHidden: func(change func(hidden.Set) hidden.Set) (hidden.Set, error) { return change(set), nil },
	})
	m = send(m, tea.WindowSizeMsg{Width: testWidth, Height: goldenHeight})
	return send(m, rowsMsg{rows: rows})
}

// TestGolden pins the --once and TUI output, so that moving the board
// logic around cannot change what is drawn.
func TestGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  func() string
	}{
		{"snapshot_plain", func() string { return Snapshot(goldenRows(), goldenHidden(), testNow, testWidth, false) }},
		{"snapshot_colour", func() string { return Snapshot(goldenRows(), goldenHidden(), testNow, testWidth, true) }},
		{"snapshot_narrow", func() string { return Snapshot(goldenRows(), goldenHidden(), testNow, goldenNarrow, false) }},
		{"view_plain", func() string { return send(goldenModel(false), keyDown).View().Content }},
		{"view_colour", func() string { return send(goldenModel(true), keyDown).View().Content }},
		{"view_show_hidden", func() string { return send(send(goldenModel(false), keyShowHidden), keyEnd).View().Content }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := strings.ReplaceAll(tt.got(), testNow.Local().Format(timeLayout), goldenNow)
			path := filepath.Join(goldenDir, tt.name+".golden")
			if *updateGolden {
				if err := os.WriteFile(path, []byte(got), goldenFileMode); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("%s differs from %s:\ngot:\n%s\nwant:\n%s", tt.name, path, got, want)
			}
		})
	}
}
