package apitest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

var amara = apitest.Person{Sub: "g-amara", Email: "amara@example.org", Name: "Amara Okafor"}

type authResult struct {
	User struct {
		ID            string `json:"id"`
		ParticipantID int64  `json:"participantId"`
		FullName      string `json:"fullName"`
		Email         string `json:"email"`
		AuthProvider  string `json:"authProvider"`
		Role          string `json:"role"`
		Consent       string `json:"consent"`
	} `json:"user"`
	IsNewUser bool `json:"isNewUser"`
}

func signIn(t *testing.T, e *apitest.Env, client *http.Client, token string) (*http.Response, authResult) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"idToken": token})
	res, raw := e.DoWith(t, client, "POST", "/api/auth/google", string(body))
	var out authResult
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatal(err)
		}
	}
	return res, out
}

func sessionCookie(res *http.Response) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == accounts.CookieName {
			return c
		}
	}
	return nil
}

// TestSigningInWithGoogle: a first sign-in creates the account and a
// session; the session cookie is HttpOnly and SameSite=Lax; a second
// sign-in finds the account; signing out ends the session.
func TestSigningInWithGoogle(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	browser := e.NewClient()
	if res, _ := e.DoWith(t, browser, "GET", "/api/auth/me", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", res.StatusCode)
	}
	res, first := signIn(t, e, browser, e.Google.Token(amara, e.Clock.Now()))
	if res.StatusCode != http.StatusOK || !first.IsNewUser || first.User.FullName != amara.Name || first.User.Email != amara.Email ||
		first.User.AuthProvider != "google" || first.User.Role != "community_scientist" || first.User.Consent != "not_needed" {
		t.Fatalf("first sign-in: %d %+v", res.StatusCode, first)
	}
	c := sessionCookie(res)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure || c.MaxAge < int(accounts.DefaultSessionTTL.Seconds())-5 {
		t.Fatalf("the session cookie: %+v", c)
	}
	var count int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE token_hash = sha256($1::bytea)`,
		[]byte(c.Value)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("the session is stored by its token's hash: %d, %v", count, err)
	}
	res, raw := e.DoWith(t, browser, "GET", "/api/auth/me", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("/auth/me: %d %s", res.StatusCode, raw)
	}

	other := e.NewClient()
	_, second := signIn(t, e, other, e.Google.Token(amara, e.Clock.Now()))
	if second.IsNewUser || second.User.ID != first.User.ID || second.User.ParticipantID != first.User.ParticipantID {
		t.Fatalf("second sign-in: %+v", second)
	}

	res, _ = e.DoWith(t, browser, "POST", "/api/auth/sign-out", "")
	if res.StatusCode != http.StatusNoContent || sessionCookie(res) == nil || sessionCookie(res).MaxAge >= 0 {
		t.Fatalf("signing out: %d %+v", res.StatusCode, sessionCookie(res))
	}
	if res, _ := e.DoWith(t, browser, "GET", "/api/auth/me", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("after signing out: %d", res.StatusCode)
	}
	if res, _ := e.DoWith(t, browser, "POST", "/api/auth/sign-out", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("signing out again: %d", res.StatusCode)
	}
	if res, _ := e.DoWith(t, other, "GET", "/api/auth/me", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("the other browser's session ended too: %d", res.StatusCode)
	}
}

// TestTokensThatDoNotVerify: a token Google did not sign, for another
// client, expired, from another issuer, or not a token, is refused with
// invalid_input, and starts no session.
func TestTokensThatDoNotVerify(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	now := e.Clock.Now()
	for name, token := range map[string]string{
		"another key":    e.Google.TokenFromAnotherKey(t, amara, now),
		"another client": e.Google.Token(amara, now, func(c jwt.MapClaims) { c["aud"] = "someone-else.apps.googleusercontent.com" }),
		"expired":        e.Google.Token(amara, now.Add(-2*time.Hour)),
		"another issuer": e.Google.Token(amara, now, func(c jwt.MapClaims) { c["iss"] = "https://id.example" }),
		"no subject":     e.Google.Token(amara, now, func(c jwt.MapClaims) { delete(c, "sub") }),
		"not a token":    "g-amara",
		"empty":          "",
	} {
		browser := e.NewClient()
		res, _ := signIn(t, e, browser, token)
		if res.StatusCode != http.StatusBadRequest || sessionCookie(res) != nil {
			t.Errorf("%s: %d", name, res.StatusCode)
		}
	}
	browser := e.NewClient()
	if res, _ := signIn(t, e, browser, e.Google.Token(amara, now.Add(-time.Hour+10*time.Second))); res.StatusCode != http.StatusOK {
		t.Fatalf("a token within the leeway of its expiry: %d", res.StatusCode)
	}
}

// TestAnEmailOfAnotherAccount: a new Google account whose email already
// belongs to an account is refused with conflict.
func TestAnEmailOfAnotherAccount(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	if res, _ := signIn(t, e, e.NewClient(), e.Google.Token(amara, e.Clock.Now())); res.StatusCode != http.StatusOK {
		t.Fatalf("the first account: %d", res.StatusCode)
	}
	again := apitest.Person{Sub: "g-amara-2", Email: "AMARA@example.org", Name: "Amara"}
	res, _ := signIn(t, e, e.NewClient(), e.Google.Token(again, e.Clock.Now()))
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("another Google account with the email: %d", res.StatusCode)
	}
}

// TestSessionsSlideAndEnd: a session used an hour or more after it was
// last extended is extended, and its cookie sent again; one unused for the
// session lifetime ends.
func TestSessionsSlideAndEnd(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	browser := e.NewClient()
	signIn(t, e, browser, e.Google.Token(amara, e.Clock.Now()))
	e.Clock.Add(30 * time.Minute)
	if res, _ := e.DoWith(t, browser, "GET", "/api/auth/me", ""); res.StatusCode != http.StatusOK || sessionCookie(res) != nil {
		t.Fatalf("within the hour: %d, cookie %v", res.StatusCode, sessionCookie(res))
	}
	e.Clock.Add(accounts.DefaultSessionTTL - time.Hour)
	res, _ := e.DoWith(t, browser, "GET", "/api/auth/me", "")
	if res.StatusCode != http.StatusOK || sessionCookie(res) == nil {
		t.Fatalf("near its end: %d, cookie %v", res.StatusCode, sessionCookie(res))
	}
	e.Clock.Add(accounts.DefaultSessionTTL - time.Minute)
	if res, _ := e.DoWith(t, browser, "GET", "/api/auth/me", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("extended, it lasts another lifetime: %d", res.StatusCode)
	}
	e.Clock.Add(accounts.DefaultSessionTTL + time.Minute)
	if res, _ := e.DoWith(t, browser, "GET", "/api/auth/me", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unused for its lifetime: %d", res.StatusCode)
	}
}

// TestDemoSignIn: with demo mode on, the demo accounts are offered and sign
// in by subject; with it off, none are, and a subject is no token.
func TestDemoSignIn(t *testing.T) {
	t.Parallel()
	demo := apitest.NewWith(t, apitest.Options{Demo: true})
	off := apitest.New(t)
	for _, e := range []*apitest.Env{demo, off} {
		if _, err := e.Pool.Exec(context.Background(), `INSERT INTO demo_accounts (subject, name, email, type)
			VALUES ('g-tesfaye', 'Tesfaye Bekele', 'tesfaye@example.org', 'Institution head')`); err != nil {
			t.Fatal(err)
		}
	}
	res, raw := demo.Do(t, "GET", "/api/auth/demo-accounts", "")
	var listed []map[string]string
	if err := json.Unmarshal([]byte(raw), &listed); err != nil || res.StatusCode != http.StatusOK || len(listed) != 1 ||
		listed[0]["sub"] != "g-tesfaye" || listed[0]["type"] != "Institution head" {
		t.Fatalf("demo accounts: %d %s", res.StatusCode, raw)
	}
	res, got := signIn(t, demo, demo.NewClient(), "g-tesfaye")
	if res.StatusCode != http.StatusOK || !got.IsNewUser || got.User.FullName != "Tesfaye Bekele" {
		t.Fatalf("a demo sign-in: %d %+v", res.StatusCode, got)
	}

	if _, raw := off.Do(t, "GET", "/api/auth/demo-accounts", ""); raw != "[]\n" {
		t.Fatalf("demo accounts without demo mode: %s", raw)
	}
	if res, _ := signIn(t, off, off.NewClient(), "g-tesfaye"); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("a demo sign-in without demo mode: %d", res.StatusCode)
	}
}

// TestSignInNeverWaitsOnSAM: with SAM down, signing in creates the account
// and a session at once; the account reaches SAM once it is back.
func TestSignInNeverWaitsOnSAM(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	e.SAM.Stop()
	res, got := signIn(t, e, e.NewClient(), e.Google.Token(amara, e.Clock.Now()))
	if res.StatusCode != http.StatusOK || !got.IsNewUser {
		t.Fatalf("signing in with SAM down: %d", res.StatusCode)
	}
	e.SAM.Start()
	e.RunBackground()
	w := e.Worker()
	deadline := time.Now().Add(time.Minute)
	for {
		if _, err := w.Pass(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Access.Service().FindUser(context.Background(), e.Access.Tenant().IssuerID(),
			strconv.FormatInt(got.User.ParticipantID, 10)); err == nil {
			return
		}
		if time.Now().After(deadline) {
			held, _ := samaccess.HeldChanges(context.Background(), e.Pool)
			t.Fatalf("the account did not reach SAM; held %+v", held)
		}
		e.Clock.Add(samaccess.MaxBackoff)
		time.Sleep(100 * time.Millisecond)
	}
}
