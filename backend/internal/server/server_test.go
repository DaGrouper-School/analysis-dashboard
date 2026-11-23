package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

const front = "https://app.example"

// echo is a module with one read and one write.
type echo struct{}

func (echo) Routes(mux *http.ServeMux, prefix string) {
	ok := func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	}
	mux.HandleFunc("GET "+prefix+"/echo", ok)
	mux.HandleFunc("POST "+prefix+"/echo", ok)
}

func app() http.Handler {
	a := New(Deps{AllowedOrigins: []string{front}})
	a.modules = append(a.modules, echo{})
	return a.Handler()
}

func do(t *testing.T, h http.Handler, method, path string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader("{}"))
	r.Host = "api.example"
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func code(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("body %q: %v", w.Body, err)
	}
	return e.Code
}

// pinger is a database whose ping answers err; it is never queried.
type pinger struct {
	db.Querier
	err error
}

func (p pinger) Ping(context.Context) error { return p.err }

func TestHealthz(t *testing.T) {
	t.Parallel()
	w := do(t, app(), "GET", "/healthz", nil)
	if w.Code != http.StatusOK || w.Body.String() != "{\"status\":\"ok\"}\n" || w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("%d %q %v", w.Code, w.Body, w.Header())
	}
	up := New(Deps{DB: pinger{}}).Handler()
	if w := do(t, up, "GET", "/healthz", nil); w.Code != http.StatusOK {
		t.Fatalf("with the database up: %d %q", w.Code, w.Body)
	}
	down := New(Deps{DB: pinger{err: errors.New("connection refused")}}).Handler()
	if w := do(t, down, "GET", "/healthz", nil); w.Code != http.StatusServiceUnavailable || w.Body.String() != "{\"status\":\"unavailable\"}\n" {
		t.Fatalf("with the database down: %d %q", w.Code, w.Body)
	}
}

func TestModulesAreMounted(t *testing.T) {
	t.Parallel()
	if w := do(t, app(), "GET", Prefix+"/echo", nil); w.Code != http.StatusOK {
		t.Fatalf("module route: %d", w.Code)
	}
	w := do(t, app(), "GET", Prefix+"/nothing/here", nil)
	if w.Code != http.StatusNotFound || code(t, w) != httpx.CodeNotFound {
		t.Fatalf("unknown route: %d %s", w.Code, w.Body)
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()
	w := do(t, app(), "OPTIONS", Prefix+"/echo", map[string]string{"Origin": front, "Access-Control-Request-Method": "POST"})
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != front ||
		w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("preflight from the front end: %d %v", w.Code, w.Header())
	}
	w = do(t, app(), "OPTIONS", Prefix+"/echo", map[string]string{"Origin": "https://evil.example", "Access-Control-Request-Method": "POST"})
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("preflight from another site allowed: %v", w.Header())
	}
}

func TestWritesFromOtherSitesAreRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, origin string
		want         int
	}{
		{"front end", front, http.StatusOK},
		{"same origin", "https://api.example", http.StatusOK},
		{"no origin", "", http.StatusOK},
		{"another site", "https://evil.example", http.StatusForbidden},
	}
	for _, c := range cases {
		h := map[string]string{}
		if c.origin != "" {
			h["Origin"] = c.origin
		}
		if w := do(t, app(), "POST", Prefix+"/echo", h); w.Code != c.want {
			t.Errorf("%s: %d, want %d", c.name, w.Code, c.want)
		}
	}
	if w := do(t, app(), "GET", Prefix+"/echo", map[string]string{"Origin": "https://evil.example"}); w.Code != http.StatusOK {
		t.Errorf("a read from another site: %d", w.Code)
	}
}

func TestServes(t *testing.T) {
	t.Parallel()
	a := New(Deps{})
	a.modules = append(a.modules, echo{})
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{"GET", "/api/echo", true},
		{"POST", "/api/echo", true},
		{"DELETE", "/api/echo", false},
		{"GET", "/api/other", false},
		{"GET", "/healthz", true},
	} {
		if got := a.Serves(tc.method, tc.path); got != tc.want {
			t.Errorf("%s %s: %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
