package apitest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// FakeGoogle signs ID tokens as Google does and publishes its key, for
// sign-in to verify.
type FakeGoogle struct {
	// ClientID is the audience of its tokens, which the service is
	// configured with.
	ClientID string
	// KeysURL is where it publishes its key.
	KeysURL string
	key     *rsa.PrivateKey
	kid     string
}

func newFakeGoogle(t testing.TB) *FakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	g := &FakeGoogle{ClientID: "dgs-test.apps.googleusercontent.com", key: key, kid: "fake-google-1"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
			"kid": g.kid, "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(srv.Close)
	g.KeysURL = srv.URL
	return g
}

// Person is who a fake ID token names.
type Person struct {
	Sub, Email, Name string
}

// Token is an ID token for p, issued at now and good for an hour; change
// alters its claims before it is signed.
func (g *FakeGoogle) Token(p Person, now time.Time, change ...func(jwt.MapClaims)) string {
	claims := jwt.MapClaims{"iss": "https://accounts.google.com", "aud": g.ClientID, "sub": p.Sub, "email": p.Email,
		"email_verified": true, "name": p.Name, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()}
	for _, c := range change {
		c(claims)
	}
	return g.sign(g.key, claims)
}

// TokenFromAnotherKey is a token for p signed by a key Google does not
// publish.
func (g *FakeGoogle) TokenFromAnotherKey(t testing.TB, p Person, now time.Time) string {
	t.Helper()
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{"iss": "https://accounts.google.com", "aud": g.ClientID, "sub": p.Sub, "email": p.Email,
		"email_verified": true, "name": p.Name, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()}
	return g.sign(other, claims)
}

func (g *FakeGoogle) sign(key *rsa.PrivateKey, claims jwt.MapClaims) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = g.kid
	s, err := tok.SignedString(key)
	if err != nil {
		panic(err)
	}
	return s
}
