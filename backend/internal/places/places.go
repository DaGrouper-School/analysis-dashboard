// Package places is the places module: address autocomplete and lookup
// through a provider whose key stays on the server, and the calling codes a
// phone number can have (docs/design/accounts-and-sign-in.md, "Profile and
// onboarding"). Development and test use an offline provider.
package places

import (
	"context"
	"errors"
	"net/http"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// Config is how places are looked up.
type Config struct {
	// APIKey is the Google Places key; without one the offline list is
	// used, which only development and test allow.
	APIKey string
	// BaseURL is the Places API's; empty for Google's own. Tests point it
	// at a fake.
	BaseURL string
	// PerUserPerHour is how many lookups a user makes an hour, autocomplete
	// and addresses together; zero for the default.
	PerUserPerHour int
}

// DefaultPerUserPerHour allows a person typing a few addresses, with the
// front end asking as they type, and stops a script.
const DefaultPerUserPerHour = 300

// Limiter takes one use of a per-user limit; the accounts module is one.
type Limiter interface {
	Limit(ctx context.Context, msg, key string, max int) error
}

// Module is the places module.
type Module struct {
	provider Provider
	sessions *accounts.Sessions
	limits   Limiter
	perHour  int
}

// New builds the module: Google Places with a key, the offline list
// without.
func New(cfg Config, sessions *accounts.Sessions, limits Limiter) *Module {
	var p Provider = Offline{}
	if cfg.APIKey != "" {
		p = NewGoogle(cfg.APIKey, cfg.BaseURL)
	}
	if cfg.PerUserPerHour <= 0 {
		cfg.PerUserPerHour = DefaultPerUserPerHour
	}
	return &Module{provider: p, sessions: sessions, limits: limits, perHour: cfg.PerUserPerHour}
}

const msgTooManyLookups = "Too many address lookups. Please wait a moment and try again."

// Routes serves the places operations of the front end's contract.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	mux.Handle("GET "+prefix+"/places/autocomplete", m.sessions.RequireUser(httpx.Handler(m.autocomplete)))
	mux.Handle("GET "+prefix+"/places/dial-codes", httpx.Handler(m.dialCodes))
	mux.Handle("GET "+prefix+"/places/{placeId}", m.sessions.RequireUser(httpx.Handler(m.address)))
}

// limit takes one of the signed-in user's lookups.
func (m *Module) limit(r *http.Request) error {
	u, _ := accounts.CurrentUser(r.Context())
	return m.limits.Limit(r.Context(), msgTooManyLookups, "places:user:"+u.ID.String(), m.perHour)
}

func (m *Module) autocomplete(w http.ResponseWriter, r *http.Request) error {
	if err := m.limit(r); err != nil {
		return err
	}
	out, err := m.provider.Autocomplete(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) address(w http.ResponseWriter, r *http.Request) error {
	if err := m.limit(r); err != nil {
		return err
	}
	a, err := m.provider.Address(r.Context(), r.PathValue("placeId"))
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("That place could not be found.")
	}
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, a)
	return nil
}

// DialCode is the front end's DialCode.
type DialCode struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Dial string `json:"dial"`
}

// dialCodeList is the calling codes, pilot countries first, as the front
// end's mock lists them.
var dialCodeList = []DialCode{
	{"ET", "Ethiopia", "+251"},
	{"BS", "Bahamas", "+1242"},
	{"SO", "Somalia", "+252"},
	{"JM", "Jamaica", "+1876"},
	{"US", "United States", "+1"},
	{"GB", "United Kingdom", "+44"},
	{"KE", "Kenya", "+254"},
}

func (m *Module) dialCodes(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	httpx.WriteJSON(w, http.StatusOK, dialCodeList)
	return nil
}
