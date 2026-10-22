package source

// Enrich lets the Kelos Task working on a Vikunja task or PR speak for it:
// that row takes the Task's state (its activity while it runs, else
// blocked, pending or waiting), its time, and stops needing you, since an
// agent is on it. A finished Task changes nothing. When the Task source
// failed nothing is known, so every row keeps its own state.
func Enrich(rows []Row) []Row {
	if sourceFailed(rows, SourceKelosTask) {
		return rows
	}
	speakers := make(map[string]Row)
	for _, r := range rows {
		if r.Source != SourceKelosTask || !taskInFlight(r) {
			continue
		}
		for _, ref := range r.ParentRefs {
			if cur, ok := speakers[ref]; !ok || outranks(r, cur) {
				speakers[ref] = r
			}
		}
	}
	for i, r := range rows {
		speaker, ok := speakers[r.Ref]
		if !ok || r.Source == SourceKelosTask {
			continue
		}
		rows[i].State, rows[i].Busy, rows[i].NeedsYou, rows[i].Since = speaker.State, speaker.Busy, false, speaker.Since
	}
	return rows
}

func sourceFailed(rows []Row, source string) bool {
	for _, r := range rows {
		if r.Source == source && r.State == StateError {
			return true
		}
	}
	return false
}

// taskInFlight reports whether a Task row is still going to do something.
func taskInFlight(r Row) bool {
	return r.State != stateDone && r.State != StateFailed
}

// outranks prefers a running Task, then the most recent one.
func outranks(a, b Row) bool {
	if a.Busy != b.Busy {
		return a.Busy
	}
	return a.Since.After(b.Since)
}
