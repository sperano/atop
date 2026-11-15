package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	// apiPrefix is the API's subtree: its unknown paths are a 404 of the
	// API, never the UI's page.
	apiPrefix = "/api/"
	// indexFile is the UI's entry page, served for every path that is none
	// of its files.
	indexFile = "index.html"
	// cacheControlHeader and noCache make the browser ask again for the entry
	// page, which names the hashed assets of the build it belongs to.
	cacheControlHeader = "Cache-Control"
	noCache            = "no-cache"
)

// withFrontend routes the API's paths to api and every other path to the
// UI's files. A nil frontend leaves api alone.
func withFrontend(api http.Handler, frontend fs.FS) http.Handler {
	if frontend == nil {
		return api
	}
	mux := http.NewServeMux()
	mux.Handle(apiPrefix, api)
	mux.Handle(healthzPath, api)
	mux.Handle(readyzPath, api)
	mux.Handle("/", frontendHandler(frontend))
	return mux
}

// frontendHandler serves frontend's files, and its index.html for any other
// path, so that a deep link loads the app. A missing path whose last segment
// has an extension is a missing file, not a page (an asset an old page names
// after a deploy): it is a 404, as HTML in its place would fail in the
// browser as the wrong type.
func frontendHandler(frontend fs.FS) http.Handler {
	files := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		switch {
		case name != "" && name != indexFile && isFile(frontend, name):
			files.ServeHTTP(w, r)
		case path.Ext(name) != "" && name != indexFile:
			http.NotFound(w, r)
		default:
			w.Header().Set(cacheControlHeader, noCache)
			http.ServeFileFS(w, r, frontend, indexFile)
		}
	})
}

// isFile reports whether name is a regular file of fsys: a directory is no
// file to serve, so it gets the entry page rather than a listing.
func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
