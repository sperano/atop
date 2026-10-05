package source

import (
	"context"
	"errors"
	"testing"
)

func TestFetchAll(t *testing.T) {
	t.Parallel()
	ok := Named{Name: "good", Fetch: func(context.Context) ([]Row, error) {
		return []Row{{Source: "good", Name: "a"}, {Source: "good", Name: "b"}}, nil
	}}
	tests := []struct {
		name      string
		bad       Named
		wantState string
		wantText  string
	}{
		{"error", Named{Name: "bad", Fetch: func(context.Context) ([]Row, error) {
			return nil, errors.New("no kubeconfig")
		}}, StateError, "no kubeconfig"},
		{"panic", Named{Name: "bad", Fetch: func(context.Context) ([]Row, error) {
			panic("kaboom")
		}}, StateError, "kaboom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rows := FetchAll(context.Background(), []Named{ok, tt.bad})
			var good, bad []Row
			for _, r := range rows {
				if r.Source == "good" {
					good = append(good, r)
				} else {
					bad = append(bad, r)
				}
			}
			if len(good) != 2 {
				t.Errorf("good rows = %d, want 2", len(good))
			}
			if len(bad) != 1 {
				t.Fatalf("error rows = %d, want 1", len(bad))
			}
			r := bad[0]
			if r.Source != "bad" || r.State != tt.wantState || !r.NeedsYou {
				t.Errorf("unexpected error row %+v", r)
			}
			if !containsFold(r.Detail, tt.wantText) {
				t.Errorf("detail %q lacks %q", r.Detail, tt.wantText)
			}
		})
	}
}

func TestFetchAllEmpty(t *testing.T) {
	t.Parallel()
	if rows := FetchAll(context.Background(), nil); len(rows) != 0 {
		t.Errorf("got %d rows", len(rows))
	}
}
