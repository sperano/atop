package ui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sperano/atop/internal/hidden"
	"github.com/sperano/atop/internal/source"
)

var (
	keyHide       = keyRune('h')
	keyShowHidden = keyRune('H')
)

// fakeStore records every saved set and fails with err.
type fakeStore struct {
	saved []hidden.Set
	err   error
}

func (f *fakeStore) save(s hidden.Set) error {
	f.saved = append(f.saved, s)
	return f.err
}

func (f *fakeStore) last() []hidden.Entry {
	if len(f.saved) == 0 {
		return nil
	}
	return f.saved[len(f.saved)-1].Entries()
}

func newHidingModel(store *fakeStore, set hidden.Set, rows ...source.Row) Model {
	m := New(Options{
		Fetch:      func(context.Context) []source.Row { return rows },
		Now:        fixedNow,
		Opener:     &fakeOpener{},
		Interval:   time.Minute,
		Hidden:     set,
		SaveHidden: store.save,
	})
	return sized(send(m, rowsMsg{rows: rows}))
}

func entryNames(m Model) []string {
	names := make([]string, len(m.entries))
	for i, e := range m.entries {
		names[i] = e.Row.Name
	}
	return names
}

func prEntry(name string) hidden.Entry {
	return hidden.Entry{Source: "pr", Name: name}
}

func TestHideKey(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	m := newHidingModel(store, hidden.Set{}, named("a", "b", "c")...)
	m = send(m, keyDown) // on "b"
	m = send(m, keyHide)
	if got := entryNames(m); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("entries = %v, want [a c]", got)
	}
	if row, _ := m.selectedRow(); row.Name != "c" {
		t.Errorf("selected %q, want the next row c", row.Name)
	}
	if got := store.last(); !slices.Equal(got, []hidden.Entry{prEntry("b")}) {
		t.Errorf("saved %v, want [b]", got)
	}
	if !strings.HasPrefix(m.status, "hid pr b") {
		t.Errorf("status = %q", m.status)
	}
}

func TestHideLastRowSelectsNewLast(t *testing.T) {
	t.Parallel()
	m := newHidingModel(&fakeStore{}, hidden.Set{}, named("a", "b")...)
	m = send(m, keyEnd)
	m = send(m, keyHide)
	if row, _ := m.selectedRow(); row.Name != "a" {
		t.Errorf("selected %q, want a", row.Name)
	}
}

func TestHideRefusesSourceError(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	m := newHidingModel(store, hidden.Set{}, source.ErrorRow("pr", errors.New("gh logged out")))
	m = send(m, keyHide)
	if len(m.entries) != 1 || len(store.saved) != 0 || m.status != statusCannotHide {
		t.Errorf("entries=%d saved=%d status=%q", len(m.entries), len(store.saved), m.status)
	}
}

func TestShowHiddenAndUnhide(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	m := newHidingModel(store, hidden.New(prEntry("b")), named("a", "b")...)
	if got := entryNames(m); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("entries = %v, want [a]", got)
	}
	m = send(m, keyShowHidden)
	if got := entryNames(m); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("with hidden shown, entries = %v, want [a b]", got)
	}
	if m.entries[0].Hidden || !m.entries[1].Hidden {
		t.Errorf("Hidden flags = %v, %v; want false, true", m.entries[0].Hidden, m.entries[1].Hidden)
	}
	if !strings.Contains(m.statusLine(), statusShowsHidden) {
		t.Errorf("status line %q does not say hidden rows are shown", m.statusLine())
	}
	m = send(m, keyEnd)
	m = send(m, keyHide)
	if len(store.saved) != 1 || len(store.last()) != 0 || m.entries[1].Hidden {
		t.Errorf("unhide: saved=%v Hidden=%v", store.last(), m.entries[1].Hidden)
	}
	if row, _ := m.selectedRow(); row.Name != "b" {
		t.Errorf("selection moved to %q after unhiding", row.Name)
	}
	m = send(m, keyShowHidden)
	if m.status != statusHidesHidden || strings.Contains(m.statusLine(), statusShowsHidden) {
		t.Errorf("after second H: status line %q", m.statusLine())
	}
}

