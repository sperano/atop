package source

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		in    string
		zero  bool
		equal time.Time
	}{
		{"empty", "", true, time.Time{}},
		{"go zero time", "0001-01-01T00:00:00Z", true, time.Time{}},
		{"garbage", "yesterday", true, time.Time{}},
		{"utc", "2026-10-05T12:00:00Z", false, testNow},
		{"offset", "2026-10-05T08:00:00-04:00", false, testNow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseTime(tt.in)
			if got.IsZero() != tt.zero {
				t.Fatalf("IsZero = %v, want %v", got.IsZero(), tt.zero)
			}
			if !tt.zero && !got.Equal(tt.equal) {
				t.Errorf("got %s, want %s", got, tt.equal)
			}
		})
	}
}

func TestStalenessIsStale(t *testing.T) {
	t.Parallel()
	const days = 30
	s := Staleness{Days: days, Now: testNow}
	window := days * 24 * time.Hour
	tests := []struct {
		name  string
		since time.Time
		want  bool
	}{
		{"zero time is never stale", time.Time{}, false},
		{"fresh", testNow.Add(-time.Hour), false},
		{"exactly at window", testNow.Add(-window), false},
		{"just past window", testNow.Add(-window - time.Second), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := s.IsStale(tt.since); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSnippet(t *testing.T) {
	t.Parallel()
	const limit = 100
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"strips tags and collapses space", "<p>a</p>\n<ul><li>b</li></ul>", "a b"},
		{"unescapes entities", "<p>PR: <b>#180</b> &amp; more</p>", "PR: #180 & more"},
		{"plain text", "  hello   world ", "hello world"},
		{"truncates to limit runes", strings.Repeat("é", limit+20), strings.Repeat("é", limit)},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := snippet(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVikunjaURLs(t *testing.T) {
	t.Parallel()
	const web = "https://v.example"
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"web base strips api suffix", VikunjaWebBase(web + "/api/v1"), web},
		{"web base strips trailing slash", VikunjaWebBase(web + "/api/v1/"), web},
		{"web base without suffix", VikunjaWebBase(web), web},
		{"task url", VikunjaTaskURL(web, "575"), web + "/tasks/575"},
		{"task url trims base slash", VikunjaTaskURL(web+"/", "575"), web + "/tasks/575"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestRowKeyAndErrorRow(t *testing.T) {
	t.Parallel()
	a := Row{Source: "pr", Name: "x"}
	b := Row{Source: "pr", Name: "y"}
	if a.Key() == b.Key() {
		t.Error("distinct names must give distinct keys")
	}
	row := ErrorRow(SourcePR, errors.New("gh logged out"))
	if row.Source != SourcePR || row.State != StateError || !row.NeedsYou || row.Detail != "gh logged out" {
		t.Errorf("unexpected error row %+v", row)
	}
}
