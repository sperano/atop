package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const (
	testToken   = "s3cret"
	pagesHeader = "x-pagination-total-pages" // lowercase on the wire
	apiPrefix   = "/api/v1"
)

// vikunjaServer serves tasks and comments and records what it was asked.
type vikunjaServer struct {
	mu       sync.Mutex
	tasks    []vikunjaTask
	projects []vikunjaProject
	// projectPages are the page and is_archived of each project request.
	projectPages []string
	comments     map[int][]vikunjaComment // oldest first
	auths        []string
	pages        []string // "<taskID>:<page>" for each comment request
	orders       []string
	perPages     []string
	filters      []string
	expands      []string
	status       int
	// pageCap, when set, caps per_page like service.maxitemsperpage and
	// sends the total-pages header on task lists.
	pageCap int
}

func (s *vikunjaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auths = append(s.auths, r.Header.Get("Authorization"))
	if s.status != 0 {
		w.WriteHeader(s.status)
		return
	}
	q := r.URL.Query()
	path := strings.TrimPrefix(r.URL.Path, apiPrefix)
	if path == "/projects" {
		s.projectPages = append(s.projectPages, q.Get("page")+" "+q.Get("is_archived"))
		serveSlice(w, s.projects, s.pageCap, q.Get("page"), q.Get("per_page"))
		return
	}
	if path == "/tasks" {
		s.filters = append(s.filters, q.Get("filter"))
		s.expands = append(s.expands, strings.Join(q["expand"], ","))
		s.serveTasks(w, q.Get("page"), q.Get("per_page"))
		return
	}
	var id int
	if _, err := fmt.Sscanf(path, "/tasks/%d/comments", &id); err != nil {
		http.NotFound(w, r)
		return
	}
	s.pages = append(s.pages, fmt.Sprintf("%d:%s", id, q.Get("page")))
	s.orders = append(s.orders, q.Get("order_by"))
	s.perPages = append(s.perPages, q.Get("per_page"))
	s.serveComment(w, id, q.Get("page"), q.Get("per_page"), q.Get("order_by"))
}

func (s *vikunjaServer) serveTasks(w http.ResponseWriter, page, perPage string) {
	serveSlice(w, s.tasks, s.pageCap, page, perPage)
}

// serveSlice serves one page of items, capping per_page at pageCap and
// sending the total-pages header when pageCap is set.
func serveSlice[T any](w http.ResponseWriter, items []T, pageCap int, page, perPage string) {
	p, _ := strconv.Atoi(page)
	n, _ := strconv.Atoi(perPage)
	if pageCap > 0 {
		n = min(n, pageCap)
		w.Header()[pagesHeader] = []string{strconv.Itoa((len(items) + n - 1) / n)}
	}
	lo := min((p-1)*n, len(items))
	hi := min(p*n, len(items))
	_ = json.NewEncoder(w).Encode(items[lo:hi])
}

// serveComment orders newest first like Vikunja: by created, to the
// second, leaving same-second comments oldest first.
func (s *vikunjaServer) serveComment(w http.ResponseWriter, id int, page, perPage, order string) {
	all := s.comments[id]
	if order == newestFirst {
		all = slices.Clone(all)
		slices.SortStableFunc(all, func(a, b vikunjaComment) int {
			return parseTime(b.Created).Compare(parseTime(a.Created))
		})
	}
	p, _ := strconv.Atoi(page)
	n, _ := strconv.Atoi(perPage)
	n = max(n, 1)
	out := []vikunjaComment{}
	if p >= 1 {
		out = append(out, all[min((p-1)*n, len(all)):min(p*n, len(all))]...)
	}
	if len(all) > 0 {
		// Written straight into the map: Go's Header.Set would canonicalise it.
		w.Header()[pagesHeader] = []string{strconv.Itoa((len(all) + n - 1) / n)}
	}
	_ = json.NewEncoder(w).Encode(out)
}

func newVikunja(t *testing.T, s *vikunjaServer) *Vikunja {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	v := testVikunjaSource()
	v.APIURL = srv.URL + apiPrefix
	v.Token = testToken
	v.InProgressLabel = 1
	v.Client = srv.Client()
	return v
}

// lastCommentID numbers test comments in creation order, as Vikunja does.
var lastCommentID atomic.Int64

func commentsOf(n int, authors ...string) []vikunjaComment {
	out := make([]vikunjaComment, n)
	for i := range out {
		out[i].ID = int(lastCommentID.Add(1))
		out[i].Comment = "comment " + strconv.Itoa(i+1)
		out[i].Created = iso(testNow)
		out[i].Author.Username = authors[min(i, len(authors)-1)]
	}
	return out
}

