package classrooms

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/media"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Module is the classrooms module: classrooms, kept here and created in
// SAM, and what platform admins keep of them: the stages, regions and
// consent text.
type Module struct {
	db       db.Querier
	sessions *accounts.Sessions
	now      func() time.Time
	// root is the tenant's root space, under which classrooms are created.
	root uuid.UUID
	cfg  Config
}

// Config is how invitations are sent.
type Config struct {
	// Email sends them; nil writes them to the log.
	Email notify.Email
	// InvitationTTL is how long one stays open.
	InvitationTTL time.Duration
	// FrontEnd is the front end's origin, which links point to.
	FrontEnd string
	// ShowTokens returns invitations' tokens to the client, in demo mode
	// only; otherwise they reach only the invitee, by email.
	ShowTokens bool
	// SAM is how the reconciler reads SAM, and where the public's
	// principal comes from; nil leaves the public grants alone.
	SAM *samaccess.Access
	// Media keeps institution photos, and MediaURL is the address its
	// files are served under: this service's API, such as
	// https://dgs.example/api.
	Media    *media.Media
	MediaURL string
}

// New builds the module. Classrooms are created in SAM under root.
func New(q db.Querier, sessions *accounts.Sessions, now func() time.Time, root uuid.UUID, cfg Config) *Module {
	if cfg.Email == nil {
		cfg.Email = notify.LogEmail{}
	}
	if cfg.InvitationTTL <= 0 {
		cfg.InvitationTTL = DefaultInvitationTTL
	}
	return &Module{db: q, sessions: sessions, now: now, root: root, cfg: cfg}
}

// DefaultInvitationTTL is how long an invitation stays open by default.
const DefaultInvitationTTL = 14 * 24 * time.Hour

// Routes serves the classroom operations of the front end's contract, and
// the admin operations on classrooms, stages, regions and the consent
// text, which are in the contract's addendum.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	admin := func(h httpx.Handler) http.Handler {
		return m.sessions.RequireAdmin("Only platform admins can publish the consent text.", h)
	}
	config := func(h httpx.Handler) http.Handler {
		return m.sessions.RequireAdmin("Only platform admins can change the platform's configuration.", h)
	}
	mux.Handle("GET "+prefix+"/classrooms", m.sessions.RequireUser(httpx.Handler(m.listMine)))
	mux.Handle("POST "+prefix+"/classrooms", m.sessions.RequireUser(httpx.Handler(m.create)))
	mux.Handle("GET "+prefix+"/classrooms/{classroomId}", m.sessions.RequireUser(httpx.Handler(m.get)))
	mux.Handle("PATCH "+prefix+"/classrooms/{classroomId}", m.sessions.RequireUser(httpx.Handler(m.update)))
	mux.Handle("PUT "+prefix+"/classrooms/{classroomId}/sharing", m.sessions.RequireUser(httpx.Handler(m.updateSharing)))
	mux.Handle("PUT "+prefix+"/classrooms/{classroomId}/institution/photo", m.sessions.RequireUser(httpx.Handler(m.setPhoto)))
	mux.Handle("PATCH "+prefix+"/admin/classrooms/{classroomId}", config(m.configure))
	user := func(h httpx.Handler) http.Handler { return m.sessions.RequireUser(h) }
	mux.Handle("POST "+prefix+"/classrooms/{classroomId}/invitations", user(m.invite))
	mux.Handle("DELETE "+prefix+"/invitations/{invitationId}", user(m.revokeInvitation))
	mux.Handle("PATCH "+prefix+"/classrooms/{classroomId}/members/{userId}", user(m.setMemberRole))
	mux.Handle("DELETE "+prefix+"/classrooms/{classroomId}/members/{userId}", user(m.removeMember))
	mux.Handle("GET "+prefix+"/invitations/by-token/{token}", httpx.Handler(m.getInvitation))
	mux.Handle("GET "+prefix+"/me/invitations", user(m.listMyInvitations))
	mux.Handle("POST "+prefix+"/invitations/by-token/{token}/accept", user(m.acceptInvitation))
	mux.Handle("GET "+prefix+"/admin/stages", config(m.listStages))
	mux.Handle("PUT "+prefix+"/admin/stages/{stageId}", config(m.putList(PutStage, "stageId", m.listStages)))
	mux.Handle("DELETE "+prefix+"/admin/stages/{stageId}", config(m.deleteFrom(DeleteStage, "stageId", "That stage no longer exists.",
		"A classroom is at this stage. Move it to another stage first.")))
	mux.Handle("GET "+prefix+"/admin/regions", config(m.listRegions))
	mux.Handle("PUT "+prefix+"/admin/regions/{regionId}", config(m.putList(PutRegion, "regionId", m.listRegions)))
	mux.Handle("DELETE "+prefix+"/admin/regions/{regionId}", config(m.deleteFrom(DeleteRegion, "regionId", "That region no longer exists.",
		"A classroom is in this region. Move it to another region first.")))
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
	version, err := PublishConsent(r.Context(), m.db, m.now(), actor(r), raw)
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

func (m *Module) listStages(w http.ResponseWriter, r *http.Request) error {
	list, err := Stages(r.Context(), m.db)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, nonNil(list))
	return nil
}

func (m *Module) listRegions(w http.ResponseWriter, r *http.Request) error {
	list, err := Regions(r.Context(), m.db)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, nonNil(list))
	return nil
}

func nonNil[T any](l []T) []T {
	if l == nil {
		return []T{}
	}
	return l
}

// actor is the signed-in admin, as the change log names them.
func actor(r *http.Request) settings.Actor {
	u, _ := accounts.CurrentUser(r.Context())
	return settings.Actor{UserID: &u.ID, Name: u.FullName}
}

// putList adds or changes the row named by the path's param with put, as
// the admin, and answers with the list.
func (m *Module) putList(put func(context.Context, db.Querier, time.Time, settings.Actor, string, []byte) (bool, error),
	param string, list httpx.Handler) httpx.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBody))
		if err != nil {
			return httpx.New(httpx.CodeInvalidInput, "The request could not be read.")
		}
		_, err = put(r.Context(), m.db, m.now(), actor(r), r.PathValue(param), raw)
		if invalid := (*jsoncheck.Invalid)(nil); errors.As(err, &invalid) {
			return httpx.Invalid(invalid.Fields())
		}
		if err != nil {
			return err
		}
		return list(w, r)
	}
}

// deleteFrom deletes the row named by the path's param with del, as the
// admin.
func (m *Module) deleteFrom(del func(context.Context, db.Querier, time.Time, settings.Actor, string) error,
	param, gone, inUse string) httpx.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		err := del(r.Context(), m.db, m.now(), actor(r), r.PathValue(param))
		switch {
		case errors.Is(err, ErrNotFound):
			return httpx.NotFound(gone)
		case errors.Is(err, ErrInUse):
			return httpx.New(httpx.CodeConflict, inUse)
		case err != nil:
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}
