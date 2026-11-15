// Package server serves the board over HTTP: a JSON snapshot, a stream of
// Server-Sent Events, refreshes and hidden rows. It has no authentication of
// its own: it is meant to run behind a proxy that does it.
package server

import (
	"context"
	"sync"
	"time"

	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

const (
	// DefaultFetchTimeout bounds one refresh, so that a hung source cannot
	// hold off every later one.
	DefaultFetchTimeout = time.Minute
	// DefaultMaxStreams caps the open event streams.
	DefaultMaxStreams = 64
)

// Options wires the server to its data, clock and hidden-row store.
type Options struct {
	Fetch        func(ctx context.Context) []source.Row // every source's rows, in any order
	Now          func() time.Time
	Interval     time.Duration // between background refreshes
	FetchTimeout time.Duration // zero means DefaultFetchTimeout
	MaxStreams   int           // zero means DefaultMaxStreams
	// UpdateHidden applies a change to the stored hidden rows and returns
	// them as stored; nil keeps them in memory only.
	UpdateHidden func(change func(hidden.Set) hidden.Set) (hidden.Set, error)
	// Tick returns a channel that fires every d and a func that stops it;
	// nil uses a time.Ticker.
	Tick func(d time.Duration) (<-chan time.Time, func())
}

// Server keeps the last snapshot and refreshes it in the background.
type Server struct {
	opts   Options
	events *broker
	// life is the server's lifetime, ended by Close; refreshes run under it
	// rather than under the request that started them.
	life context.Context
	end  context.CancelFunc

	// hiddenMu serialises hidden-row updates, so that they are adopted in
	// the order they were stored.
	hiddenMu sync.Mutex

	mu        sync.Mutex
	rows      []source.Row
	hidden    hidden.Set
	fetchedAt time.Time     // zero until the first refresh ends
	running   chan struct{} // closed when the running refresh ends; nil when none runs
	lastErr   string        // the last hidden-row update's error, empty when it worked
}

// New returns a server with no rows yet; Run or Refresh fetches them.
func New(opts Options) *Server {
	if opts.FetchTimeout <= 0 {
		opts.FetchTimeout = DefaultFetchTimeout
	}
	if opts.MaxStreams <= 0 {
		opts.MaxStreams = DefaultMaxStreams
	}
	if opts.Tick == nil {
		opts.Tick = tick
	}
	life, end := context.WithCancel(context.Background())
	return &Server{opts: opts, events: newBroker(opts.MaxStreams), life: life, end: end}
}

func tick(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Run refreshes now and every Interval until ctx ends or the server closes.
func (s *Server) Run(ctx context.Context) {
	ticks, stop := s.opts.Tick(s.opts.Interval)
	defer stop()
	s.Refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.life.Done():
			return
		case <-ticks:
			s.Refresh()
		}
	}
}

// Refresh starts a refresh, or joins the one running, and returns a channel
// closed when it ends. Two refreshes never run at once.
func (s *Server) Refresh() <-chan struct{} {
	s.mu.Lock()
	if s.running != nil {
		defer s.mu.Unlock()
		return s.running
	}
	done := make(chan struct{})
	s.running = done
	s.mu.Unlock()
	go s.refresh(done)
	s.events.publish()
	return done
}

func (s *Server) refresh(done chan struct{}) {
	ctx, cancel := context.WithTimeout(s.life, s.opts.FetchTimeout)
	rows := s.opts.Fetch(ctx)
	cancel()
	closed := s.life.Err() != nil // the rows are the errors of a cancelled fetch
	if !closed {
		_ = s.applyHidden(func(set hidden.Set) hidden.Set { return set.Prune(rows) }, true)
	}
	s.mu.Lock()
	if !closed {
		s.rows = rows
		s.fetchedAt = s.opts.Now()
	}
	s.running = nil
	s.mu.Unlock()
	close(done)
	s.events.publish()
}

// Close cancels the running refresh and ends the event streams. The server
// still answers requests, for a graceful shutdown.
func (s *Server) Close() {
	s.end()
	s.events.close()
}

// applyHidden applies change to the stored hidden rows and adopts the
// result, recording the outcome as the last error. When the store fails,
// keep says whether the change still holds in memory.
func (s *Server) applyHidden(change func(hidden.Set) hidden.Set, keep bool) error {
	s.hiddenMu.Lock()
	defer s.hiddenMu.Unlock()
	s.mu.Lock()
	current := s.hidden
	s.mu.Unlock()
	var next hidden.Set
	var err error
	if s.opts.UpdateHidden != nil {
		next, err = s.opts.UpdateHidden(change)
	} else {
		next = change(current)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastErr = ""
	if err != nil {
		s.lastErr = "hidden rows: " + err.Error()
		if !keep {
			return err
		}
		next = change(current)
	}
	s.hidden = next
	return err
}

// findRow returns the row of the last refresh with this identity.
func (s *Server) findRow(src, name string) (source.Row, bool) {
	key := source.Row{Source: src, Name: name}.Key()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		if r.Key() == key {
			return r, true
		}
	}
	return source.Row{}, false
}

// ready reports whether the first refresh has ended.
func (s *Server) ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.fetchedAt.IsZero()
}
