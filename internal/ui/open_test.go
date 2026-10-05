package ui

import "testing"

func TestURLCommand(t *testing.T) {
	t.Parallel()
	tests := []struct{ goos, want string }{
		{"darwin", "open"},
		{"linux", "xdg-open"},
		{"freebsd", "xdg-open"},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()
			if got := URLCommand(tt.goos); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunReportsFailure(t *testing.T) {
	t.Parallel()
	if err := run("true"); err != nil {
		t.Errorf("true: %v", err)
	}
	if err := run("false"); err == nil {
		t.Error("false: expected an error")
	}
}
