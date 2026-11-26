// Package apitest drives this service over HTTP, as the front end does,
// against a database of its own, a SAM built from source with this service
// onboarded, and a fake Google. Every response is checked against the
// contract and its addendum, and a breach fails the test. Test code only.
package apitest

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/contract"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/places"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/server"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testenv"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// Outside are the paths under /api that lie outside the contract: the key
// set SAM fetches and the media files pages link to.
var Outside = []string{"/api/.well-known/", "/api/media/"}

// Gaps are what this service answers that the contract does not declare
// yet, each with why. They belong in the front end's routes, which the
// contract is generated from; until they are there, the contract check lets
// them through, provided the body is an Error.
var Gaps = []contract.Gap{
	{Operation: "auth.getCurrentUser", Status: http.StatusUnauthorized,
		Why: "the front end reads 401 as signed out, but the route lists no errors"},
	{Operation: "auth.signInWithGoogle", Status: http.StatusConflict,
		Why: "a Google sign-in whose email belongs to another account"},
	{Operation: "classrooms.create", Status: http.StatusNotFound,
		Why: "an applicationId naming no host application, which the mock refuses with not_found"},
	{Operation: "profile.requestGuardianConsent", Status: http.StatusTooManyRequests,
		Why: "asking past the hourly limits, since each request sends a link to whoever is named"},
	{Operation: "labs.uploadReadings", Status: http.StatusConflict,
		Why: "a device SAM does not hold yet, still being set up"},
	{Operation: "labs.submitObservation", Status: http.StatusConflict,
		Why: "a lab or device SAM does not hold yet, still being set up"},
	{Operation: "exports.rawValues", Status: http.StatusNotFound,
		Why: "a classroom the user is not a member of, which the mock refuses with not_found"},
}

// Audience is the audience of the user tokens this service signs for SAM.
const Audience = "sam"

// FrontEnd is the origin the service is told the front end runs on.
const FrontEnd = "https://app.test"

// Options are how a test's service is set up.
type Options struct {
	// Keys sign user tokens; nil for a key generated for the test.
	Keys *samaccess.Keys
	// Demo turns demo mode on.
	Demo bool
	// SendsPerHour is the overall SMS limit; zero for the default.
	SendsPerHour int
	// Places is how places are looked up; zero for the offline list.
	Places places.Config
}

// ClientHeader is the header the service reads a test client's address
// from, as from a trusted proxy.
const ClientHeader = "X-Test-Client"

// Env is this service running for one test.
type Env struct {
	// URL is where this service answers.
	URL  string
	Pool *pgxpool.Pool
	SAM  *testenv.SAM
	// Google is the fake Google sign-in trusts.
	Google *FakeGoogle
	// Clock is the service's clock, which a test moves.
	Clock *Clock
	// SMS keeps the texts the service sends, and Email its emails.
	SMS   *notify.SMSRecorder
	Email *notify.EmailRecorder
	// Access is how this service calls SAM, and App the service, as last
	// started.
	Access *samaccess.Access
	App    *server.App

	t       testing.TB
	opts    Options
	sam     samaccess.Config
	handler atomic.Pointer[http.Handler]
	check   func(http.Handler) http.Handler
	http    *http.Client
}

// New starts this service and SAM until t ends, signing user tokens with a
// key generated for the test. SAM is started first, knowing the address
// this service will listen on, because onboarding names the service's key
// set.
func New(t testing.TB) *Env {
	t.Helper()
	return NewWith(t, Options{})
}

// NewWithKeys is New, signing with keys.
func NewWithKeys(t testing.TB, keys *samaccess.Keys) *Env {
	t.Helper()
	return NewWith(t, Options{Keys: keys})
}

