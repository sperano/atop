package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
)

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func TestServeShutsDownGracefully(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	ln := listen(t)
	base := "http://" + ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- h.srv.Serve(ctx, ln) }()
	<-h.fetch.started // Run's first refresh
	h.fetch.rows <- testRows()

	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz = %d, want 200", resp.StatusCode)
	}
	stream, err := http.Get(base + eventsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	r := bufio.NewReader(stream.Body)
	readFrame(t, r)

	cancel()
	if err := <-served; err != nil {
		t.Errorf("Serve = %v, want nil after a clean shutdown", err)
	}
	for {
		if _, err := r.ReadString('\n'); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Errorf("stream ended with %v, want EOF", err)
			}
			break
		}
	}
}

func TestServeReportsAListenerError(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	ln := listen(t)
	ln.Close()
	if err := h.srv.Serve(context.Background(), ln); !errors.Is(err, net.ErrClosed) {
		t.Errorf("Serve = %v, want %v", err, net.ErrClosed)
	}
}
