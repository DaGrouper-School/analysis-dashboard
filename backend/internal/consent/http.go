package consent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
)

// Module is the guardian consent module.
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
}

// Config is what the module sends and limits with.
type Config struct {
	// Email sends links to guardians named by email; nil writes them to the
	// log.
	Email notify.Email
	// SMS sends links to guardians named by phone; nil refuses them.
	SMS notify.SMS
	// FrontEnd is the front end's origin, which links point to.
	FrontEnd string
	// ShowTokens returns a new link's token to the minor, in demo mode only;
	// otherwise it reaches only the guardian.
	ShowTokens bool
	// Limits limits asking, which sends to whoever the minor names; nil
	// limits nothing.
	Limits Limiter
}

// The hourly limits on asking: per minor and per guardian contact.
const (
	AsksPerUser    = 5
	AsksPerContact = 3
)

// New builds the module.
func New(q db.Querier, sessions *accounts.Sessions, now func() time.Time, cfg Config) *Module {
	if cfg.Email == nil {
		cfg.Email = notify.LogEmail{}
	}
	if cfg.SMS == nil {
		cfg.SMS = notify.NoSMS{}
	}
	return &Module{db: q, sessions: sessions, now: now, cfg: cfg}
}

// Routes serves the minor's side, signed in, and the guardian's, by link.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	mux.Handle("POST "+prefix+"/me/guardian-consent", m.sessions.RequireUser(httpx.Handler(m.ask)))
	mux.Handle("GET "+prefix+"/me/guardian-consent", m.sessions.RequireUser(httpx.Handler(m.mine)))
	mux.Handle("GET "+prefix+"/consent/{token}", httpx.Handler(m.get))
	mux.Handle("POST "+prefix+"/consent/{token}", httpx.Handler(m.decide))
}

const (
	msgNoLink      = "This consent link is not valid any more."
	msgTooManyAsks = "Too many consent requests. Please try again later."
	msgNoTexts     = "Text messages cannot be sent yet. Enter your guardian’s email address instead."
	msgChanged     = "The consent text has changed. Read the new one before you decide."
)

// View is the front end's GuardianConsent: the minor's consent as they
// see it.
type View struct {
	Status          accounts.Consent `json:"status"`
	Method          string           `json:"method"`
	GuardianName    string           `json:"guardianName"`
	GuardianContact string           `json:"guardianContact"`
	RequestedAt     time.Time        `json:"requestedAt"`
	DecidedAt       *time.Time       `json:"decidedAt"`
	SignedOn        *string          `json:"signedOn"`
	RecordedByName  *string          `json:"recordedByName"`
	// DevToken is a new link's token, in demo mode only.
	DevToken *string `json:"devToken"`
}

func (m *Module) view(ctx context.Context, q db.Querier, u accounts.User, l Link) (View, error) {
	v := View{Status: accounts.ConsentPending, Method: "link", GuardianName: l.GuardianName, GuardianContact: l.GuardianContact, RequestedAt: l.RequestedAt}
	if u.Consent != nil {
		v.Status = *u.Consent
	}
	r, found, err := LatestRecord(ctx, q, u.ID, &l.ID)
	if found {
		v.DecidedAt = &r.DecidedAt
	}
	return v, err
}

// link is where a guardian's link points.
func (m *Module) link(token string) string {
	return strings.TrimRight(m.cfg.FrontEnd, "/") + "/consent/" + token
}

func (m *Module) ask(w http.ResponseWriter, r *http.Request) error {
	var in GuardianInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(CheckGuardian(in)); err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	contact, byEmail := Contact(in.GuardianContact)
	if m.cfg.Limits != nil {
		if err := m.cfg.Limits.Limits(r.Context(), msgTooManyAsks, map[string]int{
			"consent-ask:user:" + u.ID.String(): AsksPerUser, "consent-ask:contact:" + contact: AsksPerContact}); err != nil {
			return err
		}
	}
	var out View
	err := pgx.BeginFunc(r.Context(), m.db, func(tx pgx.Tx) error {
		now := m.now()
		l, token, err := Ask(r.Context(), tx, now, u.ID, in)
		if err != nil {
			return err
		}
		// Sent before the transaction commits, so a link that cannot be
		// sent leaves the old one working.
		if err := m.send(r.Context(), u, l, token, byEmail); err != nil {
			return err
		}
		cur, err := accounts.Get(r.Context(), tx, u.ID)
		if err != nil {
			return err
		}
		if out, err = m.view(r.Context(), tx, cur, l); err != nil {
			return err
		}
		if m.cfg.ShowTokens {
			out.DevToken = &token
		}
		return nil
	})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// send sends the guardian their link. A phone with no SMS provider is
// refused; any other failure is logged, and asking again sends a new link.
func (m *Module) send(ctx context.Context, u accounts.User, l Link, token string, byEmail bool) error {
	name := firstName(u.FullName)
	if byEmail {
		text := fmt.Sprintf(`Hello %s,

%s would like to take part in a Global Classroom of the Da Grouper School programme. Anyone under 18 needs a parent or guardian’s consent first.

Read what taking part means, and give or refuse consent, here:

%s

You can change your answer later from the same link.
`, l.GuardianName, name, m.link(token))
		if err := m.cfg.Email.Send(ctx, notify.Mail{To: l.GuardianContact, Subject: name + " asks for your consent", Text: text}); err != nil {
			slog.ErrorContext(ctx, "sending a consent link failed", "link", l.ID, "err", err)
		}
		return nil
	}
	err := m.cfg.SMS.Send(ctx, l.GuardianContact, fmt.Sprintf("%s asks for your consent to take part in a Da Grouper School Global Classroom: %s", name, m.link(token)))
	if errors.Is(err, notify.ErrNoProvider) {
		return httpx.FieldError(httpx.CodeInvalidInput, "guardianContact", msgNoTexts)
	}
	if err != nil {
		slog.ErrorContext(ctx, "texting a consent link failed", "link", l.ID, "err", err)
	}
	return nil
}

func firstName(full string) string {
	if f := strings.Fields(full); len(f) > 0 {
		return f[0]
	}
	return "Your child"
}

func (m *Module) mine(w http.ResponseWriter, r *http.Request) error {
	u, _ := accounts.CurrentUser(r.Context())
	cur, err := accounts.Get(r.Context(), m.db, u.ID)
	if err != nil {
		return err
	}
	l, err := LiveLink(r.Context(), m.db, u.ID)
	if errors.Is(err, ErrNoLink) {
		httpx.WriteJSON(w, http.StatusOK, nil)
		return nil
	}
	if err != nil {
		return err
	}
	v, err := m.view(r.Context(), m.db, cur, l)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, v)
	return nil
}

