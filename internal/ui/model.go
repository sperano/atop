package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sperano/atop/internal/source"
)

// Fallback size until the first WindowSizeMsg arrives.
const (
	defaultWidth  = 120
	defaultHeight = 40
)

// Options wires the model to its data, clock and side effects.
type Options struct {
	Fetch    func(ctx context.Context) []source.Row // every source's rows, in any order
	Now      func() time.Time
	Opener   Opener
	InTmux   bool
	Interval time.Duration
	Color    bool
}

// Model is the Bubble Tea model of the interactive table.
type Model struct {
	opts       Options
	styles     styles
	rows       []source.Row // as fetched, for the summary
	entries    []Entry      // rows in display order
	selected   int
	offset     int // index of the first visible row
	width      int
	height     int
	refreshing bool
	status     string
}

type rowsMsg struct {
	rows []source.Row
}

type tickMsg struct{}

type openedMsg struct {
	what string
	err  error
}

// New returns a model whose first refresh starts in Init.
func New(opts Options) Model {
	return Model{
		opts:       opts,
		styles:     newStyles(opts.Color),
		width:      defaultWidth,
		height:     defaultHeight,
		refreshing: true,
	}
}

// Init starts the first refresh and the refresh timer.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), m.tickCmd())
}

func (m Model) fetchCmd() tea.Cmd {
	fetch := m.opts.Fetch
	return func() tea.Msg {
		return rowsMsg{rows: fetch(context.Background())}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.opts.Interval, func(time.Time) tea.Msg { return tickMsg{} })
}

// refresh starts a fetch unless one is already running.
func (m Model) refresh() (Model, tea.Cmd) {
	if m.refreshing {
		return m, nil
	}
	m.refreshing = true
	return m, m.fetchCmd()
}

// Update handles keys, resizes, refresh results and the timer.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scrollToSelection()
		return m, nil
	case rowsMsg:
		m.setRows(msg.rows)
		m.refreshing = false
		return m, nil
	case tickMsg:
		var cmd tea.Cmd
		m, cmd = m.refresh()
		return m, tea.Batch(cmd, m.tickCmd())
	case openedMsg:
		m.status = "opened " + msg.what
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
		}
		return m, nil
	}
	return m, nil
}

// setRows replaces the rows, keeping the selection on the same row
// (source and name) when it is still there.
func (m *Model) setRows(rows []source.Row) {
	var key string
	if row, ok := m.selectedRow(); ok {
		key = row.Key()
	}
	m.rows, m.entries = rows, Arrange(rows)
	m.selected = m.clamp(m.selected)
	for i, e := range m.entries {
		if e.Row.Key() == key {
			m.selected = i
			break
		}
	}
	m.scrollToSelection()
}

func (m Model) selectedRow() (source.Row, bool) {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return source.Row{}, false
	}
	return m.entries[m.selected].Row, true
}

func (m Model) clamp(i int) int {
	return max(0, min(i, len(m.entries)-1))
}

// bodyHeight is how many rows fit between the header and the footer.
func (m Model) bodyHeight() int {
	return max(1, m.height-chromeLines)
}

// scrollToSelection moves the window so that the selected row is visible.
func (m *Model) scrollToSelection() {
	body := m.bodyHeight()
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+body {
		m.offset = m.selected - body + 1
	}
	m.offset = max(0, min(m.offset, len(m.entries)-body))
}
