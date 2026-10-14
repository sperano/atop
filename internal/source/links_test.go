package source

import (
	"slices"
	"strconv"
	"testing"
)

const (
	linkPRURL  = "https://github.com/sperano/cocovm/pull/175"
	linkTaskID = "554"
)

func TestBranchTask(t *testing.T) {
	t.Parallel()
	tests := []struct {
		branch, want string
		ok           bool
	}{
		{"kelos/vikunja-554", "554", true},
		{"kelos/vikunja-", "", false},
		{"kelos/vikunja-55a", "", false},
		{"feat/pretty-logs", "", false},
		{"main", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			t.Parallel()
			if got, ok := branchTask(tt.branch); got != tt.want || ok != tt.ok {
				t.Errorf("branchTask(%q) = %q, %v; want %q, %v", tt.branch, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPRRefTrimsSlash(t *testing.T) {
	t.Parallel()
	if PRRef(linkPRURL+"/") != PRRef(linkPRURL) {
		t.Error("a trailing slash must not change the ref")
	}
}

func TestPRRowLinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		branch string
		want   []string
	}{
		{"agent branch links to its vikunja task", "kelos/vikunja-" + linkTaskID, []string{VikunjaRef(linkTaskID)}},
		{"other branch has no parent", "feat/pretty-logs", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pr := pullRequest{Number: 175, Title: "t", URL: linkPRURL, HeadRefName: tt.branch}
			row := prRow(pr, testStaleness())
			if row.Ref != PRRef(linkPRURL) {
				t.Errorf("Ref = %q", row.Ref)
			}
			if !slices.Equal(row.ParentRefs, tt.want) {
				t.Errorf("ParentRefs = %q, want %q", row.ParentRefs, tt.want)
			}
		})
	}
}

func TestTaskRowLinks(t *testing.T) {
	t.Parallel()
	branch := obj{"branch": "kelos/vikunja-" + linkTaskID}
	withPR := obj{"pr": linkPRURL, "branch": "kelos/vikunja-" + linkTaskID}
	label := obj{vikunjaTaskLabel: "182"}
	tests := []struct {
		name    string
		results obj
		labels  obj
		want    []string
	}{
		{"PR first, then the labelled task", withPR, label, []string{PRRef(linkPRURL), VikunjaRef("182")}},
		{"pr-followup: PR, then the task from its branch", withPR, nil, []string{PRRef(linkPRURL), VikunjaRef(linkTaskID)}},
		{"task from the branch alone", branch, nil, []string{VikunjaRef(linkTaskID)}},
		{"task from the label alone", nil, label, []string{VikunjaRef("182")}},
		{"merge task has no links", obj{"branch": "main"}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := decode[kelosTask](t, taskObj("Running", obj{"results": tt.results}, tt.labels))
			if !slices.Equal(taskParents(task, taskPR(task)), tt.want) {
				t.Errorf("ParentRefs = %q, want %q", taskParents(task, taskPR(task)), tt.want)
			}
		})
	}
}

func TestTaskRowLinksFromSpecAndPrompt(t *testing.T) {
	t.Parallel()
	promptURL := "https://github.com/sperano/atop/pull/3"
	tests := []struct {
		name   string
		spec   obj
		labels obj
		want   []string
	}{
		{"conflict fix: the prompt's PR, then the task from the branch it runs on",
			obj{"prompt": conflictPrompt, "branch": "kelos/vikunja-840"}, obj{roleLabel: rolePRConflict},
			[]string{PRRef(promptURL), VikunjaRef("840")}},
		{"follow-up on a hand-made branch links to the PR only",
			obj{"prompt": promptOf(ownerRequest), "branch": "feat/x"}, obj{roleLabel: rolePRFollowUp},
			[]string{PRRef(promptURL)}},
		{"an implementation prompt is not read for a PR",
			obj{"prompt": "PR: https://x/pull/9"}, obj{vikunjaTaskLabel: "840"},
			[]string{VikunjaRef("840")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := decode[kelosTask](t, taskObj("Running", tt.spec, tt.labels))
			row, _ := testKelos().taskRow(task, nil, testNow)
			if !slices.Equal(row.ParentRefs, tt.want) {
				t.Errorf("ParentRefs = %q, want %q", row.ParentRefs, tt.want)
			}
		})
	}
}

func TestVikunjaRowRef(t *testing.T) {
	t.Parallel()
	task := testTask()
	row := testVikunjaSource().row(task, nil, testStaleness(), testWeb)
	if row.Ref != VikunjaRef(strconv.Itoa(task.ID)) {
		t.Errorf("Ref = %q", row.Ref)
	}
}