func TestNewestComment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		comments  []vikunjaComment
		want      string // comment text, "" for none
		wantPages []string
	}{
		{"three comments", commentsOf(3, "a"), "comment 3", []string{"1:1"}},
		{"single page", commentsOf(1, "a"), "comment 1", []string{"1:1"}},
		{"empty", nil, "", []string{"1:1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &vikunjaServer{comments: map[int][]vikunjaComment{1: tt.comments}}
			got, err := newVikunja(t, s).newestComment(context.Background(), testToken, 1)
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("got %+v, want nil", got)
			case tt.want != "" && (got == nil || got.Comment != tt.want):
				t.Errorf("got %+v, want %q", got, tt.want)
			}
			if strings.Join(s.pages, ",") != strings.Join(tt.wantPages, ",") {
				t.Errorf("requests = %v, want %v", s.pages, tt.wantPages)
			}
			if len(s.orders) != 1 || s.orders[0] != newestFirst {
				t.Errorf("order_by = %v, want [%s]", s.orders, newestFirst)
			}
			if want := strconv.Itoa(newestCommentWindow); len(s.perPages) != 1 || s.perPages[0] != want {
				t.Errorf("per_page = %v, want [%s]", s.perPages, want)
			}
		})
	}
}

func TestVikunjaFetchUsesCompleteExpandedComments(t *testing.T) {
	t.Parallel()
	count := 2
	s := &vikunjaServer{
		tasks: []vikunjaTask{{
			ID: 1, Title: "expanded", Updated: iso(testNow),
			Comments: commentsOf(count, "eric", "claude"), CommentCount: &count,
		}},
		comments: map[int][]vikunjaComment{1: commentsOf(count, "eric", "claude")},
	}
	rows, err := newVikunja(t, s).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].State != "reply from claude" {
		t.Fatalf("rows = %+v", rows)
	}
	if len(s.pages) != 0 {
		t.Errorf("comment requests = %v, want none", s.pages)
	}
	if len(s.expands) != 1 || s.expands[0] != "comments,comment_count" {
		t.Errorf("expand = %v", s.expands)
	}
}

func TestVikunjaFetchCachesIncompleteExpandedComments(t *testing.T) {
	t.Parallel()
	count := 3
	all := commentsOf(count, "eric", "eric", "claude")
	s := &vikunjaServer{
		tasks: []vikunjaTask{{
			ID: 1, Updated: iso(testNow), Comments: all[:2], CommentCount: &count,
		}},
		comments: map[int][]vikunjaComment{1: all},
	}
	v := newVikunja(t, s)
	for range 2 {
		rows, err := v.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if rows[0].State != "reply from claude" {
			t.Errorf("state = %q", rows[0].State)
		}
	}
	if len(s.pages) != 1 {
		t.Fatalf("unchanged task made %d comment requests, want 1", len(s.pages))
	}
	count++
	s.comments[1] = append(s.comments[1], commentsOf(1, "codex")...)
	rows, err := v.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.pages) != 2 {
		t.Errorf("changed comment count made %d total comment requests, want 2", len(s.pages))
	}
	if rows[0].State != "reply from codex" {
		t.Errorf("state after new comment = %q", rows[0].State)
	}
	if s.tasks[0].Updated != iso(testNow) {
		t.Errorf("updated = %q, want unchanged", s.tasks[0].Updated)
	}
	s.comments[1][len(s.comments[1])-1] = commentsOf(1, "gemini")[0]
	s.tasks[0].Updated = "2026-10-24T20:00:00Z"
	rows, err = v.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.pages) != 3 {
		t.Errorf("changed task made %d total comment requests, want 3", len(s.pages))
	}
	if rows[0].State != "reply from gemini" {
		t.Errorf("state after edited comment = %q", rows[0].State)
	}
}

func TestVikunjaFetchDoesNotCacheWithoutCommentCount(t *testing.T) {
	t.Parallel()
	all := commentsOf(2, "eric", "claude")
	s := &vikunjaServer{
		tasks:    []vikunjaTask{{ID: 1, Updated: iso(testNow)}},
		comments: map[int][]vikunjaComment{1: all},
	}
	v := newVikunja(t, s)
	for range 2 {
		rows, err := v.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if rows[0].State != "reply from claude" {
			t.Errorf("state = %q", rows[0].State)
		}
	}
	if len(s.pages) != 2 {
		t.Errorf("comment requests = %d, want 2", len(s.pages))
	}
}

func TestVikunjaFetchEndToEnd(t *testing.T) {
	t.Parallel()
	s := &vikunjaServer{
		tasks: []vikunjaTask{
			{ID: 1, Title: "replied", Updated: iso(testNow)},
			{ID: 2, Title: "operator last", Updated: iso(testNow)},
			{ID: 3, Title: "silent", Updated: iso(testNow)},
			labelled(vikunjaTask{ID: 4, Title: "pr open", Updated: iso(testNow)}, testPROpenLabel),
		},
		comments: map[int][]vikunjaComment{
			1: commentsOf(3, "eric", "eric", "claude"),
			2: commentsOf(2, "claude", "eric"),
		},
	}
	rows, err := newVikunja(t, s).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, r := range rows {
		states[r.Name] = r.State
	}
	want := map[string]string{"#1": "reply from claude", "#2": StateIdle, "#3": StateIdle, "#4": StatePROpen}
	for name, state := range want {
		if states[name] != state {
			t.Errorf("%s: state = %q, want %q", name, states[name], state)
		}
	}
	for _, a := range s.auths {
		if a != "Bearer "+testToken {
			t.Fatalf("Authorization = %q", a)
		}
	}
	if len(s.filters) != 1 || s.filters[0] != "labels in 1 && done = false" {
		t.Errorf("filters = %q", s.filters)
	}
	if len(s.expands) != 1 || s.expands[0] != "comments,comment_count" {
		t.Errorf("expand = %v", s.expands)
	}
}

