package source

// Enrich lets the Kelos Task working on a Vikunja task or PR speak for it:
// that row takes the Task's state (its activity while it runs, else
// blocked, pending or waiting), its time, and stops needing you, since an
// agent is on it. A row no Task links to by ref is spoken for by the Task
// its start notice named (AgentTask). A finished Task changes nothing. When
// the Task source failed nothing is known, so every row keeps its own state.
func Enrich(rows []Row) []Row {
	if sourceFailed(rows, SourceKelosTask) {
		return rows
	}
	speakers := make(map[string]Row)
	named := make(map[string]Row)
	for _, r := range rows {
		if r.Source != SourceKelosTask || !taskInFlight(r) {
			continue
		}
		named[r.Name] = r
		for _, ref := range r.ParentRefs {
			if cur, ok := speakers[ref]; !ok || outranks(r, cur) {
				speakers[ref] = r
			}
		}
	}
	for i, r := range rows {
		if r.Source == SourceKelosTask {
			continue
		}
		speaker, ok := speakers[r.Ref]
		if !ok && r.AgentTask != "" {
			speaker, ok = named[r.AgentTask]
		}
		if !ok {
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
	return r.State != StateDone && r.State != StateFailed && r.State != stateCapped
}

// outranks prefers a running Task, then the most recent one.
func outranks(a, b Row) bool {
	if a.Busy != b.Busy {
		return a.Busy
	}
	return a.Since.After(b.Since)
}
