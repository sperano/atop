package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
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
	comments map[int][]vikunjaComment // oldest first
	auths    []string
	pages    []string // "<taskID>:<page>" for each comment request
	filters  []string
	status   int
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
	if path == "/tasks" {
		s.filters = append(s.filters, q.Get("filter"))
		s.serveTasks(w, q.Get("page"), q.Get("per_page"))
		return
	}
	var id int
	if _, err := fmt.Sscanf(path, "/tasks/%d/comments", &id); err != nil {
		http.NotFound(w, r)
		return
	}
	s.pages = append(s.pages, fmt.Sprintf("%d:%s", id, q.Get("page")))
	s.serveComment(w, id, q.Get("page"))
}

func (s *vikunjaServer) serveTasks(w http.ResponseWriter, page, perPage string) {
	p, _ := strconv.Atoi(page)
	n, _ := strconv.Atoi(perPage)
	lo := min((p-1)*n, len(s.tasks))
	hi := min(p*n, len(s.tasks))
	_ = json.NewEncoder(w).Encode(s.tasks[lo:hi])
}

func (s *vikunjaServer) serveComment(w http.ResponseWriter, id int, page string) {
	all := s.comments[id]
	p, _ := strconv.Atoi(page)
	out := []vikunjaComment{}
	if p >= 1 && p <= len(all) {
		out = all[p-1 : p]
	}
	if len(all) > 0 {
		// Written straight into the map: Go's Header.Set would canonicalise it.
		w.Header()[pagesHeader] = []string{strconv.Itoa(len(all))}
	}
	_ = json.NewEncoder(w).Encode(out)
}

func newVikunja(t *testing.T, s *vikunjaServer) Vikunja {
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

func commentsOf(n int, authors ...string) []vikunjaComment {
	out := make([]vikunjaComment, n)
	for i := range out {
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
		{"three pages reads page 3", commentsOf(3, "a"), "comment 3", []string{"1:1", "1:3"}},
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
		})
	}
}

func TestVikunjaFetchEndToEnd(t *testing.T) {
	t.Parallel()
	s := &vikunjaServer{
		tasks: []vikunjaTask{
			{ID: 1, Title: "replied", Updated: iso(testNow)},
			{ID: 2, Title: "operator last", Updated: iso(testNow)},
			{ID: 3, Title: "silent", Updated: iso(testNow)},
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
	want := map[string]string{"#1": "reply from claude", "#2": "waiting on agent", "#3": "in progress"}
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
	encoded := base64.StdEncoding.EncodeToString([]byte(testToken)) + "\n"
	tests := []struct {
		name      string
		token     string
		runner    *fakeRunner
		want      string
		wantErr   bool
		wantCalls int
	}{
		{"explicit token wins", testToken, &fakeRunner{}, testToken, false, 0},
		{"read from secret", "", &fakeRunner{replies: map[string]string{"get secret": encoded}}, testToken, false, 1},
		{"bad base64", "", &fakeRunner{replies: map[string]string{"get secret": "!!!"}}, "", true, 1},
		{"kubectl failure", "", &fakeRunner{errs: map[string]error{"get secret": errors.New("denied")}}, "", true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := Vikunja{Token: tt.token, Secret: secret, Run: tt.runner.run}
			got, err := v.token(context.Background())
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("got %q err=%v", got, err)
			}
			if len(tt.runner.calls) != tt.wantCalls {
				t.Errorf("calls = %v", tt.runner.calls)
			}
			if tt.wantCalls == 1 && !strings.HasPrefix(tt.runner.calls[0],
				"kubectl -n vikunja get secret agent-claude -o jsonpath={.data.token}") {
				t.Errorf("command = %q", tt.runner.calls[0])
			}
		})
	}
}
