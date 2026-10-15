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

// Hide returns the set with r hidden.
func (s Set) Hide(r source.Row) Set {
	byKey := maps.Clone(s.byKey)
	if byKey == nil {
		byKey = make(map[string]Entry)
	}
	byKey[r.Key()] = Entry{Source: r.Source, Name: r.Name}
	return Set{byKey: byKey}
}

// Unhide returns the set with r shown again.
func (s Set) Unhide(r source.Row) Set {
	byKey := maps.Clone(s.byKey)
	delete(byKey, r.Key())
	return Set{byKey: byKey}
}

// Equal reports whether both sets hide the same rows.
func (s Set) Equal(other Set) bool {
	return maps.Equal(s.byKey, other.byKey)
}

// truncatedAt is the row count at which a source may have left rows out:
// the PR search returns its first PRSearchLimit hits only.
var truncatedAt = map[string]int{source.SourcePR: source.PRSearchLimit}

// Prune forgets the entries whose source was read in full and no longer
// lists them (a merged PR, a finished task), so that a later row reusing
// the name is not hidden. An entry whose source failed, or may have been
// cut short, is kept.
func (s Set) Prune(rows []source.Row) Set {
	unsure := make(map[string]bool)
	counts := make(map[string]int)
	present := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r.IsSourceError() {
			unsure[r.Source] = true
		}
		counts[r.Source]++
		present[r.Key()] = true
	}
	for src, limit := range truncatedAt {
		if counts[src] >= limit {
			unsure[src] = true
		}
	}
	kept := maps.Clone(s.byKey)
	maps.DeleteFunc(kept, func(key string, e Entry) bool {
		return !unsure[e.Source] && !present[key]
	})
	return Set{byKey: kept}
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
