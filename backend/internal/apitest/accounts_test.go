package apitest_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// accountsBench is the accounts module and an outbox worker sending its
// changes through access.
type accountsBench struct {
	t      *testing.T
	e      *apitest.Env
	m      *accounts.Module
	worker *samaccess.Worker
	clock  *manualClock
}

func newAccounts(t *testing.T, e *apitest.Env, access *samaccess.Access) *accountsBench {
	t.Helper()
	b := &accountsBench{t: t, e: e, m: accounts.New(e.Pool, e.Access, time.Now), clock: &manualClock{now: time.Now()}}
	var err error
	b.worker, err = samaccess.NewWorker(e.Pool, access, b.m.OutboxKinds(), b.m, b.clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *accountsBench) create() accounts.User {
	b.t.Helper()
	a, err := accounts.Create(context.Background(), b.e.Pool, b.clock.Now())
	if err != nil {
		b.t.Fatal(err)
	}
	return a
}

func (b *accountsBench) pass() {
	b.t.Helper()
	pass(b.t, b.worker)
	b.clock.Add(samaccess.MaxBackoff)
}

func (b *accountsBench) guard() {
	b.t.Helper()
	if err := b.m.Guard().Check(context.Background()); err != nil {
		b.t.Fatal(err)
	}
}

func (b *accountsBench) get(a accounts.User) accounts.User {
	b.t.Helper()
	got, err := accounts.Get(context.Background(), b.e.Pool, a.ID)
	if err != nil {
		b.t.Fatal(err)
	}
	return got
}

// inSAM registers participant ID n as a user of this service's issuer
// directly, as a principal registered before a database restore would be.
func inSAM(t *testing.T, e *apitest.Env, n int64) client.Principal {
	t.Helper()
	tn := e.Access.Tenant()
	issuer := tn.IssuerID()
	reg, err := e.Access.Service().Register(context.Background(), client.PrincipalInput{Type: "user", HomeSpaceID: tn.RootSpaceID,
		IssuerID: &issuer, Subject: strconv.FormatInt(n, 10)})
	if err != nil {
		t.Fatal(err)
	}
	return reg.Principal
}

// samUser is the principal SAM has for participant ID n.
func samUser(t *testing.T, e *apitest.Env, n int64) client.Principal {
	t.Helper()
	p, err := e.Access.Service().FindUser(context.Background(), e.Access.Tenant().IssuerID(), strconv.FormatInt(n, 10))
	if err != nil {
		t.Fatalf("participant %d in SAM: %v", n, err)
	}
	return p
}

// TestAnAccountIsRegistered: the worker registers a new account's
// principal under its participant ID and keeps it.
func TestAnAccountIsRegistered(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	b := newAccounts(t, e, e.Access)
	b.guard()
	a := b.create()
	b.pass()
	got := b.get(a)
	if got.Principal == nil || *got.Principal != samUser(t, e, a.ParticipantID).ID {
		t.Fatalf("account %+v", got)
	}
	if p, err := b.m.Person(context.Background(), a.ID); err != nil || !p.Registered {
		t.Fatalf("as an actor: %+v, %v", p, err)
	}
}

// dropFirstRegistration loses SAM's answer to the first registration.
type dropFirstRegistration struct {
	mu      sync.Mutex
	dropped bool
}

func (d *dropFirstRegistration) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := http.DefaultTransport.RoundTrip(r)
	d.mu.Lock()
	defer d.mu.Unlock()
	if err == nil && !d.dropped && r.Method == http.MethodPost && r.URL.Path == "/v1/principals" {
		d.dropped = true
		_ = res.Body.Close()
		return nil, errors.New("connection reset after SAM answered")
	}
	return res, err
}