func TestRefreshKeepsHiddenAndPrunesGone(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	m := newHidingModel(store, hidden.New(prEntry("b"), prEntry("gone")), named("a", "b")...)
	if got := store.last(); !slices.Equal(got, []hidden.Entry{prEntry("b")}) {
		t.Fatalf("after first rows, saved %v, want [b]", got)
	}
	m = send(m, rowsMsg{rows: named("a", "b", "c")})
	if got := entryNames(m); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("entries = %v, want [a c]", got)
	}
	if len(store.saved) != 1 {
		t.Errorf("saved %d times, want once: nothing changed on the second refresh", len(store.saved))
	}
	m = send(m, rowsMsg{rows: []source.Row{source.ErrorRow("pr", errors.New("down"))}})
	if len(store.saved) != 1 || !m.hidden.Hides(named("b")[0]) {
		t.Error("a failed source must not forget its hidden rows")
	}
}

func TestSaveErrorShown(t *testing.T) {
	t.Parallel()
	m := newHidingModel(&fakeStore{err: errors.New("read-only")}, hidden.Set{}, named("a", "b")...)
	m = send(m, keyHide)
	if !strings.Contains(m.status, "read-only") {
		t.Errorf("status = %q, want the save error", m.status)
	}
	if got := entryNames(m); !slices.Equal(got, []string{"b"}) {
		t.Errorf("entries = %v, want the row hidden for this run anyway", got)
	}
}

func TestHiddenParentLeavesChildAtTop(t *testing.T) {
	t.Parallel()
	parent := source.Row{Source: "vikunja", Name: "#7", State: "idle", Ref: "vikunja:7"}
	child := source.Row{Source: "pr", Name: "atop#3", State: "review", ParentRefs: []string{"vikunja:7"}}
	set := hidden.New(hidden.Entry{Source: "vikunja", Name: "#7"})
	entries := arrangeShown([]source.Row{parent, child}, set, false)
	if len(entries) != 1 || entries[0].Row.Name != "atop#3" || entries[0].Prefix != "" {
		t.Errorf("entries = %+v, want the PR alone at the top level", entries)
	}
	entries = arrangeShown([]source.Row{parent, child}, set, true)
	if len(entries) != 2 || entries[1].Prefix != branchLast || !entries[0].Hidden {
		t.Errorf("with hidden shown, entries = %+v, want the PR nested under the hidden task", entries)
	}
}

func TestTitleCountsHidden(t *testing.T) {
	t.Parallel()
	rows := []source.Row{
		{Source: "pr", Name: "a", State: "review", NeedsYou: true},
		{Source: "pr", Name: "b", State: "review", NeedsYou: true},
	}
	tests := []struct {
		name string
		set  hidden.Set
		want string
	}{
		{"nothing hidden", hidden.Set{}, "2 need you · 0 busy · 0 idle"},
		{"one hidden", hidden.New(prEntry("b")), "1 need you · 0 busy · 0 idle · 1 hidden"},
		{"hidden entry not listed", hidden.New(prEntry("gone")), "2 need you · 0 busy · 0 idle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := titleLine(rows, tt.set, testNow); !strings.HasSuffix(got, tt.want) {
				t.Errorf("title = %q, want suffix %q", got, tt.want)
			}
		})
	}
}

func TestSnapshotLeavesOutHidden(t *testing.T) {
	t.Parallel()
	out := Snapshot(named("keep", "drop"), hidden.New(prEntry("drop")), testNow, testWidth, false)
	if !strings.Contains(out, "keep") || strings.Contains(out, "drop") {
		t.Errorf("snapshot:\n%s", out)
	}
}

func TestHiddenRowMarkedInView(t *testing.T) {
	t.Parallel()
	row := source.Row{Source: "pr", Name: "a", State: "review", NeedsYou: true}
	tests := []struct {
		name   string
		hidden bool
		want   string
	}{
		{"needs you", false, needsYouMark + " pr"},
		{"hidden", true, hiddenMark + " pr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := rowLine(Entry{Row: row, Hidden: tt.hidden}, testNow, testWidth); !strings.HasPrefix(got, tt.want) {
				t.Errorf("row line %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestHideWithoutStore(t *testing.T) {
	t.Parallel()
	m := sized(newTestModel(&fakeOpener{}, false, named("a", "b")...))
	m = send(m, keyHide)
	if got := entryNames(m); !slices.Equal(got, []string{"b"}) {
		t.Errorf("entries = %v, want [b]", got)
	}
}
