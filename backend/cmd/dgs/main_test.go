package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestServeDrains: serve answers until its context ends, then waits for a
// request in flight to finish before returning.
func TestServeDrains(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, h) }()

	got := make(chan int, 1)
	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			got <- 0
			return
		}
		res.Body.Close()
		got <- res.StatusCode
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		t.Fatalf("serve returned %v with a request in flight", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if s := <-got; s != http.StatusOK {
		t.Fatalf("request in flight: status %d", s)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(shutdownTimeout + time.Second):
		t.Fatal("serve did not return after its context ended")
	}
}

// setSAM points the configuration at a SAM; the commands tested here
// never call it.
func setSAM(t *testing.T) {
	t.Helper()
	t.Setenv("DGS_SAM_URL", "http://127.0.0.1:1")
	t.Setenv("DGS_SAM_TENANT_FILE", filepath.Join(t.TempDir(), "tenant.json"))
	t.Setenv("DGS_SAM_ISSUER", "http://127.0.0.1:8090")
	t.Setenv("DGS_SIGNING_KEYS_FILE", "")
}

func TestRunRefuses(t *testing.T) {
	for _, args := range [][]string{{"frobnicate"}, {"keys"}, {"keys", "old"}, {"serve", "now"}, {"outbox"}, {"outbox", "release"}, {"accounts", "renumber"}, {"accounts", "adopt"}} {
		if err := run(context.Background(), io.Discard, args); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("%v: %v", args, err)
		}
	}
	t.Setenv("DGS_ENV", "")
	if err := run(context.Background(), io.Discard, []string{"serve"}); err == nil || !strings.Contains(err.Error(), "DGS_ENV") {
		t.Fatalf("no environment: %v", err)
	}
}

// TestKeysNew: dgs keys new prints a key that, once it signs, is a valid
// key file entry.
func TestKeysNew(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), &out, []string{"keys", "new"}); err != nil {
		t.Fatal(err)
	}
	var k samaccess.KeyEntry
	if err := json.Unmarshal(out.Bytes(), &k); err != nil || k.Signs {
		t.Fatalf("printed %q: %v", out.String(), err)
	}
	k.Signs = true
	file, _ := json.Marshal(map[string]any{"keys": []samaccess.KeyEntry{k}})
	if keys, err := samaccess.ParseKeys(file); err != nil || keys.SigningKeyID() != k.ID {
		t.Fatalf("the printed key in a key file: %v", err)
	}
}

// TestMigrate: dgs migrate brings an empty database up to date, and a
// second run applies nothing.
func TestMigrate(t *testing.T) {
	dsn := testutil.EmptyDB(t)
	t.Setenv("DGS_ENV", "test")
	t.Setenv("DGS_DATABASE_URL", dsn)
	setSAM(t)
	for range 2 {
		if err := run(context.Background(), io.Discard, []string{"migrate"}); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := testutil.Pool(t, dsn).QueryRow(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("after migrating: %v", err)
	}
}

// TestServeNeedsTheDatabase: serve stops before listening when the
// database does not answer.
func TestServeNeedsTheDatabase(t *testing.T) {
	t.Setenv("DGS_ENV", "test")
	t.Setenv("DGS_DATABASE_URL", "postgres://dgs@127.0.0.1:1/dgs?connect_timeout=2")
	setSAM(t)
	if err := run(context.Background(), io.Discard, []string{"serve"}); err == nil {
		t.Fatal("serve started without a database")
	}
}

// TestOutboxCommands: dgs outbox held lists what is held, and release
// sends an entity's held change again.
func TestOutboxCommands(t *testing.T) {
	dsn := testutil.EmptyDB(t)
	t.Setenv("DGS_ENV", "test")
	t.Setenv("DGS_DATABASE_URL", dsn)
	setSAM(t)
	ctx := context.Background()
	if err := run(ctx, io.Discard, []string{"migrate"}); err != nil {
		t.Fatal(err)
	}
	pool := testutil.Pool(t, dsn)
	if _, err := samaccess.Enqueue(ctx, pool, time.Now(), samaccess.Change{Entity: "user:a", Kind: "user", Payload: 1}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(ctx, &out, []string{"outbox", "held"}); err != nil || !strings.Contains(out.String(), "nothing is held") {
		t.Fatalf("with nothing held: %q, %v", out.String(), err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sam_outbox SET state = 'held', last_error = 'sam: 403 forbidden'`); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(ctx, &out, []string{"outbox", "held"}); err != nil || !strings.Contains(out.String(), "user:a") ||
		!strings.Contains(out.String(), "403 forbidden") || !strings.Contains(out.String(), "this service") {
		t.Fatalf("held: %q, %v", out.String(), err)
	}
	if err := run(ctx, io.Discard, []string{"outbox", "release", "user:a"}); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, io.Discard, []string{"outbox", "release", "user:a"}); !errors.Is(err, samaccess.ErrNothingHeld) {
		t.Fatalf("releasing it again: %v", err)
	}
}

// TestAccountsRenumber: dgs accounts renumber gives an account a new
// participant ID, and refuses what is not an account.
func TestAccountsRenumber(t *testing.T) {
	dsn := testutil.EmptyDB(t)
	t.Setenv("DGS_ENV", "test")
	t.Setenv("DGS_DATABASE_URL", dsn)
	setSAM(t)
	ctx := context.Background()
	if err := run(ctx, io.Discard, []string{"migrate"}); err != nil {
		t.Fatal(err)
	}
	a, err := accounts.Create(ctx, testutil.Pool(t, dsn), time.Now(), accounts.NewUser{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(ctx, &out, []string{"accounts", "renumber", a.ID.String()}); err != nil || !strings.Contains(out.String(), "participant ID 2") {
		t.Fatalf("renumbering: %q, %v", out.String(), err)
	}
	if err := run(ctx, io.Discard, []string{"accounts", "renumber", "someone"}); err == nil || !strings.Contains(err.Error(), "not a user ID") {
		t.Fatalf("not an ID: %v", err)
	}
	if err := run(ctx, io.Discard, []string{"accounts", "renumber", uuid.NewString()}); err == nil || !strings.Contains(err.Error(), "no user") {
		t.Fatalf("an unknown account: %v", err)
	}
}
