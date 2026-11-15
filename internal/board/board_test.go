package board

import (
	"errors"
	"testing"
	"time"

	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

func TestHiddenParentLeavesChildAtTop(t *testing.T) {
	t.Parallel()
	parent := source.Row{Source: "vikunja", Name: "#7", State: "idle", Ref: "vikunja:7"}
	child := source.Row{Source: "pr", Name: "vigie#3", State: "review", ParentRefs: []string{"vikunja:7"}}
	set := hidden.New(hidden.Entry{Source: "vikunja", Name: "#7"})
	entries := Build([]source.Row{parent, child}, set, false, testNow).Entries
	if len(entries) != 1 || entries[0].Row.Name != "vigie#3" || entries[0].Prefix != "" || entries[0].Depth != 0 {
		t.Errorf("entries = %+v, want the PR alone at the top level", entries)
	}
	entries = Build([]source.Row{parent, child}, set, true, testNow).Entries
	if len(entries) != 2 || entries[1].Prefix != branchLast || entries[1].Depth != 1 || !entries[0].Hidden {
		t.Errorf("with hidden shown, entries = %+v, want the PR nested under the hidden task", entries)
	}
}

func TestBuildDecoratesEntries(t *testing.T) {
	t.Parallel()
	const url = "https://github.com/o/r/pull/1"
	rows := []source.Row{
		{Source: "pr", Origin: source.OriginGitHub, Name: "needs", State: "review", Since: testNow.Add(-time.Hour), NeedsYou: true,
			Ref: "pr:1", Target: source.Target{URL: url}},
		{Source: "kelos task", Name: "child", State: "busy", Busy: true, ParentRefs: []string{"pr:1"}},
		{Source: "pr", Origin: source.OriginGitHub, Name: "gone", State: "review", Since: testNow.Add(-2 * time.Hour), NeedsYou: true},
	}
	set := hidden.New(hidden.Entry{Source: "pr", Name: "gone"})
	want := []Entry{
		{Row: rows[0], Mark: MarkNeedsYou, Origin: source.OriginGitHub, Age: "1h", Target: Target{Kind: TargetURL, Value: url},
			Severity: SeverityAttention},
		{Row: rows[1], Depth: 1, Prefix: branchLast, Origin: source.OriginUnknown, Age: unknownAge, Severity: SeverityBusy},
		{Row: rows[2], Hidden: true, Mark: MarkHidden, Origin: source.OriginGitHub, Age: "2h", Severity: SeverityQuiet},
	}
	got := Build(rows, set, true, testNow).Entries
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if g, w := got[i], want[i]; g.Row.Name != w.Row.Name || g.Depth != w.Depth || g.Prefix != w.Prefix ||
			g.Hidden != w.Hidden || g.Mark != w.Mark || g.Origin != w.Origin || g.Age != w.Age || g.Target != w.Target ||
			g.Severity != w.Severity {
			t.Errorf("entry %d = %+v\nwant      %+v", i, g, w)
		}
	}
}

func TestBuildDepths(t *testing.T) {
	t.Parallel()
	rows := []source.Row{
		{Source: "vikunja", Name: "#7", State: "idle", Ref: "vikunja:7"},
		{Source: "pr", Name: "vigie#3", State: "review", Ref: "pr:3", ParentRefs: []string{"vikunja:7"}},
		{Source: "kelos task", Name: "fix", State: "busy", Busy: true, ParentRefs: []string{"pr:3"}},
	}
	got := Build(rows, hidden.Set{}, false, testNow).Entries
	want := []struct {
		name  string
		depth int
	}{{"#7", 0}, {"vigie#3", 1}, {"fix", 2}}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Row.Name != w.name || got[i].Depth != w.depth {
			t.Errorf("entry %d = %s at depth %d, want %s at depth %d", i, got[i].Row.Name, got[i].Depth, w.name, w.depth)
		}
	}
}

func TestBuildCounts(t *testing.T) {
	t.Parallel()
	rows := []source.Row{
		{Source: "pr", Name: "a", State: "review", NeedsYou: true},
		{Source: "pr", Name: "b", State: "review", NeedsYou: true},
		{Source: "local claude", Name: "c", State: "busy", Busy: true},
		{Source: "local claude", Name: "d", State: "busy", Busy: true},
		source.ErrorRow("pr", errors.New("down")),
	}
	tests := []struct {
		name       string
		set        hidden.Set
		showHidden bool
		want       Counts
	}{
		{"nothing hidden", hidden.Set{}, false, Counts{NeedYou: 3, Busy: 2}},
		{"one hidden", hidden.New(hidden.Entry{Source: "pr", Name: "b"}), false, Counts{NeedYou: 2, Busy: 2, Hidden: 1}},
		{"shown hidden rows are still left out", hidden.New(hidden.Entry{Source: "pr", Name: "b"}), true, Counts{NeedYou: 2, Busy: 2, Hidden: 1}},
		{"hidden busy agent", hidden.New(hidden.Entry{Source: "local claude", Name: "d"}), false, Counts{NeedYou: 3, Busy: 1, Hidden: 1}},
		{"hidden entry not listed", hidden.New(hidden.Entry{Source: "pr", Name: "gone"}), false, Counts{NeedYou: 3, Busy: 2}},
		{"an error row is never hidden", hidden.New(hidden.Entry{Source: "pr", Name: "-"}), false, Counts{NeedYou: 3, Busy: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Build(rows, tt.set, tt.showHidden, testNow).Counts; got != tt.want {
				t.Errorf("counts = %+v, want %+v", got, tt.want)
			}
		})
	}
}
