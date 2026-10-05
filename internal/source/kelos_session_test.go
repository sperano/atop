package source

import (
	"context"
	"testing"
	"time"
)

func sessionObj(phase string, conds []obj, created time.Time) obj {
	status := obj{"conditions": conds}
	if phase != "" {
		status["phase"] = phase
	}
	return obj{
		"metadata": obj{"name": "s", "labels": obj{vikunjaProjectLabel: "puckdb"}, "creationTimestamp": iso(created)},
		"status":   status,
	}
}

func activeCond(status string, at time.Time) []obj {
	return []obj{{"type": "Active", "status": status, "lastTransitionTime": iso(at)}}
}

func TestSessionRow(t *testing.T) {
	t.Parallel()
	created := testNow.Add(-24 * time.Hour)
	transitioned := testNow.Add(-time.Minute)
	tests := []struct {
		name      string
		session   obj
		wantState string
		wantBusy  bool
		wantNeeds bool
		wantSince time.Time
	}{
		{"active ready is busy", sessionObj("Ready", activeCond("True", transitioned), created), "busy", true, false, transitioned},
		{"inactive ready is idle", sessionObj("Ready", activeCond("False", transitioned), created), "idle", false, false, transitioned},
		{"ready without condition is idle, since creation", sessionObj("Ready", nil, created), "idle", false, false, created},
		{"failed needs you", sessionObj("Failed", nil, created), "failed", false, true, created},
		{"non-ready phase lowercased", sessionObj("Terminating", nil, created), "terminating", false, false, created},
		{"empty phase is pending", sessionObj("", nil, created), "pending", false, false, created},
		{"condition with no time falls back to creation",
			sessionObj("Ready", []obj{{"type": "Active", "status": "True"}}, created), "busy", true, false, created},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := testKelos().sessionRow(decode[kelosSession](t, tt.session))
			if row.Source != SourceKelosSession || row.Name != "s" || row.Detail != "puckdb" {
				t.Errorf("identity/detail = %q/%q/%q", row.Source, row.Name, row.Detail)
			}
			if row.State != tt.wantState || row.Busy != tt.wantBusy || row.NeedsYou != tt.wantNeeds {
				t.Errorf("state=%q busy=%v needs=%v", row.State, row.Busy, row.NeedsYou)
			}
			if !row.Since.Equal(tt.wantSince) {
				t.Errorf("since = %s, want %s", row.Since, tt.wantSince)
			}
			if row.Target.URL != testConsole {
				t.Errorf("target = %q, want console URL", row.Target.URL)
			}
		})
	}
}

func TestKelosSessionsFetch(t *testing.T) {
	t.Parallel()
	list := obj{"items": []obj{sessionObj("Ready", activeCond("True", testNow), testNow)}}
	run := &fakeRunner{replies: map[string]string{"get sessions.kelos.dev": string(mustJSON(t, list))}}
	k := testKelos()
	k.Run = run.run
	rows, err := k.Sessions(context.Background())
	if err != nil || len(rows) != 1 || rows[0].State != StateBusy {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if len(run.calls) != 1 || run.calls[0] != "kubectl -n kelos-agents get sessions.kelos.dev -o json" {
		t.Errorf("calls = %v", run.calls)
	}
	k.Run = (&fakeRunner{}).run
	if _, err := k.Sessions(context.Background()); err == nil {
		t.Error("expected a runner error")
	}
}
