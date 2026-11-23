// Package apitest drives this service over HTTP, as the front end does,
// against a database of its own and a SAM built from source with this
// service onboarded. Every response is checked against the contract, and a
// breach fails the test. Test code only.
package apitest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/contract"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/server"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testenv"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// Outside are the paths under /api that lie outside the contract: the key
// set SAM fetches and the media files pages link to.
var Outside = []string{"/api/.well-known/", "/api/media/"}

// Audience is the audience of the user tokens this service signs for SAM.
const Audience = "sam"

// FrontEnd is the origin the service is told the front end runs on.
const FrontEnd = "https://app.test"

// Env is this service running for one test.
type Env struct {
	// URL is where this service answers.
	URL  string
	Pool *pgxpool.Pool
	SAM  *testenv.SAM
	// Access is how this service calls SAM, and App the service, as last
	// started.
	Access *samaccess.Access
	App    *server.App
	http   *http.Client

	t       testing.TB
	sam     samaccess.Config
	handler atomic.Pointer[http.Handler]
	check   func(http.Handler) http.Handler
}

// New starts this service and SAM until t ends, signing user tokens with a
// key generated for the test. SAM is started first, knowing the address
// this service will listen on, because onboarding names the service's key
// set.
func New(t testing.TB) *Env {
	t.Helper()
	keys, err := samaccess.GenerateKeys(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return NewWithKeys(t, keys)
}

// NewWithKeys is New, signing with keys.
func NewWithKeys(t testing.TB, keys *samaccess.Keys) *Env {
	t.Helper()
	c, err := contract.Find()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(nil)
	url := "http://" + srv.Listener.Addr().String()
	opts := testenv.Options{Issuer: url, KeySetURL: url + "/api" + samaccess.KeySetPath, Audience: Audience}
	e := &Env{URL: url, Pool: testutil.DB(t), SAM: testenv.Start(t, opts), t: t,
		check: func(h http.Handler) http.Handler {
			return c.Checked(h, func(err error) { t.Errorf("contract: %v", err) }, Outside...)
		}}
	e.sam = e.SAM.Config(opts)
	e.Restart(keys)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { (*e.handler.Load()).ServeHTTP(w, r) })
	srv.Start()
	t.Cleanup(srv.Close)
	e.http = srv.Client()
	return e
}

// Restart builds this service again with keys, as a restart after a change
// to its key file would, and serves it at the same address.
func (e *Env) Restart(keys *samaccess.Keys) {
	e.t.Helper()
	access, err := samaccess.New(e.sam, keys, time.Now)
	if err != nil {
		e.t.Fatal(err)
	}
	e.Access = access
	e.App = server.New(server.Deps{DB: e.Pool, SAM: access, AllowedOrigins: []string{FrontEnd}, Now: time.Now})
	h := e.check(e.App.Handler())
	e.handler.Store(&h)
}

// Do calls this service and returns the response with its body read.
func (e *Env) Do(t testing.TB, method, path, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, e.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := e.http.Do(req)
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