// TestALostRegistrationIsAdopted: after a lost answer, the retry's key
// gets SAM's first answer back, the account's own principal, which is
// adopted rather than refused.
func TestALostRegistrationIsAdopted(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	keys, err := samaccess.GenerateKeys(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cfg := e.SAM.Config(optionsOf(e))
	cfg.HTTPClient = &http.Client{Transport: &dropFirstRegistration{}, Timeout: samaccess.CallTimeout}
	lossy, err := samaccess.New(cfg, keys, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	b := newAccounts(t, e, lossy)
	b.guard()
	a := b.create()
	b.pass()
	if got := b.get(a); got.Principal != nil {
		t.Fatal("the lost answer was recorded")
	}
	b.pass()
	got := b.get(a)
	if got.Principal == nil || *got.Principal != samUser(t, e, a.ParticipantID).ID {
		t.Fatalf("after the retry: %+v", got)
	}
}

// TestAReusedIDIsHeld: after a restore, an account given an ID SAM knows
// as someone else's is held, never adopted; renumbered, it is registered.
func TestAReusedIDIsHeld(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	b := newAccounts(t, e, e.Access)
	before := inSAM(t, e, 1)
	a := b.create()
	b.guard()
	b.pass()
	if got := b.get(a); got.Principal != nil {
		t.Fatalf("the principal of participant 1 from before the restore was adopted: %+v", got)
	}
	held, err := samaccess.HeldChanges(context.Background(), e.Pool)
	if err != nil || len(held) != 1 || held[0].Entity != accounts.Entity(a.ID) {
		t.Fatalf("held %+v, %v", held, err)
	}
	if next := b.create(); next.ParticipantID != 2 {
		t.Fatalf("the next account after the guard got %d", next.ParticipantID)
	}
	r, err := accounts.Renumber(context.Background(), e.Pool, b.clock.Now(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	b.pass()
	got := b.get(a)
	if got.Principal == nil || *got.Principal == before.ID || *got.Principal != samUser(t, e, r.ParticipantID).ID {
		t.Fatalf("renumbered: %+v", got)
	}
}

// TestTheGuardAfterARestore: with a run of IDs SAM knows above the
// sequence, the guard moves the sequence past its end.
func TestTheGuardAfterARestore(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	b := newAccounts(t, e, e.Access)
	for n := int64(1); n <= 37; n++ {
		inSAM(t, e, n)
	}
	b.guard()
	if a := b.create(); a.ParticipantID != 38 {
		t.Fatalf("the first account after the guard got %d, want 38", a.ParticipantID)
	}
	b.guard()
	if a := b.create(); a.ParticipantID != 39 {
		t.Fatalf("running the guard again moved the sequence: %d", a.ParticipantID)
	}
}

// TestRegistrationsWaitForTheGuard: while SAM cannot be asked, the guard
// has not run and registrations wait; once SAM is back, they go through.
func TestRegistrationsWaitForTheGuard(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	b := newAccounts(t, e, e.Access)
	e.SAM.Stop()
	if err := b.m.Guard().Check(context.Background()); err == nil || b.m.Guard().Done() {
		t.Fatalf("the guard ran without SAM: %v", err)
	}
	a := b.create()
	b.pass()
	if s, _ := state(t, e, accounts.Entity(a.ID)); s != "pending" {
		t.Fatalf("a registration before the guard: %s", s)
	}
	e.SAM.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	b.m.Guard().Run(ctx)
	if !b.m.Guard().Done() {
		t.Fatal("the guard did not run once SAM was back")
	}
	b.pass()
	if got := b.get(a); got.Principal == nil {
		t.Fatal("the registration did not go through")
	}
}

// TestARestoredUserIsAdopted: a user restored from a backup whose
// registration reached SAM after the backup is held like any 409; once an
// operator adopts SAM's principal as theirs, the registration completes.
func TestARestoredUserIsAdopted(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	b := newAccounts(t, e, e.Access)
	ctx := context.Background()
	a := b.create()
	theirs := inSAM(t, e, a.ParticipantID)
	b.guard()
	b.pass()
	if s, _ := state(t, e, accounts.Entity(a.ID)); s != "held" {
		t.Fatalf("the registration: %s", s)
	}
	p, err := b.m.Adopt(ctx, a.ID)
	if err != nil || p.ID != theirs.ID {
		t.Fatalf("adopting: %+v, %v", p, err)
	}
	b.pass()
	if s, _ := state(t, e, accounts.Entity(a.ID)); s != "done" {
		t.Fatalf("after adopting: %s", s)
	}
	if got := b.get(a); got.Principal == nil || *got.Principal != theirs.ID {
		t.Fatalf("user %+v", got)
	}
	if _, err := b.m.Adopt(ctx, a.ID); !errors.Is(err, accounts.ErrRegistered) {
		t.Fatalf("adopting again: %v", err)
	}
	other := b.create()
	if _, err := b.m.Adopt(ctx, other.ID); !errors.Is(err, accounts.ErrNotInSAM) {
		t.Fatalf("adopting with nothing in SAM: %v", err)
	}
}
