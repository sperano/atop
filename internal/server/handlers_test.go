package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sperano/vigie/internal/board"
	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

const (
	prPath      = "/api/v1/hidden/pr/vigie%233"
	refreshURL  = "/api/v1/refresh"
	errRowPath  = "/api/v1/hidden/" + mirrorSource + "/-"
	snapshotURL = "/api/v1/snapshot"
)

// do sends one request to the server's handler.
func do(t *testing.T, srv *Server, method, target string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec.Result()
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSnapshotBeforeTheFirstRefresh(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	resp := do(t, h.srv, http.MethodGet, snapshotURL)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("status, type = %d, %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	const want = `{"fetched_at":null,"refreshing":false,"error":"","counts":{"need_you":0,"busy":0,"idle":0,"hidden":0},"rows":[]}` + "\n"
	if got := body(t, resp); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestSnapshotRowJSON(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows()[1:2])
	const want = `{"fetched_at":"2026-11-15T12:00:00Z","refreshing":false,"error":"",` +
		`"counts":{"need_you":1,"busy":0,"idle":0,"hidden":0},"rows":[{"source":"pr","name":"vigie#3","origin":"github",` +
		`"depth":0,"guides":"","mark":"●","state":"review","since":"2026-11-15T11:00:00Z","age":"1h","detail":"Serve the board",` +
		`"target":"` + prURL + `","severity":"attention","needs_you":true,"busy":false,"hidden":false}]}` + "\n"
	if got := body(t, do(t, h.srv, http.MethodGet, snapshotURL)); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestSnapshotNestsAndSorts(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	snap := decodeSnapshot(t, do(t, h.srv, http.MethodGet, snapshotURL))
	want := []Row{
		{Source: source.SourceVikunja, Name: "#7", Depth: 0, Target: taskURL, Severity: board.SeverityQuiet},
		{Source: source.SourcePR, Name: "vigie#3", Depth: 1, Guides: "└─ ", Mark: board.MarkNeedsYou, Target: prURL, Severity: board.SeverityAttention},
		{Source: source.SourceKelosTask, Name: "fix", Depth: 2, Guides: "   └─ ", Severity: board.SeverityBusy, Busy: true},
		{Source: mirrorSource, Name: "-", Depth: 0, Mark: board.MarkNeedsYou, Severity: board.SeverityFailed},
		{Source: source.SourceKelosSession, Name: "eric", Depth: 0, Severity: board.SeverityQuiet},
	}
	if len(snap.Rows) != len(want) {
		t.Fatalf("rows = %q, want %d", rowNames(snap), len(want))
	}
	for i, w := range want {
		g := snap.Rows[i]
		if g.Source != w.Source || g.Name != w.Name || g.Depth != w.Depth || g.Guides != w.Guides ||
			g.Mark != w.Mark || g.Target != w.Target || g.Severity != w.Severity || g.Busy != w.Busy {
			t.Errorf("row %d = %+v\nwant   %+v", i, g, w)
		}
	}
	if want := (Counts{NeedYou: 2, Busy: 1, Idle: 2}); snap.Counts != want {
		t.Errorf("counts = %+v, want %+v", snap.Counts, want)
	}
}

func TestShowHidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	if err := h.file.Save(hidden.New(hidden.Entry{Source: source.SourceKelosSession, Name: "eric"})); err != nil {
		t.Fatal(err)
	}
	h.refreshWith(testRows())
	tests := []struct {
		query      string
		wantListed bool
	}{
		{"", false},
		{"?show_hidden=false", false},
		{"?show_hidden=true", true},
		{"?show_hidden=1", true},
	}
	for _, tt := range tests {
		snap := decodeSnapshot(t, do(t, h.srv, http.MethodGet, snapshotURL+tt.query))
		i := slices.IndexFunc(snap.Rows, func(r Row) bool { return r.Name == "eric" })
		if listed := i >= 0; listed != tt.wantListed {
			t.Errorf("%q: hidden row listed = %v, want %v", tt.query, listed, tt.wantListed)
			continue
		}
		if tt.wantListed && (!snap.Rows[i].Hidden || snap.Rows[i].Mark != board.MarkHidden) {
			t.Errorf("%q: hidden row = %+v, want hidden and marked %q", tt.query, snap.Rows[i], board.MarkHidden)
		}
		if snap.Counts.Hidden != 1 || snap.Counts.Idle != 1 {
			t.Errorf("%q: counts = %+v, want 1 hidden and the other idle row", tt.query, snap.Counts)
		}
	}
}

func TestBadShowHidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	for _, target := range []string{snapshotURL, "/api/v1/events", "/api/v1/refresh"} {
		method := http.MethodGet
		if target == "/api/v1/refresh" {
			method = http.MethodPost
		}
		resp := do(t, h.srv, method, target+"?show_hidden=maybe")
		const want = `{"error":"show_hidden must be true or false"}` + "\n"
		if got := body(t, resp); resp.StatusCode != http.StatusBadRequest || got != want {
			t.Errorf("%s %s = %d %s, want 400 %s", method, target, resp.StatusCode, got, want)
		}
	}
	if got := h.fetch.calls.Load(); got != 0 {
		t.Errorf("a bad refresh request fetched %d times", got)
	}
}

func TestWrongMethod(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	tests := []struct{ method, target, allow string }{
		{http.MethodGet, "/api/v1/refresh", http.MethodPost},
		{http.MethodPost, snapshotURL, "GET, HEAD"},
		{http.MethodPost, prPath, "DELETE, PUT"},
	}
	for _, tt := range tests {
		resp := do(t, h.srv, tt.method, tt.target)
		if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != tt.allow {
			t.Errorf("%s %s = %d (Allow %q), want 405 (Allow %q)", tt.method, tt.target, resp.StatusCode, resp.Header.Get("Allow"), tt.allow)
		}
	}
}

func TestHealthAndReadiness(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	if resp := do(t, h.srv, http.MethodGet, "/healthz"); resp.StatusCode != http.StatusOK {
		t.Errorf("healthz = %d, want 200", resp.StatusCode)
	}
	resp := do(t, h.srv, http.MethodGet, "/readyz")
	if got, want := body(t, resp), `{"error":"`+errNotReady+`"}`+"\n"; resp.StatusCode != http.StatusServiceUnavailable || got != want {
		t.Errorf("readyz before the first refresh = %d %s, want 503 %s", resp.StatusCode, got, want)
	}
	h.refreshWith(nil)
	if resp := do(t, h.srv, http.MethodGet, "/readyz"); resp.StatusCode != http.StatusOK {
		t.Errorf("readyz after the first refresh = %d, want 200", resp.StatusCode)
	}
}

func TestPostRefreshStartsOrJoins(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	resp := do(t, h.srv, http.MethodPost, "/api/v1/refresh")
	if resp.StatusCode != http.StatusAccepted || !decodeSnapshot(t, resp).Refreshing {
		t.Errorf("first POST = %d, want 202 and a refresh running", resp.StatusCode)
	}
	<-h.fetch.started
	resp = do(t, h.srv, http.MethodPost, "/api/v1/refresh")
	if resp.StatusCode != http.StatusAccepted || !decodeSnapshot(t, resp).Refreshing {
		t.Errorf("second POST = %d, want 202 and the refresh still running", resp.StatusCode)
	}
	done := h.srv.Refresh() // the running one
	h.fetch.rows <- testRows()
	<-done
	if got := h.fetch.calls.Load(); got != 1 {
		t.Errorf("fetches = %d, want 1", got)
	}
}

func TestPostRefreshCooldown(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	const halfSecond = time.Second / 2
	tests := []struct {
		advance    time.Duration
		retryAfter string
	}{
		{0, "10"},
		{DefaultRefreshCooldown - time.Second - halfSecond, "2"}, // 1.5 s left, rounded up
	}
	for _, tt := range tests {
		h.clock.advance(tt.advance)
		resp := do(t, h.srv, http.MethodPost, refreshURL)
		if got, want := body(t, resp), `{"error":"`+errTooSoon+`"}`+"\n"; resp.StatusCode != http.StatusTooManyRequests || got != want {
			t.Errorf("POST after %s = %d %s, want 429 %s", tt.advance, resp.StatusCode, got, want)
		}
		if got := resp.Header.Get("Retry-After"); got != tt.retryAfter {
			t.Errorf("Retry-After = %q, want %q", got, tt.retryAfter)
		}
	}
	if got := h.fetch.calls.Load(); got != 1 {
		t.Errorf("fetches during the cooldown = %d, want only the first refresh", got)
	}
	h.clock.advance(time.Second + halfSecond)
	if resp := do(t, h.srv, http.MethodPost, refreshURL); resp.StatusCode != http.StatusAccepted {
		t.Errorf("POST after the cooldown = %d, want 202", resp.StatusCode)
	}
	done := h.srv.Refresh() // the one the POST started
	<-h.fetch.started
	h.fetch.rows <- testRows()
	<-done
}

func TestPostRefreshShowHidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	if resp := do(t, h.srv, http.MethodPut, prPath); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT = %d", resp.StatusCode)
	}
	for _, tt := range []struct {
		query      string
		wantListed bool
	}{{"", false}, {"?show_hidden=true", true}} {
		h.clock.advance(DefaultRefreshCooldown)
		resp := do(t, h.srv, http.MethodPost, refreshURL+tt.query)
		snap := decodeSnapshot(t, resp)
		i := slices.IndexFunc(snap.Rows, func(r Row) bool { return r.Name == "vigie#3" })
		if resp.StatusCode != http.StatusAccepted || (i >= 0) != tt.wantListed || (tt.wantListed && !snap.Rows[i].Hidden) {
			t.Errorf("POST %q = %d, rows %q; want 202, hidden PR listed (and hidden) = %v", tt.query, resp.StatusCode, rowNames(snap), tt.wantListed)
		}
		done := h.srv.Refresh()
		<-h.fetch.started
		h.fetch.rows <- testRows()
		<-done
	}
}

