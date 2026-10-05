// Package source fetches the agent signals atop shows, one row per agent,
// PR or task. Parsing is kept apart from fetching so that every state
// mapping can be tested on plain JSON.
package source

import (
	"html"
	"regexp"
	"strings"
	"time"
)

// Source names, shown in the SOURCE column and used in row identity.
const (
	SourceLocal        = "local claude"
	SourceKelosSession = "kelos session"
	SourceKelosTask    = "kelos task"
	SourcePR           = "pr"
	SourceVikunja      = "vikunja"
)

// Shared state names.
const (
	StateStale = "stale"
	StateError = "error"
	StateIdle  = "idle"
	StateBusy  = "busy"
)

const (
	hoursPerDay = 24
	// snippetRunes caps the comment excerpt shown in a Vikunja row.
	snippetRunes = 100
	// zeroTimePrefix marks Go's zero time as Kubernetes serialises it.
	zeroTimePrefix = "0001-"
)

// Target is what enter opens for a row: a URL, a tmux pane, or nothing.
type Target struct {
	URL      string
	TmuxPane string
}

// Row is one line of the table.
type Row struct {
	Source   string
	Name     string
	State    string
	Since    time.Time // zero when unknown
	Detail   string
	NeedsYou bool
	Busy     bool
	Target   Target
}

// Key identifies a row across refreshes.
func (r Row) Key() string {
	return r.Source + "\x00" + r.Name
}

// ErrorRow reports a source that could not be read.
func ErrorRow(source string, err error) Row {
	return Row{Source: source, Name: "-", State: StateError, Detail: err.Error(), NeedsYou: true}
}

// Staleness decides when a needs-you row has been untouched long enough to
// stop flagging it.
type Staleness struct {
	Days int
	Now  time.Time
}

// IsStale reports whether since is older than the staleness window.
func (s Staleness) IsStale(since time.Time) bool {
	window := time.Duration(s.Days) * hoursPerDay * time.Hour
	return !since.IsZero() && s.Now.Sub(since) > window
}

// parseTime reads an RFC 3339 timestamp, returning the zero time for an
// empty, unparsable or zero value.
func parseTime(value string) time.Time {
	if value == "" || strings.HasPrefix(value, zeroTimePrefix) {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

var (
	tagRE   = regexp.MustCompile(`<[^>]+>`)
	spaceRE = regexp.MustCompile(`\s+`)
)

// snippet turns an HTML comment into one line of plain text.
func snippet(text string) string {
	plain := html.UnescapeString(tagRE.ReplaceAllString(text, " "))
	plain = strings.TrimSpace(spaceRE.ReplaceAllString(plain, " "))
	if runes := []rune(plain); len(runes) > snippetRunes {
		return string(runes[:snippetRunes])
	}
	return plain
}
