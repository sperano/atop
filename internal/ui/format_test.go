package ui

import "testing"

func TestFitAndPad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		fn   func(string, int) string
		in   string
		w    int
		want string
	}{
		{"fit fits", fit, "abc", 3, "abc"},
		{"fit truncates with ellipsis", fit, "abcdef", 4, "abc…"},
		{"fit wide runes", fit, "日本語日本語", 5, "日本…"},
		{"padRight pads", padRight, "ab", 5, "ab   "},
		{"padRight truncates", padRight, "abcdefgh", 5, "abcd…"},
		{"padLeft pads", padLeft, "ab", 5, "   ab"},
		{"padLeft truncates", padLeft, "abcdefgh", 5, "abcd…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.fn(tt.in, tt.w); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
