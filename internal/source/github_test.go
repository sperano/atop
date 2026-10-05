package source

import (
	"context"
	"testing"
	"time"
)

const testStaleDays = 30

func testStaleness() Staleness { return Staleness{Days: testStaleDays, Now: testNow} }

type prOpts struct {
	draft     bool
	mergeable string
	checks    string
	review    string
	labels    []string
	updated   time.Time
}

func prObj(o prOpts) obj {
	if o.mergeable == "" {
		o.mergeable = "MERGEABLE"
	}
	if o.updated.IsZero() {
		o.updated = testNow
	}
	var rollup any
	if o.checks != "" {
		rollup = obj{"state": o.checks}
	}
	labels := []obj{}
	for _, l := range o.labels {
		labels = append(labels, obj{"name": l})
	}
	var review any
	if o.review != "" {
		review = o.review
	}
	return obj{
		"number": 7, "title": "Fix it", "url": "https://github.com/o/cocovm/pull/7",
		"isDraft": o.draft, "mergeable": o.mergeable, "reviewDecision": review,
		"updatedAt":  iso(o.updated),
		"repository": obj{"name": "cocovm"},
		"labels":     obj{"nodes": labels},
		"commits":    obj{"nodes": []obj{{"commit": obj{"statusCheckRollup": rollup}}}},
	}
}

func TestPRRowStates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		pr        prOpts
		wantState string
		wantNeeds bool
		wantBusy  bool
	}{
		{"draft beats conflicts", prOpts{draft: true, mergeable: "CONFLICTING"}, "draft", false, false},
		{"draft beats failing checks", prOpts{draft: true, checks: "FAILURE"}, "draft", false, false},
		{"conflicts", prOpts{mergeable: "CONFLICTING"}, "conflicts", true, false},
		{"conflicts beat failing checks", prOpts{mergeable: "CONFLICTING", checks: "FAILURE"}, "conflicts", true, false},
		{"checks failure", prOpts{checks: "FAILURE"}, "checks failing", true, false},
		{"checks error", prOpts{checks: "ERROR"}, "checks failing", true, false},
		{"checks pending", prOpts{checks: "PENDING"}, "checks running", false, true},
		{"checks expected", prOpts{checks: "EXPECTED"}, "checks running", false, true},
		{"changes requested", prOpts{review: "CHANGES_REQUESTED"}, "changes requested", false, false},
		{"changes requested with green checks", prOpts{checks: "SUCCESS", review: "CHANGES_REQUESTED"}, "changes requested", false, false},
		{"approved and green awaits you", prOpts{checks: "SUCCESS", review: "APPROVED"}, "review", true, false},
		{"no checks at all", prOpts{}, "review", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := prRow(decode[pullRequest](t, prObj(tt.pr)), testStaleness())
			if row.State != tt.wantState || row.NeedsYou != tt.wantNeeds || row.Busy != tt.wantBusy {
				t.Errorf("state=%q needs=%v busy=%v", row.State, row.NeedsYou, row.Busy)
			}
			if row.Source != SourcePR || row.Name != "cocovm#7" {
				t.Errorf("identity = %q/%q", row.Source, row.Name)
			}
			if row.Target.URL != "https://github.com/o/cocovm/pull/7" || !row.Since.Equal(testNow) {
				t.Errorf("target=%q since=%s", row.Target.URL, row.Since)
			}
		})
	}
}

func TestPRRowDetail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{"agent variant prefix", []string{"bug", "kelos:claude"}, "[claude] Fix it"},
		{"no kelos label", []string{"bug"}, "Fix it"},
		{"no labels", nil, "Fix it"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := prRow(decode[pullRequest](t, prObj(prOpts{labels: tt.labels})), testStaleness())
			if row.Detail != tt.want {
				t.Errorf("detail = %q, want %q", row.Detail, tt.want)
			}
		})
	}
}

func TestPRRowStaleness(t *testing.T) {
	t.Parallel()
	old := testNow.Add(-(testStaleDays*24*time.Hour + time.Hour))
	tests := []struct {
		name      string
		pr        prOpts
		wantState string
		wantNeeds bool
	}{
		{"stale review PR stops needing you", prOpts{updated: old}, "stale", false},
		{"stale conflicts PR stops needing you", prOpts{mergeable: "CONFLICTING", updated: old}, "stale", false},
		{"stale failing PR stops needing you", prOpts{checks: "FAILURE", updated: old}, "stale", false},
		{"stale draft stays draft", prOpts{draft: true, updated: old}, "draft", false},
		{"stale changes requested stays", prOpts{review: "CHANGES_REQUESTED", updated: old}, "changes requested", false},
		{"stale running checks stay busy", prOpts{checks: "PENDING", updated: old}, "checks running", false},
		{"fresh review still needs you", prOpts{}, "review", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := prRow(decode[pullRequest](t, prObj(tt.pr)), testStaleness())
			if row.State != tt.wantState || row.NeedsYou != tt.wantNeeds {
				t.Errorf("state=%q needs=%v", row.State, row.NeedsYou)
			}
		})
	}
}

func TestGitHubFetch(t *testing.T) {
	t.Parallel()
	resp := obj{"data": obj{"search": obj{"nodes": []any{
		prObj(prOpts{}),
		obj{}, // a non-PR search hit decodes as an empty object
		nil,
	}}}}
	run := &fakeRunner{replies: map[string]string{"gh api graphql": string(mustJSON(t, resp))}}
	g := GitHub{Owner: "sperano", StaleDays: testStaleDays, Run: run.run, Now: fixedNow}
	rows, err := g.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "cocovm#7" {
		t.Fatalf("rows = %+v", rows)
	}
	wantQuery := "q=is:pr is:open owner:sperano archived:false"
	if !containsFold(run.calls[0], wantQuery) || !containsFold(run.calls[0], "n=50") {
		t.Errorf("command = %q", run.calls[0])
	}
	g.Run = (&fakeRunner{}).run
	if _, err := g.Fetch(context.Background()); err == nil {
		t.Error("expected a runner error")
	}
}
