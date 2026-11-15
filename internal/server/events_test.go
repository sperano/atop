package server

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const eventsURL = "/api/v1/events"

// streamWriter hands each write of an event stream to the test, which reads
// them from frames; with fail set, every write fails.
type streamWriter struct {
	header    http.Header
	frames    chan string
	deadlines atomic.Int32
	fail      error
}

func newStreamWriter() *streamWriter {
	return &streamWriter{header: make(http.Header), frames: make(chan string)}
}

func (w *streamWriter) Header() http.Header { return w.header }
func (w *streamWriter) WriteHeader(int)     {}
func (w *streamWriter) Flush()              {}

func (w *streamWriter) Write(p []byte) (int, error) {
	if w.fail != nil {
		return 0, w.fail
	}
	w.frames <- string(p)
	return len(p), nil
}

func (w *streamWriter) SetWriteDeadline(time.Time) error {
	w.deadlines.Add(1)
	return nil
}

// stream serves an event stream to w until ctx ends; the returned channel
// closes when the handler returns.
func (h *harness) stream(ctx context.Context, w http.ResponseWriter, query string) <-chan struct{} {
	done := make(chan struct{})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, eventsURL+query, nil)
	go func() {
		h.srv.Handler().ServeHTTP(w, req)
		close(done)
	}()
	return done
}

func TestEventsOverHTTP(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	ts := httptest.NewServer(h.srv.Handler())
	t.Cleanup(ts.Close)
	t.Cleanup(h.srv.Close) // ends the stream before ts.Close waits on it
	resp, err := http.Get(ts.URL + eventsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); resp.StatusCode != http.StatusOK || got != "text/event-stream" {
		t.Fatalf("status, type = %d, %q; want 200, text/event-stream", resp.StatusCode, got)
	}
	r := bufio.NewReader(resp.Body)
	if snap := frameSnapshot(t, readFrame(t, r)); snap.FetchedAt != nil || snap.Refreshing || len(snap.Rows) != 0 {
		t.Errorf("first event = %+v, want the empty snapshot", snap)
	}
	done := h.srv.Refresh()
	<-h.fetch.started
	if snap := frameSnapshot(t, readFrame(t, r)); !snap.Refreshing {
		t.Error("no event says the refresh started")
	}
	h.fetch.rows <- testRows()
	<-done
	if snap := frameSnapshot(t, readFrame(t, r)); snap.Refreshing || len(snap.Rows) != len(testRows()) {
		t.Errorf("event after the refresh: refreshing %v, %d rows; want false, %d", snap.Refreshing, len(snap.Rows), len(testRows()))
	}
	h.ticks.fire(KeepaliveInterval)
	if got := readFrame(t, r); got != keepaliveFrame {
		t.Errorf("keepalive frame = %q, want %q", got, keepaliveFrame)
	}
	if resp := do(t, h.srv, http.MethodPut, prPath); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT = %d", resp.StatusCode)
	}
	if snap := frameSnapshot(t, readFrame(t, r)); snap.Counts.Hidden != 1 {
		t.Errorf("event after hiding: %d hidden, want 1", snap.Counts.Hidden)
	}
}

func TestEventsShowHidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	if resp := do(t, h.srv, http.MethodPut, prPath); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT = %d", resp.StatusCode)
	}
	for _, tt := range []struct {
		query string
		rows  int
	}{{"", len(testRows()) - 1}, {"?show_hidden=true", len(testRows())}} {
		ctx, cancel := context.WithCancel(context.Background())
		w := newStreamWriter()
		done := h.stream(ctx, w, tt.query)
		if snap := frameSnapshot(t, <-w.frames); len(snap.Rows) != tt.rows {
			t.Errorf("%q: %d rows, want %d", tt.query, len(snap.Rows), tt.rows)
		}
		cancel()
		<-done
	}
}

func TestEventStreamLeavesWhenTheClientGoes(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	w := newStreamWriter()
	done := h.stream(ctx, w, "")
	<-w.frames
	if got := h.srv.events.len(); got != 1 {
		t.Errorf("streams while connected = %d, want 1", got)
	}
	cancel()
	<-done
	if got := h.srv.events.len(); got != 0 {
		t.Errorf("streams after the client left = %d, want 0", got)
	}
}

func TestEventStreamLeavesOnAWriteError(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	w := newStreamWriter()
	w.fail = errors.New("broken pipe")
	<-h.stream(context.Background(), w, "")
	if got := h.srv.events.len(); got != 0 {
		t.Errorf("streams after a failed write = %d, want 0", got)
	}
}

func TestEventFramesEachHaveAWriteDeadline(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	w := newStreamWriter()
	done := h.stream(ctx, w, "")
	<-w.frames
	h.ticks.fire(KeepaliveInterval)
	<-w.frames
	cancel()
	<-done
	if got := w.deadlines.Load(); got != 2 {
		t.Errorf("write deadlines = %d, want one per frame (2)", got)
	}
	if got := w.header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestEventStreamLimit(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(o *Options) { o.MaxStreams = 1 })
	ctx, cancel := context.WithCancel(context.Background())
	w := newStreamWriter()
	done := h.stream(ctx, w, "")
	<-w.frames
	resp := do(t, h.srv, http.MethodGet, eventsURL)
	if got, want := body(t, resp), `{"error":"too many event streams"}`+"\n"; resp.StatusCode != http.StatusServiceUnavailable || got != want {
		t.Errorf("second stream = %d %s, want 503 %s", resp.StatusCode, got, want)
	}
	cancel()
	<-done
}

func TestCloseEndsTheStreams(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	w := newStreamWriter()
	done := h.stream(context.Background(), w, "")
	<-w.frames
	h.srv.Close()
	<-done
	resp := do(t, h.srv, http.MethodGet, eventsURL)
	if got, want := body(t, resp), `{"error":"shutting down"}`+"\n"; resp.StatusCode != http.StatusServiceUnavailable || got != want {
		t.Errorf("stream after Close = %d %s, want 503 %s", resp.StatusCode, got, want)
	}
}

func TestBrokerKeepsOneSignalPerStream(t *testing.T) {
	t.Parallel()
	b := newBroker(DefaultMaxStreams)
	ch, err := b.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		b.publish()
	}
	if got := len(ch); got != 1 {
		t.Errorf("pending signals = %d, want 1", got)
	}
}
