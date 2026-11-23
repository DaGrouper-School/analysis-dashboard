package accounts

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeKeys serves a key set the test changes, counting fetches.
type fakeKeys struct {
	mu      sync.Mutex
	keys    map[string]*rsa.PrivateKey
	fetches int
	down    bool
}

func (f *fakeKeys) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetches++
	if f.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var set []map[string]string
	for kid, k := range f.keys {
		set = append(set, map[string]string{"kid": kid, "kty": "RSA", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": set})
}

func (f *fakeKeys) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.fetches }

func rsaKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func idToken(t *testing.T, key *rsa.PrivateKey, kid string, now time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": "accounts.google.com", "aud": "client", "sub": "g-1",
		"email": "a@example.org", "email_verified": true, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestGoogleKeys: keys are fetched once and reused for an hour; an unknown
// key fetches them again at most once a minute; a cached key still holds
// while Google cannot be reached.
func TestGoogleKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a, b := rsaKey(t), rsaKey(t)
	keys := &fakeKeys{keys: map[string]*rsa.PrivateKey{"a": a}}
	srv := httptest.NewServer(keys)
	t.Cleanup(srv.Close)
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	g := newGoogle("client", srv.URL, func() time.Time { return now })

	verify := func(key *rsa.PrivateKey, kid string) error {
		_, err := g.verify(ctx, idToken(t, key, kid, now))
		return err
	}
	if err := verify(a, "a"); err != nil {
		t.Fatal(err)
	}
	if err := verify(a, "a"); err != nil || keys.count() != 1 {
		t.Fatalf("reused: %v, %d fetches", err, keys.count())
	}

	keys.mu.Lock()
	keys.keys["b"] = b
	keys.mu.Unlock()
	if err := verify(b, "b"); err == nil || keys.count() != 1 {
		t.Fatalf("an unknown key within a minute of the last fetch: %v, %d fetches", err, keys.count())
	}
	now = now.Add(61 * time.Second)
	if err := verify(b, "b"); err != nil || keys.count() != 2 {
		t.Fatalf("an unknown key after a minute: %v, %d fetches", err, keys.count())
	}

	keys.mu.Lock()
	keys.down = true
	keys.mu.Unlock()
	now = now.Add(2 * time.Hour)
	if err := verify(a, "a"); err != nil {
		t.Fatalf("a cached key while Google is down: %v", err)
	}
	if keys.count() != 3 {
		t.Fatalf("stale keys were not fetched again: %d fetches", keys.count())
	}
	if err := verify(rsaKey(t), "c"); err == nil {
		t.Fatal("an unknown key verified while Google is down")
	}
	if _, err := newGoogle("", srv.URL, func() time.Time { return now }).verify(ctx, idToken(t, a, "a", now)); err == nil {
		t.Fatal("a token verified with no client ID configured")
	}
}
