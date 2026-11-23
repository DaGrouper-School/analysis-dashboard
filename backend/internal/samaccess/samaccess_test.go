package samaccess

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

func entry(t *testing.T, signs bool) KeyEntry {
	t.Helper()
	k, err := NewKey(time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	k.Signs = signs
	return k
}

func keyFileOf(t *testing.T, entries ...KeyEntry) []byte {
	t.Helper()
	b, err := json.Marshal(keyFile{Keys: entries})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestKeyFile: exactly one key signs, every key is published, and a key
// file with a bad entry is refused.
func TestKeyFile(t *testing.T) {
	t.Parallel()
	a, b := entry(t, false), entry(t, true)
	if !strings.HasPrefix(a.ID, "20261123-") || a.ID == b.ID {
		t.Fatalf("new key IDs %q and %q", a.ID, b.ID)
	}
	k, err := ParseKeys(keyFileOf(t, a, b))
	if err != nil {
		t.Fatal(err)
	}
	set := k.KeySet()
	if k.SigningKeyID() != b.ID || len(set.Keys) != 2 || set.Keys[0].KeyID != a.ID {
		t.Fatalf("signing %s, published %+v", k.SigningKeyID(), set.Keys)
	}
	for _, j := range set.Keys {
		if j.KeyType != "OKP" || j.Curve != "Ed25519" || j.Algorithm != "EdDSA" || j.Use != "sig" {
			t.Errorf("published %+v", j)
		}
	}
	short := entry(t, true)
	short.Seed = base64.RawURLEncoding.EncodeToString([]byte("too short"))
	badID := entry(t, true)
	badID.ID = "a key"
	for name, raw := range map[string][]byte{
		"no keys":        keyFileOf(t),
		"none signs":     keyFileOf(t, a),
		"two sign":       keyFileOf(t, b, entry(t, true)),
		"a repeated kid": keyFileOf(t, b, KeyEntry{ID: b.ID, Seed: a.Seed}),
		"a short seed":   keyFileOf(t, short),
		"a bad kid":      keyFileOf(t, badID),
		"not JSON":       []byte("keys"),
	} {
		if _, err := ParseKeys(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func tenantFile(t *testing.T, tn Tenant) string {
	t.Helper()
	b, err := json.Marshal(tn)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "tenant.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func aTenant() Tenant {
	tn := Tenant{Name: "dgs", RootSpaceID: uuid.New(), ServiceID: uuid.New(), IssuerIDs: []uuid.UUID{uuid.New()},
		Namespace: "dgs", AnonymousID: uuid.New()}
	tn.Key = &struct {
		Secret string `json:"key"`
	}{Secret: "sam_x_y"}
	return tn
}

func TestLoadTenant(t *testing.T) {
	t.Parallel()
	if _, err := LoadTenant(tenantFile(t, aTenant())); err != nil {
		t.Fatal(err)
	}
	noKey, noIssuer, noRoot := aTenant(), aTenant(), aTenant()
	noKey.Key = nil
	noIssuer.IssuerIDs = nil
	noRoot.RootSpaceID = uuid.Nil
	for name, tn := range map[string]Tenant{"no key": noKey, "no issuer": noIssuer, "no root": noRoot} {
		if _, err := LoadTenant(tenantFile(t, tn)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := LoadTenant(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("a missing file: accepted")
	}
}

// TestTokens: a user token names its subject, issuer, audience and this
// service, lasts five minutes, and verifies against the published key set
// by its kid.
func TestTokens(t *testing.T) {
	t.Parallel()
	keys, err := ParseKeys(keyFileOf(t, entry(t, false), entry(t, true)))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	a, err := New(Config{URL: "http://sam.test", TenantFile: tenantFile(t, aTenant()), Issuer: "https://dgs.test", Audience: "sam"},
		keys, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	raw, err := a.Token("42")
	if err != nil {
		t.Fatal(err)
	}
	set := keys.KeySet()
	var c claims
	_, err = jwt.ParseWithClaims(raw, &c, func(tok *jwt.Token) (any, error) {
		for _, j := range set.Keys {
			if j.KeyID == tok.Header["kid"] {
				x, err := base64.RawURLEncoding.DecodeString(j.X)
				return ed25519.PublicKey(x), err
			}
		}
		return nil, fmt.Errorf("unknown kid %v", tok.Header["kid"])
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithTimeFunc(func() time.Time { return now.Add(time.Minute) }),
		jwt.WithIssuer("https://dgs.test"), jwt.WithAudience("sam"), jwt.WithSubject("42"))
	if err != nil {
		t.Fatal(err)
	}
	if c.AuthorizedParty != "dgs" || c.ExpiresAt.Sub(c.IssuedAt.Time) != TokenLifetime {
		t.Fatalf("claims %+v", c)
	}
	if _, err := a.User(Person{ParticipantID: 42}); !errors.Is(err, ErrUnregistered) {
		t.Fatalf("an unregistered person: %v", err)
	}
	if _, err := a.User(Person{ParticipantID: 42, Registered: true}); err != nil {
		t.Fatalf("a registered person: %v", err)
	}
}

func TestKeySetRoute(t *testing.T) {
	t.Parallel()
	keys, err := GenerateKeys(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Config{URL: "http://sam.test", TenantFile: tenantFile(t, aTenant()), Issuer: "i", Audience: "sam"}, keys, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	a.Routes(mux, "/api")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/.well-known/jwks.json", nil))
	var set KeySet
	if err := json.Unmarshal(w.Body.Bytes(), &set); err != nil || w.Code != 200 || len(set.Keys) != 1 ||
		set.Keys[0].KeyID != keys.SigningKeyID() || w.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("%d %s %v %v", w.Code, w.Body, w.Header(), err)
	}
}

// TestTranslate: SAM's refusals become the front end's refusals, and only
// SAM failing, or a call this service should never have made, is answered
// as internal.
func TestTranslate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sam := func(status int, code string) error {
		return &client.Error{Status: status, Code: code, Detail: "SAM's words"}
	}
	for _, tc := range []struct {
		err  error
		want string
	}{
		{sam(403, "forbidden"), httpx.CodeForbidden},
		{sam(401, "unauthenticated"), httpx.CodeForbidden},
		{sam(404, "not_found"), httpx.CodeNotFound},
		{sam(422, "invalid_request"), httpx.CodeInvalidInput},
		{sam(409, "already_exists"), httpx.CodeConflict},
		{sam(409, "version_mismatch"), httpx.CodeConflict},
		{sam(409, "site_outside_footprint"), httpx.CodeConflict},
		{sam(429, "rate_limited"), httpx.CodeRateLimited},
		{fmt.Errorf("listing: %w", sam(403, "forbidden")), httpx.CodeForbidden},
		{ErrUnregistered, httpx.CodeForbidden},
	} {
		got := Translate(ctx, tc.err)
		var e *httpx.Error
		if !errors.As(got, &e) || e.Code != tc.want || strings.Contains(e.Message, "SAM's words") {
			t.Errorf("%v became %v, want %s", tc.err, got, tc.want)
		}
	}
	limited := Translate(ctx, &client.Error{Status: 429, Code: "rate_limited", RetryAfter: 3 * time.Second})
	if e, ok := limited.(*httpx.Error); !ok || e.RetryAfter != 3*time.Second {
		t.Errorf("Retry-After was lost: %+v", limited)
	}
	for _, err := range []error{
		sam(500, "internal"), sam(503, "unavailable"), sam(422, "idempotency_key_reused"), sam(405, "method_not_allowed"),
		sam(413, "body_too_large"), context.DeadlineExceeded, errors.New("connection refused"),
	} {
		var e *httpx.Error
		if got := Translate(ctx, err); errors.As(got, &e) {
			t.Errorf("%v became %v, want a failure", err, got)
		}
	}
	if Translate(ctx, nil) != nil {
		t.Error("nil became an error")
	}
}
