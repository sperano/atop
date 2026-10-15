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

// fakeStore is a hidden-rows file: it holds a set, counts the writes that
// change it, and fails every update with err.
type fakeStore struct {
	set    hidden.Set
	writes int
	err    error
}

func (f *fakeStore) update(change func(hidden.Set) hidden.Set) (hidden.Set, error) {
	if f.err != nil {
		return hidden.Set{}, f.err
	}
	next := change(f.set)
	if !next.Equal(f.set) {
		f.writes++
	}
	f.set = next
	return next, nil
}

func newHidingModel(store *fakeStore, rows ...source.Row) Model {
	opts := Options{
		Fetch:    func(context.Context) []source.Row { return rows },
		Now:      fixedNow,
		Opener:   &fakeOpener{},
		Interval: time.Minute,
	}
	if store != nil {
		opts.UpdateHidden = store.update
	}
	return sized(send(New(opts), rowsMsg{rows: rows}))
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
	m := newHidingModel(store, named("a", "b", "c")...)
	m = send(m, keyDown) // on "b"
	m = send(m, keyHide)
	if got := entryNames(m); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("entries = %v, want [a c]", got)
	}
	if row, _ := m.selectedRow(); row.Name != "c" {
		t.Errorf("selected %q, want the next row c", row.Name)
	}
	if got := store.set.Entries(); !slices.Equal(got, []hidden.Entry{prEntry("b")}) {
		t.Errorf("stored %v, want [b]", got)
	}
	if !strings.HasPrefix(m.status, "hid pr b") {
		t.Errorf("status = %q", m.status)
	}
}

func TestHideOutcomes(t *testing.T) {
	t.Parallel()
	sourceError := source.ErrorRow("pr", errors.New("gh logged out"))
	tests := []struct {
		name         string
		store        *fakeStore // nil: no store
		rows         []source.Row
		end          bool // select the last row first
		wantNames    []string
		wantSelected string
		wantStatus   string // prefix
	}{
		{"last row hidden selects the new last", &fakeStore{}, named("a", "b"), true, []string{"a"}, "a", "hid pr b"},
		{"source error refused", &fakeStore{}, []source.Row{sourceError}, false, []string{"-"}, "-", statusCannotHide},
		{"store failure hides for this run", &fakeStore{err: errors.New("read-only")}, named("a", "b"), false, []string{"b"}, "b", "error: hidden rows: read-only"},
		{"no store hides in memory", nil, named("a", "b"), false, []string{"b"}, "b", "hid pr a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newHidingModel(tt.store, tt.rows...)
			if tt.end {
				m = send(m, keyEnd)
			}
			m = send(m, keyHide)
			if got := entryNames(m); !slices.Equal(got, tt.wantNames) {
				t.Errorf("entries = %v, want %v", got, tt.wantNames)
			}
			if row, _ := m.selectedRow(); row.Name != tt.wantSelected {
				t.Errorf("selected %q, want %q", row.Name, tt.wantSelected)
			}
			if !strings.HasPrefix(m.status, tt.wantStatus) {
				t.Errorf("status = %q, want prefix %q", m.status, tt.wantStatus)
			}
		})
	}
}

func TestShowHiddenAndUnhide(t *testing.T) {
	t.Parallel()
	store := &fakeStore{set: hidden.New(prEntry("b"))}
	m := newHidingModel(store, named("a", "b")...)
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
	if store.writes != 1 || store.set.Len() != 0 || m.entries[1].Hidden {
		t.Errorf("unhide: writes=%d stored=%v Hidden=%v", store.writes, store.set.Entries(), m.entries[1].Hidden)
	}
	if row, _ := m.selectedRow(); row.Name != "b" {
		t.Errorf("selection moved to %q after unhiding", row.Name)
	}
	m = send(m, keyShowHidden)
	if m.status != statusHidesHidden || strings.Contains(m.statusLine(), statusShowsHidden) {
		t.Errorf("after second H: status line %q", m.statusLine())
	}
}

func TestRefreshSyncsHidden(t *testing.T) {
	t.Parallel()
	store := &fakeStore{set: hidden.New(prEntry("b"), prEntry("gone"))}
	m := newHidingModel(store, named("a", "b")...)
	if got := store.set.Entries(); !slices.Equal(got, []hidden.Entry{prEntry("b")}) || store.writes != 1 {
		t.Fatalf("after first rows, stored %v in %d writes, want [b] in 1", got, store.writes)
	}
	m = send(m, rowsMsg{rows: named("a", "b", "c")})
	if got := entryNames(m); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("entries = %v, want [a c]", got)
	}
	if store.writes != 1 {
		t.Errorf("%d writes, want 1: nothing changed on the second refresh", store.writes)
	}
	store.set = store.set.Hide(named("c")[0]) // another atop hides c
	m = send(m, rowsMsg{rows: named("a", "b", "c")})
	if got := entryNames(m); !slices.Equal(got, []string{"a"}) {
		t.Errorf("after c was hidden elsewhere, entries = %v, want [a]", got)
	}
	m = send(m, rowsMsg{rows: []source.Row{source.ErrorRow("pr", errors.New("down"))}})
	if store.set.Len() != 2 || !m.hidden.Hides(named("b")[0]) {
		t.Error("a failed source must not forget its hidden rows")
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