func TestVikunjaFetchPaginatesTasks(t *testing.T) {
	t.Parallel()
	total := VikunjaPageSize + 5
	s := &vikunjaServer{comments: map[int][]vikunjaComment{}}
	for i := 1; i <= total; i++ {
		s.tasks = append(s.tasks, vikunjaTask{ID: i, Updated: iso(testNow)})
	}
	rows, err := newVikunja(t, s).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != total {
		t.Errorf("rows = %d, want %d", len(rows), total)
	}
	if len(s.filters) != 2 {
		t.Errorf("task list requests = %d, want 2", len(s.filters))
	}
}

func TestVikunjaFetchFollowsTotalPagesWhenServerCapsPageSize(t *testing.T) {
	t.Parallel()
	const (
		serverCap = 2
		taskCount = 5
		wantPages = 3
	)
	s := &vikunjaServer{comments: map[int][]vikunjaComment{}, pageCap: serverCap}
	for i := 1; i <= taskCount; i++ {
		s.tasks = append(s.tasks, vikunjaTask{ID: i})
	}
	rows, err := newVikunja(t, s).Fetch(context.Background())
	if err != nil || len(rows) != taskCount {
		t.Fatalf("rows=%d err=%v, want %d rows", len(rows), err, taskCount)
	}
	if len(s.filters) != wantPages {
		t.Errorf("task list requests = %d, want %d", len(s.filters), wantPages)
	}
}

func TestVikunjaFetchExactFullPageAsksOnceMore(t *testing.T) {
	t.Parallel()
	s := &vikunjaServer{comments: map[int][]vikunjaComment{}}
	for i := 1; i <= VikunjaPageSize; i++ {
		s.tasks = append(s.tasks, vikunjaTask{ID: i})
	}
	rows, err := newVikunja(t, s).Fetch(context.Background())
	if err != nil || len(rows) != VikunjaPageSize {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	if len(s.filters) != 2 {
		t.Errorf("task list requests = %d, want 2 (full page forces a probe)", len(s.filters))
	}
}

func TestVikunjaFetchErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"server error", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &vikunjaServer{status: tt.status}
			_, err := newVikunja(t, s).Fetch(context.Background())
			if err == nil || !strings.Contains(err.Error(), strconv.Itoa(tt.status)) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestVikunjaFetchCommentErrorFailsFetch(t *testing.T) {
	t.Parallel()
	s := &vikunjaServer{tasks: []vikunjaTask{{ID: 1}}}
	v := newVikunja(t, s)
	v.Client = &http.Client{Transport: failingComments{base: v.Client.Transport}}
	if _, err := v.Fetch(context.Background()); err == nil {
		t.Error("expected the comment failure to surface")
	}
}

type failingComments struct{ base http.RoundTripper }

func (f failingComments) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/comments") {
		return nil, errors.New("connection reset")
	}
	return f.base.RoundTrip(r)
}

func TestVikunjaToken(t *testing.T) {
	t.Parallel()
	secret := TokenSecret{Namespace: "vikunja", Name: "agent-claude", Key: "token"}
	// The explicit token must not touch the cluster.
	noCluster := failingClient(errors.New("cluster read"))
	tests := []struct {
		name    string
		token   string
		kube    func(t *testing.T) KubeClient
		want    string
		wantErr string
	}{
		{"explicit token wins", testToken, func(*testing.T) KubeClient { return noCluster }, testToken, ""},
		{"explicit token trimmed", testToken + "\n", func(*testing.T) KubeClient { return noCluster }, testToken, ""},
		{"blank explicit token reads the secret", " ", func(t *testing.T) KubeClient {
			return fakeKube(t, secretObj(obj{"token": b64(testToken)})).client(t)
		}, testToken, ""},
		{"secret token trimmed", "", func(t *testing.T) KubeClient {
			return fakeKube(t, secretObj(obj{"token": b64(testToken + "\n")})).client(t)
		}, testToken, ""},
		{"empty secret value", "", func(t *testing.T) KubeClient {
			return fakeKube(t, secretObj(obj{"token": b64("\n")})).client(t)
		}, "", `vikunja token: secret vikunja/agent-claude key "token" is empty`},
		{"bad base64", "", func(t *testing.T) KubeClient {
			return fakeKube(t, secretObj(obj{"token": "!!!"})).client(t)
		}, "", `vikunja token: decoding secret vikunja/agent-claude key "token": illegal base64 data at input byte 0`},
		{"no cluster", "", func(*testing.T) KubeClient { return failingClient(errBoom) }, "", "vikunja token: boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := Vikunja{Token: tt.token, Secret: secret, Kube: tt.kube(t)}
			got, err := v.token(context.Background())
			if got != tt.want || errText(err) != tt.wantErr {
				t.Errorf("got %q err=%v, want %q err=%q", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
