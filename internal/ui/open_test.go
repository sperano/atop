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

func TestCheckURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw    string
		wantOK bool
	}{
		{"https://github.com/sperano/atop/pull/1", true},
		{"http://localhost:8080/tasks/1", true},
		{"file:///etc/passwd", false},
		{"-a Calculator", false},
		{"slack://open", false},
		{"https://", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			if err := checkURL(tt.raw); (err == nil) != tt.wantOK {
				t.Errorf("checkURL(%q) = %v, want ok=%v", tt.raw, err, tt.wantOK)
			}
		})
	}
}

func TestOpenURLRefusesBadURLWithoutRunning(t *testing.T) {
	t.Parallel()
	if err := (ExecOpener{}).OpenURL("file:///etc/passwd"); err == nil {
		t.Error("expected a refusal")
	}
}
