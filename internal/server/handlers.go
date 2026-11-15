package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/source"
)

// API routes.
const (
	routeSnapshot = "GET /api/v1/snapshot"
	routeEvents   = "GET /api/v1/events"
	routeRefresh  = "POST /api/v1/refresh"
	routeHide     = "PUT /api/v1/hidden/{source}/{name}"
	routeUnhide   = "DELETE /api/v1/hidden/{source}/{name}"
	routeHealthz  = "GET " + healthzPath
	routeReadyz   = "GET " + readyzPath
	healthzPath   = "/healthz"
	readyzPath    = "/readyz"
	// showHiddenParam lists the hidden rows too, as H does in the TUI.
	showHiddenParam = "show_hidden"
)

const (
	errNoRow      = "no such row"
	errCannotHide = "a source error cannot be hidden"
	errNotReady   = "first refresh not done"
	errTooSoon    = "refreshed too recently"
)

// Handler routes the API, and every other path to the web UI when the
// server has one.
func (s *Server) Handler() http.Handler {
	return withFrontend(s.apiHandler(), s.opts.Frontend)
}

func (s *Server) apiHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(routeSnapshot, s.handleSnapshot)
	mux.HandleFunc(routeEvents, s.handleEvents)
	mux.HandleFunc(routeRefresh, s.handleRefresh)
	mux.HandleFunc(routeHide, s.handleHide)
	mux.HandleFunc(routeUnhide, s.handleUnhide)
	mux.HandleFunc(routeHealthz, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(routeReadyz, s.handleReadyz)
	return mux
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if showHidden, ok := parseShowHidden(w, r); ok {
		writeJSON(w, http.StatusOK, s.snapshot(showHidden))
	}
}

// handleRefresh starts a refresh or joins the running one, and answers at
// once; the new snapshot arrives on the event stream. Within the cooldown of
// the last refresh it starts none and answers 429 with Retry-After.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	showHidden, ok := parseShowHidden(w, r)
	if !ok {
		return
	}
	if _, wait := s.refreshAfter(s.opts.RefreshCooldown); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int((wait+time.Second-1)/time.Second)))
		writeError(w, http.StatusTooManyRequests, errTooSoon)
		return
	}
	writeJSON(w, http.StatusAccepted, s.snapshot(showHidden))
}

// handleHide hides a row of the last refresh; a source's error row cannot be.
func (s *Server) handleHide(w http.ResponseWriter, r *http.Request) {
	row, ok := s.findRow(r.PathValue("source"), r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, errNoRow)
		return
	}
	if !hidden.Hideable(row) {
		writeError(w, http.StatusConflict, errCannotHide)
		return
	}
	s.changeHidden(w, func(set hidden.Set) hidden.Set { return set.Hide(row) })
}

// handleUnhide shows a row again, listed or not: unhiding what is not
// hidden changes nothing.
func (s *Server) handleUnhide(w http.ResponseWriter, r *http.Request) {
	row := source.Row{Source: r.PathValue("source"), Name: r.PathValue("name")}
	s.changeHidden(w, func(set hidden.Set) hidden.Set { return set.Unhide(row) })
}

// changeHidden stores a hidden-row change and tells the event streams. A
// change the store refuses is not applied.
func (s *Server) changeHidden(w http.ResponseWriter, change func(hidden.Set) hidden.Set) {
	err := s.applyHidden(change, false)
	s.events.publish() // the last error changed either way
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hidden rows: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.ready() {
		writeError(w, http.StatusServiceUnavailable, errNotReady)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// parseShowHidden reads the show_hidden query parameter, answering 400
// when it is not a boolean.
func parseShowHidden(w http.ResponseWriter, r *http.Request) (bool, bool) {
	v := r.URL.Query().Get(showHiddenParam)
	if v == "" {
		return false, true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, showHiddenParam+" must be true or false")
		return false, false
	}
	return b, true
}

type errorBody struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // the client may have left; nothing to tell it
}
