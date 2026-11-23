// Command dgs runs the DaGrouper School backend.
//
//	dgs serve      apply new migrations, then serve the API until SIGINT or
//	               SIGTERM (the default)
//	dgs migrate    apply new migrations and exit
//	dgs keys new   print a new signing key for the key file
//
// Configuration comes from the environment; docs/development.md lists every
// variable and its default.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/config"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/logx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/server"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"serve"}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Stdout, args); err != nil {
		slog.Error(strings.Join(args, " ")+" failed", "err", err)
		os.Exit(1)
	}
}

const usage = "usage: dgs serve|migrate|keys new"

func run(ctx context.Context, out io.Writer, args []string) error {
	cmd := args[0]
	switch {
	case cmd == "keys" && len(args) == 2 && args[1] == "new":
		return newKey(out)
	case (cmd == "serve" || cmd == "migrate") && len(args) == 1:
	default:
		return fmt.Errorf("unknown command %q; %s", strings.Join(args, " "), usage)
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
	sam, err := samAccess(ctx, cfg)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	app := server.New(server.Deps{DB: pool, SAM: sam, AllowedOrigins: cfg.AllowedOrigins, Now: time.Now})
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

// samAccess reads the tenant file and the signing keys. Without a key file,
// which only development and test allow, it signs with a key generated now.
func samAccess(ctx context.Context, cfg config.Config) (*samaccess.Access, error) {
	var keys *samaccess.Keys
	var err error
	if cfg.SigningKeysFile != "" {
		keys, err = samaccess.LoadKeys(cfg.SigningKeysFile)
	} else {
		slog.WarnContext(ctx, "no DGS_SIGNING_KEYS_FILE: signing user tokens with a key generated now, which a restart replaces")
		keys, err = samaccess.GenerateKeys(time.Now())
	}
	if err != nil {
		return nil, err
	}
	return samaccess.New(cfg.SAM, keys, time.Now)
}

// newKey prints a key to add to the key file. It does not sign until its
// entry's signs is set; see docs/operations.md.
func newKey(out io.Writer) error {
	k, err := samaccess.NewKey(time.Now())
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(k)
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
