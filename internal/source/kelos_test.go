package source

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

const (
	testConsole  = "https://console.example"
	testVikunja  = "https://v.example"
	testNS       = "kelos-agents"
	testPodName  = "t-pod"
	testTaskName = "t"
	testPRURL    = "https://x/pull/1"
)

type obj = map[string]any

func decode[T any](t *testing.T, v any) T {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func testKelos() Kelos {
	return Kelos{Namespace: testNS, ConsoleURL: testConsole, VikunjaWeb: testVikunja, Now: fixedNow}
}

// taskObj builds a Task; extra goes into its status, except "prompt" and
// "branch", which go into its spec.
func taskObj(phase string, extra obj, labels obj) obj {
	status := obj{"phase": phase, "podName": testPodName, "startTime": iso(testNow.Add(-5 * time.Minute))}
	spec := obj{}
	for k, v := range extra {
		if k == "prompt" || k == "branch" {
			spec[k] = v
		} else {
			status[k] = v
		}
	}
	return obj{"metadata": obj{"name": testTaskName, "labels": labels}, "spec": spec, "status": status}
}

func initPod(name, state string) pod {
	var p pod
	p.Status.InitContainerStatuses = []containerStatus{{Name: name, State: map[string]any{state: obj{}}}}
	return p
}

func blockedPod() pod { return initPod(waitForBlockers, "running") }

func finishedInitPod() pod { return initPod(waitForBlockers, "terminated") }

func TestTaskRow(t *testing.T) {
	t.Parallel()
	vikunjaLabel := obj{vikunjaTaskLabel: "575"}
	oldDone := iso(testNow.Add(-RecentDone - time.Second))
	edgeDone := iso(testNow.Add(-RecentDone))
	recentDone := iso(testNow.Add(-time.Minute))
	tests := []struct {
		name       string
		task       obj
		pods       map[string]pod
		wantShown  bool
		wantState  string
		wantBusy   bool
		wantNeeds  bool
		wantDetail string
		wantURL    string
	}{
		{"running implementation with vikunja and pr", taskObj("Running", obj{"results": obj{"pr": testPRURL}}, vikunjaLabel),
			nil, true, ActivityImplementing, true, false, "vikunja #575 · " + testPRURL, testPRURL},
		{"running conflict fix on the prompt's PR", taskObj("Running", obj{"prompt": conflictPrompt}, obj{roleLabel: rolePRConflict}),
			nil, true, ActivityConflictFix, true, false, "https://github.com/sperano/atop/pull/3", "https://github.com/sperano/atop/pull/3"},
		{"running review fix", taskObj("Running", obj{"prompt": promptOf(cubicReport)}, obj{roleLabel: rolePRFollowUp}),
			nil, true, "cubic fix", true, false, "https://github.com/sperano/atop/pull/3", "https://github.com/sperano/atop/pull/3"},
		{"running no labels", taskObj("Running", nil, nil), nil, true, "running", true, false, "",
			testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"blocked by wait-for-blockers", taskObj("Running", nil, vikunjaLabel),
			map[string]pod{testPodName: blockedPod()}, true, "blocked", false, false, "implementing · vikunja #575",
			testVikunja + "/tasks/575"},
		{"waiting phase keeps the activity in detail", taskObj("Waiting", nil, obj{roleLabel: rolePRConflict}),
			nil, true, "waiting", false, false, ActivityConflictFix,
			testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"terminated init container is not blocked", taskObj("Running", nil, nil),
			map[string]pod{testPodName: finishedInitPod()}, true, "running", true, false, "",
			testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"running init container of another name is not blocked", taskObj("Running", nil, nil),
			map[string]pod{testPodName: initPod("other", "running")}, true, "running", true, false, "",
			testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"pending phase", taskObj("Pending", nil, nil), nil, true, "pending", false, false, "",
			testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"empty phase is pending", taskObj("", nil, nil), nil, true, "pending", false, false, "",
			testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"recent success shown", taskObj("Succeeded", obj{"completionTime": recentDone}, nil), nil,
			true, "done", false, false, "", testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"success at one hour still shown", taskObj("Succeeded", obj{"completionTime": edgeDone}, nil), nil,
			true, "done", false, false, "", testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"old success hidden", taskObj("Succeeded", obj{"completionTime": oldDone}, nil), nil,
			false, "done", false, false, "", testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"success without completion time hidden", taskObj("Succeeded", nil, nil), nil,
			false, "done", false, false, "", testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"failure needs you with message", taskObj("Failed", obj{"completionTime": iso(testNow), "message": "boom"}, nil),
			nil, true, "failed", false, true, "boom", testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"failed activity prefixes the message", taskObj("Failed", obj{"message": "boom"}, obj{roleLabel: rolePRConflict}),
			nil, true, "failed", false, true, "conflict fix · boom", testConsole + "/api/resources/tasks/kelos-agents/t/logs"},
		{"failure without message keeps detail", taskObj("Failed", nil, vikunjaLabel), nil,
			true, "failed", false, true, "implementing · vikunja #575", testVikunja + "/tasks/575"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row, shown := testKelos().taskRow(decode[kelosTask](t, tt.task), tt.pods, testNow)
			if shown != tt.wantShown {
				t.Fatalf("shown = %v, want %v", shown, tt.wantShown)
			}
			if row.Source != SourceKelosTask || row.Name != testTaskName {
				t.Errorf("identity = %q/%q", row.Source, row.Name)
			}
			if row.State != tt.wantState || row.Busy != tt.wantBusy || row.NeedsYou != tt.wantNeeds {
				t.Errorf("state=%q busy=%v needs=%v", row.State, row.Busy, row.NeedsYou)
			}
			if row.Detail != tt.wantDetail || row.Target.URL != tt.wantURL {
				t.Errorf("detail=%q url=%q", row.Detail, row.Target.URL)
			}
		})
	}
}

