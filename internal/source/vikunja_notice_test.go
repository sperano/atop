package source

import (
	"testing"
	"time"
)

const (
	testAgentTask = "vikunja-fu-vigie-1558-c5434"
	startHTML     = "<p>HEARD YOU LOUD AND CLEAR!</p><p>COFFEE TIME! REPORT TO FOLLOW.</p>" +
		"<!-- kelos:start task=" + testAgentTask + " -->"
)

func TestParseNotice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		comment string
		want    notice
		wantOK  bool
	}{
		{"start with its task", startHTML, notice{kind: kindStart, task: testAgentTask}, true},
		{"kind alone", "<p>WHAT A BEAUTY. CASE CLOSED.</p><!-- kelos:merged -->", notice{kind: kindMerged}, true},
		{"hyphenated kind", "<!-- kelos:restart-gave-up -->", notice{kind: "restart-gave-up"}, true},
		{"marker anywhere", "<!-- kelos:stop --><p>OFF THE CASE</p>", notice{kind: kindStop}, true},
		{"the last marker wins", "<!-- kelos:stop --><p>x</p><!-- kelos:start task=t1 -->", notice{kind: kindStart, task: "t1"}, true},
		{"no marker", "<p>COFFEE TIME! REPORT TO FOLLOW.</p>", notice{}, false},
		{"other HTML comment", "<!-- note -->", notice{}, false},
		{"no spaces is not a marker", "<!--kelos:start-->", notice{}, false},
		{"upper-case kind is not a marker", "<!-- kelos:START -->", notice{}, false},
		{"bad task name is not a marker", "<!-- kelos:start task=-x -->", notice{}, false},
		{"escaped marker is text", "&lt;!-- kelos:start --&gt;", notice{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseNotice(tt.comment)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("parseNotice = %+v, %v; want %+v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func noticeComment(author, kind string, created time.Time) *vikunjaComment {
	c := testComment(author, created)
	c.Comment = "<p>GORDON.</p><!-- kelos:" + kind + " -->"
	return c
}

func startComment(author string, created time.Time) *vikunjaComment {
	c := testComment(author, created)
	c.Comment = startHTML
	return c
}

func TestVikunjaRowKelosNotice(t *testing.T) {
	t.Parallel()
	old := testNow.Add(-(testStaleDays*24*time.Hour + time.Hour))
	tests := []struct {
		name          string
		task          vikunjaTask
		comment       *vikunjaComment
		wantState     string
		wantNeeds     bool
		wantBusy      bool
		wantDetail    string
		wantAgentTask string
	}{
		{"start is agent started", testTask(), startComment(testKelosUser, testNow), StateAgentStarted, false, true,
			"mcp: peek — " + testAgentTask, testAgentTask},
		{"start without a task", testTask(), noticeComment(testKelosUser, kindStart, testNow), StateAgentStarted, false, true,
			"mcp: peek", ""},
		{"old start is still agent started", testTask(), startComment(testKelosUser, old), StateAgentStarted, false, true,
			"mcp: peek — " + testAgentTask, testAgentTask},
		{"restart", testTask(), noticeComment(testKelosUser, kindRestart, testNow), StateAgentStarted, false, true, "mcp: peek", ""},
		{"queued", testTask(), noticeComment(testKelosUser, kindQueued, testNow), StateAgentStarted, false, true, "mcp: peek", ""},
		{"branch locked", testTask(), noticeComment(testKelosUser, kindBranchLocked, testNow), StateAgentStarted, false, true, "mcp: peek", ""},
		{"chain started", testTask(), noticeComment(testKelosUser, kindChainStarted, testNow), StateAgentStarted, false, true, "mcp: peek", ""},
		{"stop", testTask(), noticeComment(testKelosUser, kindStop, testNow), StateAgentStopped, false, false, "mcp: peek", ""},
		{"merged", testTask(), noticeComment(testKelosUser, kindMerged, testNow), "merged", false, false, "mcp: peek", ""},
		{"ready", testTask(), noticeComment(testKelosUser, kindReady, testNow), "ready", false, false, "mcp: peek", ""},
		{"discuss", testTask(), noticeComment(testKelosUser, kindDiscuss, testNow), "discuss", false, false, "mcp: peek", ""},
		{"pr open", testTask(), noticeComment(testKelosUser, kindPROpen, testNow), StatePROpen, false, false, "mcp: peek", ""},
		{"another notice needs you", testTask(), noticeComment(testKelosUser, "review-cap", testNow), "reply from gordon", true, false,
			"mcp: peek — GORDON.", ""},
		{"an unknown kind needs you", testTask(), noticeComment(testKelosUser, "brand-new", testNow), "reply from gordon", true, false,
			"mcp: peek — GORDON.", ""},
		{"no marker needs you", testTask(), testComment(testKelosUser, testNow), "reply from gordon", true, false,
			"mcp: peek — PR: #180 & more", ""},
		{"a marker from someone else needs you", testTask(), startComment("claude", testNow), "reply from claude", true, false,
			"mcp: peek — HEARD YOU LOUD AND CLEAR! COFFEE TIME! REPORT TO FOLLOW.", ""},
		{"the PR open label wins", labelled(testTask(), testPROpenLabel), startComment(testKelosUser, testNow), StatePROpen, false, false,
			"mcp: peek", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := testVikunjaSource().row(tt.task, tt.comment, testStaleness(), testWeb)
			if row.State != tt.wantState || row.NeedsYou != tt.wantNeeds || row.Busy != tt.wantBusy {
				t.Errorf("state=%q needs=%v busy=%v, want %q %v %v", row.State, row.NeedsYou, row.Busy, tt.wantState, tt.wantNeeds, tt.wantBusy)
			}
			if row.Detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", row.Detail, tt.wantDetail)
			}
			if row.AgentTask != tt.wantAgentTask {
				t.Errorf("agent task = %q, want %q", row.AgentTask, tt.wantAgentTask)
			}
			if created := parseTime(tt.comment.Created); !row.Since.Equal(created) {
				t.Errorf("since = %s, want %s", row.Since, created)
			}
		})
	}
}

func TestVikunjaRowWithoutKelosUser(t *testing.T) {
	t.Parallel()
	v := testVikunjaSource()
	v.KelosUser = ""
	row := v.row(testTask(), startComment(testKelosUser, testNow), testStaleness(), testWeb)
	if row.State != "reply from gordon" || !row.NeedsYou || row.Busy {
		t.Errorf("state=%q needs=%v busy=%v, want a reply that needs you", row.State, row.NeedsYou, row.Busy)
	}
}