// Request is the front end's ConsentRequest, with the current consent
// text in the guardian's language, filled in, and its version, which a
// decision names (the contract's addendum).
type Request struct {
	ContributorName    string                    `json:"contributorName"`
	GuardianName       string                    `json:"guardianName"`
	ClassroomNames     []string                  `json:"classroomNames"`
	Status             accounts.Consent          `json:"status"`
	DecidedAt          *time.Time                `json:"decidedAt"`
	CoordinateDecimals int                       `json:"coordinateDecimals"`
	Version            int                       `json:"version"`
	Text               classrooms.ConsentContent `json:"text"`
}

// request is what the guardian of l sees, in language.
func (m *Module) request(ctx context.Context, l Link, language string) (Request, error) {
	u, err := accounts.Get(ctx, m.db, l.UserID)
	if err != nil {
		return Request{}, err
	}
	listed, err := classrooms.List(ctx, m.db, u.ID, false)
	if err != nil {
		return Request{}, err
	}
	names, institutions := []string{}, []string{}
	for _, c := range listed {
		names = append(names, c.Name)
		if !slices.Contains(institutions, c.InstitutionName) {
			institutions = append(institutions, c.InstitutionName)
		}
	}
	decimals, err := Decimals(ctx, m.db)
	if err != nil {
		return Request{}, err
	}
	details := classrooms.ConsentDetails{Classroom: joinNames(names, "the Global Classroom they join"),
		Institution: joinNames(institutions, "Their school"), Decimals: decimals}
	version, text, err := classrooms.CurrentConsent(ctx, m.db, language, details)
	if err != nil {
		return Request{}, err
	}
	out := Request{ContributorName: firstName(u.FullName), GuardianName: l.GuardianName, ClassroomNames: names, Status: accounts.ConsentPending,
		CoordinateDecimals: decimals, Version: version, Text: text}
	if u.Consent != nil {
		out.Status = *u.Consent
	}
	r, found, err := LatestRecord(ctx, m.db, u.ID, &l.ID)
	if found {
		out.DecidedAt = &r.DecidedAt
	}
	return out, err
}

// joinNames is names as a phrase, "A, B and C", or none when there are
// none.
func joinNames(names []string, none string) string {
	switch len(names) {
	case 0:
		return none
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func (m *Module) get(w http.ResponseWriter, r *http.Request) error {
	l, err := ByToken(r.Context(), m.db, r.PathValue("token"))
	if errors.Is(err, ErrNoLink) {
		return httpx.NotFound(msgNoLink)
	}
	if err != nil {
		return err
	}
	out, err := m.request(r.Context(), l, r.URL.Query().Get("language"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) decide(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Give    *bool `json:"give"`
		Version *int  `json:"version"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Give == nil || in.Version == nil {
		return httpx.New(httpx.CodeInvalidInput, "Give or withdraw consent, naming the consent text's version.")
	}
	current, err := classrooms.CurrentConsentVersion(r.Context(), m.db)
	if err != nil {
		return err
	}
	decimals, err := Decimals(r.Context(), m.db)
	if err != nil {
		return err
	}
	token := r.PathValue("token")
	l, err := Decide(r.Context(), m.db, m.now(), token, *in.Give, *in.Version, current, decimals)
	switch {
	case errors.Is(err, ErrNoLink):
		return httpx.NotFound(msgNoLink)
	case errors.Is(err, ErrTextChanged):
		return httpx.New(httpx.CodeConsentTextChanged, msgChanged)
	case err != nil:
		return err
	}
	out, err := m.request(r.Context(), l, r.URL.Query().Get("language"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}
