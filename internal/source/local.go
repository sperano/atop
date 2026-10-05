package source

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LocalSessionsDir is where Claude Code writes one <pid>.json per running
// session, relative to the home directory. It is a Claude Code internal,
// not a documented interface.
const LocalSessionsDir = ".claude/sessions"

const (
	localStatusUnknown = "unknown"
	sessionFileGlob    = "*.json"
)

type localSession struct {
	PID             int    `json:"pid"`
	Name            string `json:"name"`
	Cwd             string `json:"cwd"`
	Tmux            string `json:"tmux"`
	Status          string `json:"status"`
	StatusUpdatedAt int64  `json:"statusUpdatedAt"` // Unix milliseconds
	UpdatedAt       int64  `json:"updatedAt"`
}

// LocalSessions reads the local Claude Code sessions.
type LocalSessions struct {
	Home  string
	Alive func(pid int) bool
}

// PIDAlive reports whether a process exists; EPERM means it does but
// belongs to someone else.
func PIDAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Fetch lists the sessions whose process is still alive. Files of exited
// sessions can linger, and a file being rewritten may not parse; both are
// skipped.
func (l LocalSessions) Fetch() ([]Row, error) {
	paths, err := filepath.Glob(filepath.Join(l.Home, LocalSessionsDir, sessionFileGlob))
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	var rows []Row
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s localSession
		if json.Unmarshal(data, &s) != nil || s.PID <= 0 || !l.Alive(s.PID) {
			continue
		}
		rows = append(rows, localRow(s, l.Home))
	}
	return rows, nil
}

func localRow(s localSession, home string) Row {
	status := s.Status
	if status == "" {
		status = localStatusUnknown
	}
	updated := s.StatusUpdatedAt
	if updated == 0 {
		updated = s.UpdatedAt
	}
	var since time.Time
	if updated != 0 {
		since = time.UnixMilli(updated).UTC()
	}
	name := s.Name
	if name == "" {
		name = strconv.Itoa(s.PID)
	}
	return Row{
		Source:   SourceLocal,
		Name:     name,
		State:    status,
		Since:    since,
		Detail:   localDetail(s, home),
		NeedsYou: status != StateIdle && status != StateBusy,
		Busy:     status == StateBusy,
		Target:   Target{TmuxPane: s.Tmux},
	}
}

func localDetail(s localSession, home string) string {
	cwd := s.Cwd
	if home != "" && (cwd == home || strings.HasPrefix(cwd, home+string(filepath.Separator))) {
		cwd = "~" + strings.TrimPrefix(cwd, home)
	}
	parts := []string{}
	if cwd != "" {
		parts = append(parts, cwd)
	}
	if s.Tmux != "" {
		parts = append(parts, "tmux "+s.Tmux)
	}
	return strings.Join(parts, " ")
}
