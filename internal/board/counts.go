package board

import (
	"fmt"

	"github.com/sperano/vigie/internal/source"
)

// countSep separates the header counts.
const countSep = " · "

// Counts are the header counts.
type Counts struct {
	NeedYou int
	Busy    int // agents, see IsAgent
	Idle    int
	Hidden  int
}

// String renders the counts, the hidden ones only when there are some.
func (c Counts) String() string {
	s := fmt.Sprintf("%d need you%s%d busy%s%d idle", c.NeedYou, countSep, c.Busy, countSep, c.Idle)
	if c.Hidden > 0 {
		s += fmt.Sprintf("%s%d hidden", countSep, c.Hidden)
	}
	return s
}

// agentSources are the sources whose rows are agents; a busy Vikunja task
// or PR (an agent speaks for it, its checks run, its agent started) is not
// an agent, and is not counted.
var agentSources = map[string]bool{
	source.SourceLocal:        true,
	source.SourceKelosSession: true,
	source.SourceKelosTask:    true,
}

// IsAgent reports whether r is an agent rather than a PR or task it works on.
func IsAgent(r source.Row) bool {
	return agentSources[r.Source]
}

// count counts the rows that need you, the busy agents, and the idle rows.
func count(rows []source.Row) Counts {
	var c Counts
	for _, r := range rows {
		switch {
		case r.NeedsYou:
			c.NeedYou++
		case r.Busy && IsAgent(r):
			c.Busy++
		}
		if r.State == source.StateIdle {
			c.Idle++
		}
	}
	return c
}
