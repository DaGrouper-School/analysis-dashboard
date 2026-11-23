package db_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func TestOpenRefusesUnboundedPools(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, cfg := range []db.Config{{}, {MaxConns: 4}, {StatementTimeout: time.Second}} {
		if _, err := db.Open(ctx, "postgres://localhost/none", cfg); err == nil {
			t.Errorf("pool %+v opened", cfg)
		}
	}
}

func TestOpenSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, err := db.Open(ctx, testutil.EmptyDB(t), db.Config{MaxConns: 2, StatementTimeout: 1500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var timeout string
	if err := pool.QueryRow(ctx, "SELECT current_setting('statement_timeout')").Scan(&timeout); err != nil || timeout != "1500ms" {
		t.Fatalf("statement_timeout %q, %v", timeout, err)
	}
	if pool.Config().MaxConns != 2 {
		t.Errorf("max conns %d", pool.Config().MaxConns)
	}
}

// fixtures are migrations for testing the runner: two that create tables,
// in an order their names give, not the map's.
var fixtures = fstest.MapFS{
	"0002_b.sql": {Data: []byte("-- b\nCREATE TABLE b (id int REFERENCES a(id));")},
	"0001_a.sql": {Data: []byte("-- a\nCREATE TABLE a (id int PRIMARY KEY);")},
	"README.md":  {Data: []byte("not a migration")},
}

// TestMigrationsApplyOnce: in name order, each recorded, nothing on a
// second run; instances starting together apply each exactly once.
func TestMigrationsApplyOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.Pool(t, testutil.EmptyDB(t))
	const instances = 4
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		applied []string
		errs    []error
	)
	for range instances {
		wg.Go(func() {
			got, err := db.MigrateFS(ctx, pool, fixtures)
			mu.Lock()
			defer mu.Unlock()
			applied, errs = append(applied, got...), append(errs, err)
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
	slices.Sort(applied)
	if !slices.Equal(applied, []string{"0001_a.sql", "0002_b.sql"}) {
		t.Fatalf("instances together applied %v", applied)
	}
	again, err := db.MigrateFS(ctx, pool, fixtures)
	if err != nil || len(again) != 0 {
		t.Fatalf("a second run applied %v, %v", again, err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("%d recorded, %v", n, err)
	}
}

// TestAFailedMigrationStops: it rolls back and is named; those before it
// stay applied, and the next run starts with it.
func TestAFailedMigrationStops(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.Pool(t, testutil.EmptyDB(t))
	bad := fstest.MapFS{
		"0001_a.sql":   fixtures["0001_a.sql"],
		"0002_bad.sql": {Data: []byte("CREATE TABLE half (id int); SELECT no_such_function();")},
	}
	applied, err := db.MigrateFS(ctx, pool, bad)
	if err == nil || !slices.Equal(applied, []string{"0001_a.sql"}) {
		t.Fatalf("applied %v, %v", applied, err)
	}
	var half bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('half') IS NOT NULL`).Scan(&half); err != nil || half {
		t.Fatalf("the failed migration left its table: %v", err)
	}
	fixed := fstest.MapFS{"0001_a.sql": fixtures["0001_a.sql"], "0002_bad.sql": {Data: []byte("CREATE TABLE half (id int);")}}
	if applied, err := db.MigrateFS(ctx, pool, fixed); err != nil || !slices.Equal(applied, []string{"0002_bad.sql"}) {
		t.Fatalf("after the fix: %v, %v", applied, err)
	}
}

// TestTheServicesMigrations: every one applies on an empty database, and
// the migrated template has them all.
func TestTheServicesMigrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	names, err := db.Names(db.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	pool := testutil.Pool(t, testutil.EmptyDB(t))
	applied, err := db.Migrate(ctx, pool)
	if err != nil || !slices.Equal(applied, names) {
		t.Fatalf("applied %v of %v, %v", applied, names, err)
	}
	var n int
	if err := testutil.DB(t).QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != len(names) {
		t.Fatalf("the template holds %d migrations of %d, %v", n, len(names), err)
	}
}

// TestInTx: a transaction commits or rolls back with fn; inside WithTx,
// a nested InTx is a savepoint whose rollback leaves the outer work.
func TestInTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.Pool(t, testutil.EmptyDB(t))
	if _, err := pool.Exec(ctx, `CREATE TABLE t (v int)`); err != nil {
		t.Fatal(err)
	}
	insert := func(v int) func(pgx.Tx) error {
		return func(tx pgx.Tx) error { _, err := tx.Exec(ctx, `INSERT INTO t VALUES ($1)`, v); return err }
	}
	failed := errors.New("failed")
	if err := db.InTx(ctx, pool, insert(1)); err != nil {
		t.Fatal(err)
	}
	if err := db.InTx(ctx, pool, func(tx pgx.Tx) error { _ = insert(2)(tx); return failed }); !errors.Is(err, failed) {
		t.Fatal(err)
	}
	err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		inner := db.WithTx(ctx, tx)
		if err := db.InTx(inner, pool, insert(3)); err != nil {
			return err
		}
		_ = db.InTx(inner, pool, func(tx pgx.Tx) error { _ = insert(4)(tx); return failed })
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := pool.Query(ctx, `SELECT v FROM t ORDER BY v`)
	got, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil || !slices.Equal(got, []int{1, 3}) {
		t.Fatalf("rows %v, %v", got, err)
	}
}

// TestTryLock: one transaction holds a lock; another is told it is taken
// until the first ends.
func TestTryLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.Pool(t, testutil.EmptyDB(t))
	const key = 8301500
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := db.TryLock(ctx, first, key); err != nil || !ok {
		t.Fatalf("the first lock: %v, %v", ok, err)
	}
	try := func() bool {
		t.Helper()
		var ok bool
		err := db.InTx(ctx, pool, func(tx pgx.Tx) error { var err error; ok, err = db.TryLock(ctx, tx, key); return err })
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if try() {
		t.Fatal("a second transaction took a held lock")
	}
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if !try() {
		t.Fatal("the lock was not released with its transaction")
	}
}
