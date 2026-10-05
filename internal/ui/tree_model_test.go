package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/sperano/atop/internal/source"
)

const (
	childURL = "https://example.test/child"
	otherURL = "https://example.test/other"
)

// nestedRows display as (other, #1, └─ child): equal ranks keep fetch order
// among roots, and the child moves under its parent, so display and fetch
// indexes differ.
func nestedRows() []source.Row {
	return []source.Row{
		{Source: "kelos task", Name: "child", ParentRefs: []string{taskRef}, Target: source.Target{URL: childURL}},
		{Source: "pr", Name: "other", Target: source.Target{URL: otherURL}},
		{Source: "vikunja", Name: "#1", Ref: taskRef},
	}
}

func TestNestedSelectionFollowsDisplayOrder(t *testing.T) {
	t.Parallel()
	op := &fakeOpener{}
	m := sized(newTestModel(op, false, nestedRows()...))
	m = send(m, keyEnd) // display index 2: child
	rows := nestedRows()
	slices.Reverse(rows)
	// Now displayed as (#1, └─ child, other).
	m = send(m, rowsMsg{rows: rows})
	if row, _ := m.selectedRow(); row.Name != "child" || m.selected != 1 {
		t.Fatalf("selected %d (%q), want index 1 \"child\"", m.selected, row.Name)
	}
	_, cmd := m.Update(keyEnter)
	if cmd == nil {
		t.Fatal("enter returned no command")
	}
	cmd()
	if !slices.Equal(op.urls, []string{childURL}) {
		t.Errorf("opened %q, want %q", op.urls, childURL)
	}
}

func TestSnapshotDrawsTree(t *testing.T) {
	t.Parallel()
	const deepRef = "pr:deep"
	long := strings.Repeat("n", nameWidth)
	rows := []source.Row{
		{Source: "vikunja", Name: "#1", Ref: taskRef},
		{Source: "pr", Name: "r#7", Ref: deepRef, ParentRefs: []string{taskRef}},
		{Source: "kelos task", Name: long, ParentRefs: []string{deepRef}},
	}
	lines := snapshotLines(t, rows, false)[3:]
	nameCol := func(line string) string {
		return strings.TrimRight(ansi.Cut(line, markWidth+sourceWidth+2*len(columnGap), markWidth+sourceWidth+2*len(columnGap)+nameWidth), " ")
	}
	want := []string{"#1", "└─ r#7", "   └─ " + string([]rune(long)[:nameWidth-len([]rune("   └─ "))-1]) + ellipsis}
	for i, w := range want {
		if got := nameCol(lines[i]); got != w {
			t.Errorf("line %d NAME = %q, want %q", i, got, w)
		}
	}
}
