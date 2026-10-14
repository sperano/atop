package source

import (
	"fmt"
	"testing"
)

const (
	followUpPrompt = `Follow-up request on pull request #3 in sperano/atop: Fix it

PR: https://github.com/sperano/atop/pull/3
Branch: kelos/vikunja-840
Owner: sperano
Review bot: cubic-dev-ai[bot]
Event: pull_request_review
Request: https://github.com/sperano/atop/pull/3#pullrequestreview-9

<request>
%s
</request>

Follow the PR follow-up procedure in your instructions.`
	cubicReport    = "**2 issues found** across 3 files"
	ownerRequest   = "/kelos also cover the empty case"
	conflictPrompt = `Resolve the confirmed merge conflict on pull request #3 in sperano/atop: Fix it

PR: https://github.com/sperano/atop/pull/3
Branch: kelos/vikunja-840`
	mergeClosePrompt = `A Kelos pull request in sperano/atop was closed.

pr: https://github.com/sperano/atop/pull/3
branch: kelos/vikunja-840
merged: true`
)

func promptOf(request string) string {
	return fmt.Sprintf(followUpPrompt, request)
}

func TestTaskActivity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		task   obj
		prompt string
		want   string
	}{
		{"implementation task", taskObj("Running", nil, obj{vikunjaTaskLabel: "840"}), "", ActivityImplementing},
		{"discuss task", obj{"metadata": obj{"name": "vikunja-discuss-atop-claude-fable-840",
			"labels": obj{vikunjaTaskLabel: "840"}}}, "", ActivityDiscussing},
		{"unknown task", taskObj("Running", nil, nil), "", ""},
		{"conflict fix", taskObj("Running", nil, obj{roleLabel: rolePRConflict}), conflictPrompt, ActivityConflictFix},
		{"owner's PR follow-up", taskObj("Running", nil, obj{roleLabel: rolePRFollowUp}), promptOf(ownerRequest), ActivityPRFollowUp},
		{"review bot fix named after the bot", taskObj("Running", nil, obj{roleLabel: rolePRFollowUp}), promptOf(cubicReport), "cubic fix"},
		{"follow-up without a prompt", taskObj("Running", nil, obj{roleLabel: rolePRFollowUp}), "", ActivityPRFollowUp},
		{"task follow-up", taskObj("Running", nil, obj{roleLabel: roleTaskFollowUp}), "", ActivityTaskFollowUp},
		{"plumbing role shown as is", taskObj("Running", nil, obj{roleLabel: "merge-close"}), mergeClosePrompt, "merge-close"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := decode[kelosTask](t, tt.task)
			task.Spec.Prompt = tt.prompt
			if got := taskActivity(task); got != tt.want {
				t.Errorf("taskActivity = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReviewBotRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, prompt, want string
	}{
		{"bot report", promptOf(cubicReport), "cubic"},
		{"owner command", promptOf(ownerRequest), ""},
		{"no review bot line", "PR: x\n\n<request>\n" + cubicReport + "\n</request>", ""},
		{"no request block", "Review bot: cubic-dev-ai[bot]\n", ""},
		{"bot without suffixes", "Review bot: reviewer\n<request>\nfindings\n</request>", "reviewer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := reviewBotRequest(tt.prompt); got != tt.want {
				t.Errorf("reviewBotRequest = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPromptPR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, prompt, want string
	}{
		{"follow-up", promptOf(ownerRequest), "https://github.com/sperano/atop/pull/3"},
		{"conflict", conflictPrompt, "https://github.com/sperano/atop/pull/3"},
		{"plumbing lowercase key", mergeClosePrompt, "https://github.com/sperano/atop/pull/3"},
		{"none", "Vikunja task 840 in project atop: we need a refresh", ""},
		{"not at line start", "see PR: https://x/pull/1 now", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := promptPR(tt.prompt); got != tt.want {
				t.Errorf("promptPR = %q, want %q", got, tt.want)
			}
		})
	}
}
