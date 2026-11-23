// Command dgs runs the DaGrouper School backend.
//
//	dgs serve     apply new migrations, then serve the API until SIGINT or
//	              SIGTERM (the default)
//	dgs migrate   apply new migrations and exit
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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/config"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
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
	if cmd != "serve" && cmd != "migrate" {
		return fmt.Errorf("unknown command %q (serve, migrate)", cmd)
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
	pool, err := migrate(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	if cmd == "migrate" {
		return nil
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	app := server.New(server.Deps{DB: pool, AllowedOrigins: cfg.AllowedOrigins, Now: time.Now})
	slog.InfoContext(ctx, "listening", "addr", ln.Addr().String(), "env", cfg.Environment)
	return serve(ctx, ln, app.Handler())
}

// migrate opens the database and applies the migrations it has not had,
// logging each. Every instance does this on start; the migration lock
// makes the others wait while one applies them.
func migrate(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	pool, err := db.Open(ctx, cfg.DatabaseURL, cfg.DB)
	if err != nil {
		return nil, err
	}
	applied, err := db.Migrate(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	for _, name := range applied {
		slog.InfoContext(ctx, "applied migration", "name", name)
	}
	slog.InfoContext(ctx, "database is up to date", "applied", len(applied))
	return pool, nil
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
