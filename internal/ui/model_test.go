package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sperano/atop/internal/source"
)

const (
	testHeight = 15 // body height = testHeight - chromeLines
	rowCount   = 30
)

var (
	keyUp     = tea.KeyPressMsg{Code: tea.KeyUp}
	keyDown   = tea.KeyPressMsg{Code: tea.KeyDown}
	keyPgUp   = tea.KeyPressMsg{Code: tea.KeyPgUp}
	keyPgDown = tea.KeyPressMsg{Code: tea.KeyPgDown}
	keyHome   = tea.KeyPressMsg{Code: tea.KeyHome}
	keyEnd    = tea.KeyPressMsg{Code: tea.KeyEnd}
	keyEnter  = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyCtrlC  = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
)

func keyRune(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func sized(m Model) Model {
	return send(m, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
}

func manyRows() []source.Row {
	names := make([]string, rowCount)
	for i := range names {
		names[i] = string(rune('a'+i/10)) + string(rune('0'+i%10))
	}
	return named(names...)
}

func TestKeyStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg  tea.KeyPressMsg
		want string
	}{
		{keyUp, "up"}, {keyDown, "down"}, {keyPgUp, "pgup"}, {keyPgDown, "pgdown"},
		{keyHome, "home"}, {keyEnd, "end"}, {keyEnter, "enter"}, {keyCtrlC, "ctrl+c"},
		{keyRune('j'), "j"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.msg.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMoveKeys(t *testing.T) {
	t.Parallel()
	body := testHeight - chromeLines
	tests := []struct {
		name  string
		start int
		keys  []tea.KeyPressMsg
		want  int
	}{
		{"down", 0, []tea.KeyPressMsg{keyDown}, 1},
		{"j", 0, []tea.KeyPressMsg{keyRune('j')}, 1},
		{"up", 5, []tea.KeyPressMsg{keyUp}, 4},
		{"k", 5, []tea.KeyPressMsg{keyRune('k')}, 4},
		{"clamp at top", 0, []tea.KeyPressMsg{keyUp, keyRune('k')}, 0},
		{"clamp at bottom", rowCount - 1, []tea.KeyPressMsg{keyDown, keyRune('j')}, rowCount - 1},
		{"pgdn by body height", 0, []tea.KeyPressMsg{keyPgDown}, body},
		{"pgdn clamps", rowCount - 2, []tea.KeyPressMsg{keyPgDown}, rowCount - 1},
		{"pgup by body height", 20, []tea.KeyPressMsg{keyPgUp}, 20 - body},
		{"pgup clamps", 3, []tea.KeyPressMsg{keyPgUp}, 0},
		{"home", 12, []tea.KeyPressMsg{keyHome}, 0},
		{"end", 0, []tea.KeyPressMsg{keyEnd}, rowCount - 1},
		{"unrelated key", 4, []tea.KeyPressMsg{keyRune('x')}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := sized(newTestModel(&fakeOpener{}, false, manyRows()...))
			m.selected = tt.start
			for _, k := range tt.keys {
				m = send(m, k)
			}
			if m.selected != tt.want {
				t.Errorf("selected = %d, want %d", m.selected, tt.want)
			}
			if m.selected < m.offset || m.selected >= m.offset+body {
				t.Errorf("selection %d outside window [%d,%d)", m.selected, m.offset, m.offset+body)
			}
		})
	}
}

func TestMoveOnEmptyTable(t *testing.T) {
	t.Parallel()
	m := sized(newTestModel(&fakeOpener{}, false))
	for _, k := range []tea.KeyPressMsg{keyDown, keyUp, keyEnd, keyPgDown, keyHome, keyEnter} {
		m = send(m, k)
	}
	if m.selected != 0 {
		t.Errorf("selected = %d", m.selected)
	}
}

func TestSelectionKeptAcrossRefresh(t *testing.T) {
	t.Parallel()
	m := sized(newTestModel(&fakeOpener{}, false, named("a", "b", "c")...))
	m = send(m, keyDown) // on "b"
	m = send(m, rowsMsg{rows: named("x", "y", "a", "b", "c")})
	if row, _ := m.selectedRow(); row.Name != "b" || m.selected != 3 {
		t.Errorf("selected %d (%q), want index 3 \"b\"", m.selected, row.Name)
	}
}

func TestSelectionClampedWhenRowDisappears(t *testing.T) {
	t.Parallel()
	m := sized(newTestModel(&fakeOpener{}, false, named("a", "b", "c")...))
	m = send(m, keyEnd) // on "c"
	m = send(m, rowsMsg{rows: named("a", "b")})
	if m.selected != 1 {
		t.Errorf("selected = %d, want 1", m.selected)
	}
	m = send(m, rowsMsg{rows: nil})
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0 on empty", m.selected)
	}
}

func TestSelectionIdentityIncludesSource(t *testing.T) {
	t.Parallel()
	first := []source.Row{{Source: "pr", Name: "n"}, {Source: "vikunja", Name: "n"}}
	m := newTestModel(&fakeOpener{}, false, first...)
	m = send(m, keyDown)
	m = send(m, rowsMsg{rows: []source.Row{first[1], first[0]}})
	if row, _ := m.selectedRow(); row.Source != "vikunja" {
		t.Errorf("selected source %q, want vikunja", row.Source)
	}
}

func TestRefreshKey(t *testing.T) {
	t.Parallel()
	m := newTestModel(&fakeOpener{}, false, named("a")...)
	if m.refreshing {
		t.Fatal("model should be idle after rows arrive")
	}
	next, cmd := m.Update(keyRune('r'))
	m = next.(Model)
	if !m.refreshing || cmd == nil {
		t.Fatalf("refreshing=%v cmd nil=%v", m.refreshing, cmd == nil)
	}
	if msg, ok := cmd().(rowsMsg); !ok || len(msg.rows) != 1 {
		t.Errorf("fetch cmd returned %#v", msg)
	}
	if _, cmd = m.Update(keyRune('r')); cmd != nil {
		t.Error("r while refreshing must not start a second fetch")
	}
}

func TestTickRefreshesUnlessBusy(t *testing.T) {
	t.Parallel()
	m := newTestModel(&fakeOpener{}, false, named("a")...)
	next, cmd := m.Update(tickMsg{})
	if !next.(Model).refreshing || cmd == nil {
		t.Error("tick on idle model must start a refresh and re-arm the timer")
	}
}

func TestQuitKeys(t *testing.T) {
	t.Parallel()
	for _, k := range []tea.KeyPressMsg{keyRune('q'), keyCtrlC} {
		t.Run(k.String(), func(t *testing.T) {
			t.Parallel()
			_, cmd := newTestModel(&fakeOpener{}, false).Update(k)
			if cmd == nil {
				t.Fatal("no cmd")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("cmd returned %T, want tea.QuitMsg", cmd())
			}
		})
	}
}

func TestWindowResizeKeepsSelectionVisible(t *testing.T) {
	t.Parallel()
	m := sized(newTestModel(&fakeOpener{}, false, manyRows()...))
	m = send(m, keyEnd)
	m = send(m, tea.WindowSizeMsg{Width: testWidth, Height: chromeLines + 3})
	if m.width != testWidth || m.selected < m.offset || m.selected >= m.offset+m.bodyHeight() {
		t.Errorf("selected=%d offset=%d body=%d", m.selected, m.offset, m.bodyHeight())
	}
}
