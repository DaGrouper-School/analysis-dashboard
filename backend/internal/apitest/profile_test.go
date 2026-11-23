package apitest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
)

// profile is a signed-in person filling in their profile.
type profile struct {
	t      *testing.T
	e      *apitest.Env
	client *http.Client
	id     string
	pid    int64
}

type userJSON struct {
	ID                  string          `json:"id"`
	FullName            string          `json:"fullName"`
	Email               *string         `json:"email"`
	Phone               *string         `json:"phone"`
	PhoneVerified       bool            `json:"phoneVerified"`
	Address             json.RawMessage `json:"address"`
	OnboardingCompleted bool            `json:"onboardingCompleted"`
	PublicProfile       bool            `json:"publicProfile"`
	Minor               bool            `json:"minor"`
	Consent             string          `json:"consent"`
}

type refusal struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

func signedIn(t *testing.T, e *apitest.Env, p apitest.Person) *profile {
	t.Helper()
	pr := &profile{t: t, e: e, client: e.NewClient()}
	res, got := signIn(t, e, pr.client, e.Google.Token(p, e.Clock.Now()))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("signing in %s: %d", p.Sub, res.StatusCode)
	}
	pr.id, pr.pid = got.User.ID, got.User.ParticipantID
	return pr
}

// call sends body to path and decodes the user, or the refusal.
func (p *profile) call(method, path string, body any) (int, userJSON, refusal) {
	p.t.Helper()
	raw := ""
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	res, out := p.e.DoFrom(p.t, p.client, "198.51.100.7", method, path, raw)
	var u userJSON
	var r refusal
	if res.StatusCode == http.StatusOK {
		_ = json.Unmarshal([]byte(out), &u)
	} else {
		_ = json.Unmarshal([]byte(out), &r)
	}
	return res.StatusCode, u, r
}

// verifyPhone verifies number for p, reading the code from the text.
func (p *profile) verifyPhone(number string) (int, userJSON, refusal) {
	p.t.Helper()
	b, _ := json.Marshal(map[string]string{"phone": number})
	res, raw := p.e.DoFrom(p.t, p.client, "198.51.100.7", "POST", "/api/me/phone/start", string(b))
	if res.StatusCode != http.StatusOK {
		var r refusal
		_ = json.Unmarshal([]byte(raw), &r)
		return res.StatusCode, userJSON{}, r
	}
	var c challenge
	_ = json.Unmarshal([]byte(raw), &c)
	msg, _ := p.e.SMS.Last(c.Phone)
	return p.call("POST", "/api/me/phone/verify", map[string]string{"requestId": c.RequestID, "code": sixDigits.FindString(msg.Text)})
}

var addisAddress = map[string]any{"street": "Bole Road 12", "city": "Addis Ababa", "state": "", "zip": "1000", "country": "Ethiopia"}

// samMinor is the minor SAM holds about participant n, after the outbox
// has sent what is waiting.
func samMinor(t *testing.T, e *apitest.Env, n int64) string {
	t.Helper()
	ctx := context.Background()
	if err := e.App.Accounts().Guard().Check(ctx); err != nil {
		t.Fatal(err)
	}
	w := e.Worker()
	for range 3 {
		if _, err := w.Pass(ctx); err != nil {
			t.Fatal(err)
		}
		e.Clock.Add(time.Second)
	}
	return held(t, e, n)["minor"]
}

// TestOnboarding: a person fills in their name and email, verifies their
// phone, gives an address, and completes onboarding, which tells SAM they
// are an adult.
func TestOnboarding(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	p := signedIn(t, e, amara)

	if status, _, r := p.call("POST", "/api/me/onboarding/complete", nil); status != http.StatusBadRequest ||
		r.Message != "Still needed: verified phone, address." {
		t.Fatalf("completing too soon: %d %+v", status, r)
	}
	if status, _, r := p.call("PATCH", "/api/me/profile", map[string]string{"fullName": "A", "email": "not-an-email"}); status != http.StatusBadRequest ||
		r.Fields["fullName"] == "" || r.Fields["email"] == "" {
		t.Fatalf("a bad profile: %d %+v", status, r)
	}
	status, u, _ := p.call("PATCH", "/api/me/profile", map[string]string{"fullName": " Amara N. Okafor ", "email": "Amara@Example.org"})
	if status != http.StatusOK || u.FullName != "Amara N. Okafor" || *u.Email != "amara@example.org" {
		t.Fatalf("the profile: %d %+v", status, u)
	}
	status, u, _ = p.verifyPhone("+251 91 123 4567")
	if status != http.StatusOK || !u.PhoneVerified || *u.Phone != "+251911234567" {
		t.Fatalf("verifying the phone: %d %+v", status, u)
	}
	if status, _, r := p.call("PUT", "/api/me/address", map[string]any{"street": " ", "city": "", "state": "", "zip": "", "country": ""}); status != http.StatusBadRequest ||
		len(r.Fields) != 3 {
		t.Fatalf("an empty address: %d %+v", status, r)
	}
	status, u, _ = p.call("PUT", "/api/me/address", addisAddress)
	if status != http.StatusOK || !strings.Contains(string(u.Address), "Bole Road 12") {
		t.Fatalf("the address: %d %s", status, u.Address)
	}
	status, u, _ = p.call("POST", "/api/me/onboarding/complete", nil)
	if status != http.StatusOK || !u.OnboardingCompleted || u.Minor || u.Consent != "not_needed" {
		t.Fatalf("completing onboarding: %d %+v", status, u)
	}
	if got := samMinor(t, e, p.pid); got != "false" {
		t.Fatalf("SAM holds minor=%q for an adult who completed onboarding", got)
	}
	if status, u, _ := p.call("PUT", "/api/me/public-profile", map[string]bool{"show": true}); status != http.StatusOK || !u.PublicProfile {
		t.Fatalf("an adult's public profile: %d %+v", status, u)
	}
}

