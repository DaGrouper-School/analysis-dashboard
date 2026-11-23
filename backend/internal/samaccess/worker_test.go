package samaccess

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func TestBackoff(t *testing.T) {
	t.Parallel()
	for attempts, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 4: 8 * time.Second, 9: 256 * time.Second, 10: MaxBackoff, 40: MaxBackoff} {
		if got := backoff(attempts); got != want {
			t.Errorf("after %d attempts: %s, want %s", attempts, got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()
	sam := func(status int, code string) error { return &client.Error{Status: status, Code: code} }
	for _, tc := range []struct {
		err  error
		want outcome
	}{
		{nil, done},
		{Hold(errors.New("unreadable")), hold},
		{ErrNotYet, retry},
		{context.DeadlineExceeded, retry},
		{errors.New("connection refused"), retry},
		{sam(503, "unavailable"), retry},
		{sam(500, "internal"), retry},
		{sam(429, "rate_limited"), retry},
		{sam(409, "idempotency_in_progress"), retry},
		{sam(502, "bad_gateway"), retry},
		{sam(403, "forbidden"), hold},
		{sam(401, "unauthenticated"), hold},
		{sam(422, "invalid_request"), hold},
		{sam(403, "attenuation_violation"), hold},
		{sam(422, "idempotency_key_reused"), hold},
		{sam(409, "version_mismatch"), hold},
	} {
		if got, _ := classify(tc.err); got != tc.want {
			t.Errorf("%v: %d, want %d", tc.err, got, tc.want)
		}
	}
	if _, wait := classify(&client.Error{Status: 429, Code: "rate_limited", RetryAfter: 7 * time.Second}); wait != 7*time.Second {
		t.Errorf("Retry-After was not kept: %s", wait)
	}
}

// clock is a time tests move by hand.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// bench is a worker on a database of its own, with kinds that record what
// they were asked to send and answer as told.
type bench struct {
	t      *testing.T
	pool   *pgxpool.Pool
	clock  *clock
	worker *Worker
	mu     sync.Mutex
	sent   []Attempt
	answer error
}

func newBench(t *testing.T, actors Actors) *bench {
	t.Helper()
	b := &bench{t: t, pool: testutil.DB(t), clock: &clock{now: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)}}
	keys, err := GenerateKeys(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	access, err := New(Config{URL: "http://127.0.0.1:1", TenantFile: tenantFile(t, aTenant()), Issuer: "i", Audience: "sam"}, keys, b.clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(_ context.Context, a Attempt) error {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.sent = append(b.sent, a)
		return b.answer
	}
	b.worker, err = NewWorker(b.pool, access, []Kind{{Name: "thing", Apply: apply}, {Name: "personal", ByPerson: true, Apply: apply}},
		actors, b.clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *bench) enqueue(c Change) int64 {
	b.t.Helper()
	v, err := Enqueue(context.Background(), b.pool, b.clock.Now(), c)
	if err != nil {
		b.t.Fatal(err)
	}
	return v
}

func (b *bench) pass() {
	b.t.Helper()
	ran, err := b.worker.Pass(context.Background())
	if err != nil || !ran {
		b.t.Fatalf("pass: ran %v, %v", ran, err)
	}
}

func (b *bench) answering(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.answer = err
}

// taken empties what was sent so far.
func (b *bench) taken() []Attempt {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.sent
	b.sent = nil
	return s
}

// states are the entity's rows' states by version.
func (b *bench) states(entity string) map[int64]string {
	b.t.Helper()
	rows, err := b.pool.Query(context.Background(), `SELECT version, state FROM sam_outbox WHERE entity = $1`, entity)
	if err != nil {
		b.t.Fatal(err)
	}
	out := map[int64]string{}
	for rows.Next() {
		var v int64
		var s string
		if err := rows.Scan(&v, &s); err != nil {
			b.t.Fatal(err)
		}
		out[v] = s
	}
	return out
}

// TestEnqueue: versions count per entity, and a change is queued only if
// its transaction commits.
func TestEnqueue(t *testing.T) {
	t.Parallel()
	b := newBench(t, nil)
	for want := int64(1); want <= 3; want++ {
		if v := b.enqueue(Change{Entity: "thing:a", Kind: "thing", Payload: want}); v != want {
			t.Fatalf("version %d, want %d", v, want)
		}
	}
	if v := b.enqueue(Change{Entity: "thing:b", Kind: "thing", Payload: 1}); v != 1 {
		t.Fatalf("another entity starts at %d", v)
	}
	ctx := context.Background()
	err := db.InTx(ctx, b.pool, func(tx pgx.Tx) error {
		if _, err := Enqueue(ctx, tx, b.clock.Now(), Change{Entity: "thing:c", Kind: "thing", Payload: 1}); err != nil {
			return err
		}
		return errors.New("the change fails")
	})
	if err == nil || len(b.states("thing:c")) != 0 {
		t.Fatalf("a rolled-back change was queued: %v", b.states("thing:c"))
	}
	if _, err := Enqueue(ctx, b.pool, b.clock.Now(), Change{Kind: "thing"}); err == nil {
		t.Fatal("a change with no entity was queued")
	}
}

// TestOnlyTheLatestIsSent: each payload is the whole desired state, so of
// three waiting versions only the last is sent, and it supersedes the rest.
func TestOnlyTheLatestIsSent(t *testing.T) {
	t.Parallel()
	b := newBench(t, nil)
	for i := 1; i <= 3; i++ {
		b.enqueue(Change{Entity: "thing:a", Kind: "thing", Payload: map[string]int{"n": i}})
	}
	b.enqueue(Change{Entity: "thing:b", Kind: "thing", Payload: map[string]int{"n": 9}})
	b.pass()
	sent := b.taken()
	if len(sent) != 2 {
		t.Fatalf("sent %d changes, want one per entity", len(sent))
	}
	for _, a := range sent {
		if a.Entity == "thing:a" && (a.Version != 3 || string(a.Payload) != `{"n": 3}` || a.Key("space") != "thing:a:space:v3" || !a.Replayed) {
			t.Fatalf("sent %+v", a)
		}
	}
	if got := b.states("thing:a"); got[1] != "superseded" || got[2] != "superseded" || got[3] != "done" {
		t.Fatalf("states %v", got)
	}
	b.pass()
	if sent := b.taken(); len(sent) != 0 {
		t.Fatalf("sent again: %+v", sent)
	}
}

// TestRetriesAndHolds: SAM unavailable is retried with backoff; a refusal
// is held until an operator releases it.
func TestRetriesAndHolds(t *testing.T) {
	t.Parallel()
	b := newBench(t, nil)
	ctx := context.Background()
	b.enqueue(Change{Entity: "thing:a", Kind: "thing", Payload: 1})
	b.answering(&client.Error{Status: 503, Code: "unavailable"})
	b.pass()
	b.pass()
	if sent := b.taken(); len(sent) != 1 {
		t.Fatalf("retried before its backoff: %d attempts", len(sent))
	}
	b.clock.Add(time.Second)
	b.answering(&client.Error{Status: 429, Code: "rate_limited", RetryAfter: time.Minute})
	b.pass()
	b.clock.Add(30 * time.Second)
	b.pass()
	if sent := b.taken(); len(sent) != 1 {
		t.Fatalf("retried before Retry-After: %d attempts", len(sent))
	}
	b.clock.Add(time.Minute)
	b.answering(&client.Error{Status: 403, Code: "forbidden", Detail: "no grant"})
	b.pass()
	if got := b.states("thing:a"); got[1] != "held" {
		t.Fatalf("a refusal left %v", got)
	}
	held, err := HeldChanges(ctx, b.pool)
	if err != nil || len(held) != 1 || held[0].Entity != "thing:a" || held[0].Attempts != 3 || held[0].Error == "" {
		t.Fatalf("held %+v, %v", held, err)
	}
	b.clock.Add(time.Hour)
	b.taken()
	b.pass()
	if sent := b.taken(); len(sent) != 0 {
		t.Fatal("a held change was sent")
	}
	if err := Release(ctx, b.pool, b.clock.Now(), "thing:b"); !errors.Is(err, ErrNothingHeld) {
		t.Fatalf("releasing what is not held: %v", err)
	}
	if err := Release(ctx, b.pool, b.clock.Now(), "thing:a"); err != nil {
		t.Fatal(err)
	}
	b.answering(nil)
	b.pass()
	if got := b.states("thing:a"); got[1] != "done" {
		t.Fatalf("after release %v", got)
	}
}

// TestALaterVersionSupersedesAHeldOne: a held change does not block its
// entity: the next version is sent, and the held one is superseded.
func TestALaterVersionSupersedesAHeldOne(t *testing.T) {
	t.Parallel()
	b := newBench(t, nil)
	b.enqueue(Change{Entity: "thing:a", Kind: "thing", Payload: 1})
	b.answering(Hold(errors.New("no")))
	b.pass()
	b.enqueue(Change{Entity: "thing:a", Kind: "thing", Payload: 2})
	b.answering(nil)
	b.pass()
	if got := b.states("thing:a"); got[1] != "superseded" || got[2] != "done" {
		t.Fatalf("states %v", got)
	}
}

// actors is accounts for the tests: registered, waiting or gone.
type actors map[uuid.UUID]Person

func (a actors) Person(_ context.Context, id uuid.UUID) (Person, error) {
	p, ok := a[id]
	if !ok {
		return Person{}, ErrActorGone
	}
	return p, nil
}

// TestWhoActs: a person's change waits until their principal is
// registered, is held when they are gone, and is never sent as this
// service; a kind's declared actor is enforced.
func TestWhoActs(t *testing.T) {
	t.Parallel()
	waiting, registered, gone := uuid.New(), uuid.New(), uuid.New()
	people := actors{waiting: {ParticipantID: 1}, registered: {ParticipantID: 2, Registered: true}}
	b := newBench(t, people)
	b.enqueue(Change{Entity: "p:waiting", Kind: "personal", Payload: 1, Actor: &waiting})
	b.enqueue(Change{Entity: "p:registered", Kind: "personal", Payload: 1, Actor: &registered})
	b.enqueue(Change{Entity: "p:gone", Kind: "personal", Payload: 1, Actor: &gone})
	b.enqueue(Change{Entity: "p:nobody", Kind: "personal", Payload: 1})
	b.enqueue(Change{Entity: "t:somebody", Kind: "thing", Payload: 1, Actor: &registered})
	b.enqueue(Change{Entity: "x:unknown", Kind: "nothing-known", Payload: 1})
	b.pass()
	sent := b.taken()
	if len(sent) != 1 || sent[0].Entity != "p:registered" || sent[0].SAM == nil {
		t.Fatalf("sent %+v", sent)
	}
	for entity, want := range map[string]string{"p:waiting": "pending", "p:registered": "done", "p:gone": "held",
		"p:nobody": "held", "t:somebody": "held", "x:unknown": "held"} {
		if got := b.states(entity)[1]; got != want {
			t.Errorf("%s: %s, want %s", entity, got, want)
		}
	}
	people[waiting] = Person{ParticipantID: 1, Registered: true}
	b.clock.Add(time.Second)
	b.pass()
	if got := b.states("p:waiting")[1]; got != "done" {
		t.Fatalf("once registered: %s", got)
	}
}

// TestTheReplayWindow: a change first sent more than a day ago is told
// that SAM no longer replays its answers.
func TestTheReplayWindow(t *testing.T) {
	t.Parallel()
	b := newBench(t, nil)
	b.enqueue(Change{Entity: "thing:a", Kind: "thing", Payload: 1})
	b.answering(&client.Error{Status: 503, Code: "unavailable"})
	b.pass()
	b.clock.Add(time.Hour)
	b.pass()
	b.clock.Add(ReplayWindow)
	b.pass()
	sent := b.taken()
	if len(sent) != 3 || !sent[0].Replayed || !sent[1].Replayed || sent[2].Replayed {
		t.Fatalf("replayed: %v %v %v", sent[0].Replayed, sent[1].Replayed, sent[2].Replayed)
	}
}

// TestOneWorkerAtATime: while another instance holds the outbox lock, a
// pass sends nothing.
func TestOneWorkerAtATime(t *testing.T) {
	t.Parallel()
	b := newBench(t, nil)
	ctx := context.Background()
	b.enqueue(Change{Entity: "thing:a", Kind: "thing", Payload: 1})
	other, err := b.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Rollback(ctx) }()
	if ok, err := db.TryLock(ctx, other, db.OutboxLock); err != nil || !ok {
		t.Fatalf("taking the lock: %v %v", ok, err)
	}
	if ran, err := b.worker.Pass(ctx); err != nil || ran {
		t.Fatalf("a second worker ran: %v %v", ran, err)
	}
	if len(b.taken()) != 0 {
		t.Fatal("a second worker sent a change")
	}
}

func TestKindsAreChecked(t *testing.T) {
	t.Parallel()
	apply := func(context.Context, Attempt) error { return nil }
	if _, err := NewWorker(nil, nil, []Kind{{Name: "a", Apply: apply}, {Name: "a", Apply: apply}}, nil, time.Now); err == nil {
		t.Error("a kind declared twice")
	}
	if _, err := NewWorker(nil, nil, []Kind{{Name: "a"}}, nil, time.Now); err == nil {
		t.Error("a kind with no applier")
	}
}

// TestReconciler: every check runs on a pass, a failing one does not stop
// the rest, and only the instance holding the lock passes.
func TestReconciler(t *testing.T) {
	t.Parallel()
	pool := testutil.DB(t)
	ctx := context.Background()
	var ran []string
	r, err := NewReconciler(pool, []Check{
		{Name: "users", Run: func(context.Context) error { ran = append(ran, "users"); return errors.New("SAM is down") }},
		{Name: "sharing", Run: func(context.Context) error { ran = append(ran, "sharing"); return nil }},
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	passed, err := r.Pass(ctx)
	if !passed || err == nil || len(ran) != 2 {
		t.Fatalf("pass: %v %v, ran %v", passed, err, ran)
	}
	other, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Rollback(ctx) }()
	if ok, err := db.TryLock(ctx, other, db.ReconcilerLock); err != nil || !ok {
		t.Fatal("taking the lock")
	}
	ran = nil
	if passed, err := r.Pass(ctx); passed || err != nil || len(ran) != 0 {
		t.Fatalf("a second reconciler ran: %v %v %v", passed, err, ran)
	}
	if _, err := NewReconciler(pool, nil, 0); err == nil {
		t.Error("no interval")
	}
}
