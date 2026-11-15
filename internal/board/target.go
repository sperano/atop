package board

import "github.com/sperano/vigie/internal/source"

// TargetKind says what enter does for a row.
type TargetKind string

// Target kinds.
const (
	TargetNone TargetKind = ""
	TargetURL  TargetKind = "url"
	TargetTmux TargetKind = "tmux"
)

// Target is what enter opens for a row: a URL, a tmux pane, or nothing.
type Target struct {
	Kind  TargetKind
	Value string // the URL or the pane; empty for TargetNone
}

// TargetOf picks a row's target: its URL, else its tmux pane.
func TargetOf(t source.Target) Target {
	switch {
	case t.URL != "":
		return Target{Kind: TargetURL, Value: t.URL}
	case t.TmuxPane != "":
		return Target{Kind: TargetTmux, Value: t.TmuxPane}
	default:
		return Target{}
	}
}
