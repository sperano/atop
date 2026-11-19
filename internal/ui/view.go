package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sperano/vigie/internal/board"
	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

const (
	appName    = "vigie"
	timeLayout = "2006-01-02 15:04:05"
	titleSep   = " · "
	keyHelp    = "↑/↓ j/k move · pgup/pgdn home/end · enter open · h hide · H show hidden · d hide done · r refresh · q quit"
)

// styles colours the table; the zero value renders plain text.
type styles struct {
	attention, failed, blocked, busy, rest, bold, selected lipgloss.Style
}

func newStyles(color bool) styles {
	s := styles{selected: lipgloss.NewStyle().Reverse(true)}
	if !color {
		return s
	}
	s.failed = lipgloss.NewStyle().Foreground(lipgloss.Red)
	s.blocked = lipgloss.NewStyle().Foreground(lipgloss.Yellow)
	s.busy = lipgloss.NewStyle().Foreground(lipgloss.Green)
	s.rest = lipgloss.NewStyle().Faint(true)
	s.bold = lipgloss.NewStyle().Bold(true)
	return s
}

// entry styles an entry by its severity, apart from its attention mark.
func (s styles) entry(e board.Entry) lipgloss.Style {
	switch e.Severity {
	case board.SeverityFailed:
		return s.failed
	case board.SeverityBlocked:
		return s.blocked
	case board.SeverityBusy:
		return s.busy
	case board.SeverityAttention:
		return s.attention
	default:
		return s.rest
	}
}

// titleLine is the name, the time and the counts.
func titleLine(counts board.Counts, now time.Time) string {
	return appName + titleSep + now.Local().Format(timeLayout) + titleSep + counts.String()
}

// Snapshot renders the table once, for --once, leaving out the hidden rows.
// rows may be in any order.
func Snapshot(rows []source.Row, set hidden.Set, now time.Time, width int, color bool) string {
	st := newStyles(color)
	b := board.Build(rows, set, false, false, now)
	lines := []string{
		st.bold.Render(titleLine(b.Counts, now)),
		"",
		st.bold.Render(trimLine(headerLine(width))),
	}
	for _, e := range b.Entries {
		lines = append(lines, st.entry(e).Render(trimLine(rowLine(e, width))))
	}
	return strings.Join(lines, "\n") + "\n"
}

// chromeLines counts the TUI lines around the rows: title, blank line,
// column header, status line and key help.
const chromeLines = 5

const (
	statusRefreshing = "refreshing…"
	statusNoRows     = "no rows"
)

// View renders the full-screen table, its ages as of now.
func (m Model) View() tea.View {
	now := m.opts.Now()
	b := m.build(now)
	st := m.styles
	lines := []string{
		st.bold.Render(fit(titleLine(b.Counts, now), m.width)),
		"",
		st.bold.Render(fit(headerLine(m.width), m.width)),
	}
	lines = append(lines, m.bodyLines(b.Entries)...)
	lines = append(lines, fit(m.statusLine(), m.width), st.rest.Render(fit(keyHelp, m.width)))
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

// bodyLines renders the visible entries, padded to the body height. entries
// are those of the board View builds for the current time.
func (m Model) bodyLines(entries []board.Entry) []string {
	body := m.bodyHeight()
	lines := make([]string, 0, body)
	if len(entries) == 0 && !m.refreshing {
		lines = append(lines, m.styles.rest.Render(statusNoRows))
	}
	end := min(len(entries), m.offset+body)
	for i := m.offset; i < end; i++ {
		e := entries[i]
		style := m.styles.entry(e)
		if i == m.selected {
			style = style.Inherit(m.styles.selected)
		}
		lines = append(lines, style.Render(fit(rowLine(e, m.width), m.width)))
	}
	for len(lines) < body {
		lines = append(lines, "")
	}
	return lines
}

func (m Model) statusLine() string {
	var parts []string
	if m.refreshing {
		parts = append(parts, statusRefreshing)
	}
	if m.showHidden {
		parts = append(parts, statusShowsHidden)
	}
	if m.hideDone {
		parts = append(parts, statusHidesDone)
	}
	if m.status != "" {
		parts = append(parts, m.status)
	}
	return strings.Join(parts, titleSep)
}

// trimLine drops the padding after DETAIL, which only matters in the TUI
// where the selected row is highlighted across the full width.
func trimLine(s string) string {
	return strings.TrimRight(s, " ")
}
