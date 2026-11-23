// Package db owns the service's connection to its PostgreSQL database: the
// pool, migrations, transactions and the advisory locks. SAM has its own
// database, which this service never reads.
package db

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is satisfied by *pgxpool.Pool, *pgxpool.Conn and pgx.Tx. Begin
// starts a transaction, or a savepoint inside one, so a write of several
// statements is atomic whatever it is given.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Config sizes the pool and bounds its statements.
type Config struct {
	// MaxConns caps the pool's connections; it must be at least 1.
	MaxConns int32
	// StatementTimeout cancels a statement that runs longer; it must be
	// longer than zero.
	StatementTimeout time.Duration
}

// Open connects a pool and checks that it reaches the database.
func Open(ctx context.Context, url string, cfg Config) (*pgxpool.Pool, error) {
	if cfg.MaxConns < 1 || cfg.StatementTimeout <= 0 {
		return nil, errors.New("db: a pool needs at least one connection and a statement timeout")
	}
	pc, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	pc.MaxConns = cfg.MaxConns
	pc.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
