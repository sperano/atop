package hidden

import (
	"errors"
	"slices"
	"testing"

	"github.com/sperano/atop/internal/source"
)

var (
	prRow      = source.Row{Source: source.SourcePR, Name: "atop#3", State: "review"}
	taskRow    = source.Row{Source: source.SourceVikunja, Name: "#874", State: "idle"}
	sessionRow = source.Row{Source: source.SourceLocal, Name: "atop", State: source.StateError}
	errorRow   = source.ErrorRow(source.SourcePR, errors.New("gh logged out"))
)

func entryOf(r source.Row) Entry {
	return Entry{Source: r.Source, Name: r.Name}
}

func TestHides(t *testing.T) {
	t.Parallel()
	set := New(entryOf(prRow), entryOf(sessionRow), entryOf(errorRow))
	tests := []struct {
		name string
		row  source.Row
		want bool
	}{
		{"hidden row", prRow, true},
		{"other row", taskRow, false},
		{"same name, other source", source.Row{Source: source.SourceVikunja, Name: prRow.Name}, false},
		{"raw error status is an ordinary row", sessionRow, true},
		{"source error is never hidden", errorRow, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := set.Hides(tt.row); got != tt.want {
				t.Errorf("Hides(%v) = %v, want %v", tt.row.Key(), got, tt.want)
			}
		})
	}
}

func TestToggle(t *testing.T) {
	t.Parallel()
	var empty Set
	once, hiddenAfter := empty.Toggle(prRow)
	if !hiddenAfter || !once.Hides(prRow) {
		t.Fatalf("first toggle: hidden=%v Hides=%v, want both true", hiddenAfter, once.Hides(prRow))
	}
	if empty.Hides(prRow) || empty.Len() != 0 {
		t.Error("Toggle changed its receiver")
	}
	twice, hiddenAfter := once.Toggle(prRow)
	if hiddenAfter || twice.Hides(prRow) || twice.Len() != 0 {
		t.Errorf("second toggle: hidden=%v Hides=%v Len=%d, want unhidden and empty", hiddenAfter, twice.Hides(prRow), twice.Len())
	}
	if !once.Hides(prRow) {
		t.Error("Toggle changed its receiver")
	}
}

func TestPrune(t *testing.T) {
	t.Parallel()
	set := New(entryOf(prRow), entryOf(taskRow))
	otherPR := source.Row{Source: source.SourcePR, Name: "atop#9"}
	tests := []struct {
		name        string
		rows        []source.Row
		wantKept    []Entry
		wantChanged bool
	}{
		{"both listed", []source.Row{prRow, taskRow}, []Entry{entryOf(prRow), entryOf(taskRow)}, false},
		{"PR merged", []source.Row{otherPR, taskRow}, []Entry{entryOf(taskRow)}, true},
		{"both gone", nil, []Entry{}, true},
		{"PR source failed", []source.Row{errorRow, taskRow}, []Entry{entryOf(prRow), entryOf(taskRow)}, false},
		{
			"a raw error row does not keep its source's entries",
			[]source.Row{{Source: source.SourcePR, Name: "atop#9", State: source.StateError}, taskRow},
			[]Entry{entryOf(taskRow)}, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kept, changed := set.Prune(tt.rows)
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if got := kept.Entries(); !slices.Equal(nonNil(got), tt.wantKept) {
				t.Errorf("kept %v, want %v", got, tt.wantKept)
			}
			if set.Len() != 2 {
				t.Error("Prune changed its receiver")
			}
		})
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()
	set := New(entryOf(prRow))
	shown, n := set.Split([]source.Row{taskRow, prRow, errorRow})
	if n != 1 || !slices.EqualFunc(shown, []source.Row{taskRow, errorRow}, func(a, b source.Row) bool { return a.Key() == b.Key() }) {
		t.Errorf("Split = %v, %d hidden; want task and error rows, 1 hidden", shown, n)
	}
}

func TestEntriesSorted(t *testing.T) {
	t.Parallel()
	b := Entry{Source: source.SourceVikunja, Name: "#1"}
	a2 := Entry{Source: source.SourcePR, Name: "b#1"}
	a1 := Entry{Source: source.SourcePR, Name: "a#1"}
	if got, want := New(b, a2, a1).Entries(), []Entry{a1, a2, b}; !slices.Equal(got, want) {
		t.Errorf("Entries() = %v, want %v", got, want)
	}
}
