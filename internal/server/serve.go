package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	// ShutdownTimeout bounds the wait for requests in flight on shutdown.
	ShutdownTimeout = 10 * time.Second
)

// Serve runs the refresh loop and answers HTTP on ln until ctx ends, then
// ends the event streams and shuts down gracefully. It returns nil after a
// clean shutdown. No write timeout is set: event streams set a deadline per
// frame instead.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	hs := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: readHeaderTimeout}
	var loop sync.WaitGroup
	loopCtx, stopLoop := context.WithCancel(ctx)
	defer stopLoop()
	loop.Go(func() { s.Run(loopCtx) })
	served := make(chan error, 1)
	go func() { served <- hs.Serve(ln) }()

	var err error
	select {
	case err = <-served:
	case <-ctx.Done():
	}
	stopLoop()
	s.Close()
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ShutdownTimeout)
	defer cancel()
	err = errors.Join(err, hs.Shutdown(shutdownCtx))
	loop.Wait()
	return err
}
