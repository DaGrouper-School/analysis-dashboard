package accounts

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Google's ID tokens (docs/design/accounts-and-sign-in.md, "Google sign-in").
const (
	// GoogleKeysURL is where Google publishes the keys it signs ID tokens
	// with.
	GoogleKeysURL = "https://www.googleapis.com/oauth2/v3/certs"
	// googleKeysMaxAge is how long fetched keys are used; googleRefetch is
	// how often at most an unknown key fetches them again.
	googleKeysMaxAge = time.Hour
	googleRefetch    = time.Minute
	// googleLeeway is the clock difference allowed on a token's times.
	googleLeeway = 30 * time.Second
	// googleFetchTimeout bounds fetching the keys.
	googleFetchTimeout = 10 * time.Second
)

// googleIssuers are the issuers Google's ID tokens carry.
var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

// GoogleClaims is what a verified ID token says about the person.
type GoogleClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// errGoogleToken is the answer for any token that does not verify. Why is
// logged, never shown.
var errGoogleToken = errors.New("the Google ID token does not verify")

// google verifies Google ID tokens for one client ID.
type google struct {
	clientID string
	keysURL  string
	http     *http.Client
	now      func() time.Time

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func newGoogle(clientID, keysURL string, now func() time.Time) *google {
	if keysURL == "" {
		keysURL = GoogleKeysURL
	}
	return &google{clientID: clientID, keysURL: keysURL, http: &http.Client{Timeout: googleFetchTimeout}, now: now}
}

// verify checks raw's signature against Google's keys, its issuer, its
// audience (this service's client ID) and its expiry, and returns its
// claims.
func (g *google) verify(ctx context.Context, raw string) (GoogleClaims, error) {
	if g.clientID == "" {
		return GoogleClaims{}, fmt.Errorf("%w: no Google client ID is configured", errGoogleToken)
	}
	var c GoogleClaims
	_, err := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return g.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithAudience(g.clientID), jwt.WithLeeway(googleLeeway),
		jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(g.now))
	if err != nil {
		return GoogleClaims{}, fmt.Errorf("%w: %w", errGoogleToken, err)
	}
	if !slices.Contains(googleIssuers, c.Issuer) {
		return GoogleClaims{}, fmt.Errorf("%w: issuer %q", errGoogleToken, c.Issuer)
	}
	if c.Subject == "" {
		return GoogleClaims{}, fmt.Errorf("%w: no subject", errGoogleToken)
	}
	return c, nil
}

// key is Google's key kid, from the cached set, fetched again when the set
// is old or, at most once a minute, when it lacks kid.
func (g *google) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	k, ok := g.keys[kid]
	stale := now.Sub(g.fetchedAt) > googleKeysMaxAge
	if ok && !stale {
		return k, nil
	}
	if stale || now.Sub(g.fetchedAt) > googleRefetch {
		keys, err := g.fetch(ctx)
		if err != nil {
			if ok {
				// Google's keys cannot be fetched: the cached one still holds.
				return k, nil
			}
			return nil, err
		}
		g.keys, g.fetchedAt = keys, now
		if k, ok := keys[kid]; ok {
			return k, nil
		}
	}
	return nil, fmt.Errorf("no Google key %q", kid)
}

// jwks is Google's published key set, as far as RSA signing keys go.
type jwks struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (g *google) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.keysURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Google's keys: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching Google's keys: %s", res.Status)
	}
	var set jwks
	if err := json.NewDecoder(res.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("decoding Google's keys: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	return keys, nil
}
