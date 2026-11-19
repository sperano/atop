package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	// KeepaliveInterval spaces the comments that keep an idle stream open,
	// well under Envoy's five-minute stream idle timeout.
	KeepaliveInterval = 15 * time.Second
	// frameWriteTimeout bounds the write of one frame to a stalled client.
	frameWriteTimeout = 10 * time.Second
	snapshotEvent     = "snapshot"
	keepaliveFrame    = ": keepalive\n\n"
)

// broker tells the open event streams that the snapshot changed. Each stream
// holds at most one pending signal: a slow client skips to the latest
// snapshot instead of queueing every one.
type broker struct {
	mu     sync.Mutex
	subs   map[chan struct{}]struct{}
	max    int
	closed bool
}

func newBroker(maxStreams int) *broker {
	return &broker{subs: make(map[chan struct{}]struct{}), max: maxStreams}
}

// Why a stream is refused.
var (
	errTooManyStreams = errors.New("too many event streams")
	errShuttingDown   = errors.New("shutting down")
)

// subscribe adds a stream, unless the broker is full or closed.
func (b *broker) subscribe() (chan struct{}, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errShuttingDown
	}
	if len(b.subs) >= b.max {
		return nil, errTooManyStreams
	}
	ch := make(chan struct{}, 1)
	b.subs[ch] = struct{}{}
	return ch, nil
}

func (b *broker) unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, ch)
}

func (b *broker) publish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default: // a signal is already pending
		}
	}
}

// close ends every stream and refuses new ones.
func (b *broker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		close(ch)
		delete(b.subs, ch)
	}
}

func (b *broker) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

// handleEvents streams a snapshot event on connection, on every refresh
// start and end and every hidden-row change, until the client leaves, a
// write fails or the server closes.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	showHidden, ok := parseShowHidden(w, r)
	if !ok {
		return
	}
	hideDone, ok := parseHideDone(w, r)
	if !ok {
		return
	}
	changed, err := s.events.subscribe()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	defer s.events.unsubscribe(changed)
	keepalive, stop := s.opts.Tick(KeepaliveInterval)
	defer stop()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	stream := frameWriter{w: w, rc: http.NewResponseController(w)}
	err = stream.snapshot(s.snapshot(showHidden, hideDone))
	for err == nil {
		select {
		case <-r.Context().Done():
			return
		case _, open := <-changed:
			if !open {
				return
			}
			err = stream.snapshot(s.snapshot(showHidden, hideDone))
		case <-keepalive:
			err = stream.write([]byte(keepaliveFrame))
		}
	}
}

// frameWriter writes whole SSE frames, each under its own write deadline.
type frameWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func (f frameWriter) snapshot(snap Snapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return f.write(fmt.Appendf(nil, "event: %s\ndata: %s\n\n", snapshotEvent, data))
}

func (f frameWriter) write(frame []byte) error {
	err := f.rc.SetWriteDeadline(time.Now().Add(frameWriteTimeout))
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := f.w.Write(frame); err != nil {
		return err
	}
	return f.rc.Flush()
}
