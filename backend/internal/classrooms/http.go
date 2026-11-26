package classrooms

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Module is the classrooms module. So far it serves the consent text's
// versions to platform admins.
type Module struct {
	db       db.Querier
	sessions *accounts.Sessions
	now      func() time.Time
}

// New builds the module.
func New(q db.Querier, sessions *accounts.Sessions, now func() time.Time) *Module {
	return &Module{db: q, sessions: sessions, now: now}
}

// Routes serves the consent text's admin operations, which are in the
// contract's addendum.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	admin := func(h httpx.Handler) http.Handler {
		return m.sessions.RequireUser(httpx.Handler(func(w http.ResponseWriter, r *http.Request) error {
			if u, _ := accounts.CurrentUser(r.Context()); !u.PlatformAdmin {
				return httpx.Forbidden("Only platform admins can publish the consent text.")
			}
			return h(w, r)
		}))
	}
	mux.Handle("GET "+prefix+"/admin/consent-texts", admin(m.listConsentTexts))
	mux.Handle("GET "+prefix+"/admin/consent-texts/{version}", admin(m.getConsentText))
	mux.Handle("POST "+prefix+"/admin/consent-texts", admin(m.publishConsentText))
}

func (m *Module) listConsentTexts(w http.ResponseWriter, r *http.Request) error {
	list, err := ConsentVersions(r.Context(), m.db)
	if err != nil {
		return err
	}
	if list == nil {
		list = []ConsentVersion{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

// ConsentTextView is one version with its words in every language.
type ConsentTextView struct {
	ConsentVersion
	Texts ConsentTexts `json:"texts"`
}

const msgNoVersion = "There is no such version of the consent text."

func (m *Module) getConsentText(w http.ResponseWriter, r *http.Request) error {
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version < 1 {
		return httpx.NotFound(msgNoVersion)
	}
	return m.writeVersion(w, r, version)
}

func (m *Module) writeVersion(w http.ResponseWriter, r *http.Request, version int) error {
	v, texts, err := ConsentText(r.Context(), m.db, version)
	if errors.Is(err, ErrNoConsentText) {
		return httpx.NotFound(msgNoVersion)
	}
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, ConsentTextView{ConsentVersion: v, Texts: texts})
	return nil
}

// publishConsentText publishes a new version, every language at once, as
// the admin.
func (m *Module) publishConsentText(w http.ResponseWriter, r *http.Request) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBody))
	if err != nil {
		return httpx.New(httpx.CodeInvalidInput, "The request could not be read.")
	}
	u, _ := accounts.CurrentUser(r.Context())
	version, err := PublishConsent(r.Context(), m.db, m.now(), settings.Actor{UserID: &u.ID, Name: u.FullName}, raw)
	var invalid *jsoncheck.Invalid
	switch {
	case errors.As(err, &invalid):
		return httpx.Invalid(invalid.Fields())
	case errors.Is(err, ErrConsentRace):
		return httpx.New(httpx.CodeConflict, "Another version was published at the same time. Reload it before publishing.")
	case err != nil:
		return err
	}
	return m.writeVersion(w, r, version)
}
