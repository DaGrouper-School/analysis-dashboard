// Package server assembles the service: it builds every module from one set
// of dependencies and mounts their routes behind the shared middleware. The
// command and the integration tests both build the service here, so tests
// run exactly what production runs.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/hosting"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/logx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/places"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings/settingsapi"
)

// Prefix is where the API is served: the front end calls /api on its own
// origin.
const Prefix = "/api"

// Database is the database modules query and the health check pings.
type Database interface {
	db.Querier
	Ping(ctx context.Context) error
}

// Deps is everything the service is built from.
type Deps struct {
	// DB is the database. The health check reports the service unavailable
	// while it does not answer.
	DB Database
	// AllowedOrigins are the front-end origins that may call with
	// credentials and send writes.
	AllowedOrigins []string
	// Now is the clock every module uses for expiries, limits and caches.
	Now func() time.Time
	// SAM calls SAM, and serves the key set SAM verifies user tokens with.
	// Nil only in tests that need no SAM.
	SAM *samaccess.Access
	// Accounts is how people sign in.
	Accounts accounts.Config
	// Places is how places are looked up.
	Places places.Config
	// Email sends email; nil writes it to the log.
	Email notify.Email
	// InvitationTTL is how long an invitation to a classroom stays open;
	// zero for the default.
	InvitationTTL time.Duration
}

// Module is a group of endpoints. Each module registers its own routes
// under prefix; see docs/design/README.md, "Modules".
type Module interface {
	Routes(mux *http.ServeMux, prefix string)
}

// App is the assembled service.
type App struct {
	Deps
	modules  []Module
	accounts *accounts.Module
	registry *registry.Module
}

// New builds the service. Each module is built here from d and added to
// the list its routes are mounted from.
func New(d Deps) *App {
	if d.Now == nil {
		d.Now = time.Now
	}
	a := &App{Deps: d}
	if d.SAM != nil {
		a.modules = append(a.modules, d.SAM)
	}
	if d.SAM != nil && d.DB != nil {
		a.accounts = accounts.New(d.DB, d.SAM, d.Now, d.Accounts)
		a.registry = registry.New(d.DB, d.SAM.Service(), d.SAM.Tenant().RootSpaceID, d.Now, a.accounts.Sessions())
		a.modules = append(a.modules, a.accounts, places.New(d.Places, a.accounts.Sessions(), a.accounts), a.registry,
			classrooms.New(d.DB, a.accounts.Sessions(), d.Now, d.SAM.Tenant().RootSpaceID, classrooms.Config{
				Email: d.Email, InvitationTTL: d.InvitationTTL, FrontEnd: frontEnd(d.AllowedOrigins), ShowTokens: d.Accounts.Demo}), hosting.New(d.DB, a.accounts.Sessions(), d.Now),
			settingsapi.New(d.DB, a.accounts.Sessions(), d.Now), labs.New(d.DB, a.accounts.Sessions(), d.Now, a.registry.DeviceType))
	}
	return a
}

// frontEnd is the front end's origin, the first allowed.
func frontEnd(origins []string) string {
	if len(origins) == 0 {
		return ""
	}
	return origins[0]
}

// What modules declare to the background work, beside their routes.
type (
	// outboxSender is a module whose changes reach SAM through the outbox.
	outboxSender interface{ OutboxKinds() []samaccess.Kind }
	// reconciled is a module whose entities the reconciler compares.
	reconciled interface{ Checks() []samaccess.Check }
	// background is a module with work to run while the service serves.
	background interface {
		Background() []func(context.Context)
	}
)

// Background is every module's work to run while the service serves, each
// until its context ends.
func (a *App) Background() []func(context.Context) {
	var out []func(context.Context)
	for _, m := range a.modules {
		if b, ok := m.(background); ok {
			out = append(out, b.Background()...)
		}
	}
	return out
}

// OutboxKinds are every module's kinds of outbox change.
func (a *App) OutboxKinds() []samaccess.Kind {
	var out []samaccess.Kind
	for _, m := range a.modules {
		if s, ok := m.(outboxSender); ok {
			out = append(out, s.OutboxKinds()...)
		}
	}
	return out
}

// Checks are every module's reconciler checks.
func (a *App) Checks() []samaccess.Check {
	var out []samaccess.Check
	for _, m := range a.modules {
		if r, ok := m.(reconciled); ok {
			out = append(out, r.Checks()...)
		}
	}
	return out
}

// Actors is the module that finds the account an outbox change is sent as,
// or nil while there is none.
func (a *App) Actors() samaccess.Actors {
	for _, m := range a.modules {
		if ac, ok := m.(samaccess.Actors); ok {
			return ac
		}
	}
	return nil
}

// Accounts is the accounts module, whose Sessions the modules that guard
// signed-in routes are built with; nil without a database and SAM.
func (a *App) Accounts() *accounts.Module { return a.accounts }

// Registry is the registry module, nil without a database and SAM.
func (a *App) Registry() *registry.Module { return a.registry }

// Handler is the HTTP API.
func (a *App) Handler() http.Handler {
	return requestIDs(logRequests(a.cors(a.checkOrigin(a.routes()))))
}

// routes is every endpoint, with a catch-all that answers not_found.
func (a *App) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	for _, m := range a.modules {
		m.Routes(mux, Prefix)
	}
	mux.HandleFunc(notFoundPattern, func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.NotFound("No such endpoint."))
	})
	return mux
}

// pingTimeout bounds the health check's database ping, well inside the
// probe's own timeout.
const pingTimeout = 2 * time.Second

// healthz answers 200 while the database answers, and 503 otherwise, so a
// load balancer stops sending requests to an instance that cannot serve them.
func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	if a.DB != nil {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()
		if err := a.DB.Ping(ctx); err != nil {
			slog.WarnContext(ctx, "health check: the database did not answer", "err", err)
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Serves reports whether a module serves method on path, so the
// route-coverage report can list the operations the contract has and the
// service does not yet.
func (a *App) Serves(method, path string) bool {
	r, err := http.NewRequestWithContext(context.Background(), method, path, nil)
	if err != nil {
		return false
	}
	_, pattern := a.routes().Handler(r)
	return pattern != "" && pattern != notFoundPattern
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
