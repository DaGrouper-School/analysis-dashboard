package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrations holds the SQL files the service applies, and the rules for
// writing them.
//
//go:embed migrations
var migrations embed.FS

// Migrations is the service's migrations, as a file system of NNNN_*.sql
// files.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err) // the directory is embedded; it is always there
	}
	return sub
}

// Migrate applies the service's migrations that have not been applied
// yet, and returns the names it applied.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	return MigrateFS(ctx, pool, Migrations())
}

// MigrateFS applies the migrations in fsys that have not been applied yet,
// in name order, each in its own transaction, and returns the names it
// applied. It holds MigrationLock while it runs, so instances starting
// together apply each migration once: the others wait, then find nothing
// left to do. A migration that fails rolls back and stops the run; those
// before it stay applied.
func MigrateFS(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) ([]string, error) {
	names, err := Names(fsys)
	if err != nil {
		return nil, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	unlock, err := Lock(ctx, conn, MigrationLock)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return nil, err
	}
	rows, err := conn.Query(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	done, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	applied := []string{}
	for _, name := range names {
		if slices.Contains(done, name) {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return applied, err
		}
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name)
			return err
		})
		if err != nil {
			return applied, fmt.Errorf("migration %s: %w", name, err)
		}
		applied = append(applied, name)
	}
	return applied, nil
}

// Names lists the migrations in fsys, the .sql files at its top, in the
// order they apply.
func Names(fsys fs.FS) ([]string, error) {
	paths, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = path.Base(p)
	}
	slices.Sort(names)
	return names, nil
}
