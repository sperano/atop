package server

import (
	"net/http"
	"testing"
	"testing/fstest"
)

const (
	testIndex = "<!doctype html><div id=app></div>"
	testAsset = "console.log('vigie')"
)

func testFrontend() fstest.MapFS {
	return fstest.MapFS{
		indexFile:             {Data: []byte(testIndex)},
		"assets/index-abc.js": {Data: []byte(testAsset)},
	}
}

func TestFrontendRoutes(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(o *Options) { o.Frontend = testFrontend() })
	h.refreshWith(testRows()[1:2])
	tests := []struct {
		name, method, target string
		status               int
		body                 string // exact body; empty skips the check
		cacheControl         string
	}{
		{"root is the entry page", http.MethodGet, "/", http.StatusOK, testIndex, noCache},
		{"a deep link is the entry page", http.MethodGet, "/some/page", http.StatusOK, testIndex, noCache},
		{"a directory is the entry page", http.MethodGet, "/assets/", http.StatusOK, testIndex, noCache},
		{"head of the entry page", http.MethodHead, "/", http.StatusOK, "", noCache},
		{"an asset", http.MethodGet, "/assets/index-abc.js", http.StatusOK, testAsset, ""},
		{"a missing asset", http.MethodGet, "/assets/index-old.js", http.StatusNotFound, "404 page not found\n", ""},
		{"index.html redirects to the root", http.MethodGet, "/index.html", http.StatusMovedPermanently, "", noCache},
		{"an unknown API path is the API's 404", http.MethodGet, "/api/v1/nope", http.StatusNotFound, "404 page not found\n", ""},
		{"the API still answers", http.MethodGet, readyzPath, http.StatusOK, "", ""},
		{"healthz is the API's", http.MethodGet, healthzPath, http.StatusOK, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := do(t, h.srv, tt.method, tt.target)
			got := body(t, resp)
			if resp.StatusCode != tt.status {
				t.Errorf("status = %d, want %d (body %q)", resp.StatusCode, tt.status, got)
			}
			if tt.body != "" && got != tt.body {
				t.Errorf("body = %q, want %q", got, tt.body)
			}
			if cc := resp.Header.Get(cacheControlHeader); cc != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", cc, tt.cacheControl)
			}
		})
	}
}

func TestFrontendKeepsTheAPIsJSONAndMethods(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(o *Options) { o.Frontend = testFrontend() })
	h.refreshWith(testRows()[1:2])
	resp := do(t, h.srv, http.MethodGet, snapshotURL)
	if snap := decodeSnapshot(t, resp); len(snap.Rows) != 1 || snap.Rows[0].Name != "vigie#3" {
		t.Errorf("snapshot rows = %v, want vigie#3", rowNames(snap))
	}
	tests := []struct{ method, target, allow string }{
		{http.MethodPost, snapshotURL, "GET, HEAD"},
		{http.MethodGet, refreshURL, http.MethodPost},
		{http.MethodPost, healthzPath, "GET, HEAD"},
		{http.MethodPost, "/", "GET, HEAD"},
		{http.MethodDelete, "/some/page", "GET, HEAD"},
	}
	for _, tt := range tests {
		resp := do(t, h.srv, tt.method, tt.target)
		if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != tt.allow {
			t.Errorf("%s %s = %d (Allow %q), want 405 (Allow %q)", tt.method, tt.target, resp.StatusCode, resp.Header.Get("Allow"), tt.allow)
		}
	}
}

func TestNoFrontendServesTheAPIAlone(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	if resp := do(t, h.srv, http.MethodGet, "/"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET / without a frontend = %d, want 404", resp.StatusCode)
	}
}
