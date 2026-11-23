package apitest_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// spaceKind creates a space under the tenant root as this service, once
// per version: a change whose effect is visible, and would show if it were
// applied twice.
func spaceKind(root func() samaccess.Tenant) samaccess.Kind {
	return samaccess.Kind{Name: "test-space", Apply: func(ctx context.Context, a samaccess.Attempt) error {
		_, err := a.SAM.CreateSpace(a.WithKey(ctx, "space"), client.SpaceInput{ParentID: root().RootSpaceID})
		return err
	}}
}

// manualClock is a clock the test moves; the worker runs in the test's
// goroutine, so no lock is needed.
type manualClock struct{ now time.Time }

func (c *manualClock) Now() time.Time      { return c.now }
func (c *manualClock) Add(d time.Duration) { c.now = c.now.Add(d) }

func worker(t *testing.T, e *apitest.Env, access *samaccess.Access, c *manualClock) *samaccess.Worker {
	t.Helper()
	w, err := samaccess.NewWorker(e.Pool, access, []samaccess.Kind{spaceKind(access.Tenant)}, nil, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func pass(t *testing.T, w *samaccess.Worker) {
	t.Helper()
	if ran, err := w.Pass(context.Background()); err != nil || !ran {
		t.Fatalf("pass: %v %v", ran, err)
	}
}

// spaces counts the spaces under the tenant root, as the operator sees
// them.
func spaces(t *testing.T, e *apitest.Env) int {
	t.Helper()
	page, err := e.SAM.Operator().Children(context.Background(), e.SAM.Tenant.RootSpaceID, client.PageQuery{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	return len(page.Items)
}

func state(t *testing.T, e *apitest.Env, entity string) (string, int) {
	t.Helper()
	var s string
	var attempts int
	err := e.Pool.QueryRow(context.Background(), `SELECT state, attempts FROM sam_outbox WHERE entity = $1 ORDER BY version DESC LIMIT 1`,
		entity).Scan(&s, &attempts)
	if err != nil {
		t.Fatal(err)
	}
	return s, attempts
}

// TestTheOutboxConvergesAfterAnOutage: while SAM is down a change waits
// and is retried; once SAM is back it arrives, once.
func TestTheOutboxConvergesAfterAnOutage(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	c := &manualClock{now: time.Now()}
	w := worker(t, e, e.Access, c)

	e.SAM.Stop()
	if _, err := samaccess.Enqueue(context.Background(), e.Pool, c.Now(), samaccess.Change{Entity: "test:a", Kind: "test-space", Payload: 1}); err != nil {
		t.Fatal(err)
	}
	pass(t, w)
	c.Add(time.Second)
	pass(t, w)
	if s, attempts := state(t, e, "test:a"); s != "pending" || attempts != 2 {
		t.Fatalf("while SAM is down: %s after %d attempts", s, attempts)
	}

	e.SAM.Start()
	c.Add(samaccess.MaxBackoff)
	pass(t, w)
	if s, _ := state(t, e, "test:a"); s != "done" {
		t.Fatalf("once SAM is back: %s", s)
	}
	if n := spaces(t, e); n != 1 {
		t.Fatalf("%d spaces under the tenant root, want 1", n)
	}
}

// dropFirstAnswer lets the first request for a space reach SAM, then loses
// SAM's answer, as a connection dropped after SAM committed would.
type dropFirstAnswer struct {
	mu      sync.Mutex
	dropped bool
}

func (d *dropFirstAnswer) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := http.DefaultTransport.RoundTrip(r)
	d.mu.Lock()
	defer d.mu.Unlock()
	if err == nil && !d.dropped && r.Method == http.MethodPost && r.URL.Path == "/v1/spaces" {
		d.dropped = true
		_ = res.Body.Close()
		return nil, errors.New("connection reset after SAM answered")
	}
	return res, err
}

// TestALostAnswerIsNotAppliedTwice: the retry after a lost answer carries
// the same idempotency key, so SAM replays its first answer instead of
// creating a second space.
func TestALostAnswerIsNotAppliedTwice(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	keys, err := samaccess.GenerateKeys(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cfg := e.SAM.Config(optionsOf(e))
	cfg.HTTPClient = &http.Client{Transport: &dropFirstAnswer{}, Timeout: samaccess.CallTimeout}
	lossy, err := samaccess.New(cfg, keys, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	c := &manualClock{now: time.Now()}
	w := worker(t, e, lossy, c)
	if _, err := samaccess.Enqueue(context.Background(), e.Pool, c.Now(), samaccess.Change{Entity: "test:a", Kind: "test-space", Payload: 1}); err != nil {
		t.Fatal(err)
	}
	pass(t, w)
	if s, attempts := state(t, e, "test:a"); s != "pending" || attempts != 1 {
		t.Fatalf("after the lost answer: %s after %d attempts", s, attempts)
	}
	if n := spaces(t, e); n != 1 {
		t.Fatalf("SAM did not act on the first request: %d spaces", n)
	}
	c.Add(time.Second)
	pass(t, w)
	if s, _ := state(t, e, "test:a"); s != "done" {
		t.Fatalf("after the retry: %s", s)
	}
	if n := spaces(t, e); n != 1 {
		t.Fatalf("%d spaces under the tenant root: the change was applied twice", n)
	}
}

// TestARefusalIsHeld: a change SAM refuses this service is held, not
// retried.
func TestARefusalIsHeld(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	c := &manualClock{now: time.Now()}
	sys, err := e.SAM.Service().System(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	above := samaccess.Kind{Name: "test-space", Apply: func(ctx context.Context, a samaccess.Attempt) error {
		_, err := a.SAM.CreateSpace(a.WithKey(ctx, "space"), client.SpaceInput{ParentID: sys.RootSpaceID})
		return err
	}}
	w, err := samaccess.NewWorker(e.Pool, e.Access, []samaccess.Kind{above}, nil, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := samaccess.Enqueue(context.Background(), e.Pool, c.Now(), samaccess.Change{Entity: "test:a", Kind: "test-space", Payload: 1}); err != nil {
		t.Fatal(err)
	}
	pass(t, w)
	if s, _ := state(t, e, "test:a"); s != "held" {
		t.Fatalf("a refused change: %s", s)
	}
}
