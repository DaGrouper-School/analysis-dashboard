package main

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestServeDrains: serve answers until its context ends, then waits for a
// request in flight to finish before returning.
func TestServeDrains(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, h) }()

	got := make(chan int, 1)
	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			got <- 0
			return
		}
		res.Body.Close()
		got <- res.StatusCode
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		t.Fatalf("serve returned %v with a request in flight", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if s := <-got; s != http.StatusOK {
		t.Fatalf("request in flight: status %d", s)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(shutdownTimeout + time.Second):
		t.Fatal("serve did not return after its context ended")
	}
}

func TestRunRefuses(t *testing.T) {
	if err := run(context.Background(), "frobnicate"); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unknown command: %v", err)
	}
	t.Setenv("DGS_ENV", "")
	if err := run(context.Background(), "serve"); err == nil || !strings.Contains(err.Error(), "DGS_ENV") {
		t.Fatalf("no environment: %v", err)
	}
}
