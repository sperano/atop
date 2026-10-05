package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/sperano/atop/internal/source"
)

const (
	escape     = "\x1b"
	longDetail = 500
)

func snapshotLines(t *testing.T, rows []source.Row, color bool) []string {
	t.Helper()
	return strings.Split(strings.TrimRight(Snapshot(rows, testNow, testWidth, color), "\n"), "\n")
}

func TestSnapshotPlain(t *testing.T) {
	t.Parallel()
	rows := []source.Row{
		{Source: "pr", Name: "cocovm#1", State: "review", Since: testNow, Detail: strings.Repeat("x", longDetail), NeedsYou: true},
		{Source: "pr", Name: "cocovm#2", State: "draft", Since: testNow, Detail: "multi\nline"},
	}
	out := Snapshot(rows, testNow, testWidth, false)
	if strings.Contains(out, escape) {
		t.Error("plain snapshot contains ESC bytes")
	}
	lines := snapshotLines(t, rows, false)
	if !strings.Contains(lines[0], "1 need you") {
		t.Errorf("title = %q", lines[0])
	}
	if !strings.Contains(lines[2], "AGE") || !strings.Contains(lines[2], "DETAIL") {
		t.Errorf("header = %q", lines[2])
	}
	if !strings.HasPrefix(lines[3], "● pr") {
		t.Errorf("needs-you row = %q", lines[3])
	}
	if got := ansi.StringWidth(lines[3]); got != testWidth {
		t.Errorf("row width = %d, want %d", got, testWidth)
	}
	if !strings.HasSuffix(lines[3], "…") {
		t.Errorf("long detail not truncated with ellipsis: %q", lines[3])
	}
	if !strings.HasPrefix(lines[4], "  pr") || !strings.HasSuffix(lines[4], "multi line") {
		t.Errorf("second row = %q", lines[4])
	}
}

func TestSnapshotColour(t *testing.T) {
	t.Parallel()
	rows := []source.Row{{Source: "pr", Name: "a", State: "review", NeedsYou: true}}
	if out := Snapshot(rows, testNow, testWidth, true); !strings.Contains(out, escape) {
		t.Error("colour snapshot has no ANSI sequences")
	}
}

func TestSnapshotNarrowTerminalKeepsMinDetail(t *testing.T) {
	t.Parallel()
	rows := []source.Row{{Source: "pr", Name: "a", State: "review", Detail: strings.Repeat("y", longDetail)}}
	line := strings.Split(Snapshot(rows, testNow, fixedWidthForTest, false), "\n")[3]
	if got := ansi.StringWidth(line); got != fixedWidth+minDetailWidth {
		t.Errorf("width = %d, want %d", got, fixedWidth+minDetailWidth)
	}
}

const fixedWidthForTest = 40

func TestViewStatus(t *testing.T) {
	t.Parallel()
	m := New(Options{Fetch: nil, Now: fixedNow, Opener: &fakeOpener{}})
	if v := m.View().Content; !strings.Contains(v, "refreshing…") {
		t.Errorf("refreshing model view lacks status: %q", v)
	}
	m = send(m, rowsMsg{rows: named("a")})
	v := m.View().Content
	if strings.Contains(v, "refreshing…") {
		t.Error("idle model still says refreshing")
	}
	if !strings.Contains(v, "a") || !strings.Contains(v, "q quit") {
		t.Errorf("view = %q", v)
	}
}

func TestViewNoRows(t *testing.T) {
	t.Parallel()
	m := send(New(Options{Now: fixedNow, Opener: &fakeOpener{}}), rowsMsg{})
	if v := m.View().Content; !strings.Contains(v, statusNoRows) {
		t.Errorf("view lacks %q: %q", statusNoRows, v)
	}
}
