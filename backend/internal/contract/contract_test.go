package contract_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/contract"
)

func load(t *testing.T) *contract.Contract {
	t.Helper()
	c, err := contract.Find()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestEveryOperationCompiles: the whole contract loads, each operation
// with an ID and at least one response.
func TestEveryOperationCompiles(t *testing.T) {
	t.Parallel()
	ops := load(t).Operations()
	if len(ops) == 0 {
		t.Fatal("no operations")
	}
	for _, op := range ops {
		if op.ID == "" {
			t.Errorf("%s %s has no operationId", op.Method, op.Path)
		}
	}
}

// TestMatch: a request is matched to its operation by method and path, a
// literal segment winning over a parameter.
func TestMatch(t *testing.T) {
	t.Parallel()
	c := load(t)
	for _, tc := range []struct{ method, path, want string }{
		{"GET", "/api/places/dial-codes", "places.dialCodes"},
		{"GET", "/api/places/ChIJ123", "places.getAddress"},
		{"GET", "/api/auth/me", "auth.getCurrentUser"},
		{"POST", "/api/auth/sign-out", "auth.signOut"},
	} {
		op := c.Match(tc.method, tc.path)
		if op == nil || op.ID != tc.want {
			t.Errorf("%s %s matched %v, want %s", tc.method, tc.path, op, tc.want)
		}
	}
	for _, tc := range []struct{ method, path string }{
		{"DELETE", "/api/auth/me"},
		{"GET", "/api/places"},
		{"GET", "/api/places/"},
		{"GET", "/places/dial-codes"},
		{"GET", "/apiplaces/dial-codes"},
		{"GET", "/healthz"},
	} {
		if op := c.Match(tc.method, tc.path); op != nil {
			t.Errorf("%s %s matched %s", tc.method, tc.path, op.ID)
		}
	}
	for _, op := range c.Operations() {
		if got := c.Match(op.Method, op.Example()); got == nil || (got != op && got.Path != op.Path) {
			t.Errorf("%s's example %s matched %v", op.ID, op.Example(), got)
		}
	}
}

var jsonHeader = http.Header{"Content-Type": {"application/json; charset=utf-8"}}

// TestCheck: a response passes only with a status the operation declares
// and a body its schema accepts.
func TestCheck(t *testing.T) {
	t.Parallel()
	c := load(t)
	dial := c.Match("GET", "/api/places/dial-codes")
	signOut := c.Match("POST", "/api/auth/sign-out")
	ok := []struct {
		name   string
		op     *contract.Operation
		status int
		header http.Header
		body   string
	}{
		{"a matching body", dial, 200, jsonHeader, `[{"code":"ET","name":"Ethiopia","dial":"+251"}]`},
		{"no content", signOut, 204, http.Header{}, ""},
		{"a declared error", signOut, 401, jsonHeader, `{"code":"unauthenticated","message":"Sign in first.","fields":{}}`},
		{"an unexpected failure", dial, 500, jsonHeader, `{"code":"internal","message":"Something went wrong.","fields":{}}`},
	}
	for _, tc := range ok {
		if err := tc.op.Check(tc.status, tc.header, []byte(tc.body)); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	bad := []struct {
		name   string
		op     *contract.Operation
		status int
		header http.Header
		body   string
	}{
		{"a missing field", dial, 200, jsonHeader, `[{"code":"ET","name":"Ethiopia"}]`},
		{"a wrong type", dial, 200, jsonHeader, `{"code":"ET"}`},
		{"not JSON", dial, 200, jsonHeader, `ET,Ethiopia`},
		{"another content type", dial, 200, http.Header{"Content-Type": {"text/plain"}}, `[]`},
		{"an undeclared status", dial, 404, jsonHeader, `{"code":"not_found","message":"No.","fields":{}}`},
		{"a body on no content", signOut, 204, jsonHeader, `{}`},
		{"a failure that is not an Error", dial, 500, jsonHeader, `{}`},
		{"a failure with another code", dial, 500, jsonHeader, `{"code":"not_found","message":"No.","fields":{}}`},
	}
	for _, tc := range bad {
		if err := tc.op.Check(tc.status, tc.header, []byte(tc.body)); err == nil {
			t.Errorf("%s passed", tc.name)
		}
	}
}

// TestChecked: the middleware reports a response the contract refuses and a
// path it lacks, and lets the rest through untouched.
func TestChecked(t *testing.T) {
	t.Parallel()
	c := load(t)
	body := `[{"code":"ET","name":"Ethiopia","dial":"+251"}]`
	mux := http.NewServeMux()
	answer := func(s string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(s))
		}
	}
	mux.HandleFunc("GET /api/places/dial-codes", answer(body))
	mux.HandleFunc("GET /api/places/autocomplete", answer(`{"wrong":true}`))
	mux.HandleFunc("/", answer(`{}`))
	var reports []error
	h := c.Checked(mux, func(err error) { reports = append(reports, err) }, "/api/.well-known/")
	for _, tc := range []struct {
		path    string
		reports int
	}{
		{"/api/places/dial-codes", 0},
		{"/api/places/autocomplete?q=addis", 1},
		{"/api/no-such-thing", 1},
		{"/api/.well-known/jwks.json", 0},
		{"/healthz", 0},
	} {
		reports = nil
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if len(reports) != tc.reports {
			t.Errorf("%s: %d reports, want %d: %v", tc.path, len(reports), tc.reports, reports)
		}
		if tc.path == "/api/places/dial-codes" && w.Body.String() != body {
			t.Errorf("the response was changed on its way through: %q", w.Body)
		}
	}
}

// TestGaps: a gap lets an undeclared status through, provided its body is
// an Error, for that operation only.
func TestGaps(t *testing.T) {
	t.Parallel()
	c := load(t).WithGaps(contract.Gap{Operation: "auth.getCurrentUser", Status: 401, Why: "test"})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_, _ = w.Write([]byte(r.Header.Get("X-Body")))
	})
	mux.HandleFunc("GET /api/places/dial-codes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"code":"unauthenticated","message":"No.","fields":{}}`))
	})
	var reports []error
	h := c.Checked(mux, func(err error) { reports = append(reports, err) })
	for _, tc := range []struct {
		path, body string
		reports    int
	}{
		{"/api/auth/me", `{"code":"unauthenticated","message":"Sign in.","fields":{}}`, 0},
		{"/api/auth/me", `{}`, 1},
		{"/api/places/dial-codes", "", 1},
	} {
		reports = nil
		r := httptest.NewRequest("GET", tc.path, nil)
		r.Header.Set("X-Body", tc.body)
		h.ServeHTTP(httptest.NewRecorder(), r)
		if len(reports) != tc.reports {
			t.Errorf("%s %s: %d reports, want %d: %v", tc.path, tc.body, len(reports), tc.reports, reports)
		}
	}
}
