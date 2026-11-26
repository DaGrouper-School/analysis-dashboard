package apitest_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	b := &accountsBench{t: t, e: e, m: accounts.New(e.Pool, e.Access, time.Now, accounts.Config{}), clock: &manualClock{now: time.Now()}}
	var err error
	b.worker, err = samaccess.NewWorker(e.Pool, access, b.m.OutboxKinds(), b.m, b.clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *accountsBench) create() accounts.User {
	b.t.Helper()
	a, err := accounts.Create(context.Background(), b.e.Pool, b.clock.Now(), accounts.NewUser{})
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

// countAssertions counts the attribute assertions sent to SAM.
type countAssertions struct {
	mu sync.Mutex
	n  int
}

func (c *countAssertions) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/attributes") {
		c.mu.Lock()
		c.n++
		c.mu.Unlock()
	}
	return http.DefaultTransport.RoundTrip(r)
}

func (c *countAssertions) count() int { c.mu.Lock(); defer c.mu.Unlock(); return c.n }

// held is what SAM holds about participant n at the tenant root.
func held(t *testing.T, e *apitest.Env, n int64) map[string]string {
	t.Helper()
	p := samUser(t, e, n)
	root := e.Access.Tenant().RootSpaceID
	attrs, err := e.Access.Service().Attributes(context.Background(), p.ID, &root)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, a := range attrs {
		out[a.Name] = a.Value
	}
	return out
}

func (b *accountsBench) setMinor(u accounts.User, minor bool) {
	b.t.Helper()
	if _, err := accounts.SetMinor(context.Background(), b.e.Pool, b.clock.Now(), u.ID, minor); err != nil {
		b.t.Fatal(err)
	}
}

func (b *accountsBench) setConsent(u accounts.User, c accounts.Consent) {
	b.t.Helper()
	if _, err := accounts.SetConsent(context.Background(), b.e.Pool, b.clock.Now(), u.ID, c); err != nil {
		b.t.Fatal(err)
	}
}

// TestMinorAndConsentReachSAM: a user's minor and consent are asserted at
// the tenant root; a value SAM already holds is never asserted again; and
// consent that is withdrawn, or asked for again after it was given, is
// withdrawn in SAM.
func TestMinorAndConsentReachSAM(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	keys, err := samaccess.GenerateKeys(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	counter := &countAssertions{}
	cfg := e.SAM.Config(optionsOf(e))
	cfg.HTTPClient = &http.Client{Transport: counter, Timeout: samaccess.CallTimeout}
	counting, err := samaccess.New(cfg, keys, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	b := newAccounts(t, e, counting)
	b.guard()

	adult := b.create()
	b.setMinor(adult, false)
	minor := b.create()
	b.setMinor(minor, true)
	b.setConsent(minor, accounts.ConsentGiven)
	b.pass()
	if got := held(t, e, adult.ParticipantID); got["minor"] != "false" || got["consent"] != "" {
		t.Fatalf("the adult in SAM: %v", got)
	}
	if got := held(t, e, minor.ParticipantID); got["minor"] != "true" || got["consent"] != "given" {
		t.Fatalf("the minor in SAM: %v", got)
	}
	sent := counter.count()
	if sent != 3 {
		t.Fatalf("%d assertions, want 3", sent)
	}

	b.setMinor(minor, true)
	b.setConsent(minor, accounts.ConsentGiven)
	b.pass()
	if counter.count() != sent {
		t.Fatalf("values SAM already holds were asserted again: %d assertions", counter.count()-sent)
	}

	b.setConsent(minor, accounts.ConsentPending)
	b.pass()
	if got := held(t, e, minor.ParticipantID); got["consent"] != "withdrawn" {
		t.Fatalf("asked again after consent was given: SAM holds %v", got)
	}
	b.setConsent(minor, accounts.ConsentGiven)
	b.pass()
	b.setConsent(minor, accounts.ConsentWithdrawn)
	b.pass()
	if got := held(t, e, minor.ParticipantID); got["consent"] != "withdrawn" {
		t.Fatalf("after a withdrawal: SAM holds %v", got)
	}
}

// TestUsersConvergeAfterAnOutage: consent given while SAM is down reaches
// SAM once it is back.
func TestUsersConvergeAfterAnOutage(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	b := newAccounts(t, e, e.Access)
	b.guard()
	u := b.create()
	b.setMinor(u, true)
	b.pass()
	e.SAM.Stop()
	b.setConsent(u, accounts.ConsentGiven)
	b.pass()
	if s, _ := state(t, e, accounts.Entity(u.ID)); s != "pending" {
		t.Fatalf("while SAM is down: %s", s)
	}
	e.SAM.Start()
	b.pass()
	if got := held(t, e, u.ParticipantID); got["consent"] != "given" {
		t.Fatalf("once SAM is back: SAM holds %v", got)
	}
}

// TestTheReconcilerCorrectsSAM: consent given in SAM by hand, after it was
// withdrawn here, is found by the reconciler and withdrawn again, queued as
// taking access away.
func TestTheReconcilerCorrectsSAM(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	b := newAccounts(t, e, e.Access)
	ctx := context.Background()
	b.guard()
	u := b.create()
	b.setMinor(u, true)
	b.setConsent(u, accounts.ConsentWithdrawn)
	b.pass()
	root := e.Access.Tenant().RootSpaceID
	if _, err := e.Access.Service().Assert(ctx, samUser(t, e, u.ParticipantID).ID,
		client.AssertionInput{Name: "consent", Value: "given", ScopeSpaceID: &root}); err != nil {
		t.Fatal(err)
	}
	checks := b.m.Checks()
	if len(checks) != 2 || checks[0].Name != "users" || checks[1].Name != "admins" {
		t.Fatalf("checks %+v", checks)
	}
	if err := checks[0].Run(ctx); err != nil {
		t.Fatal(err)
	}
	var away bool
	if err := e.Pool.QueryRow(ctx, `SELECT takes_access_away FROM sam_outbox WHERE entity = $1 AND state = 'pending'`,
		accounts.Entity(u.ID)).Scan(&away); err != nil || !away {
		t.Fatalf("the correction: takes access away %v, %v", away, err)
	}
	b.pass()
	if got := held(t, e, u.ParticipantID); got["consent"] != "withdrawn" {
		t.Fatalf("after the reconciler: SAM holds %v", got)
	}
	if err := checks[0].Run(ctx); err != nil {
		t.Fatal(err)
	}
	if waiting, err := samaccess.Waiting(ctx, e.Pool, accounts.Entity(u.ID)); err != nil || waiting {
		t.Fatalf("a second pass queued again: %v %v", waiting, err)
	}
}
