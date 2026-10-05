package ui

import (
	"testing"
	"time"

	"github.com/sperano/atop/internal/source"
)

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

func TestSortRows(t *testing.T) {
	t.Parallel()
	old, fresh := testNow.Add(-time.Hour), testNow
	rows := []source.Row{
		{Name: "idle", State: "idle", Since: fresh},
		{Name: "busy", State: "busy", Since: fresh, Busy: true},
		{Name: "needs-old", State: "review", Since: old, NeedsYou: true},
		{Name: "needs-new", State: "review", Since: fresh, NeedsYou: true},
		{Name: "undated", State: "idle"},
	}
	SortRows(rows)
	var got []string
	for _, r := range rows {
		got = append(got, r.Name)
	}
	want := []string{"needs-new", "needs-old", "busy", "idle", "undated"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rows []source.Row
		want string
	}{
		{"empty", nil, "0 need you · 0 busy · 0 idle"},
		{"mixed", []source.Row{
			{State: "review", NeedsYou: true},
			{State: "busy", Busy: true},
			{State: "idle"},
		}, "1 need you · 1 busy · 1 idle"},
		{"needs-you idle counts both", []source.Row{{State: "idle", NeedsYou: true}}, "1 need you · 0 busy · 1 idle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Summary(tt.rows); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