// TestAMinor: marking a minor sets their consent pending, ends any public
// profile, refuses a new one, and tells SAM they are a minor.
func TestAMinor(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	p := signedIn(t, e, amara)
	if status, _, _ := p.call("PUT", "/api/me/public-profile", map[string]bool{"show": true}); status != http.StatusOK {
		t.Fatalf("a public profile before onboarding: %d", status)
	}
	status, u, _ := p.call("POST", "/api/me/minor", nil)
	if status != http.StatusOK || !u.Minor || u.Consent != "pending" || u.PublicProfile {
		t.Fatalf("marking a minor: %d %+v", status, u)
	}
	if status, _, r := p.call("PUT", "/api/me/public-profile", map[string]bool{"show": true}); status != http.StatusForbidden ||
		r.Code != "forbidden" {
		t.Fatalf("a minor's public profile: %d %+v", status, r)
	}
	if got := samMinor(t, e, p.pid); got != "true" {
		t.Fatalf("SAM holds minor=%q for a minor", got)
	}
	if status, u, _ := p.call("POST", "/api/me/minor", nil); status != http.StatusOK || u.Consent != "pending" {
		t.Fatalf("marking again: %d %+v", status, u)
	}
}

// TestOtherAccountsEmailAndPhone: an email or number another account has
// verified is refused with conflict; a code sent to one account does not
// verify another's phone.
func TestOtherAccountsEmailAndPhone(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	first := signedIn(t, e, amara)
	if status, _, _ := first.verifyPhone(ethiopia); status != http.StatusOK {
		t.Fatalf("the first verifies: %d", status)
	}
	second := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	if status, _, r := second.call("PATCH", "/api/me/profile", map[string]string{"fullName": "Kofi Mensah", "email": amara.Email}); status != http.StatusConflict ||
		r.Fields["email"] == "" {
		t.Fatalf("another account's email: %d %+v", status, r)
	}
	e.Clock.Add(31 * time.Second)
	if status, _, r := second.verifyPhone(ethiopia); status != http.StatusConflict || r.Fields["phone"] == "" {
		t.Fatalf("another account's phone: %d %+v", status, r)
	}

	b, _ := json.Marshal(map[string]string{"phone": "+254712345678"})
	res, raw := e.DoFrom(t, first.client, "198.51.100.7", "POST", "/api/me/phone/start", string(b))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("the first asks for a code: %d", res.StatusCode)
	}
	var c challenge
	_ = json.Unmarshal([]byte(raw), &c)
	msg, _ := e.SMS.Last(c.Phone)
	status, _, r := second.call("POST", "/api/me/phone/verify", map[string]string{"requestId": c.RequestID, "code": sixDigits.FindString(msg.Text)})
	if status != http.StatusBadRequest || r.Code != "code_invalid" {
		t.Fatalf("the second uses the first's code: %d %+v", status, r)
	}
}

// TestTheProfileNeedsASession: every profile call answers 401 without one.
func TestTheProfileNeedsASession(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	for _, c := range []struct{ method, path, body string }{
		{"PATCH", "/api/me/profile", `{"fullName":"A B","email":"a@b.co"}`},
		{"POST", "/api/me/phone/start", `{"phone":"+251911234567"}`},
		{"POST", "/api/me/phone/verify", `{"requestId":"x","code":"123456"}`},
		{"PUT", "/api/me/address", `{"street":"s","city":"c","state":"","zip":"","country":"x"}`},
		{"POST", "/api/me/onboarding/complete", ""},
		{"PUT", "/api/me/public-profile", `{"show":true}`},
		{"POST", "/api/me/minor", ""},
	} {
		if res, _ := e.Do(t, c.method, c.path, c.body); res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: %d", c.method, c.path, res.StatusCode)
		}
	}
}
