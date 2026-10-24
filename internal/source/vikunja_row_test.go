package source

import (
	"strings"
	"testing"
	"time"
)

const (
	testOperator    = "eric"
	testWeb         = "https://v.example"
	testPROpenLabel = "pr open"
	commentHTML     = "<p>PR: <b>#180</b> &amp; more</p>"
)

func testVikunjaSource() *Vikunja {
	return &Vikunja{APIURL: testWeb + "/api/v1", Operator: testOperator, PROpenLabel: testPROpenLabel,
		StaleDays: testStaleDays, Now: fixedNow}
}

func labelled(t vikunjaTask, titles ...string) vikunjaTask {
	for _, title := range titles {
		t.Labels = append(t.Labels, struct {
			Title string `json:"title"`
		}{Title: title})
	}
	return t
}

func testTask() vikunjaTask {
	return vikunjaTask{ID: 575, Title: "mcp: peek", Updated: "2026-10-05T08:00:00-04:00"}
}

func testComment(author string, created time.Time) *vikunjaComment {
	c := &vikunjaComment{Comment: commentHTML, Created: iso(created)}
	c.Author.Username = author
	return c
}

func TestVikunjaRow(t *testing.T) {
	t.Parallel()
	old := testNow.Add(-(testStaleDays*24*time.Hour + time.Hour))
	tests := []struct {
		name       string
		task       vikunjaTask
		comment    *vikunjaComment
		wantState  string
		wantNeeds  bool
		wantSince  time.Time
		wantDetail string
	}{
		{"agent reply needs you", testTask(), testComment("claude", testNow), "reply from claude", true, testNow,
			"mcp: peek — PR: #180 & more"},
		{"operator spoke last is idle", testTask(), testComment(testOperator, testNow), StateIdle, false, testNow, "mcp: peek"},
		{"no comment is idle since updated", testTask(), nil, StateIdle, false, testNow, "mcp: peek"},
		{"stale reply stops needing you", testTask(), testComment("claude", old), "stale", false, old,
			"reply from claude: mcp: peek — PR: #180 & more"},
		{"missing author shown as ?", testTask(), testComment("", testNow), "reply from ?", true, testNow,
			"mcp: peek — PR: #180 & more"},
		{"pr open label is PR open", labelled(testTask(), "in progress", testPROpenLabel), nil, StatePROpen, false, testNow, "mcp: peek"},
		{"PR open even after an agent reply", labelled(testTask(), testPROpenLabel), testComment("claude", testNow),
			StatePROpen, false, testNow, "mcp: peek"},
		{"other labels do not mean review", labelled(testTask(), "in progress"), nil, StateIdle, false, testNow, "mcp: peek"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := testVikunjaSource().row(tt.task, tt.comment, testStaleness(), testWeb)
			if row.Source != SourceVikunja || row.Name != "#575" {
				t.Errorf("identity = %q/%q", row.Source, row.Name)
			}
			if row.State != tt.wantState || row.NeedsYou != tt.wantNeeds || row.Busy {
				t.Errorf("state=%q needs=%v busy=%v", row.State, row.NeedsYou, row.Busy)
			}
			if !row.Since.Equal(tt.wantSince) {
				t.Errorf("since = %s, want %s", row.Since, tt.wantSince)
			}
			if row.Detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", row.Detail, tt.wantDetail)
			}
			if row.Target.URL != testWeb+"/tasks/575" {
				t.Errorf("target = %q", row.Target.URL)
			}
		})
	}
}

func TestVikunjaRowAssignee(t *testing.T) {
	t.Parallel()
	task := testTask()
	task.Assignees = append(task.Assignees, struct {
		Username string `json:"username"`
	}{Username: "claude-fable"})
	row := testVikunjaSource().row(task, nil, testStaleness(), testWeb)
	if row.Detail != "[claude-fable] mcp: peek" {
		t.Errorf("detail = %q", row.Detail)
	}
}

func TestVikunjaRowNoPROpenLabelConfigured(t *testing.T) {
	t.Parallel()
	v := testVikunjaSource()
	v.PROpenLabel = ""
	row := v.row(labelled(testTask(), ""), nil, testStaleness(), testWeb)
	if row.State != StateIdle {
		t.Errorf("state = %q, want %q", row.State, StateIdle)
	}
}

func TestVikunjaRowSnippetIsCapped(t *testing.T) {
	t.Parallel()
	c := testComment("claude", testNow)
	c.Comment = "<p>" + strings.Repeat("x", snippetRunes*3) + "</p>"
	row := testVikunjaSource().row(testTask(), c, testStaleness(), testWeb)
	want := "mcp: peek — " + strings.Repeat("x", snippetRunes)
	if row.Detail != want {
		t.Errorf("detail length %d, want %d", len(row.Detail), len(want))
	}
}
