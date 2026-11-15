package main

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sperano/vigie/internal/config"
	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
	"github.com/sperano/vigie/web"
)

func TestServerServesTheEmbeddedUI(t *testing.T) {
	t.Parallel()
	fetch := func(context.Context) []source.Row { return nil }
	srv := newServer(config.Config{Interval: config.DefaultInterval}, fetch, nil,
		hidden.File{Path: filepath.Join(t.TempDir(), "hidden.json")})
	t.Cleanup(srv.Close)
	want, err := fs.ReadFile(web.FS(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/", "/a/deep/link"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		got, _ := io.ReadAll(rec.Result().Body)
		if rec.Code != http.StatusOK || string(got) != string(want) {
			t.Errorf("GET %s = %d %q, want 200 with web's index.html", target, rec.Code, got)
		}
	}
}
