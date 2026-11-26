package hosting

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Module is the hosting module. So far it serves platform admins the host
// application's questions.
type Module struct {
	db       db.Querier
	sessions *accounts.Sessions
	now      func() time.Time
}

// New builds the module.
func New(q db.Querier, sessions *accounts.Sessions, now func() time.Time) *Module {
	return &Module{db: q, sessions: sessions, now: now}
}

// Routes serves the questions' admin operations, which are in the
// contract's addendum.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	admin := func(h httpx.Handler) http.Handler {
		return m.sessions.RequireAdmin("Only platform admins can change the platform's configuration.", h)
	}
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
