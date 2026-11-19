package source

import (
	"errors"
	"testing"
	"time"
)

var errNoCluster = errors.New("no kubeconfig")

const (
	enrichTaskRef = "vikunja:840"
	enrichPRRef   = "pr:https://github.com/sperano/vigie/pull/3"
)

func vikunjaRow(state string, needsYou bool) Row {
	return Row{Source: SourceVikunja, Name: "#840", State: state, NeedsYou: needsYou, Since: testNow.Add(-time.Hour), Ref: enrichTaskRef}
}

func prRowFor(state string, needsYou bool) Row {
	return Row{Source: SourcePR, Name: "vigie#3", State: state, NeedsYou: needsYou, Since: testNow.Add(-time.Hour), Ref: enrichPRRef, ParentRefs: []string{enrichTaskRef}}
}

func taskRowFor(name, state string, busy bool, since time.Time, parents ...string) Row {
	return Row{Source: SourceKelosTask, Name: name, State: state, Busy: busy, Since: since, ParentRefs: parents}
}

func startedRow(task string) Row {
	r := vikunjaRow(StateAgentStarted, false)
	r.Busy, r.AgentTask = true, task
	return r
}

func TestEnrich(t *testing.T) {
	t.Parallel()
	fresh, old := testNow, testNow.Add(-time.Minute)
	tests := []struct {
		name      string
		rows      []Row
		wantState map[string]string
		wantNeeds map[string]bool
		wantBusy  map[string]bool
	}{
		{
			name: "a running conflict fix speaks for its PR and task",
			rows: []Row{
				vikunjaRow(StatePROpen, false),
				prRowFor("conflicts", true),
				taskRowFor("pr-conflict-1", ActivityConflictFix, true, fresh, enrichPRRef, enrichTaskRef),
			},
			wantState: map[string]string{"#840": ActivityConflictFix, "vigie#3": ActivityConflictFix, "pr-conflict-1": ActivityConflictFix},
			wantNeeds: map[string]bool{"#840": false, "vigie#3": false},
			wantBusy:  map[string]bool{"#840": true, "vigie#3": true},
		},
		{
			name: "a blocked implementation is not busy but still speaks",
			rows: []Row{
				vikunjaRow("reply from kelos", true),
				taskRowFor("vikunja-vigie-claude-840", StateBlocked, false, fresh, enrichTaskRef),
			},
			wantState: map[string]string{"#840": StateBlocked},
			wantNeeds: map[string]bool{"#840": false},
			wantBusy:  map[string]bool{"#840": false},
		},
		{
			name: "finished tasks change nothing",
			rows: []Row{
				vikunjaRow("reply from claude", true),
				taskRowFor("done", StateDone, false, fresh, enrichTaskRef),
				taskRowFor("failed", StateFailed, false, fresh, enrichTaskRef),
			},
			wantState: map[string]string{"#840": "reply from claude"},
			wantNeeds: map[string]bool{"#840": true},
		},
		{
			name: "a capped task changes nothing",
			rows: []Row{
				prRowFor("review", true),
				taskRowFor("capped", stateCapped, false, fresh, enrichPRRef),
			},
			wantState: map[string]string{"vigie#3": "review"},
			wantNeeds: map[string]bool{"vigie#3": true},
		},
		{
			name: "a running task beats a newer queued one",
			rows: []Row{
				vikunjaRow(StateIdle, false),
				taskRowFor("queued", "waiting", false, fresh, enrichTaskRef),
				taskRowFor("running", ActivityImplementing, true, old, enrichTaskRef),
			},
			wantState: map[string]string{"#840": ActivityImplementing},
			wantBusy:  map[string]bool{"#840": true},
		},
		{
			name: "the newest of two queued tasks speaks",
			rows: []Row{
				vikunjaRow(StateIdle, false),
				taskRowFor("older", "pending", false, old, enrichTaskRef),
				taskRowFor("newer", "waiting", false, fresh, enrichTaskRef),
			},
			wantState: map[string]string{"#840": "waiting"},
		},
		{
			name: "nothing changes when the task source failed",
			rows: []Row{
				vikunjaRow("reply from claude", true),
				ErrorRow(SourceKelosTask, errNoCluster),
				taskRowFor("stray", ActivityImplementing, true, fresh, enrichTaskRef),
			},
			wantState: map[string]string{"#840": "reply from claude"},
			wantNeeds: map[string]bool{"#840": true},
		},
		{
			name: "the task a start notice names speaks when no ref links one",
			rows: []Row{
				startedRow("fu-1"),
				taskRowFor("fu-1", StateBlocked, false, fresh),
			},
			wantState: map[string]string{"#840": StateBlocked},
			wantBusy:  map[string]bool{"#840": false},
		},
		{
			name: "a task linked by ref beats the named one",
			rows: []Row{
				startedRow("fu-1"),
				taskRowFor("fu-1", ActivityImplementing, true, fresh),
				taskRowFor("by-ref", "waiting", false, old, enrichTaskRef),
			},
			wantState: map[string]string{"#840": "waiting"},
		},
		{
			name: "a finished named task changes nothing",
			rows: []Row{
				startedRow("fu-1"),
				taskRowFor("fu-1", StateDone, false, fresh),
			},
			wantState: map[string]string{"#840": StateAgentStarted},
			wantBusy:  map[string]bool{"#840": true},
		},
		{
			name: "a task never speaks for another task",
			rows: []Row{
				{Source: SourceKelosTask, Name: "parent", State: StateBlocked, Ref: "task:parent"},
				taskRowFor("child", ActivityImplementing, true, fresh, "task:parent"),
			},
			wantState: map[string]string{"parent": StateBlocked},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rows := Enrich(tt.rows)
			byName := make(map[string]Row, len(rows))
			for _, r := range rows {
				byName[r.Name] = r
			}
			for name, want := range tt.wantState {
				if got := byName[name].State; got != want {
					t.Errorf("%s: state = %q, want %q", name, got, want)
				}
			}
			for name, want := range tt.wantNeeds {
				if got := byName[name].NeedsYou; got != want {
					t.Errorf("%s: needs you = %v, want %v", name, got, want)
				}
			}
			for name, want := range tt.wantBusy {
				if got := byName[name].Busy; got != want {
					t.Errorf("%s: busy = %v, want %v", name, got, want)
				}
			}
		})
	}
}

func TestEnrichTakesTheTasksTime(t *testing.T) {
	t.Parallel()
	rows := Enrich([]Row{
		vikunjaRow(StateIdle, false),
		taskRowFor("t", ActivityImplementing, true, testNow, enrichTaskRef),
	})
	if !rows[0].Since.Equal(testNow) {
		t.Errorf("since = %s, want %s", rows[0].Since, testNow)
	}
}
