package source

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const (
	testHome   = "/Users/x"
	testPane   = "h:@5.%6"
	deadPID    = 999
	livePID    = 1
	otherLive  = 2
	filePerm   = 0o600
	dirPerm    = 0o755
	unixMillis = 1_791_201_600_000 // testNow
)

func localInfo(status string) localSession {
	return localSession{
		PID: livePID, Name: "hollingsworth-f0", Cwd: "/Users/x/code/h", Tmux: testPane,
		Status: status, StatusUpdatedAt: unixMillis,
	}
}

func TestLocalRow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		session    localSession
		home       string
		wantState  string
		wantBusy   bool
		wantNeeds  bool
		wantName   string
		wantDetail string
	}{
		{"busy", localInfo("busy"), testHome, "busy", true, false, "hollingsworth-f0", "~/code/h tmux " + testPane},
		{"idle", localInfo("idle"), testHome, "idle", false, false, "hollingsworth-f0", "~/code/h tmux " + testPane},
		{"unknown status needs you", localInfo("waiting"), testHome, "waiting", false, true, "hollingsworth-f0", "~/code/h tmux " + testPane},
		{"empty status is unknown and needs you", localInfo(""), testHome, "unknown", false, true, "hollingsworth-f0", "~/code/h tmux " + testPane},
		{"cwd outside home untouched", localSession{PID: livePID, Cwd: "/srv/app", Status: "idle"}, testHome, "idle", false, false, "1", "/srv/app"},
		{"home itself becomes tilde", localSession{PID: livePID, Cwd: testHome, Status: "idle"}, testHome, "idle", false, false, "1", "~"},
		{"sibling prefix is not home", localSession{PID: livePID, Cwd: "/Users/xavier/a", Status: "idle"}, testHome, "idle", false, false, "1", "/Users/xavier/a"},
		{"empty home leaves cwd", localSession{PID: livePID, Cwd: "/a", Status: "idle"}, "", "idle", false, false, "1", "/a"},
		{"no cwd no tmux", localSession{PID: livePID, Status: "idle"}, testHome, "idle", false, false, "1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := localRow(tt.session, tt.home)
			if row.Source != SourceLocal || row.Name != tt.wantName {
				t.Errorf("identity = %q/%q", row.Source, row.Name)
			}
			if row.State != tt.wantState || row.Busy != tt.wantBusy || row.NeedsYou != tt.wantNeeds {
				t.Errorf("state=%q busy=%v needs=%v", row.State, row.Busy, row.NeedsYou)
			}
			if row.Detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", row.Detail, tt.wantDetail)
			}
			if row.Target.TmuxPane != tt.session.Tmux {
				t.Errorf("pane = %q", row.Target.TmuxPane)
			}
		})
	}
}

func TestLocalRowSince(t *testing.T) {
	t.Parallel()
	withStatus := localInfo("busy")
	updatedOnly := localSession{PID: livePID, UpdatedAt: unixMillis}
	none := localSession{PID: livePID}
	if got := localRow(withStatus, testHome).Since; !got.Equal(testNow) {
		t.Errorf("statusUpdatedAt: got %s, want %s", got, testNow)
	}
	if got := localRow(updatedOnly, testHome).Since; !got.Equal(testNow) {
		t.Errorf("updatedAt fallback: got %s, want %s", got, testNow)
	}
	if got := localRow(none, testHome).Since; !got.IsZero() {
		t.Errorf("no timestamps: got %s, want zero", got)
	}
}

func writeSession(t *testing.T, home, file, content string) {
	t.Helper()
	dir := filepath.Join(home, LocalSessionsDir)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), filePerm); err != nil {
		t.Fatal(err)
	}
}

func TestLocalSessionsFetch(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	writeSession(t, home, "1.json", `{"pid":1,"name":"alive","status":"busy"}`)
	writeSession(t, home, "2.json", `{"pid":2,"status":"idle"}`)
	writeSession(t, home, strconv.Itoa(deadPID)+".json", `{"pid":999,"name":"dead","status":"busy"}`)
	writeSession(t, home, "zero.json", `{"pid":0,"name":"zero"}`)
	writeSession(t, home, "neg.json", `{"pid":-4,"name":"neg"}`)
	writeSession(t, home, "nopid.json", `{"name":"nopid"}`)
	writeSession(t, home, "partial.json", `{"pid":1,"na`)
	writeSession(t, home, "notes.txt", `{"pid":1,"name":"ignored"}`)
	l := LocalSessions{Home: home, Alive: func(pid int) bool { return pid == livePID || pid == otherLive }}

	rows, err := l.Fetch()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	if len(names) != 2 || names[0] != "alive" || names[1] != "2" {
		t.Errorf("names = %v, want [alive 2]", names)
	}
}

func TestLocalSessionsFetchNoDir(t *testing.T) {
	t.Parallel()
	rows, err := LocalSessions{Home: t.TempDir(), Alive: PIDAlive}.Fetch()
	if err != nil || len(rows) != 0 {
		t.Errorf("rows=%v err=%v", rows, err)
	}
}

func TestPIDAlive(t *testing.T) {
	t.Parallel()
	if !PIDAlive(os.Getpid()) {
		t.Error("own pid must be alive")
	}
	const impossiblePID = 1 << 30
	if PIDAlive(impossiblePID) {
		t.Error("impossible pid must not be alive")
	}
}
