package board

import (
	"errors"
	"testing"

	"github.com/sperano/vigie/internal/source"
)

func TestSeverityOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		row    source.Row
		hidden bool
		want   Severity
	}{
		{"needs review is attention", source.Row{Source: source.SourcePR, State: "awaiting your review", NeedsYou: true}, false, SeverityAttention},
		{"checks failing", source.Row{Source: source.SourcePR, State: source.StateChecksFailing}, false, SeverityFailed},
		{"failed task", source.Row{Source: source.SourceKelosTask, State: source.StateFailed, NeedsYou: true}, false, SeverityFailed},
		{"failure", source.Row{Source: source.SourceKelosTask, State: source.StateFailure}, false, SeverityFailed},
		{"source error", source.ErrorRow(source.SourcePR, errors.New("down")), false, SeverityFailed},
		{"blocked", source.Row{Source: source.SourceKelosTask, State: source.StateBlocked}, false, SeverityBlocked},
		{"conflicts", source.Row{Source: source.SourcePR, State: source.StateConflicts}, false, SeverityBlocked},
		{"active agent", source.Row{Source: source.SourceLocal, State: source.StateBusy, Busy: true, NeedsYou: true}, false, SeverityBusy},
		{"busy PR stays quiet", source.Row{Source: source.SourcePR, State: "checks running", Busy: true}, false, SeverityQuiet},
		{"PR open stays quiet", source.Row{Source: source.SourceVikunja, State: "PR open"}, false, SeverityQuiet},
		{"hidden failure is quiet", source.Row{Source: source.SourcePR, State: source.StateChecksFailing, NeedsYou: true}, true, SeverityQuiet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := SeverityOf(Entry{Row: tt.row, Hidden: tt.hidden}); got != tt.want {
				t.Errorf("SeverityOf = %q, want %q", got, tt.want)
			}
		})
	}
}
