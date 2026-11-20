package source

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestTaskLabel(t *testing.T) {
	t.Parallel()
	projects := map[int]string{3: "Hollingsworth", 4: "  ", 5: " vigie "}
	tests := []struct {
		name     string
		task     vikunjaTask
		projects map[int]string
		want     string
	}{
		{"project and number", vikunjaTask{ID: 1647, ProjectID: 3, Index: 577}, projects, "#1647 - Hollingsworth #577"},
		{"title trimmed", vikunjaTask{ID: 9, ProjectID: 5, Index: 2}, projects, "#9 - vigie #2"},
		{"unknown project", vikunjaTask{ID: 1647, ProjectID: 99, Index: 577}, projects, "#1647"},
		{"blank project title", vikunjaTask{ID: 1647, ProjectID: 4, Index: 577}, projects, "#1647"},
		{"no number", vikunjaTask{ID: 1647, ProjectID: 3}, projects, "#1647"},
		{"no projects", vikunjaTask{ID: 1647, ProjectID: 3, Index: 577}, nil, "#1647"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := taskLabel(tt.task, tt.projects); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVikunjaFetchLabelsRowsWithTheirProject(t *testing.T) {
	t.Parallel()
	const serverCap = 1
	s := &vikunjaServer{
		tasks: []vikunjaTask{
			{ID: 1647, ProjectID: 3, Index: 577, Updated: iso(testNow)},
			{ID: 1648, ProjectID: 7, Index: 12, Updated: iso(testNow)},
		},
		projects: []vikunjaProject{{ID: 3, Title: "Hollingsworth"}, {ID: 7, Title: "vigie"}},
		comments: map[int][]vikunjaComment{},
		pageCap:  serverCap,
	}
	rows, err := newVikunja(t, s).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(rows))
	for _, r := range rows {
		got = append(got, r.Name+"|"+r.Label)
	}
	want := []string{"#1647|#1647 - Hollingsworth #577", "#1648|#1648 - vigie #12"}
	if !slices.Equal(got, want) {
		t.Errorf("name|label = %q, want %q", got, want)
	}
	if want := []string{"1 true", "2 true"}; !slices.Equal(s.projectPages, want) {
		t.Errorf("project requests = %q, want %q", s.projectPages, want)
	}
}

func TestVikunjaFetchSkipsProjectsWithoutTasks(t *testing.T) {
	t.Parallel()
	s := &vikunjaServer{projects: []vikunjaProject{{ID: 3, Title: "Hollingsworth"}}}
	if _, err := newVikunja(t, s).Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(s.projectPages) != 0 {
		t.Errorf("project requests = %q, want none", s.projectPages)
	}
}

func TestVikunjaFetchProjectErrorFailsFetch(t *testing.T) {
	t.Parallel()
	s := &vikunjaServer{tasks: []vikunjaTask{{ID: 1}}, comments: map[int][]vikunjaComment{}}
	v := newVikunja(t, s)
	v.Client = &http.Client{Transport: failingProjects{base: v.Client.Transport}}
	_, err := v.Fetch(context.Background())
	if err == nil || !strings.HasSuffix(err.Error(), errProjectsDown.Error()) {
		t.Errorf("err = %v, want the project failure", err)
	}
}

var errProjectsDown = errors.New("projects down")

type failingProjects struct{ base http.RoundTripper }

func (f failingProjects) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/projects") {
		return nil, errProjectsDown
	}
	return f.base.RoundTrip(r)
}
