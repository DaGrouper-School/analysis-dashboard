// Package samaccess is how this service calls SAM: as itself, as a person
// through a user token it signs, or as anonymous. It holds the tenant file
// and the signing keys, publishes the key set SAM verifies user tokens
// with, and translates SAM's answers into the front end's errors. See
// docs/design/sam-integration.md.
package samaccess

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// TokenLifetime is how long a user token lasts; SAM accepts at most ten
// minutes.
const TokenLifetime = 5 * time.Minute

// CallTimeout bounds every call to SAM.
const CallTimeout = 15 * time.Second

// KeySetPath is where the key set is served, under the API's prefix. It
// lies outside the front end's contract.
const KeySetPath = "/.well-known/jwks.json"

// keySetMaxAge is how long a fetched key set may be cached: SAM reuses one
// for five minutes, so a key is published at least that long before it
// signs.
const keySetMaxAge = 5 * time.Minute

// Config is how this service reaches SAM.
type Config struct {
	// URL is SAM's base URL, such as https://sam.example.
	URL string
	// TenantFile is the JSON `sam tenant create -json` printed when this
	// service was onboarded.
	TenantFile string
	// Issuer and Audience are the iss and aud of the user tokens this
	// service signs, as onboarding gave them.
	Issuer   string
	Audience string
	// HTTPClient makes the calls; nil for one with CallTimeout.
	HTTPClient *http.Client
}

// Tenant is this service as SAM's tenant: the tenant file.
type Tenant struct {
	// Name is this service's name in SAM, the azp of its user tokens.
	Name        string      `json:"name"`
	RootSpaceID uuid.UUID   `json:"root_space_id"`
	ServiceID   uuid.UUID   `json:"service_id"`
	IssuerIDs   []uuid.UUID `json:"issuer_ids"`
	Namespace   string      `json:"namespace"`
	AnonymousID uuid.UUID   `json:"anonymous_id"`
	Key         *struct {
		Secret string `json:"key"`
	} `json:"key"`
}

// IssuerID is the trusted issuer this service's users belong to.
func (t Tenant) IssuerID() uuid.UUID { return t.IssuerIDs[0] }

// LoadTenant reads and checks a tenant file.
func LoadTenant(path string) (Tenant, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Tenant{}, fmt.Errorf("the tenant file: %w", err)
	}
	var t Tenant
	if err := json.Unmarshal(raw, &t); err != nil {
		return Tenant{}, fmt.Errorf("the tenant file %s: %w", path, err)
	}
	switch {
	case t.Name == "" || t.RootSpaceID == uuid.Nil || t.ServiceID == uuid.Nil || t.AnonymousID == uuid.Nil:
		return Tenant{}, fmt.Errorf("the tenant file %s lacks the name, root space, service or anonymous principal", path)
	case len(t.IssuerIDs) == 0:
		return Tenant{}, fmt.Errorf("the tenant file %s names no trusted issuer", path)
	case t.Key == nil || t.Key.Secret == "":
		return Tenant{}, fmt.Errorf("the tenant file %s holds no service key: it is printed once, by the run that brought the tenant on", path)
	}
	return t, nil
}

// Access calls SAM.
type Access struct {
	tenant   Tenant
	keys     *Keys
	issuer   string
	audience string
	now      func() time.Time
	service  *client.Client
}

// New reads the tenant file and builds the clients. now is the clock
// tokens are dated by.
func New(cfg Config, keys *Keys, now func() time.Time) (*Access, error) {
	if keys == nil {
		return nil, errors.New("no signing keys")
	}
	t, err := LoadTenant(cfg.TenantFile)
	if err != nil {
		return nil, err
	}
	h := cfg.HTTPClient
	if h == nil {
		h = &http.Client{Timeout: CallTimeout}
	}
	return &Access{tenant: t, keys: keys, issuer: cfg.Issuer, audience: cfg.Audience, now: now,
		service: client.New(cfg.URL, client.WithKey(t.Key.Secret), client.WithHTTPClient(h), client.WithUserAgent("dgs"))}, nil
}

// Tenant is the tenant file.
func (a *Access) Tenant() Tenant { return a.tenant }

// Service calls SAM as this service, for changes made by policy and checks
// before a change.
func (a *Access) Service() *client.Client { return a.service }

// Anonymous calls SAM as its anonymous principal, for public pages.
func (a *Access) Anonymous() *client.Client { return a.service.AsAnonymous() }

// Person is someone this service calls SAM for.
type Person struct {
	// ParticipantID is their subject in SAM.
	ParticipantID int64
	// Registered is whether their SAM principal exists.
	Registered bool
}

// User calls SAM as p, through a user token signed now. A person whose
// principal is not registered yet gets ErrUnregistered, without a call to
// SAM.
func (a *Access) User(p Person) (*client.Client, error) {
	if !p.Registered {
		return nil, ErrUnregistered
	}
	tok, err := a.Token(strconv.FormatInt(p.ParticipantID, 10))
	if err != nil {
		return nil, err
	}
	return a.service.As(tok), nil
}

// claims are a user token's claims: the registered ones and azp, which
// names this service so SAM knows which of an issuer's services presents
// the token.
type claims struct {
	jwt.RegisteredClaims
	AuthorizedParty string `json:"azp"`
}

// Token signs a user token for subject with the signing key.
func (a *Access) Token(subject string) (string, error) {
	now := a.now()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{a.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime)),
		},
		AuthorizedParty: a.tenant.Name,
	})
	tok.Header["kid"] = a.keys.signing.id
	return tok.SignedString(a.keys.signing.private)
}

// Routes serves the key set. It is a module: server.New mounts it.
func (a *Access) Routes(mux *http.ServeMux, prefix string) {
	set := a.keys.KeySet()
	mux.HandleFunc("GET "+prefix+KeySetPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(keySetMaxAge.Seconds())))
		httpx.WriteJSON(w, http.StatusOK, set)
	})
}
