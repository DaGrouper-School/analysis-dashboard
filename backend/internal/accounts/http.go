package accounts

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// Routes serves sign-in: the auth operations of the front end's contract.
// Each handler decodes the request, calls one service function and encodes
// the answer.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	mux.Handle("GET "+prefix+"/auth/me", m.sessions.RequireUser(httpx.Handler(m.me)))
	mux.Handle("POST "+prefix+"/auth/google", httpx.Handler(m.signInGoogle))
	mux.Handle("POST "+prefix+"/auth/sign-out", m.sessions.RequireUser(httpx.Handler(m.signOut)))
	mux.Handle("GET "+prefix+"/auth/demo-accounts", httpx.Handler(m.listDemoAccounts))
}

// me is the signed-in user; without a session RequireUser answers 401,
// which the front end reads as signed out.
func (m *Module) me(w http.ResponseWriter, r *http.Request) error {
	u, _ := CurrentUser(r.Context())
	httpx.WriteJSON(w, http.StatusOK, viewOf(u))
	return nil
}

func (m *Module) signInGoogle(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		IDToken string `json:"idToken"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.IDToken == "" {
		return errSignInFailed
	}
	u, isNew, err := m.SignInWithGoogle(r.Context(), w, in.IDToken)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, authResult{User: viewOf(u), IsNewUser: isNew})
	return nil
}

func (m *Module) signOut(w http.ResponseWriter, r *http.Request) error {
	if err := m.sessions.end(w, r); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// listDemoAccounts is the demo picker's accounts in demo mode, and none
// otherwise.
func (m *Module) listDemoAccounts(w http.ResponseWriter, r *http.Request) error {
	out := []demoAccountView{}
	if m.cfg.Demo {
		accounts, err := demoAccounts(r.Context(), m.db)
		if err != nil {
			return err
		}
		for _, d := range accounts {
			out = append(out, demoAccountView{Sub: d.Subject, Name: d.Name, Email: d.Email, Type: d.Type})
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// userView is the contract's User.
type userView struct {
	ID                  string          `json:"id"`
	ParticipantID       int64           `json:"participantId"`
	PublicProfile       bool            `json:"publicProfile"`
	AuthProvider        string          `json:"authProvider"`
	FullName            string          `json:"fullName"`
	Email               *string         `json:"email"`
	Phone               *string         `json:"phone"`
	PhoneVerified       bool            `json:"phoneVerified"`
	AvatarURL           *string         `json:"avatarUrl"`
	Role                string          `json:"role"`
	PlatformAdmin       bool            `json:"platformAdmin"`
	Address             json.RawMessage `json:"address"`
	OnboardingCompleted bool            `json:"onboardingCompleted"`
	Minor               bool            `json:"minor"`
	Consent             Consent         `json:"consent"`
	CreatedAt           string          `json:"createdAt"`
	UpdatedAt           string          `json:"updatedAt"`
}

// viewOf is u as the front end sees them. Until onboarding says, a user is
// shown as no minor and their consent as not needed, as the front end
// starts every account; SAM treats them as a minor all the same until
// onboarding asserts otherwise.
func viewOf(u User) userView {
	v := userView{ID: u.ID.String(), ParticipantID: u.ParticipantID, PublicProfile: u.PublicProfile, AuthProvider: u.AuthProvider,
		FullName: u.FullName, Email: u.Email, Phone: u.Phone, PhoneVerified: u.PhoneVerified, AvatarURL: u.AvatarURL, Role: u.Role,
		PlatformAdmin: u.PlatformAdmin, Address: json.RawMessage("null"), OnboardingCompleted: u.OnboardingCompleted,
		Consent: ConsentNotNeeded, CreatedAt: timestamp(u.CreatedAt), UpdatedAt: timestamp(u.UpdatedAt)}
	if u.Address != nil {
		v.Address = u.Address
	}
	if u.Minor != nil {
		v.Minor = *u.Minor
	}
	if u.Consent != nil {
		v.Consent = *u.Consent
	}
	return v
}

// timestamp is t as the front end's ISO strings, in UTC to the millisecond.
func timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

type authResult struct {
	User      userView `json:"user"`
	IsNewUser bool     `json:"isNewUser"`
}

type demoAccountView struct {
	Sub   string `json:"sub"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"type"`
}
