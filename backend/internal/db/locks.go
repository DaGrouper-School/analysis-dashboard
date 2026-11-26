package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Advisory lock keys, every one the service takes. They are kept here, and
// a test fails when any other package takes an advisory lock, so two tasks
// never pick the same key by accident. The numbers differ from SAM's,
// which runs on its own server, so a shared server cannot mix them up.
const (
	// MigrationLock is held while migrations apply, so instances starting
	// together apply each migration once.
	MigrationLock int64 = 8301001
	// OutboxLock is held by the instance whose worker sends the outbox to
	// SAM, for one pass at a time.
	OutboxLock int64 = 8301002
	// ReconcilerLock is held by the instance comparing SAM with this
	// database, for one pass at a time.
	ReconcilerLock int64 = 8301003
	// SeedLock is held while dgs seed runs, so two seeds never both find a
	// list empty and add its rows twice.
	SeedLock int64 = 8301004
	// RegistryLock is held while an admin changes the registry, so changes
	// run one at a time, each against the registry the one before left.
	RegistryLock int64 = 8301005
	// TestTemplateLock is held while tests build their migrated template
	// database.
	TestTemplateLock int64 = 8301999
)

// TryLock takes the transaction-level advisory lock key in tx if it is
// free, and reports whether it did. The lock is released when tx ends.
// Background tasks that run on one instance at a time take theirs this
// way.
func TryLock(ctx context.Context, tx pgx.Tx, key int64) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, key).Scan(&ok)
	return ok, err
}

// LockTx takes the transaction-level advisory lock key in tx, waiting
// until it is free. The lock is released when tx ends.
func LockTx(ctx context.Context, tx pgx.Tx, key int64) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, key)
	return err
}

// Lock takes the session-level advisory lock key on q, a single
// connection, waiting until it is free, and returns the function that
// releases it. The release runs even when ctx has ended, so the
// connection goes back without the lock.
func Lock(ctx context.Context, q Querier, key int64) (unlock func(), err error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		return nil, err
	}
	return func() { _, _ = q.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, key) }, nil
}
