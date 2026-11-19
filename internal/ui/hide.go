package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/sperano/vigie/internal/board"
	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

const (
	statusCannotHide = "a source error cannot be hidden"
	// statusShowsHidden stays in the status line while hidden rows are listed.
	statusShowsHidden = "showing hidden rows (marked " + board.MarkHidden + ")"
	statusHidesHidden = "hidden rows are hidden again"
	// statusHidesDone stays in the status line while done Kelos Task rows
	// are left out of the listing.
	statusHidesDone      = "hiding done tasks"
	statusShowsDoneAgain = "showing done tasks again"
)

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
	change := func(s hidden.Set) hidden.Set { return s.Hide(row) }
	m.status = "hid " + describe(row) + " · H shows hidden rows"
	if m.hidden.Hides(row) {
		change = func(s hidden.Set) hidden.Set { return s.Unhide(row) }
		m.status = "unhid " + describe(row)
	}
	m.updateHidden(change)
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

// toggleHideDone leaves done Kelos Task rows out of the listing, or lists
// them again.
func (m Model) toggleHideDone() (tea.Model, tea.Cmd) {
	m.hideDone = !m.hideDone
	m.status = statusShowsDoneAgain
	if m.hideDone {
		m.status = "" // the status line says so while they are hidden
	}
	m.arrange()
	return m, nil
}

// updateHidden applies change to the stored hidden rows and adopts the
// result. When the store fails, the change still holds for this run and
// the footer says why it was not saved.
func (m *Model) updateHidden(change func(hidden.Set) hidden.Set) {
	if m.opts.UpdateHidden == nil {
		m.hidden = change(m.hidden)
		return
	}
	set, err := m.opts.UpdateHidden(change)
	if err != nil {
		m.hidden = change(m.hidden)
		m.status = fmt.Sprintf("error: hidden rows: %v", err)
		return
	}
	m.hidden = set
}

func describe(r source.Row) string {
	return r.Source + " " + r.Name
}
