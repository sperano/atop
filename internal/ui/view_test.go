package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/sperano/vigie/internal/board"
	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

const (
	escape     = "\x1b"
	longDetail = 500
)

func snapshotLines(t *testing.T, rows []source.Row, color bool) []string {
	t.Helper()
	return strings.Split(strings.TrimRight(Snapshot(rows, hidden.Set{}, testNow, testWidth, color), "\n"), "\n")
}

func TestSnapshotPlain(t *testing.T) {
	t.Parallel()
	rows := []source.Row{
		{Source: "pr", Name: "cocovm#1", State: "awaiting your review", Since: testNow, Detail: strings.Repeat("x", longDetail), NeedsYou: true},
		{Source: "pr", Name: "cocovm#2", State: "draft", Since: testNow, Detail: "multi\nline"},
	}
	out := Snapshot(rows, hidden.Set{}, testNow, testWidth, false)
	if strings.Contains(out, escape) {
		t.Error("plain snapshot contains ESC bytes")
	}
	lines := snapshotLines(t, rows, false)
	if !strings.Contains(lines[0], "1 need you") {
		t.Errorf("title = %q", lines[0])
	}
	if !strings.Contains(lines[2], "TYPE") || !strings.Contains(lines[2], "ORIGIN") || strings.Contains(lines[2], "SOURCE") {
		t.Errorf("header = %q", lines[2])
	}
	if !strings.HasPrefix(lines[3], "● pr") || !strings.Contains(lines[3], "unknown") {
		t.Errorf("needs-you row = %q", lines[3])
	}
	if !strings.Contains(lines[3], "awaiting your review") {
		t.Errorf("state was truncated: %q", lines[3])
	}
	if got := ansi.StringWidth(lines[3]); got != testWidth {
		t.Errorf("row width = %d, want %d", got, testWidth)
	}
	if !strings.HasSuffix(lines[3], "…") {
		t.Errorf("long detail not truncated with ellipsis: %q", lines[3])
	}
	if !strings.HasPrefix(lines[4], "  pr") || !strings.Contains(lines[4], "unknown") || !strings.HasSuffix(lines[4], "multi line") {
		t.Errorf("second row = %q", lines[4])
	}
}

func TestSnapshotColour(t *testing.T) {
	t.Parallel()
	rows := []source.Row{{Source: "pr", Name: "a", State: "review", NeedsYou: true}}
	if out := Snapshot(rows, hidden.Set{}, testNow, testWidth, true); !strings.Contains(out, escape) {
		t.Error("colour snapshot has no ANSI sequences")
	}
}

func TestRowColourFollowsSeverity(t *testing.T) {
	t.Parallel()
	st := newStyles(true)
	tests := []struct {
		severity board.Severity
		want     string
	}{
		{board.SeverityFailed, st.failed.Render("row")},
		{board.SeverityBlocked, st.blocked.Render("row")},
		{board.SeverityBusy, st.busy.Render("row")},
		{board.SeverityAttention, st.attention.Render("row")},
		{board.SeverityQuiet, st.rest.Render("row")},
	}
	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			t.Parallel()
			if got := st.entry(board.Entry{Severity: tt.severity}).Render("row"); got != tt.want {
				t.Errorf("styled row = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSnapshotNarrowTerminalKeepsMinDetail(t *testing.T) {
	t.Parallel()
	rows := []source.Row{{Source: "pr", Name: "a", State: "review", Detail: strings.Repeat("y", longDetail)}}
	line := strings.Split(Snapshot(rows, hidden.Set{}, testNow, fixedWidthForTest, false), "\n")[3]
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

func TestViewAgesFollowTheClock(t *testing.T) {
	t.Parallel()
	now := testNow
	rows := []source.Row{{Source: "pr", Name: "a", State: "review", Since: testNow}}
	m := New(Options{Now: func() time.Time { return now }, Opener: &fakeOpener{}})
	m = send(m, rowsMsg{rows: rows})
	now = testNow.Add(time.Hour) // no refresh in between
	if v := m.View().Content; !strings.Contains(v, " 1h ") {
		t.Errorf("view does not age the row to 1h without a refresh:\n%s", v)
	}
}
