package board

import (
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestFormatAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		since time.Time
		want  string
	}{
		{"unknown", time.Time{}, "-"},
		{"zero", testNow, "0s"},
		{"future clamps to zero", testNow.Add(time.Hour), "0s"},
		{"59 seconds", testNow.Add(-59 * time.Second), "59s"},
		{"one minute", testNow.Add(-time.Minute), "1m"},
		{"90 minutes is 1h", testNow.Add(-90 * time.Minute), "1h"},
		{"23 hours", testNow.Add(-23 * time.Hour), "23h"},
		{"3d5h is 3d", testNow.Add(-(3*24 + 5) * time.Hour), "3d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatAge(tt.since, testNow); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
