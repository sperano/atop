package server

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

func TestRefreshJoinsTheRunningOne(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	first := h.srv.Refresh()
	<-h.fetch.started
	if second := h.srv.Refresh(); second != first {
		t.Error("a second Refresh while one runs started another")
	}
	if !h.srv.snapshot(false).Refreshing {
		t.Error("snapshot does not say a refresh runs")
	}
	h.fetch.rows <- testRows()
	<-first
	if got := h.fetch.calls.Load(); got != 1 {
		t.Errorf("fetches = %d, want 1", got)
	}
	if h.srv.snapshot(false).Refreshing {
		t.Error("snapshot still says a refresh runs")
	}
	next := h.srv.Refresh()
	if next == first {
		t.Error("a Refresh after the last one ended joined it")
	}
	<-h.fetch.started
	h.fetch.rows <- nil
	<-next
	if got := h.fetch.calls.Load(); got != 2 {
		t.Errorf("fetches = %d, want 2", got)
	}
}

func TestRefreshBoundsTheFetch(t *testing.T) {
	t.Parallel()
	const timeout = 3 * time.Hour
	h := newHarness(t, func(o *Options) { o.FetchTimeout = timeout })
	before := time.Now()
	done := h.srv.Refresh()
	ctx := <-h.fetch.started
	after := time.Now()
	deadline, ok := ctx.Deadline()
	h.fetch.rows <- nil
	<-done
	if !ok {
		t.Fatal("the fetch has no deadline")
	}
	if deadline.Before(before.Add(timeout)) || deadline.After(after.Add(timeout)) {
		t.Errorf("deadline %v, want %s after the refresh started (%v to %v)", deadline, timeout, before, after)
	}
}

func TestRefreshAdoptsRowsAndTime(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	if h.srv.ready() {
		t.Error("ready before the first refresh")
	}
	h.refreshWith(testRows())
	snap := h.srv.snapshot(false)
	if snap.FetchedAt == nil || !snap.FetchedAt.Equal(testNow) {
		t.Errorf("FetchedAt = %v, want %v", snap.FetchedAt, testNow)
	}
	if len(snap.Rows) != len(testRows()) {
		t.Errorf("rows = %q, want %d", rowNames(snap), len(testRows()))
	}
	if !h.srv.ready() {
		t.Error("not ready after the first refresh")
	}
}

func TestCloseCancelsTheRefreshAndKeepsTheRows(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	done := h.srv.Refresh()
	ctx := <-h.fetch.started
	h.srv.Close()
	<-ctx.Done()
	<-done
	snap := h.srv.snapshot(false)
	if snap.Refreshing {
		t.Error("still refreshing after the cancelled refresh ended")
	}
	if want := len(testRows()); len(snap.Rows) != want {
		t.Errorf("rows = %q, want the %d rows of the last full refresh", rowNames(snap), want)
	}
}

func TestRunRefreshesOnEveryTick(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	changed, err := h.srv.events.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		h.srv.Run(ctx)
		close(stopped)
	}()
	<-h.fetch.started // the refresh at start
	h.fetch.rows <- nil
	h.waitRefreshEnd(changed) // a tick while it runs would join it
	h.ticks.fire(testInterval)
	<-h.fetch.started
	h.fetch.rows <- testRows()
	cancel()
	<-stopped
	if got := h.fetch.calls.Load(); got != 2 {
		t.Errorf("fetches = %d, want 2", got)
	}
}

func TestRunStopsWhenClosed(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	stopped := make(chan struct{})
	go func() {
		h.srv.Run(context.Background())
		close(stopped)
	}()
	<-h.fetch.started
	h.srv.Close()
	<-stopped
}

func TestRefreshPrunesTheHiddenFile(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	kept := hidden.Entry{Source: source.SourcePR, Name: "vigie#3"}
	gone := hidden.Entry{Source: source.SourcePR, Name: "vigie#1"}
	if err := h.file.Save(hidden.New(kept, gone)); err != nil {
		t.Fatal(err)
	}
	h.refreshWith(testRows())
	stored, err := h.file.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.Entries(); !slices.Equal(got, []hidden.Entry{kept}) {
		t.Errorf("stored = %v, want only %v", got, kept)
	}
	if snap := h.srv.snapshot(false); snap.Counts.Hidden != 1 || snap.Error != "" {
		t.Errorf("hidden count, error = %d, %q; want 1, empty", snap.Counts.Hidden, snap.Error)
	}
}

func TestRefreshKeepsAnUnparsableHiddenFile(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	const garbage = "not json"
	if err := os.WriteFile(h.file.Path, []byte(garbage), 0o600); err != nil {
		t.Fatal(err)
	}
	h.refreshWith(testRows())
	data, err := os.ReadFile(h.file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != garbage {
		t.Errorf("file = %q, want it left as %q", data, garbage)
	}
	snap := h.srv.snapshot(false)
	if want := "hidden rows: " + h.file.Path + ": invalid character"; len(snap.Error) < len(want) || snap.Error[:len(want)] != want {
		t.Errorf("Error = %q, want it to start with %q", snap.Error, want)
	}
	if len(snap.Rows) != len(testRows()) {
		t.Errorf("rows = %q, want every row shown", rowNames(snap))
	}
}

func TestMemoryHiddenRowsWithoutAStore(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(o *Options) { o.UpdateHidden = nil })
	h.refreshWith(testRows())
	pr := testRows()[1]
	if err := h.srv.applyHidden(func(s hidden.Set) hidden.Set { return s.Hide(pr) }, false); err != nil {
		t.Fatal(err)
	}
	if snap := h.srv.snapshot(false); snap.Counts.Hidden != 1 {
		t.Errorf("hidden count = %d, want 1", snap.Counts.Hidden)
	}
	if _, err := os.Stat(h.file.Path); !os.IsNotExist(err) {
		t.Errorf("a store-less server wrote %s (stat: %v)", h.file.Path, err)
	}
}

func TestRefreshKeepsHiddenRowsOfUnreadSources(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	local := hidden.Entry{Source: source.SourceLocal, Name: "laptop"}
	gone := hidden.Entry{Source: source.SourcePR, Name: "vigie#1"}
	if err := h.file.Save(hidden.New(local, gone)); err != nil {
		t.Fatal(err)
	}
	h.refreshWith(testRows())
	if got := storedEntries(t, h.file); !slices.Equal(got, []hidden.Entry{local}) {
		t.Errorf("stored = %v, want only the local entry %v", got, local)
	}
}

func TestRefreshPrunesInMemoryWhenTheStoreFails(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.refreshWith(testRows())
	gone := source.Row{Source: source.SourcePR, Name: "vigie#1"}
	for _, r := range []source.Row{testRows()[1], gone} {
		if err := h.srv.applyHidden(func(s hidden.Set) hidden.Set { return s.Hide(r) }, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(h.file.Path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.clock.advance(testInterval)
	h.refreshWith(testRows())
	snap := h.srv.snapshot(false)
	if snap.Counts.Hidden != 1 || slices.Contains(rowNames(snap), "pr vigie#3") {
		t.Errorf("rows %q, counts %+v; want the PR still hidden", rowNames(snap), snap.Counts)
	}
	if !strings.HasPrefix(snap.Error, "hidden rows: ") {
		t.Errorf("Error = %q, want the store error", snap.Error)
	}
	h.srv.mu.Lock()
	n := h.srv.hidden.Len()
	h.srv.mu.Unlock()
	if n != 1 {
		t.Errorf("hidden entries in memory = %d, want 1 (the gone PR pruned)", n)
	}
}