func TestHideAndUnhidePersist(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	if resp := do(t, h.srv, http.MethodPut, prPath); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT = %d %s, want 204", resp.StatusCode, body(t, resp))
	}
	if got := storedEntries(t, h.file); !slices.Equal(got, []hidden.Entry{{Source: source.SourcePR, Name: "vigie#3"}}) {
		t.Errorf("stored after PUT = %v", got)
	}
	snap := h.srv.snapshot(false)
	if snap.Counts.Hidden != 1 || slices.Contains(rowNames(snap), "pr vigie#3") {
		t.Errorf("after PUT: rows %q, counts %+v; want the PR hidden", rowNames(snap), snap.Counts)
	}
	if resp := do(t, h.srv, http.MethodPut, prPath); resp.StatusCode != http.StatusNoContent {
		t.Errorf("second PUT = %d, want 204", resp.StatusCode)
	}
	if resp := do(t, h.srv, http.MethodDelete, prPath); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE = %d %s, want 204", resp.StatusCode, body(t, resp))
	}
	if got := storedEntries(t, h.file); len(got) != 0 {
		t.Errorf("stored after DELETE = %v, want none", got)
	}
	if resp := do(t, h.srv, http.MethodDelete, prPath); resp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE of a shown row = %d, want 204", resp.StatusCode)
	}
}

