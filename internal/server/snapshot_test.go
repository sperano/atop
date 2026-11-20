package server

import (
	"testing"

	"github.com/sperano/vigie/internal/board"
	"github.com/sperano/vigie/internal/source"
)

func TestNewRowLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		row       source.Row
		wantName  string
		wantLabel string
	}{
		{"no label shows the name", source.Row{Source: source.SourcePR, Name: "vigie#3"}, "vigie#3", "vigie#3"},
		{"a label is shown, the name kept", source.Row{Source: source.SourceVikunja, Name: "#1647", Label: "#1647 - Hollingsworth #577"},
			"#1647", "#1647 - Hollingsworth #577"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newRow(board.Entry{Row: tt.row})
			if got.Name != tt.wantName || got.Label != tt.wantLabel {
				t.Errorf("name, label = %q, %q, want %q, %q", got.Name, got.Label, tt.wantName, tt.wantLabel)
			}
		})
	}
}
