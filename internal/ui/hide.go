package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/sperano/atop/internal/hidden"
	"github.com/sperano/atop/internal/source"
)

const (
	statusCannotHide = "a source error cannot be hidden"
	// statusShowsHidden stays in the status line while hidden rows are listed.
	statusShowsHidden = "showing hidden rows (marked " + hiddenMark + ")"
	statusHidesHidden = "hidden rows are hidden again"
)

// arrangeShown arranges the rows that are not hidden or, with showHidden,
// every row, the hidden ones marked. A hidden row's children stay shown,
// at the top level when it is not.
func arrangeShown(rows []source.Row, set hidden.Set, showHidden bool) []Entry {
	if !showHidden {
		rows, _ = set.Split(rows)
	}
	entries := Arrange(rows)
	for i := range entries {
		entries[i].Hidden = set.Hides(entries[i].Row)
	}
	return entries
}

// toggleHidden hides the selected row, or unhides it when hidden rows are
// shown. A row that disappears gives its place to the next one.
func (m Model) toggleHidden() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	if !hidden.Hideable(row) {
		m.status = statusCannotHide
		return m, nil
	}
	next, nowHidden := m.hidden.Toggle(row)
	m.status = "unhid " + describe(row)
	if nowHidden {
		m.status = "hid " + describe(row) + " · H shows hidden rows"
	}
	m.saveHidden(next)
	m.arrange()
	return m, nil
}

// toggleShowHidden lists the hidden rows, or stops listing them.
func (m Model) toggleShowHidden() (tea.Model, tea.Cmd) {
	m.showHidden = !m.showHidden
	m.status = statusHidesHidden
	if m.showHidden {
		m.status = "" // the status line says so while they are shown
	}
	m.arrange()
	return m, nil
}

// saveHidden adopts set and persists it. A failed save keeps set for this
// run and says so in the footer.
func (m *Model) saveHidden(set hidden.Set) {
	m.hidden = set
	if m.opts.SaveHidden == nil {
		return
	}
	if err := m.opts.SaveHidden(set); err != nil {
		m.status = fmt.Sprintf("error: saving hidden rows: %v", err)
	}
}

func describe(r source.Row) string {
	return r.Source + " " + r.Name
}
