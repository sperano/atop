package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

var testNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

const (
	testInterval  = time.Hour
	taskURL       = "https://vikunja.example/tasks/7"
	prURL         = "https://github.com/o/vigie/pull/3"
	hiddenFileTmp = "hidden.json"
	mirrorSource  = "mirror" // a source that fails in testRows
)

// testRows are a Vikunja task, its PR that needs you, a Kelos Task on the
// PR, an idle session and a source error.
func testRows() []source.Row {
	return []source.Row{
		{Source: source.SourceKelosSession, Origin: source.OriginKelos, Name: "eric", State: source.StateIdle, Since: testNow.Add(-3 * time.Hour)},
		{Source: source.SourcePR, Origin: source.OriginGitHub, Name: "vigie#3", State: "review", Since: testNow.Add(-time.Hour),
			Detail: "Serve the board", NeedsYou: true, Target: source.Target{URL: prURL}, Ref: "pr:3", ParentRefs: []string{"vikunja:7"}},
		{Source: source.SourceVikunja, Origin: source.OriginVikunja, Name: "#7", State: source.StateIdle, Since: testNow.Add(-2 * time.Hour),
			Detail: "[claude] serve", Target: source.Target{URL: taskURL}, Ref: "vikunja:7"},
		{Source: source.SourceKelosTask, Name: "fix", State: source.StateBusy, Busy: true, Since: testNow.Add(-time.Minute), ParentRefs: []string{"pr:3"}},
		source.ErrorRow(mirrorSource, errors.New("down")),
	}
}

// fakeFetch blocks every fetch until the test sends its rows, so that a
// test decides when a refresh ends.
type fakeFetch struct {
	started chan context.Context // receives each fetch's context as it begins
	rows    chan []source.Row
	calls   atomic.Int32
}

func newFakeFetch() *fakeFetch {
	return &fakeFetch{started: make(chan context.Context), rows: make(chan []source.Row)}
}

func (f *fakeFetch) fetch(ctx context.Context) []source.Row {
	f.calls.Add(1)
	select {
	case f.started <- ctx:
	case <-ctx.Done():
		return []source.Row{source.ErrorRow(source.SourcePR, ctx.Err())}
	}
	select {
	case rows := <-f.rows:
		return rows
	case <-ctx.Done():
		return []source.Row{source.ErrorRow(source.SourcePR, ctx.Err())}
	}
}

// fakeTicks hands out one unbuffered channel per interval, fired by the test.
type fakeTicks struct {
	mu    sync.Mutex
	chans map[time.Duration]chan time.Time
}

func (f *fakeTicks) tick(d time.Duration) (<-chan time.Time, func()) {
	return f.channel(d), func() {}
}

func (f *fakeTicks) channel(d time.Duration) chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chans == nil {
		f.chans = make(map[time.Duration]chan time.Time)
	}
	if f.chans[d] == nil {
		f.chans[d] = make(chan time.Time)
	}
	return f.chans[d]
}

func (f *fakeTicks) fire(d time.Duration) {
	f.channel(d) <- testNow
}

type harness struct {
	srv   *Server
	fetch *fakeFetch
	ticks *fakeTicks
	file  hidden.File
}

// newHarness returns a server on a fake fetcher, fake ticks and a hidden
// file in a temporary directory; edit adjusts its options.
func newHarness(t *testing.T, edit func(*Options)) *harness {
	t.Helper()
	h := &harness{fetch: newFakeFetch(), ticks: &fakeTicks{}, file: hidden.File{Path: filepath.Join(t.TempDir(), hiddenFileTmp)}}
	opts := Options{
		Fetch: h.fetch.fetch, Now: func() time.Time { return testNow }, Interval: testInterval,
		UpdateHidden: h.file.Update, Tick: h.ticks.tick,
	}
	if edit != nil {
		edit(&opts)
	}
	h.srv = New(opts)
	t.Cleanup(h.srv.Close)
	return h
}

// refreshWith runs one refresh to its end with rows.
func (h *harness) refreshWith(rows []source.Row) {
	done := h.srv.Refresh()
	<-h.fetch.started
	h.fetch.rows <- rows
	<-done
}

// waitRefreshEnd waits on a broker subscription until no refresh runs.
func (h *harness) waitRefreshEnd(changed chan struct{}) {
	for range changed {
		if !h.srv.snapshot(false).Refreshing {
			return
		}
	}
}

// readFrame reads one SSE frame, up to its blank line.
func readFrame(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	var b strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading frame after %q: %v", b.String(), err)
		}
		b.WriteString(line)
		if line == "\n" {
			return b.String()
		}
	}
}

// frameSnapshot decodes a snapshot event frame.
func frameSnapshot(t *testing.T, frame string) Snapshot {
	t.Helper()
	data, ok := strings.CutPrefix(frame, "event: "+snapshotEvent+"\ndata: ")
	if !ok || !strings.HasSuffix(data, "\n\n") {
		t.Fatalf("frame = %q, want a snapshot event", frame)
	}
	var snap Snapshot
	if err := json.Unmarshal([]byte(strings.TrimSuffix(data, "\n\n")), &snap); err != nil {
		t.Fatalf("frame data: %v", err)
	}
	return snap
}

func decodeSnapshot(t *testing.T, resp *http.Response) Snapshot {
	t.Helper()
	var snap Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	return snap
}

func rowNames(snap Snapshot) []string {
	names := make([]string, 0, len(snap.Rows))
	for _, r := range snap.Rows {
		names = append(names, r.Source+" "+r.Name)
	}
	return names
}
