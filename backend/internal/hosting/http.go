package hosting

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Module is the hosting module: applications to host a Global Classroom
// and the readiness questions they answer.
type Module struct {
	db       db.Querier
	sessions *accounts.Sessions
	now      func() time.Time
	cfg      Config
}

// Limiter takes one use of each of several hourly limits; the accounts
// module is one.
type Limiter interface {
	Limits(ctx context.Context, msg string, keys map[string]int) error
	ClientAddress(r *http.Request) string
}

// Config is what the module sends and limits with.
type Config struct {
	// Email sends status links; nil writes them to the log.
	Email notify.Email
	// FrontEnd is the front end's origin, which links point to.
	FrontEnd string
	// ShowTokens returns status links' tokens to the client, in demo mode
	// only; otherwise they reach only the applicant, by email.
	ShowTokens bool
	// Limits limits applying, which needs no sign-in; nil limits nothing.
	Limits Limiter
}

// The hourly limits on applying: per client address, per contact email
// and overall.
const (
	AppliesPerAddress = 5
	AppliesPerEmail   = 3
	AppliesOverall    = 100
)

// New builds the module.
func New(q db.Querier, sessions *accounts.Sessions, now func() time.Time, cfg Config) *Module {
	if cfg.Email == nil {
		cfg.Email = notify.LogEmail{}
	}
	return &Module{db: q, sessions: sessions, now: now, cfg: cfg}
}

// Routes serves hosting: applying and the status link, which need no
// sign-in, deciding, for platform admins, and the questions' admin
// operations, which are in the contract's addendum.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	admin := func(h httpx.Handler) http.Handler {
		return m.sessions.RequireAdmin("Only platform admins can change the platform's configuration.", h)
	}
	reviewer := func(h httpx.Handler) http.Handler {
		return m.sessions.RequireAdmin("Only platform admins review applications.", h)
	}
	mux.Handle("GET "+prefix+"/host/questions", httpx.Handler(m.questions))
	mux.Handle("POST "+prefix+"/host/applications", httpx.Handler(m.apply))
	mux.Handle("GET "+prefix+"/host/status/{token}", httpx.Handler(m.status))
	mux.Handle("GET "+prefix+"/admin/host-applications", reviewer(m.applications))
	mux.Handle("POST "+prefix+"/admin/host-applications/{applicationId}/decision", reviewer(m.decide))
	mux.Handle("GET "+prefix+"/admin/host-questions", admin(m.list))
	mux.Handle("PUT "+prefix+"/admin/host-questions/{questionId}", admin(m.put))
	mux.Handle("POST "+prefix+"/admin/host-questions/{questionId}/retire", admin(m.retire))
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	list, err := AllQuestions(r.Context(), m.db)
	if err != nil {
		return err
	}
	if list == nil {
		list = []KeptQuestion{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

func actor(r *http.Request) settings.Actor {
	u, _ := accounts.CurrentUser(r.Context())
	return settings.Actor{UserID: &u.ID, Name: u.FullName}
}

func (m *Module) put(w http.ResponseWriter, r *http.Request) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBody))
	if err != nil {
		return httpx.New(httpx.CodeInvalidInput, "The request could not be read.")
	}
	_, err = PutQuestion(r.Context(), m.db, m.now(), actor(r), r.PathValue("questionId"), raw)
	var invalid *jsoncheck.Invalid
	switch {
	case errors.As(err, &invalid):
		return httpx.Invalid(invalid.Fields())
	case errors.Is(err, ErrRetired):
		return httpx.New(httpx.CodeConflict, "A retired question no longer changes. Add a new one instead.")
	case err != nil:
		return err
	}
	return m.list(w, r)
}

func (m *Module) retire(w http.ResponseWriter, r *http.Request) error {
	err := RetireQuestion(r.Context(), m.db, m.now(), actor(r), r.PathValue("questionId"))
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("That question no longer exists.")
	}
	if err != nil {
		return err
	}
	return m.list(w, r)
}
