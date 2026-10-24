package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

type blockingTaskServer struct {
	requests atomic.Int32
	started  chan struct{}
	release  chan struct{}
}

func (s *blockingTaskServer) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	if s.requests.Add(1) == 1 {
		close(s.started)
		<-s.release
	}
	_ = json.NewEncoder(w).Encode([]vikunjaTask{})
}

func TestVikunjaFetchRejectsConcurrentCalls(t *testing.T) {
	t.Parallel()
	handler := &blockingTaskServer{started: make(chan struct{}), release: make(chan struct{})}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		select {
		case <-handler.release:
		default:
			close(handler.release)
		}
	})
	v := testVikunjaSource()
	v.APIURL = server.URL + apiPrefix
	v.Token = testToken
	v.Client = server.Client()

	firstDone := make(chan error, 1)
	go func() {
		_, err := v.Fetch(context.Background())
		firstDone <- err
	}()
	<-handler.started
	if _, err := v.Fetch(context.Background()); !errors.Is(err, errFetchInProgress) {
		t.Fatalf("concurrent Fetch error = %v, want %v", err, errFetchInProgress)
	}
	close(handler.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	if _, err := v.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch after first completed: %v", err)
	}
	if got := handler.requests.Load(); got != 2 {
		t.Errorf("task requests = %d, want 2", got)
	}
}

func TestVikunjaFetchUsesFreshCompleteEmbeddedComment(t *testing.T) {
	t.Parallel()
	count := 1
	s := &vikunjaServer{
		tasks: []vikunjaTask{{
			ID: 1, Updated: iso(testNow), Comments: commentsOf(count, "claude"), CommentCount: &count,
		}},
		comments: map[int][]vikunjaComment{},
	}
	v := newVikunja(t, s)
	rows, err := v.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].State != "reply from claude" {
		t.Fatalf("initial state = %q", rows[0].State)
	}

	s.tasks[0].Comments = commentsOf(count, "eric")
	rows, err = v.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].State != StateIdle {
		t.Errorf("state after embedded comment changed = %q, want %q", rows[0].State, StateIdle)
	}
	if len(s.pages) != 0 {
		t.Errorf("comment requests = %v, want none", s.pages)
	}
}

func TestVikunjaFetchDoesNotRewriteUnchangedCacheEntry(t *testing.T) {
	t.Parallel()
	count := 1
	s := &vikunjaServer{
		tasks: []vikunjaTask{{
			ID: 1, Updated: iso(testNow), Comments: commentsOf(count, "claude"), CommentCount: &count,
		}},
		comments: map[int][]vikunjaComment{},
	}
	v := newVikunja(t, s)
	if _, err := v.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	cached := v.commentCache[1].last
	if _, err := v.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v.commentCache[1].last != cached {
		t.Error("unchanged cache entry was replaced")
	}
}

func TestVikunjaFetchPrunesMissingTasksFromCommentCache(t *testing.T) {
	t.Parallel()
	count := 2
	all := commentsOf(count, "eric", "claude")
	s := &vikunjaServer{
		tasks: []vikunjaTask{{
			ID: 1, Updated: iso(testNow), Comments: all[:1], CommentCount: &count,
		}},
		comments: map[int][]vikunjaComment{1: all},
	}
	v := newVikunja(t, s)
	if _, err := v.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.commentCache[1]; !ok {
		t.Fatal("comment cache has no entry for active task")
	}

	s.tasks = nil
	if _, err := v.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.commentCache[1]; ok {
		t.Error("comment cache retained missing task")
	}

	s.tasks = []vikunjaTask{{
		ID: 1, Updated: iso(testNow), Comments: all[:1], CommentCount: &count,
	}}
	if _, err := v.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(s.pages) != 2 {
		t.Errorf("comment requests = %d, want 2 after task returned", len(s.pages))
	}
}

func TestVikunjaFetchFailureKeepsExistingCommentCache(t *testing.T) {
	t.Parallel()
	count := 2
	all := commentsOf(count, "eric", "claude")
	s := &vikunjaServer{
		tasks: []vikunjaTask{{
			ID: 1, Updated: iso(testNow), Comments: all[:1], CommentCount: &count,
		}},
		comments: map[int][]vikunjaComment{1: all},
	}
	v := newVikunja(t, s)
	if _, err := v.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := v.commentCache[1]

	s.tasks = append(s.tasks, vikunjaTask{ID: 2, Updated: iso(testNow)})
	v.Client = &http.Client{Transport: failingComments{base: v.Client.Transport}}
	if _, err := v.Fetch(context.Background()); err == nil {
		t.Fatal("expected the comment failure to surface")
	}
	got, ok := v.commentCache[1]
	if !ok || got.updated != want.updated || got.count != want.count || !sameComment(got.last, want.last) {
		t.Errorf("existing cache entry changed after failed fetch: got %+v, want %+v", got, want)
	}
	if _, ok := v.commentCache[2]; ok {
		t.Error("failed lookup was cached")
	}
}
