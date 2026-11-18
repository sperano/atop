package source

import "regexp"

// States of a Vikunja task whose newest comment is a Kelos notice that
// needs nothing of you.
const (
	StateAgentStarted = "agent started"
	StateAgentStopped = "agent stopped"
	stateMerged       = "merged"
	stateReady        = "ready"
	stateDiscuss      = "discuss"
)

// Notice kinds, from the hidden marker that ends every fixed Kelos notice
// (hollingsworth helm/kelos/README.md, "Gordon Cole's voice").
const (
	kindStart        = "start"
	kindRestart      = "restart"
	kindQueued       = "queued"
	kindBranchLocked = "branch-locked"
	kindChainStarted = "chain-started"
	kindStop         = "stop"
	kindMerged       = "merged"
	kindReady        = "ready"
	kindDiscuss      = "discuss"
	kindPROpen       = "pr-open"
)

// noticeRE matches a notice's kind marker, with the Kelos Task it names on
// start and restart notices.
var noticeRE = regexp.MustCompile(`<!-- kelos:([a-z][a-z-]*)(?: task=([A-Za-z0-9][A-Za-z0-9._-]*))? -->`)

// startedKinds say an agent was started, or will be, for the task.
var startedKinds = map[string]bool{
	kindStart: true, kindRestart: true, kindQueued: true, kindBranchLocked: true, kindChainStarted: true,
}

// quietKinds report something that leaves you nothing to do, by the state
// they show.
var quietKinds = map[string]string{
	kindStop:    StateAgentStopped,
	kindMerged:  stateMerged,
	kindReady:   stateReady,
	kindDiscuss: stateDiscuss,
	kindPROpen:  StatePROpen,
}

// notice is the kind marker of a Kelos notice.
type notice struct {
	kind string
	task string // the Kelos Task a start or restart names; may be empty
}

// parseNotice reads the kind marker of a comment; the marker goes last, so
// the last one wins when a notice quotes another.
func parseNotice(comment string) (notice, bool) {
	matches := noticeRE.FindAllStringSubmatch(comment, -1)
	if len(matches) == 0 {
		return notice{}, false
	}
	m := matches[len(matches)-1]
	return notice{kind: m[1], task: m[2]}, true
}

// applyNotice gives row the state of a Kelos notice that needs nothing of
// you: busy for an agent started, quiet for a report. It returns false,
// leaving row alone, for any other notice or a comment without a marker,
// which stay replies that need you, and for a stale start notice: no agent
// works that long, so it failed or vanished without a word.
func applyNotice(row *Row, comment string, stale bool) bool {
	n, ok := parseNotice(comment)
	if !ok {
		return false
	}
	if startedKinds[n.kind] {
		if stale {
			return false
		}
		row.State, row.Busy, row.AgentTask = StateAgentStarted, true, n.task
		if n.task != "" {
			row.Detail += " — " + n.task
		}
		return true
	}
	state, ok := quietKinds[n.kind]
	if ok {
		row.State = state
	}
	return ok
}
