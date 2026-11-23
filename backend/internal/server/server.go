// Package server assembles the service: it builds every module from one set
// of dependencies and mounts their routes behind the shared middleware. The
// command and the integration tests both build the service here, so tests
// run exactly what production runs.
package server

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/logx"
)

// Prefix is where the API is served: the front end calls /api on its own
// origin.
const Prefix = "/api"

// Deps is everything the service is built from.
type Deps struct {
	// AllowedOrigins are the front-end origins that may call with
	// credentials and send writes.
	AllowedOrigins []string
	// Now is the clock every module uses for expiries, limits and caches.
	Now func() time.Time
}

// Module is a group of endpoints. Each module registers its own routes
// under prefix; see docs/design/README.md, "Modules".
type Module interface {
	Routes(mux *http.ServeMux, prefix string)
}

// App is the assembled service.
type App struct {
	Deps
	modules []Module
}

// New builds the service. Each module is built here from d and added to
// the list its routes are mounted from.
func New(d Deps) *App {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &App{Deps: d}
}

// Handler is the HTTP API.
func (a *App) Handler() http.Handler {
	return requestIDs(logRequests(a.cors(a.checkOrigin(a.routes()))))
}

// routes is every endpoint, with a catch-all that answers not_found.
func (a *App) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	for _, m := range a.modules {
		m.Routes(mux, Prefix)
	}
	mux.HandleFunc(notFoundPattern, func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.NotFound("No such endpoint."))
	})
	return mux
}

// notFoundPattern catches every path under the API no endpoint serves.
const notFoundPattern = Prefix + "/"

// cors lets the configured front-end origins call with credentials.
func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && slices.Contains(a.AllowedOrigins, origin) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				h.Set("Access-Control-Allow-Headers", "Content-Type")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// checkOrigin refuses writes from pages on other origins. With the session
// cookie set SameSite=Lax and bodies required to be JSON, this stops
// cross-site request forgery without a token: browsers send Origin on every
// cross-origin write, and a page elsewhere cannot send JSON without a
// preflight this service refuses.
func (a *App) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			origin := r.Header.Get("Origin")
			if origin != "" && !slices.Contains(a.AllowedOrigins, origin) && !sameOrigin(r, origin) {
				httpx.WriteError(w, r, httpx.Forbidden("This request came from another site."))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request, origin string) bool {
	host, ok := strings.CutPrefix(origin, "https://")
	if !ok {
		host, _ = strings.CutPrefix(origin, "http://")
	}
	return host == r.Host
}

// requestIDs gives every request an ID, in its context for log lines and
// in the X-Request-ID response header.
func requestIDs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := logx.NewRequestID()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(logx.WithRequestID(r.Context(), id)))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(s int) { r.status = s; r.ResponseWriter.WriteHeader(s) }

// Unwrap lets http.ResponseController reach the connection, to flush a
// streamed response.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// logRequests writes one line per request, except health checks.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return
		}
		slog.InfoContext(r.Context(), "request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"ms", time.Since(start).Milliseconds())
	})
}
