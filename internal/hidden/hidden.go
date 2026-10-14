// Package hidden remembers the rows the operator chose to hide, by row
// identity (source and name), across refreshes and restarts.
package hidden

import (
	"cmp"
	"maps"
	"slices"

	"github.com/sperano/atop/internal/source"
)

// Entry names a hidden row as it is stored in the file.
type Entry struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

func (e Entry) key() string {
	return source.Row{Source: e.Source, Name: e.Name}.Key()
}

// Set is the hidden rows. The zero value is empty; methods that change a
// Set return a new one and leave the receiver as it was.
type Set struct {
	byKey map[string]Entry
}

// New returns the set of the given entries.
func New(entries ...Entry) Set {
	byKey := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byKey[e.key()] = e
	}
	return Set{byKey: byKey}
}

// Hideable reports whether r may be hidden. A source's error row may not:
// hiding it would also hide the source's next, unrelated failure.
func Hideable(r source.Row) bool {
	return !r.IsSourceError()
}

// Hides reports whether r is hidden.
func (s Set) Hides(r source.Row) bool {
	_, ok := s.byKey[r.Key()]
	return ok && Hideable(r)
}

// Len is the number of hidden entries, shown or not.
func (s Set) Len() int {
	return len(s.byKey)
}

// Toggle hides r, or shows it again when it is hidden, and reports whether
// r is hidden afterwards.
func (s Set) Toggle(r source.Row) (Set, bool) {
	byKey := maps.Clone(s.byKey)
	if byKey == nil {
		byKey = make(map[string]Entry)
	}
	if _, ok := byKey[r.Key()]; ok {
		delete(byKey, r.Key())
		return Set{byKey: byKey}, false
	}
	byKey[r.Key()] = Entry{Source: r.Source, Name: r.Name}
	return Set{byKey: byKey}, true
}

// Prune forgets the entries whose source was read and no longer lists
// them (a merged PR, a finished task), so that a later row reusing the
// name is not hidden. An entry whose source failed is kept. It reports
// whether anything was dropped.
func (s Set) Prune(rows []source.Row) (Set, bool) {
	failed := make(map[string]bool)
	present := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r.IsSourceError() {
			failed[r.Source] = true
		}
		present[r.Key()] = true
	}
	kept := maps.Clone(s.byKey)
	maps.DeleteFunc(kept, func(key string, e Entry) bool {
		return !failed[e.Source] && !present[key]
	})
	return Set{byKey: kept}, len(kept) != len(s.byKey)
}

// Split returns the rows that are not hidden, in their order, and how many
// were hidden.
func (s Set) Split(rows []source.Row) ([]source.Row, int) {
	shown := make([]source.Row, 0, len(rows))
	for _, r := range rows {
		if !s.Hides(r) {
			shown = append(shown, r)
		}
	}
	return shown, len(rows) - len(shown)
}

// Entries lists the entries sorted by source, then name.
func (s Set) Entries() []Entry {
	return slices.SortedFunc(maps.Values(s.byKey), func(a, b Entry) int {
		return cmp.Or(cmp.Compare(a.Source, b.Source), cmp.Compare(a.Name, b.Name))
	})
}
