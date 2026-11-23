package samaccess

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// Check compares one kind of entity in SAM with this database, a page at a
// time, and enqueues a correction for each that differs. The module that
// owns the entity declares it.
type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

// Reconciler runs every check on an interval, on whichever instance holds
// the reconciler lock.
type Reconciler struct {
	pool     *pgxpool.Pool
	checks   []Check
	interval time.Duration
}

// NewReconciler builds the reconciler.
func NewReconciler(pool *pgxpool.Pool, checks []Check, interval time.Duration) (*Reconciler, error) {
	if interval <= 0 {
		return nil, errors.New("the reconciler needs an interval")
	}
	for _, c := range checks {
		if c.Name == "" || c.Run == nil {
			return nil, errors.New("a reconciler check needs a name and a function")
		}
	}
	return &Reconciler{pool: pool, checks: checks, interval: interval}, nil
}

// Run passes every interval until ctx ends, the first an interval after
// start-up, when the worker has had time to send what was waiting.
func (r *Reconciler) Run(ctx context.Context) {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if _, err := r.Pass(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "reconciler pass failed", "err", err)
		}
	}
}

// Pass runs every check once, if this instance holds the reconciler lock,
// and reports whether it did. A check that fails is logged and tried again
// on the next pass; the others still run.
func (r *Reconciler) Pass(ctx context.Context) (bool, error) {
	lock, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = lock.Rollback(context.WithoutCancel(ctx)) }()
	ok, err := db.TryLock(ctx, lock, db.ReconcilerLock)
	if err != nil || !ok {
		return false, err
	}
	var errs []error
	for _, c := range r.checks {
		if ctx.Err() != nil {
			break
		}
		if err := c.Run(ctx); err != nil {
			slog.ErrorContext(ctx, "reconciler check failed", "check", c.Name, "err", err)
			errs = append(errs, fmt.Errorf("%s: %w", c.Name, err))
		}
	}
	return true, errors.Join(errs...)
}