func TestHideASourceWithASpace(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	if resp := do(t, h.srv, http.MethodPut, "/api/v1/hidden/kelos%20task/fix"); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT = %d %s, want 204", resp.StatusCode, body(t, resp))
	}
	if got := storedEntries(t, h.file); !slices.Equal(got, []hidden.Entry{{Source: source.SourceKelosTask, Name: "fix"}}) {
		t.Errorf("stored = %v", got)
	}
}

func TestHideRefusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		refresh bool
		target  string
		status  int
		message string
	}{
		{"before the first refresh", false, prPath, http.StatusNotFound, errNoRow},
		{"unknown row", true, "/api/v1/hidden/pr/vigie%2399", http.StatusNotFound, errNoRow},
		{"source error row", true, errRowPath, http.StatusConflict, errCannotHide},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, nil)
			if tt.refresh {
				h.refreshWith(testRows())
			}
			resp := do(t, h.srv, http.MethodPut, tt.target)
			if got, want := body(t, resp), `{"error":"`+tt.message+`"}`+"\n"; resp.StatusCode != tt.status || got != want {
				t.Errorf("PUT = %d %s, want %d %s", resp.StatusCode, got, tt.status, want)
			}
			if _, err := os.Stat(h.file.Path); !os.IsNotExist(err) {
				t.Errorf("a refused PUT wrote the hidden file (stat: %v)", err)
			}
		})
	}
}

func TestHideWithAnUnparsableFile(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	const garbage = "not json"
	if err := os.WriteFile(h.file.Path, []byte(garbage), 0o600); err != nil {
		t.Fatal(err)
	}
	resp := do(t, h.srv, http.MethodPut, prPath)
	wantPrefix := `{"error":"hidden rows: ` + h.file.Path + `: invalid character`
	if got := body(t, resp); resp.StatusCode != http.StatusInternalServerError || !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("PUT = %d %s, want 500 starting %s", resp.StatusCode, got, wantPrefix)
	}
	if data, err := os.ReadFile(h.file.Path); err != nil || string(data) != garbage {
		t.Errorf("file = %q (%v), want it left as %q", data, err, garbage)
	}
	snap := h.srv.snapshot(false)
	if !strings.HasPrefix(snap.Error, "hidden rows: ") || snap.Counts.Hidden != 0 {
		t.Errorf("snapshot error %q, %d hidden; want the store error and nothing hidden", snap.Error, snap.Counts.Hidden)
	}
	if err := os.Remove(h.file.Path); err != nil {
		t.Fatal(err)
	}
	if resp := do(t, h.srv, http.MethodPut, prPath); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT once the file is gone = %d %s, want 204", resp.StatusCode, body(t, resp))
	}
	if snap := h.srv.snapshot(false); snap.Error != "" || snap.Counts.Hidden != 1 {
		t.Errorf("after a working PUT: error %q, %d hidden; want no error and 1 hidden", snap.Error, snap.Counts.Hidden)
	}
}

func storedEntries(t *testing.T, f hidden.File) []hidden.Entry {
	t.Helper()
	set, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	return set.Entries()
}
