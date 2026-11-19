package server

import (
	"time"

	"github.com/sperano/vigie/internal/board"
	"github.com/sperano/vigie/internal/hidden"
)

// Snapshot is what GET /api/v1/snapshot returns and every event carries.
type Snapshot struct {
	FetchedAt  *time.Time `json:"fetched_at"` // null until the first refresh ends
	Refreshing bool       `json:"refreshing"`
	Error      string     `json:"error"` // the last hidden-row update's error
	Counts     Counts     `json:"counts"`
	Rows       []Row      `json:"rows"` // nested and sorted, as they are drawn
}

// Counts are the header counts; hidden rows are left out of the others.
type Counts struct {
	NeedYou int `json:"need_you"`
	Busy    int `json:"busy"`
	Idle    int `json:"idle"`
	Hidden  int `json:"hidden"`
	Done    int `json:"done"`
}

// Row is a row placed in the tree, with what is drawn for it.
type Row struct {
	Source   string         `json:"source"`
	Name     string         `json:"name"`
	Origin   string         `json:"origin"`
	Depth    int            `json:"depth"`
	Guides   string         `json:"guides"` // tree guides drawn before the name
	Mark     string         `json:"mark"`
	State    string         `json:"state"`
	Since    *time.Time     `json:"since"` // null when unknown
	Age      string         `json:"age"`
	Detail   string         `json:"detail"`
	Target   string         `json:"target"` // the URL enter opens, empty when none
	Severity board.Severity `json:"severity"`
	NeedsYou bool           `json:"needs_you"`
	Busy     bool           `json:"busy"`
	Hidden   bool           `json:"hidden"`
	Hideable bool           `json:"hideable"` // false for a source's error row
}

// snapshot builds the current snapshot, its ages as of now. showHidden lists
// the hidden rows too, marked; hideDone leaves done Kelos Task rows out.
func (s *Server) snapshot(showHidden, hideDone bool) Snapshot {
	s.mu.Lock()
	rows, set, fetchedAt := s.rows, s.hidden, s.fetchedAt
	snap := Snapshot{Refreshing: s.running != nil, Error: s.lastErr}
	s.mu.Unlock()
	b := board.Build(rows, set, showHidden, hideDone, s.opts.Now())
	snap.FetchedAt = optionalTime(fetchedAt)
	snap.Counts = Counts{NeedYou: b.Counts.NeedYou, Busy: b.Counts.Busy, Idle: b.Counts.Idle, Hidden: b.Counts.Hidden, Done: b.Counts.Done}
	snap.Rows = make([]Row, 0, len(b.Entries))
	for _, e := range b.Entries {
		snap.Rows = append(snap.Rows, newRow(e))
	}
	return snap
}

func newRow(e board.Entry) Row {
	r := e.Row
	row := Row{
		Source: r.Source, Name: r.Name, Origin: e.Origin,
		Depth: e.Depth, Guides: e.Prefix, Mark: e.Mark,
		State: r.State, Since: optionalTime(r.Since), Age: e.Age, Detail: r.Detail,
		Severity: e.Severity, NeedsYou: r.NeedsYou, Busy: r.Busy, Hidden: e.Hidden,
		Hideable: hidden.Hideable(r),
	}
	if e.Target.Kind == board.TargetURL {
		row.Target = e.Target.Value
	}
	return row
}

// optionalTime is nil for the zero time, which JSON shows as null.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
