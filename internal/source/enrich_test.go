package source

import (
	"errors"
	"testing"
	"time"
)

var errNoCluster = errors.New("no kubeconfig")

const (
	enrichTaskRef = "vikunja:840"
	enrichPRRef   = "pr:https://github.com/sperano/atop/pull/3"
)

func vikunjaRow(state string, needsYou bool) Row {
	return Row{Source: SourceVikunja, Name: "#840", State: state, NeedsYou: needsYou, Since: testNow.Add(-time.Hour), Ref: enrichTaskRef}
}

func prRowFor(state string, needsYou bool) Row {
	return Row{Source: SourcePR, Name: "atop#3", State: state, NeedsYou: needsYou, Since: testNow.Add(-time.Hour), Ref: enrichPRRef, ParentRefs: []string{enrichTaskRef}}
}

func taskRowFor(name, state string, busy bool, since time.Time, parents ...string) Row {
	return Row{Source: SourceKelosTask, Name: name, State: state, Busy: busy, Since: since, ParentRefs: parents}
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
			wantState: map[string]string{"#840": ActivityConflictFix, "atop#3": ActivityConflictFix, "pr-conflict-1": ActivityConflictFix},
			wantNeeds: map[string]bool{"#840": false, "atop#3": false},
			wantBusy:  map[string]bool{"#840": true, "atop#3": true},
		},
		{
			name: "a blocked implementation is not busy but still speaks",
			rows: []Row{
				vikunjaRow("reply from kelos", true),
				taskRowFor("vikunja-atop-claude-840", stateBlocked, false, fresh, enrichTaskRef),
			},
			wantState: map[string]string{"#840": stateBlocked},
			wantNeeds: map[string]bool{"#840": false},
			wantBusy:  map[string]bool{"#840": false},
		},
		{
			name: "finished tasks change nothing",
			rows: []Row{
				vikunjaRow("reply from claude", true),
				taskRowFor("done", stateDone, false, fresh, enrichTaskRef),
				taskRowFor("failed", stateFailed, false, fresh, enrichTaskRef),
			},
			wantState: map[string]string{"#840": "reply from claude"},
			wantNeeds: map[string]bool{"#840": true},
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
			name: "a task never speaks for another task",
			rows: []Row{
				{Source: SourceKelosTask, Name: "parent", State: stateBlocked, Ref: "task:parent"},
				taskRowFor("child", ActivityImplementing, true, fresh, "task:parent"),
			},
			wantState: map[string]string{"parent": stateBlocked},
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
