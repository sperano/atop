package source

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var testNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func fixedNow() time.Time { return testNow }

func iso(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// fakeRunner answers a command by the first registered substring of its
// joined command line.
type fakeRunner struct {
	replies map[string]string
	errs    map[string]error
	calls   []string
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, line)
	for key, err := range f.errs {
		if strings.Contains(line, key) {
			return nil, err
		}
	}
	for key, out := range f.replies {
		if strings.Contains(line, key) {
			return []byte(out), nil
		}
	}
	return nil, fmt.Errorf("fakeRunner: unexpected command %q", line)
}
