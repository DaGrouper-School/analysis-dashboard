// Command dgs runs the DaGrouper School backend.
//
//	dgs serve                   apply new migrations, then serve the API,
//	                            send changes to SAM and reconcile with it,
//	                            until SIGINT or SIGTERM (the default)
//	dgs migrate                 apply new migrations and exit
//	dgs keys new                print a new signing key for the key file
//	dgs outbox held             list the changes for SAM held for an operator
//	dgs outbox release ENTITY   send an entity's latest held change again
//	dgs accounts renumber USER  give a user not yet in SAM a new
//	                            participant ID
//	dgs accounts adopt USER     record the principal SAM has under a
//	                            restored user's participant ID as theirs
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
	"slices"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/config"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/logx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
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

const usage = "usage: dgs serve|migrate|keys new|outbox held|outbox release ENTITY|accounts renumber USER|accounts adopt USER"

func run(ctx context.Context, out io.Writer, args []string) error {
	switch {
	case is(args, "keys", "new"):
		return newKey(out)
	case is(args, "serve"), is(args, "migrate"), is(args, "outbox", "held"), len(args) == 3 && is(args[:2], "outbox", "release"),
		len(args) == 3 && (is(args[:2], "accounts", "renumber") || is(args[:2], "accounts", "adopt")):
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
	switch args[0] {
	case "migrate":
		return nil
	case "outbox":
		return outbox(ctx, out, pool, args[1:])
	case "accounts":
		if args[1] == "adopt" {
			return adopt(ctx, out, cfg, pool, args[2])
		}
		return renumber(ctx, out, pool, args[2])
	}
	return serveAll(ctx, cfg, pool)
}

// is reports whether args are exactly want.
func is(args []string, want ...string) bool { return slices.Equal(args, want) }

// serveAll serves the API, and sends the outbox and reconciles with SAM in
// the background, until ctx ends. The background work stops before the
// database closes.
func serveAll(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) error {
	sam, err := samAccess(ctx, cfg)
	if err != nil {
		return err
	}
	// Until a deployment configures a provider, codes go to the log in
	// development and test, and nowhere elsewhere.
	cfg.Accounts.SMS = notify.ForEnvironment(cfg.Local())
	app := server.New(server.Deps{DB: pool, SAM: sam, AllowedOrigins: cfg.AllowedOrigins, Now: time.Now, Accounts: cfg.Accounts,
		Places: cfg.Places})
	worker, err := samaccess.NewWorker(pool, sam, app.OutboxKinds(), app.Actors(), time.Now)
	if err != nil {
		return err
	}
	reconciler, err := samaccess.NewReconciler(pool, app.Checks(), cfg.ReconcileInterval)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	bg, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() { worker.Run(bg) })
	wg.Go(func() { reconciler.Run(bg) })
	for _, task := range app.Background() {
		wg.Go(func() { task(bg) })
	}
	slog.InfoContext(ctx, "listening", "addr", ln.Addr().String(), "env", cfg.Environment)
	err = serve(ctx, ln, app.Handler())
	stop()
	wg.Wait()
	return err
}

// renumber gives a user SAM does not have yet a new participant ID, for
// one whose ID SAM knows as someone else's after a database restore.
func renumber(ctx context.Context, out io.Writer, pool *pgxpool.Pool, user string) error {
	id, err := uuid.Parse(user)
	if err != nil {
		return fmt.Errorf("%q is not a user ID", user)
	}
	u, err := accounts.Renumber(ctx, pool, time.Now(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("no user %s", id)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "user %s now has participant ID %d; their registration in SAM is queued\n", id, u.ParticipantID)
	return err
}

// adopt records as a user's own the principal SAM has under their
// participant ID, for a restored user whose registration reached SAM after
// the backup. The operator checks first that the principal is theirs.
func adopt(ctx context.Context, out io.Writer, cfg config.Config, pool *pgxpool.Pool, user string) error {
	id, err := uuid.Parse(user)
	if err != nil {
		return fmt.Errorf("%q is not a user ID", user)
	}
	sam, err := samAccess(ctx, cfg)
	if err != nil {
		return err
	}
	p, err := accounts.New(pool, sam, time.Now, cfg.Accounts).Adopt(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("no user %s", id)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "user %s adopted SAM principal %s, which SAM registered at %s\n", id, p.ID,
		p.CreatedAt.UTC().Format(time.RFC3339))
	return err
}

// outbox lists the changes held for an operator, or releases one entity's.
func outbox(ctx context.Context, out io.Writer, pool *pgxpool.Pool, args []string) error {
	if args[0] == "release" {
		if err := samaccess.Release(ctx, pool, time.Now(), args[1]); err != nil {
			return fmt.Errorf("%s: %w", args[1], err)
		}
		_, err := fmt.Fprintf(out, "released %s: it is sent again on the next pass\n", args[1])
		return err
	}
	held, err := samaccess.HeldChanges(ctx, pool)
	if err != nil {
		return err
	}
	if len(held) == 0 {
		_, err := fmt.Fprintln(out, "nothing is held")
		return err
	}
	// The tabwriter buffers: a write error shows when it flushes.
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ENTITY\tKIND\tVERSION\tSENT AS\tATTEMPTS\tSINCE\tTAKES ACCESS AWAY\tERROR")
	for _, h := range held {
		actor := "this service"
		if h.Actor != nil {
			actor = h.Actor.String()
		}
		away := "no"
		if h.TakesAccessAway {
			away = "YES"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%d\t%s\t%s\t%s\n", h.Entity, h.Kind, h.Version, actor, h.Attempts,
			h.CreatedAt.UTC().Format(time.RFC3339), away, h.Error)
	}
	return tw.Flush()
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
