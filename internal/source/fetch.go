package source

import (
	"context"
	"fmt"
	"sync"
)

// FetchFunc lists one source's rows.
type FetchFunc func(ctx context.Context) ([]Row, error)

// Named pairs a source name with its fetcher.
type Named struct {
	Name  string
	Fetch FetchFunc
}

// FetchAll runs every source concurrently. A source that fails, or panics,
// becomes a single error row so that the others still show.
func FetchAll(ctx context.Context, sources []Named) []Row {
	results := make([][]Row, len(sources))
	var wg sync.WaitGroup
	for i, s := range sources {
		wg.Go(func() {
			results[i] = fetchOne(ctx, s)
		})
	}
	wg.Wait()
	var rows []Row
	for _, r := range results {
		rows = append(rows, r...)
	}
	return rows
}

func fetchOne(ctx context.Context, s Named) (rows []Row) {
	defer func() {
		if r := recover(); r != nil {
			rows = []Row{ErrorRow(s.Name, fmt.Errorf("panic: %v", r))}
		}
	}()
	rows, err := s.Fetch(ctx)
	if err != nil {
		return []Row{ErrorRow(s.Name, err)}
	}
	return rows
}
