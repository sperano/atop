package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sperano/vigie/internal/board"
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
	case "h":
		return m.toggleHidden()
	case "H":
		return m.toggleShowHidden()
	case "d":
		return m.toggleHideDone()
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
	entry, ok := m.selectedEntry()
	if !ok {
		return m, nil
	}
	opener, target := m.opts.Opener, entry.Target
	switch {
	case target.Kind == board.TargetURL:
		m.status = "opening " + target.Value
		return m, func() tea.Msg {
			return openedMsg{what: target.Value, err: opener.OpenURL(target.Value)}
		}
	case target.Kind == board.TargetTmux && !m.opts.InTmux:
		m.status = statusNotInTmux
	case target.Kind == board.TargetTmux:
		m.status = "switching to " + target.Value
		return m, func() tea.Msg {
			return openedMsg{what: target.Value, err: opener.SwitchTmux(target.Value)}
		}
	default:
		m.status = statusNothing
	}
	return m, nil
}
