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
		if _, err := ExecRunner(ctx, "vigie-no-such-binary"); err == nil {
			t.Error("expected an error")
		}
	})
}
