// Command dgs runs the DaGrouper School backend.
//
//	dgs serve   serve the API until SIGINT or SIGTERM (the default)
//
// Configuration comes from the environment; docs/development.md lists every
// variable and its default.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/config"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/logx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/server"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cmd); err != nil {
		slog.Error(cmd+" failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd string) error {
	if cmd != "serve" {
		return fmt.Errorf("unknown command %q (serve)", cmd)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger, err := logx.New(os.Stderr, cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	app := server.New(server.Deps{AllowedOrigins: cfg.AllowedOrigins, Now: time.Now})
	slog.InfoContext(ctx, "listening", "addr", ln.Addr().String(), "env", cfg.Environment)
	return serve(ctx, ln, app.Handler())
}

// Server timeouts. A whole request, body included, must arrive within
// readTimeout; responses are bounded by each handler's own deadlines.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	idleTimeout       = 2 * time.Minute  // an idle keep-alive connection
	shutdownTimeout   = 10 * time.Second // to finish requests in flight
)

// serve serves h on ln until ctx ends, then stops taking requests and waits
// for those in flight before returning.
func serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: readHeaderTimeout, ReadTimeout: readTimeout, IdleTimeout: idleTimeout}
	failed := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
	}()
	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdown)
}
