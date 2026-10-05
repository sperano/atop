package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sperano/atop/internal/source"
)

const (
	appName    = "atop"
	timeLayout = "2006-01-02 15:04:05"
	titleSep   = " · "
	keyHelp    = "↑/↓ j/k move · pgup/pgdn home/end · enter open · r refresh · q quit"
)

// styles colours the table; the zero value renders plain text.
type styles struct {
	needsYou, busy, rest, bold, selected lipgloss.Style
}

func newStyles(color bool) styles {
	s := styles{selected: lipgloss.NewStyle().Reverse(true)}
	if !color {
		return s
	}
	s.needsYou = lipgloss.NewStyle().Foreground(lipgloss.Red)
	s.busy = lipgloss.NewStyle().Foreground(lipgloss.Green)
	s.rest = lipgloss.NewStyle().Faint(true)
	s.bold = lipgloss.NewStyle().Bold(true)
	return s
}

func (s styles) row(r source.Row) lipgloss.Style {
	switch {
	case r.NeedsYou:
		return s.needsYou
	case r.Busy:
		return s.busy
	default:
		return s.rest
	}
}

func titleLine(rows []source.Row, now time.Time) string {
	return appName + titleSep + now.Local().Format(timeLayout) + titleSep + Summary(rows)
}

// Snapshot renders the table once, for --once. rows may be in any order.
func Snapshot(rows []source.Row, now time.Time, width int, color bool) string {
	st := newStyles(color)
	lines := []string{
		st.bold.Render(titleLine(rows, now)),
		"",
		st.bold.Render(trimLine(headerLine(width))),
	}
	for _, e := range Arrange(rows) {
		lines = append(lines, st.row(e.Row).Render(trimLine(rowLine(e, now, width))))
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

// View renders the full-screen table.
func (m Model) View() tea.View {
	now := m.opts.Now()
	st := m.styles
	lines := []string{
		st.bold.Render(fit(titleLine(m.rows, now), m.width)),
		"",
		st.bold.Render(fit(headerLine(m.width), m.width)),
	}
	lines = append(lines, m.bodyLines(now)...)
	lines = append(lines, fit(m.statusLine(), m.width), st.rest.Render(fit(keyHelp, m.width)))
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

// bodyLines renders the visible rows, padded to the body height.
func (m Model) bodyLines(now time.Time) []string {
	body := m.bodyHeight()
	lines := make([]string, 0, body)
	if len(m.entries) == 0 && !m.refreshing {
		lines = append(lines, m.styles.rest.Render(statusNoRows))
	}
	end := min(len(m.entries), m.offset+body)
	for i := m.offset; i < end; i++ {
		e := m.entries[i]
		style := m.styles.row(e.Row)
		if i == m.selected {
			style = style.Inherit(m.styles.selected)
		}
		lines = append(lines, style.Render(fit(rowLine(e, now, m.width), m.width)))
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
