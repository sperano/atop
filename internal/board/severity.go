package board

import "github.com/sperano/vigie/internal/source"

// Severity is how a row is coloured, apart from its needs-you mark.
type Severity string

// Severities, in the order SeverityOf tests them: a busy agent ranks above
// a row that only needs you.
const (
	SeverityFailed    Severity = "failed"    // red: a failure or a source error
	SeverityBlocked   Severity = "blocked"   // yellow: blocked work, conflicts
	SeverityBusy      Severity = "busy"      // green: any busy row (an agent, a row one speaks for, running checks, an agent started)
	SeverityAttention Severity = "attention" // plain: needs you, nothing worse
	SeverityQuiet     Severity = "quiet"     // dim: everything else, hidden rows
)

// SeverityOf classifies an entry; a hidden one is quiet whatever its state.
func SeverityOf(e Entry) Severity {
	r := e.Row
	switch {
	case e.Hidden:
		return SeverityQuiet
	case failedState(r):
		return SeverityFailed
	case blockedState(r):
		return SeverityBlocked
	case r.Busy:
		return SeverityBusy
	case r.NeedsYou:
		return SeverityAttention
	default:
		return SeverityQuiet
	}
}

func failedState(r source.Row) bool {
	switch r.State {
	case source.StateFailed, source.StateFailure, source.StateError, source.StateChecksFailing:
		return true
	default:
		return r.IsSourceError()
	}
}

func blockedState(r source.Row) bool {
	return r.State == source.StateBlocked || r.State == source.StateConflicts
}
