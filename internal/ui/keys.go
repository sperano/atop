package ui

import (
	tea "charm.land/bubbletea/v2"
)

const (
	statusNotInTmux = "not in tmux"
	statusNothing   = "nothing to open"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.moveTo(m.selected - 1)
	case "down", "j":
		m.moveTo(m.selected + 1)
	case "pgup":
		m.moveTo(m.selected - m.bodyHeight())
	case "pgdown":
		m.moveTo(m.selected + m.bodyHeight())
	case "home":
		m.moveTo(0)
	case "end":
		m.moveTo(len(m.entries) - 1)
	case "r":
		return m.refresh()
	case "enter":
		return m.open()
	}
	return m, nil
}

func (m *Model) moveTo(i int) {
	m.selected = m.clamp(i)
	m.scrollToSelection()
}

// open starts opening the selected row's target.
func (m Model) open() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	opener, target := m.opts.Opener, row.Target
	switch {
	case target.URL != "":
		m.status = "opening " + target.URL
		return m, func() tea.Msg {
			return openedMsg{what: target.URL, err: opener.OpenURL(target.URL)}
		}
	case target.TmuxPane != "" && !m.opts.InTmux:
		m.status = statusNotInTmux
	case target.TmuxPane != "":
		m.status = "switching to " + target.TmuxPane
		return m, func() tea.Msg {
			return openedMsg{what: target.TmuxPane, err: opener.SwitchTmux(target.TmuxPane)}
		}
	default:
		m.status = statusNothing
	}
	return m, nil
}
