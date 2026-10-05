package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sperano/atop/internal/source"
)

const (
	testURL  = "https://github.com/o/r/pull/1"
	testPane = "h:@5.%6"
)

func TestEnter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		row        source.Row
		inTmux     bool
		wantURLs   []string
		wantPanes  []string
		wantStatus string
		wantCmd    bool
	}{
		{"url row", source.Row{Source: "pr", Name: "a", Target: source.Target{URL: testURL}}, false,
			[]string{testURL}, nil, "opening " + testURL, true},
		{"local row outside tmux", source.Row{Source: "local claude", Name: "a", Target: source.Target{TmuxPane: testPane}}, false,
			nil, nil, statusNotInTmux, false},
		{"local row inside tmux", source.Row{Source: "local claude", Name: "a", Target: source.Target{TmuxPane: testPane}}, true,
			nil, []string{testPane}, "switching to " + testPane, true},
		{"error row opens nothing", source.ErrorRow("pr", errors.New("boom")), true,
			nil, nil, statusNothing, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			op := &fakeOpener{}
			m := newTestModel(op, tt.inTmux, tt.row)
			next, cmd := m.Update(keyEnter)
			m = next.(Model)
			if m.status != tt.wantStatus {
				t.Errorf("status = %q, want %q", m.status, tt.wantStatus)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Fatalf("cmd nil = %v, want cmd = %v", cmd == nil, tt.wantCmd)
			}
			if cmd != nil {
				if len(op.urls)+len(op.panes) != 0 {
					t.Error("opener called before the cmd ran")
				}
				if _, ok := cmd().(openedMsg); !ok {
					t.Error("cmd did not return an openedMsg")
				}
			}
			if strings.Join(op.urls, ",") != strings.Join(tt.wantURLs, ",") {
				t.Errorf("urls = %v, want %v", op.urls, tt.wantURLs)
			}
			if strings.Join(op.panes, ",") != strings.Join(tt.wantPanes, ",") {
				t.Errorf("panes = %v, want %v", op.panes, tt.wantPanes)
			}
		})
	}
}

func TestEnterOpensSelectedRow(t *testing.T) {
	t.Parallel()
	rows := []source.Row{
		{Source: "pr", Name: "a", Target: source.Target{URL: "https://one"}},
		{Source: "pr", Name: "b", Target: source.Target{URL: "https://two"}},
	}
	op := &fakeOpener{}
	m := send(newTestModel(op, false, rows...), keyDown)
	_, cmd := m.Update(keyEnter)
	cmd()
	if len(op.urls) != 1 || op.urls[0] != "https://two" {
		t.Errorf("urls = %v", op.urls)
	}
}

func TestOpenedMsgStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  openedMsg
		want string
	}{
		{"success", openedMsg{what: testURL}, "opened " + testURL},
		{"failure", openedMsg{what: testURL, err: errors.New("no display")}, "error: no display"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := send(newTestModel(&fakeOpener{}, false), tt.msg)
			if m.status != tt.want {
				t.Errorf("status = %q, want %q", m.status, tt.want)
			}
		})
	}
}

func TestInitReturnsCmd(t *testing.T) {
	t.Parallel()
	if New(Options{Now: fixedNow, Interval: 1}).Init() == nil {
		t.Error("Init must start the first refresh")
	}
	var _ tea.Model = Model{}
}