func TestTaskRowSince(t *testing.T) {
	t.Parallel()
	created := iso(testNow.Add(-time.Hour))
	started := iso(testNow.Add(-time.Minute))
	noStart := obj{"metadata": obj{"name": "t", "creationTimestamp": created}, "status": obj{"phase": "Running"}}
	withStart := obj{"metadata": obj{"name": "t", "creationTimestamp": created},
		"status": obj{"phase": "Running", "startTime": started}}
	tests := []struct {
		name string
		task obj
		want time.Time
	}{
		{"start time wins", withStart, testNow.Add(-time.Minute)},
		{"falls back to creation", noStart, testNow.Add(-time.Hour)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row, _ := testKelos().taskRow(decode[kelosTask](t, tt.task), nil, testNow)
			if !row.Since.Equal(tt.want) {
				t.Errorf("since = %s, want %s", row.Since, tt.want)
			}
		})
	}
}

func TestKelosTasksFetch(t *testing.T) {
	t.Parallel()
	pods := obj{"items": []obj{{"metadata": obj{"name": testPodName}, "status": obj{
		"initContainerStatuses": []obj{{"name": waitForBlockers, "state": obj{"running": obj{}}}}}}}}
	tasks := obj{"items": []obj{
		taskObj("Running", nil, nil),
		{"metadata": obj{"name": "old"}, "status": obj{"phase": "Succeeded",
			"completionTime": iso(testNow.Add(-2 * time.Hour))}},
	}}
	run := &fakeRunner{replies: map[string]string{
		"get pods -l kelos.dev/component=task": string(mustJSON(t, pods)),
		"get tasks.kelos.dev":                  string(mustJSON(t, tasks)),
	}}
	k := testKelos()
	k.Run = run.run
	rows, err := k.Tasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].State != stateBlocked {
		t.Fatalf("rows = %+v", rows)
	}
	k.Run = (&fakeRunner{}).run
	if _, err := k.Tasks(context.Background()); err == nil {
		t.Error("expected a runner error")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
