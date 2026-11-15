package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sperano/vigie/internal/config"
	"github.com/sperano/vigie/internal/hidden"
	"github.com/sperano/vigie/internal/server"
	"github.com/sperano/vigie/internal/source"
	"github.com/sperano/vigie/web"
)

// serve runs the HTTP server, with the web UI at /, until SIGTERM or an
// interrupt.
// sources names the sources fetch reads.
func serve(cfg config.Config, fetch func(context.Context) []source.Row, sources []string, hiddenFile hidden.File) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	srv := newServer(cfg, fetch, sources, hiddenFile)
	fmt.Fprintln(os.Stderr, "vigie: listening on", ln.Addr())
	return srv.Serve(ctx, ln)
}

// newServer wires the server to the sources, the hidden-row file and the
// embedded web UI.
func newServer(cfg config.Config, fetch func(context.Context) []source.Row, sources []string, hiddenFile hidden.File) *server.Server {
	return server.New(server.Options{
		Fetch:        fetch,
		Sources:      sources,
		Now:          time.Now,
		Interval:     cfg.Interval,
		UpdateHidden: hiddenFile.Update,
		Frontend:     web.FS(),
	})
}
