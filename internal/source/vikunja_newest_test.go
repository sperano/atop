package source

import (
	"context"
	"testing"
	"time"
)

func commentAt(id int, author string, created time.Time) vikunjaComment {
	c := *testComment(author, created)
	c.ID = id
	return c
}

func TestNewest(t *testing.T) {
	t.Parallel()
	later := testNow.Add(time.Second)
	tests := []struct {
		name     string
		comments []vikunjaComment
		wantID   int // 0 for none
	}{
		{"none", nil, 0},
		{"one", []vikunjaComment{commentAt(7, "a", testNow)}, 7},
		{"the later one", []vikunjaComment{commentAt(2, "a", later), commentAt(1, "b", testNow)}, 2},
		{"created beats id", []vikunjaComment{commentAt(5, "a", later), commentAt(6, "b", testNow)}, 5},
		{"a same-second tie goes to the higher id", []vikunjaComment{commentAt(11, "a", testNow), commentAt(10, "b", testNow)}, 11},
		{"an offset does not fool the order", []vikunjaComment{
			{ID: 1, Created: "2026-11-17T21:13:04-08:00"}, {ID: 2, Created: "2026-11-18T05:13:03Z"},
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newest(tt.comments)
			gotID := 0
			if got != nil {
				gotID = got.ID
			}
			if gotID != tt.wantID {
				t.Errorf("newest id = %d, want %d", gotID, tt.wantID)
			}
		})
	}
}

// TestVikunjaFetchBreaksASameSecondTie reproduces two comments created in
// the same second: Vikunja lists the older one first.
func TestVikunjaFetchBreaksASameSecondTie(t *testing.T) {
	t.Parallel()
	count := 3
	all := []vikunjaComment{
		commentAt(1, testOperator, testNow.Add(-time.Hour)),
		commentAt(2, "claude", testNow),
		commentAt(3, "codex", testNow),
	}
	tests := []struct {
		name string
		task vikunjaTask
	}{
		{"looked up", vikunjaTask{ID: 1, Updated: iso(testNow)}},
		{"embedded", vikunjaTask{ID: 1, Updated: iso(testNow), Comments: []vikunjaComment{all[0], all[2], all[1]}, CommentCount: &count}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &vikunjaServer{tasks: []vikunjaTask{tt.task}, comments: map[int][]vikunjaComment{1: all}}
			rows, err := newVikunja(t, s).Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].State != "reply from codex" {
				t.Errorf("rows = %+v, want the reply from codex", rows)
			}
		})
	}
}
