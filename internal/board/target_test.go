package board

import (
	"testing"

	"github.com/sperano/vigie/internal/source"
)

func TestTargetOf(t *testing.T) {
	t.Parallel()
	const (
		url  = "https://github.com/o/r/pull/1"
		pane = "h:@5.%6"
	)
	tests := []struct {
		name   string
		target source.Target
		want   Target
	}{
		{"url", source.Target{URL: url}, Target{Kind: TargetURL, Value: url}},
		{"tmux pane", source.Target{TmuxPane: pane}, Target{Kind: TargetTmux, Value: pane}},
		{"url wins over a pane", source.Target{URL: url, TmuxPane: pane}, Target{Kind: TargetURL, Value: url}},
		{"nothing", source.Target{}, Target{Kind: TargetNone}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TargetOf(tt.target); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
