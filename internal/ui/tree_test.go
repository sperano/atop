package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/sperano/atop/internal/source"
)

const (
	taskRef = "vikunja:1"
	prRef   = "pr:https://github.com/o/r/pull/7"
)

// display renders entries as prefix+name, one per line.
func display(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Prefix + e.Row.Name
	}
	return out
}

func TestArrange(t *testing.T) {
	t.Parallel()
	old, fresh := testNow.Add(-time.Hour), testNow
	tests := []struct {
		name string
		rows []source.Row
		want []string
	}{
		{
			name: "flat rows: needs you, busy, rest, each most recent first, undated last",
			rows: []source.Row{
				{Name: "idle", State: "idle", Since: fresh},
				{Name: "busy", State: "busy", Since: fresh, Busy: true},
				{Name: "needs-old", State: "review", Since: old, NeedsYou: true},
				{Name: "needs-new", State: "review", Since: fresh, NeedsYou: true},
				{Name: "undated", State: "idle"},
			},
			want: []string{"needs-new", "needs-old", "busy", "idle", "undated"},
		},
		{
			name: "vikunja task, its PR, and the kelos task under the PR",
			rows: []source.Row{
				{Name: "kelos", Since: old, ParentRefs: []string{prRef, taskRef}},
				{Name: "#1", Ref: taskRef, Since: fresh, NeedsYou: true},
				{Name: "r#7", Ref: prRef, Since: old, NeedsYou: true, ParentRefs: []string{taskRef}},
				{Name: "other", Since: fresh, Busy: true},
			},
			want: []string{"#1", "└─ r#7", "   └─ kelos", "other"},
		},
		{
			name: "a parent that is not shown falls back to the next, then to a root",
			rows: []source.Row{
				{Name: "#1", Ref: taskRef, Since: fresh},
				{Name: "via task", Since: fresh, ParentRefs: []string{prRef, taskRef}},
				{Name: "orphan", Since: old, ParentRefs: []string{"pr:gone"}},
			},
			want: []string{"#1", "└─ via task", "orphan"},
		},
		{
			name: "a tree ranks as its most urgent row",
			rows: []source.Row{
				{Name: "busy", Since: fresh, Busy: true},
				{Name: "#1", Ref: taskRef, Since: old},
				{Name: "failing pr", Since: old, NeedsYou: true, ParentRefs: []string{taskRef}},
			},
			want: []string{"#1", "└─ failing pr", "busy"},
		},
		{
			name: "siblings are ranked and the guide continues past a middle child",
			rows: []source.Row{
				{Name: "#1", Ref: taskRef, Since: old},
				{Name: "idle child", Since: fresh, ParentRefs: []string{taskRef}},
				{Name: "pr", Ref: prRef, Since: old, NeedsYou: true, ParentRefs: []string{taskRef}},
				{Name: "pr task", Since: old, ParentRefs: []string{prRef}},
			},
			want: []string{"#1", "├─ pr", "│  └─ pr task", "└─ idle child"},
		},
		{
			name: "a cycle is broken rather than hiding rows",
			rows: []source.Row{
				{Name: "a", Ref: "a", Since: fresh, ParentRefs: []string{"b"}},
				{Name: "b", Ref: "b", Since: old, ParentRefs: []string{"a"}},
			},
			want: []string{"b", "└─ a"},
		},
		{
			name: "a row cannot be its own parent",
			rows: []source.Row{{Name: "self", Ref: "s", ParentRefs: []string{"s"}}},
			want: []string{"self"},
		},
		{
			name: "duplicate refs link to the first row",
			rows: []source.Row{
				{Name: "first", Ref: taskRef, Since: fresh},
				{Name: "second", Ref: taskRef, Since: old},
				{Name: "child", Since: old, ParentRefs: []string{taskRef}},
			},
			want: []string{"first", "└─ child", "second"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := display(Arrange(tt.rows)); !slices.Equal(got, tt.want) {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}
