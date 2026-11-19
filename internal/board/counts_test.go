package board

import (
	"testing"

	"github.com/sperano/vigie/internal/source"
)

func TestCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rows []source.Row
		want string
	}{
		{"empty", nil, "0 need you · 0 busy · 0 idle"},
		{"mixed", []source.Row{
			{State: "review", NeedsYou: true},
			{Source: source.SourceLocal, State: "busy", Busy: true},
			{State: "idle"},
		}, "1 need you · 1 busy · 1 idle"},
		{"a task speaking for its PR and vikunja task is one busy agent", []source.Row{
			{Source: source.SourceKelosTask, State: "conflict fix", Busy: true},
			{Source: source.SourcePR, State: "conflict fix", Busy: true},
			{Source: source.SourceVikunja, State: "conflict fix", Busy: true},
		}, "0 need you · 1 busy · 0 idle"},
		{"a checks-running PR is not a busy agent", []source.Row{{Source: source.SourcePR, State: "checks running", Busy: true}},
			"0 need you · 0 busy · 0 idle"},
		{"needs-you idle counts both", []source.Row{{State: "idle", NeedsYou: true}}, "1 need you · 0 busy · 1 idle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := count(tt.rows).String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountsString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		counts Counts
		want   string
	}{
		{"no hidden rows leaves them out", Counts{NeedYou: 1, Busy: 2, Idle: 3}, "1 need you · 2 busy · 3 idle"},
		{"hidden rows are counted last", Counts{NeedYou: 1, Busy: 2, Idle: 3, Hidden: 4}, "1 need you · 2 busy · 3 idle · 4 hidden"},
		{"done rows are counted after hidden", Counts{NeedYou: 1, Busy: 2, Idle: 3, Hidden: 4, Done: 5},
			"1 need you · 2 busy · 3 idle · 4 hidden · 5 done"},
		{"done rows alone, with none hidden", Counts{NeedYou: 1, Busy: 2, Idle: 3, Done: 5}, "1 need you · 2 busy · 3 idle · 5 done"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.counts.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
