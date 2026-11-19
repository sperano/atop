// Package board builds what every vigie front end shows from the fetched
// rows: the tree and its order, the marks, ages and enter targets, and the
// header counts. Front ends only lay it out.
package board

import (
	"time"

	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

// Marks drawn before a row; a row with neither has none.
const (
	MarkNeedsYou = "●"
	MarkHidden   = "-" // a hidden row listed so that it can be unhidden
)

// Entry is a row placed in the tree, with what is drawn for it.
type Entry struct {
	Row      source.Row
	Depth    int    // 0 for a top-level row
	Prefix   string // tree guides drawn before the name
	Hidden   bool   // listed only while hidden rows are shown
	Mark     string // MarkNeedsYou, MarkHidden or empty
	Origin   string // the row's origin, else source.OriginUnknown
	Age      string
	Target   Target
	Severity Severity
}

// Board is the rows in display order and the header counts.
type Board struct {
	Entries []Entry
	Counts  Counts
}

// Build arranges the rows that are not hidden or, with showHidden, every
// row, the hidden ones marked. A hidden row's children stay shown, at the
// top level when it is not. With hideDone, a done Kelos Task row is dropped
// the same way, its children reparented to the top level. The counts leave
// the hidden rows out either way, and count done rows regardless of
// hideDone. rows may be in any order.
func Build(rows []source.Row, set hidden.Set, showHidden bool, hideDone bool, now time.Time) Board {
	shown, hiddenCount := set.Split(rows)
	listed := shown
	if showHidden {
		listed = rows
	}
	if hideDone {
		listed = dropDone(listed)
	}
	entries := arrange(listed)
	for i := range entries {
		decorate(&entries[i], set, now)
	}
	counts := count(shown)
	counts.Hidden = hiddenCount
	counts.Done = countDone(shown)
	return Board{Entries: entries, Counts: counts}
}

// isDoneTask reports whether r is a Kelos Task row shown as done. A Task
// shows done for a grace window so the operator sees that it actually
// finished, rather than its row just vanishing; past that window (or now,
// with the hide-done toggle) its PR row carries the rest of the story.
func isDoneTask(r source.Row) bool {
	return r.Source == source.SourceKelosTask && r.State == source.StateDone
}

// dropDone leaves out the done Kelos Task rows; arrange reparents their
// children to the top level, as it already does for a hidden row.
func dropDone(rows []source.Row) []source.Row {
	listed := make([]source.Row, 0, len(rows))
	for _, r := range rows {
		if !isDoneTask(r) {
			listed = append(listed, r)
		}
	}
	return listed
}

// countDone counts the done Kelos Task rows among rows.
func countDone(rows []source.Row) int {
	var n int
	for _, r := range rows {
		if isDoneTask(r) {
			n++
		}
	}
	return n
}

// decorate fills in what is drawn for e's row.
func decorate(e *Entry, set hidden.Set, now time.Time) {
	r := e.Row
	e.Hidden = set.Hides(r)
	switch {
	case e.Hidden:
		e.Mark = MarkHidden
	case r.NeedsYou:
		e.Mark = MarkNeedsYou
	}
	e.Origin = r.Origin
	if e.Origin == "" {
		e.Origin = source.OriginUnknown
	}
	e.Age = FormatAge(r.Since, now)
	e.Target = TargetOf(r.Target)
	e.Severity = SeverityOf(*e)
}
