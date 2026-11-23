// Package testutil gives tests that need a database one of their own, on a
// real PostgreSQL.
//
// DGS_TEST_DATABASE_URL names an admin connection to a server tests may
// create databases on; `make test` sets it. Each test gets a new database,
// dropped when the test ends, so tests run in parallel and share nothing.
// Under -short the tests that need a database are skipped.
//
// A migrated database is a copy of a template migrated once per set of
// migrations, named after their hash, so tests do not each apply every
// migration.
package testutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// EnvURL is the variable holding the admin connection string.
const EnvURL = "DGS_TEST_DATABASE_URL"

// PoolConfig sizes each test's pool, so parallel tests stay within the
// server's connection limit.
var PoolConfig = db.Config{MaxConns: 6, StatementTimeout: 30 * time.Second}

var counter atomic.Int64

// EmptyDB creates a new database with nothing in it and returns its
// connection string, dropped after t. It skips t under -short, and fails
// it when DGS_TEST_DATABASE_URL is not set.
func EmptyDB(t testing.TB) string {
	t.Helper()
	return newDB(t, "")
}

// newDB creates a database, a copy of template when it is not "", dropped
// after t.
func newDB(t testing.TB, template string) string {
	t.Helper()
	admin := adminURL(t)
	name := fmt.Sprintf("dgs_test_%d_%d", os.Getpid(), counter.Add(1))
	sql := "CREATE DATABASE " + name
	if template != "" {
		sql += " TEMPLATE " + template
	}
	exec(t, admin, sql)
	t.Cleanup(func() { exec(t, admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)") })
	return withDatabase(t, admin, name)
}

func adminURL(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("needs a database; skipped with -short")
	}
	admin := os.Getenv(EnvURL)
	if admin == "" {
		t.Fatalf("this test needs a database: set %s (make test does), or run with -short", EnvURL)
	}
	return admin
}

func withDatabase(t testing.TB, dsn, name string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("%s: %v", EnvURL, err)
	}
	u.Path = "/" + name
	return u.String()
}

var (
	templateOnce sync.Once
	templateName string
	templateErr  error
)

// template returns the name of a database holding every migration, made
// once per set of migrations and kept for later runs. Packages testing at
// once take turns under an advisory lock to make it.
func template(t testing.TB) string {
	t.Helper()
	admin := adminURL(t)
	templateOnce.Do(func() { templateName, templateErr = makeTemplate(admin) })
	if templateErr != nil {
		t.Fatalf("migrated template database: %v", templateErr)
	}
	return templateName
}

func makeTemplate(admin string) (string, error) {
	fsys := db.Migrations()
	names, err := db.Names(fsys)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, n := range names {
		body, err := fs.ReadFile(fsys, n)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "%s\n%s\n", n, body) // a hash never fails to write
	}
	name := "dgs_template_" + hex.EncodeToString(h.Sum(nil))[:16]

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close(ctx) }()
	unlock, err := db.Lock(ctx, conn, db.TestTemplateLock)
	if err != nil {
		return "", err
	}
	defer unlock()
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil || exists {
		return name, err
	}
	// Built under another name and renamed, so a run that fails halfway
	// leaves no template that looks complete.
	building := name + "_building"
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+building+" WITH (FORCE)"); err != nil {
		return "", err
	}
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+building); err != nil {
		return "", err
	}
	u, err := url.Parse(admin)
	if err != nil {
		return "", err
	}
	u.Path = "/" + building
	pool, err := db.Open(ctx, u.String(), db.Config{MaxConns: 1, StatementTimeout: 5 * time.Minute})
	if err != nil {
		return "", err
	}
	_, err = db.Migrate(ctx, pool)
	pool.Close()
	if err != nil {
		return "", err
	}
	_, err = conn.Exec(ctx, "ALTER DATABASE "+building+" RENAME TO "+name)
	return name, err
}

// DB returns a pool on a new, migrated database, closed and dropped after
// t. It skips t under -short.
func DB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return Pool(t, Migrated(t))
}

// Migrated returns the connection string of a new, migrated database.
func Migrated(t testing.TB) string {
	t.Helper()
	return newDB(t, template(t))
}

// Pool opens a pool on dsn, closed after t.
func Pool(t testing.TB, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := db.Open(context.Background(), dsn, PoolConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func exec(t testing.TB, dsn, sql string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
}
