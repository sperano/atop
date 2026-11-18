package source

import (
	"context"
	"errors"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	mine := sessionObj("Ready", activeCond("True", testNow), testNow)
	other := sessionObj("Failed", nil, testNow)
	k := testKelos()
	k.Client = fakeKube(t,
		kubeObj(sessionsResource, testNS, mine),
		kubeObj(sessionsResource, "other-ns", other),
	).client(t)
	rows, err := k.Sessions(context.Background())
	if err != nil || len(rows) != 1 || rows[0].Name != "s" || rows[0].State != StateBusy {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}

func TestKelosSessionsFetchErrors(t *testing.T) {
	t.Parallel()
	kube := fakeKube(t)
	kube.failList("sessions")
	k := testKelos()
	k.Client = kube.client(t)
	if _, err := k.Sessions(context.Background()); !apierrors.IsInternalError(err) || err.Error() != "listing sessions.kelos.dev: boom" {
		t.Errorf("list error = %v", err)
	}
	k.Client = failingClient(errBoom)
	if _, err := k.Sessions(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("client error = %v, want %v", err, errBoom)
	}
}