// NewWith is New, set up by o.
func NewWith(t testing.TB, o Options) *Env {
	t.Helper()
	c, err := contract.Find()
	if err != nil {
		t.Fatal(err)
	}
	c = c.WithGaps(Gaps...)
	if o.Keys == nil {
		if o.Keys, err = samaccess.GenerateKeys(time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewUnstartedServer(nil)
	url := "http://" + srv.Listener.Addr().String()
	sam := testenv.Options{Issuer: url, KeySetURL: url + "/api" + samaccess.KeySetPath, Audience: Audience}
	e := &Env{URL: url, Pool: testutil.DB(t), SAM: testenv.Start(t, sam), Google: newFakeGoogle(t), Clock: &Clock{now: time.Now()},
		SMS: &notify.SMSRecorder{}, Email: &notify.EmailRecorder{},
		t: t, opts: o,
		check: func(h http.Handler) http.Handler {
			return c.Checked(h, func(err error) { t.Errorf("contract: %v", err) }, Outside...)
		}}
	e.sam = e.SAM.Config(sam)
	e.Restart(o.Keys)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { (*e.handler.Load()).ServeHTTP(w, r) })
	srv.Start()
	t.Cleanup(srv.Close)
	e.http = e.NewClient()
	return e
}

// Restart builds this service again with keys, as a restart after a change
// to its key file would, and serves it at the same address.
func (e *Env) Restart(keys *samaccess.Keys) {
	e.t.Helper()
	access, err := samaccess.New(e.sam, keys, e.Clock.Now)
	if err != nil {
		e.t.Fatal(err)
	}
	e.Access = access
	e.App = server.New(server.Deps{DB: e.Pool, SAM: access, AllowedOrigins: []string{FrontEnd}, Now: e.Clock.Now,
		Accounts: accounts.Config{GoogleClientID: e.Google.ClientID, GoogleKeysURL: e.Google.KeysURL, Demo: e.opts.Demo,
			SMS: e.SMS, SMSSendsPerHour: e.opts.SendsPerHour, ClientIPHeader: ClientHeader},
		Places: e.opts.Places, Email: e.Email})
	h := e.check(e.App.Handler())
	e.handler.Store(&h)
}

// NewClient is an HTTP client with a cookie jar of its own: one browser.
// Env.Do uses the first.
func (e *Env) NewClient() *http.Client {
	e.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return &http.Client{Jar: jar, Timeout: 30 * time.Second}
}

// Do calls this service and returns the response with its body read.
func (e *Env) Do(t testing.TB, method, path, body string) (*http.Response, string) {
	t.Helper()
	return e.DoWith(t, e.http, method, path, body)
}

// DoWith is Do through client.
func (e *Env) DoWith(t testing.TB, client *http.Client, method, path, body string) (*http.Response, string) {
	t.Helper()
	return e.DoFrom(t, client, "", method, path, body)
}

// DoFrom is DoWith from the client address addr, as a trusted proxy would
// pass it; "" for the connection's.
func (e *Env) DoFrom(t testing.TB, client *http.Client, addr, method, path, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, e.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if addr != "" {
		req.Header.Set(ClientHeader, addr)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, string(b)
}

// RunBackground runs the service's background work, such as the restore
// guard, until t ends.
func (e *Env) RunBackground() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, task := range e.App.Background() {
		wg.Go(func() { task(ctx) })
	}
	e.t.Cleanup(func() { cancel(); wg.Wait() })
}

// Worker is an outbox worker for the service's own kinds and accounts.
func (e *Env) Worker() *samaccess.Worker {
	e.t.Helper()
	w, err := samaccess.NewWorker(e.Pool, e.Access, e.App.OutboxKinds(), e.App.Actors(), e.Clock.Now)
	if err != nil {
		e.t.Fatal(err)
	}
	return w
}

// Clock is a time a test moves by hand.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// Now is the clock's time.
func (c *Clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }

// Add moves the clock on by d.
func (c *Clock) Add(d time.Duration) { c.mu.Lock(); defer c.mu.Unlock(); c.now = c.now.Add(d) }

// Unserved lists the operations in the contract app does not serve.
func Unserved(c *contract.Contract, app *server.App) []*contract.Operation {
	var out []*contract.Operation
	for _, op := range c.Operations() {
		if !app.Serves(op.Method, op.Example()) {
			out = append(out, op)
		}
	}
	return out
}
