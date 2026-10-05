package source

import (
	"context"
	"strings"
	"testing"
)

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func TestExecRunner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("success returns stdout", func(t *testing.T) {
		t.Parallel()
		out, err := ExecRunner(ctx, "echo", "hi")
		if err != nil || strings.TrimSpace(string(out)) != "hi" {
			t.Errorf("out=%q err=%v", out, err)
		}
	})
	t.Run("failure returns error", func(t *testing.T) {
		t.Parallel()
		if _, err := ExecRunner(ctx, "false"); err == nil {
			t.Error("expected an error")
		}
	})
	t.Run("stderr is carried", func(t *testing.T) {
		t.Parallel()
		_, err := ExecRunner(ctx, "sh", "-c", "echo oops >&2; exit 1")
		if err == nil || !strings.Contains(err.Error(), "oops") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("missing binary", func(t *testing.T) {
		t.Parallel()
		if _, err := ExecRunner(ctx, "atop-no-such-binary"); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestRunJSON(t *testing.T) {
	t.Parallel()
	f := &fakeRunner{replies: map[string]string{"good": `{"a":1}`, "bad": `not json`}}
	var v struct{ A int }
	if err := runJSON(context.Background(), f.run, &v, "good"); err != nil || v.A != 1 {
		t.Errorf("good: v=%+v err=%v", v, err)
	}
	if err := runJSON(context.Background(), f.run, &v, "bad"); err == nil {
		t.Error("bad JSON: expected an error")
	}
	if err := runJSON(context.Background(), f.run, &v, "missing"); err == nil {
		t.Error("runner error: expected it to propagate")
	}
}
