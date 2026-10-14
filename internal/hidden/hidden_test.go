package hidden

import (
	"errors"
	"slices"
	"strconv"
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

func TestHideUnhide(t *testing.T) {
	t.Parallel()
	var empty Set
	once := empty.Hide(prRow)
	if !once.Hides(prRow) || once.Len() != 1 {
		t.Fatalf("after Hide: Hides=%v Len=%d", once.Hides(prRow), once.Len())
	}
	if again := once.Hide(prRow); !again.Equal(once) {
		t.Error("hiding a hidden row changed the set")
	}
	if empty.Len() != 0 {
		t.Error("Hide changed its receiver")
	}
	back := once.Unhide(prRow)
	if back.Hides(prRow) || !back.Equal(empty) {
		t.Errorf("after Unhide: Hides=%v Len=%d", back.Hides(prRow), back.Len())
	}
	if !once.Hides(prRow) {
		t.Error("Unhide changed its receiver")
	}
	if got := empty.Unhide(prRow); got.Len() != 0 {
		t.Error("unhiding on an empty set")
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b Set
		want bool
	}{
		{"zero and empty", Set{}, New(), true},
		{"same entries", New(entryOf(prRow), entryOf(taskRow)), New(entryOf(taskRow), entryOf(prRow)), true},
		{"subset", New(entryOf(prRow)), New(entryOf(prRow), entryOf(taskRow)), false},
		{"different", New(entryOf(prRow)), New(entryOf(taskRow)), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("Equal = %v, want %v", got, tt.want)
			}
		})
	}
}

// fullPRPage is a PR search that hit its limit, without prRow.
func fullPRPage() []source.Row {
	rows := make([]source.Row, source.PRSearchLimit)
	for i := range rows {
		rows[i] = source.Row{Source: source.SourcePR, Name: "other#" + strconv.Itoa(i)}
	}
	return rows
}

func TestPrune(t *testing.T) {
	t.Parallel()
	set := New(entryOf(prRow), entryOf(taskRow))
	otherPR := source.Row{Source: source.SourcePR, Name: "atop#9"}
	tests := []struct {
		name     string
		rows     []source.Row
		wantKept []Entry
	}{
		{"both listed", []source.Row{prRow, taskRow}, []Entry{entryOf(prRow), entryOf(taskRow)}},
		{"PR merged", []source.Row{otherPR, taskRow}, []Entry{entryOf(taskRow)}},
		{"both gone", nil, []Entry{}},
		{"PR source failed", []source.Row{errorRow, taskRow}, []Entry{entryOf(prRow), entryOf(taskRow)}},
		{
			"a raw error row does not keep its source's entries",
			[]source.Row{{Source: source.SourcePR, Name: "atop#9", State: source.StateError}, taskRow},
			[]Entry{entryOf(taskRow)},
		},
		{"PR search cut short", append(fullPRPage(), taskRow), []Entry{entryOf(prRow), entryOf(taskRow)}},
		{"PR search one short of its limit", append(fullPRPage()[1:], taskRow), []Entry{entryOf(taskRow)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kept := set.Prune(tt.rows)
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
