package apitest_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
)

const ethiopia = "+251911234567"

type challenge struct {
	RequestID string `json:"requestId"`
	Phone     string `json:"phone"`
	ResendAt  string `json:"resendAvailableAt"`
	ExpiresAt string `json:"expiresAt"`
	DevCode   string `json:"devCode"`
}

var sixDigits = regexp.MustCompile(`\d{6}`)

// phone is one person signing in by phone, from one client address.
type phone struct {
	t      *testing.T
	e      *apitest.Env
	number string
	addr   string
	client *http.Client
}

func newPhone(t *testing.T, e *apitest.Env, number, addr string) *phone {
	return &phone{t: t, e: e, number: number, addr: addr, client: e.NewClient()}
}

func (p *phone) start() (*http.Response, challenge) {
	p.t.Helper()
	body, _ := json.Marshal(map[string]string{"phone": p.number})
	res, raw := p.e.DoFrom(p.t, p.client, p.addr, "POST", "/api/auth/phone/start", string(body))
	var c challenge
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			p.t.Fatal(err)
		}
	}
	return res, c
}

// mustStart sends a code and returns it, as the person reads it in the
// text.
func (p *phone) mustStart() (challenge, string) {
	p.t.Helper()
	res, c := p.start()
	if res.StatusCode != http.StatusOK {
		p.t.Fatalf("sending a code to %s: %d", p.number, res.StatusCode)
	}
	if c.DevCode != "" {
		return c, c.DevCode
	}
	msg, ok := p.e.SMS.Last(c.Phone)
	if !ok {
		p.t.Fatalf("no text to %s", c.Phone)
	}
	return c, sixDigits.FindString(msg.Text)
}

func (p *phone) verify(c challenge, code string) (*http.Response, authResult, string) {
	p.t.Helper()
	body, _ := json.Marshal(map[string]string{"requestId": c.RequestID, "code": code})
	res, raw := p.e.DoFrom(p.t, p.client, p.addr, "POST", "/api/auth/phone/verify", string(body))
	var out authResult
	var refused struct {
		Code string `json:"code"`
	}
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			p.t.Fatal(err)
		}
	} else {
		_ = json.Unmarshal([]byte(raw), &refused)
	}
	return res, out, refused.Code
}

func wrong(code string) string {
	n, _ := strconv.Atoi(code)
	return fmt.Sprintf("%06d", (n+1)%1_000_000)
}

func retryAfter(t *testing.T, res *http.Response) time.Duration {
	t.Helper()
	s, err := strconv.Atoi(res.Header.Get("Retry-After"))
	if err != nil || s <= 0 {
		t.Fatalf("Retry-After %q", res.Header.Get("Retry-After"))
	}
	return time.Duration(s) * time.Second
}

// TestPhoneSignIn: a code texted to a number signs its holder in, creating
// their account the first time; a wrong code is code_invalid; a used or
// expired one is code_expired.
func TestPhoneSignIn(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	p := newPhone(t, e, "+251 91 123 4567", "198.51.100.1")
	c, code := p.mustStart()
	if c.Phone != ethiopia || c.DevCode != "" || len(code) != 6 {
		t.Fatalf("the challenge %+v, code %q", c, code)
	}
	if res, _, why := p.verify(c, wrong(code)); res.StatusCode != http.StatusBadRequest || why != "code_invalid" {
		t.Fatalf("a wrong code: %d %s", res.StatusCode, why)
	}
	res, first, _ := p.verify(c, code)
	if res.StatusCode != http.StatusOK || !first.IsNewUser || first.User.AuthProvider != "phone" || sessionCookie(res) == nil {
		t.Fatalf("the right code: %d %+v", res.StatusCode, first)
	}
	if res, _ := e.DoWith(t, p.client, "GET", "/api/auth/me", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("signed in: %d", res.StatusCode)
	}
	if res, _, why := p.verify(c, code); res.StatusCode != http.StatusBadRequest || why != "code_expired" {
		t.Fatalf("a used code: %d %s", res.StatusCode, why)
	}

	e.Clock.Add(31 * time.Second)
	c, code = p.mustStart()
	if _, again, _ := p.verify(c, code); again.IsNewUser || again.User.ID != first.User.ID {
		t.Fatalf("signing in again: %+v", again)
	}
	e.Clock.Add(31 * time.Second)
	c, code = p.mustStart()
	e.Clock.Add(5*time.Minute + time.Second)
	if res, _, why := p.verify(c, code); res.StatusCode != http.StatusBadRequest || why != "code_expired" {
		t.Fatalf("an expired code: %d %s", res.StatusCode, why)
	}
	if res, _ := newPhone(t, e, "0911234567", "198.51.100.1").start(); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("a number without its country code: %d", res.StatusCode)
	}
}

// TestParallelWrongGuessesSpendTheBudgetExactly: of twenty wrong guesses
// at once, exactly five are compared; the rest are refused, and so is the
// right code afterwards.
func TestParallelWrongGuessesSpendTheBudgetExactly(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	p := newPhone(t, e, ethiopia, "198.51.100.1")
	c, code := p.mustStart()
	body, _ := json.Marshal(map[string]string{"requestId": c.RequestID, "code": wrong(code)})
	statuses := make(chan string, 20)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			req, _ := http.NewRequest("POST", e.URL+"/api/auth/phone/verify", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(apitest.ClientHeader, fmt.Sprintf("203.0.113.%d", i+1))
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				statuses <- err.Error()
				return
			}
			var refused struct {
				Code string `json:"code"`
			}
			_ = json.NewDecoder(res.Body).Decode(&refused)
			res.Body.Close()
			statuses <- refused.Code
		})
	}
	wg.Wait()
	close(statuses)
	counts := map[string]int{}
	for s := range statuses {
		counts[s]++
	}
	if counts["code_invalid"] != 5 || counts["rate_limited"] != 15 {
		t.Fatalf("twenty wrong guesses at once: %v", counts)
	}
	if res, _, why := p.verify(c, code); res.StatusCode != http.StatusTooManyRequests || why != "rate_limited" {
		t.Fatalf("the right code after the budget is spent: %d %s", res.StatusCode, why)
	}
}

// TestANewCodeKeepsTheAttemptCount: asking for a new code does not give a
// number more guesses; once every code has expired, the count starts again.
func TestANewCodeKeepsTheAttemptCount(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	p := newPhone(t, e, ethiopia, "198.51.100.1")
	c, code := p.mustStart()
	for range 3 {
		if _, _, why := p.verify(c, wrong(code)); why != "code_invalid" {
			t.Fatalf("a wrong guess: %s", why)
		}
	}
	e.Clock.Add(31 * time.Second)
	c, code = p.mustStart()
	for range 2 {
		if _, _, why := p.verify(c, wrong(code)); why != "code_invalid" {
			t.Fatalf("a wrong guess at the new code: %s", why)
		}
	}
	res, _, why := p.verify(c, code)
	if res.StatusCode != http.StatusTooManyRequests || why != "rate_limited" {
		t.Fatalf("the sixth attempt, with the right code: %d %s", res.StatusCode, why)
	}
	retryAfter(t, res)

	e.Clock.Add(5*time.Minute + time.Second)
	c, code = p.mustStart()
	if res, _, _ := p.verify(c, code); res.StatusCode != http.StatusOK {
		t.Fatalf("once every code has expired: %d", res.StatusCode)
	}
}

// TestTheSixthSendInAnHourIsRefused: a number is sent five codes an hour;
// the sixth is 429 with Retry-After, and a resend before thirty seconds is
// refused too.
func TestTheSixthSendInAnHourIsRefused(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	p := newPhone(t, e, ethiopia, "198.51.100.1")
	p.mustStart()
	res, _ := p.start()
	if res.StatusCode != http.StatusTooManyRequests || retryAfter(t, res) > 31*time.Second {
		t.Fatalf("a resend at once: %d", res.StatusCode)
	}
	for range 4 {
		e.Clock.Add(31 * time.Second)
		p.mustStart()
	}
	e.Clock.Add(31 * time.Second)
	res, _ = p.start()
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the sixth send in an hour: %d", res.StatusCode)
	}
	wait := retryAfter(t, res)
	if wait < 57*time.Minute || wait > time.Hour+time.Second {
		t.Fatalf("Retry-After %s, want until the first send leaves the hour", wait)
	}
	if len(e.SMS.Sent()) != 5 {
		t.Fatalf("%d texts sent, want 5", len(e.SMS.Sent()))
	}
	e.Clock.Add(wait)
	p.mustStart()
}

// TestSendLimitsPerAddressAndOverall: one client address is sent twenty
// codes an hour, to any numbers; the deployment, its overall limit.
func TestSendLimitsPerAddressAndOverall(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	for i := range 20 {
		newPhone(t, e, fmt.Sprintf("+2519112340%02d", i), "198.51.100.1").mustStart()
	}
	if res, _ := newPhone(t, e, "+251911234099", "198.51.100.1").start(); res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the twenty-first send from one address: %d", res.StatusCode)
	}
	if res, _ := newPhone(t, e, "+251911234099", "198.51.100.2").start(); res.StatusCode != http.StatusOK {
		t.Fatalf("a send from another address: %d", res.StatusCode)
	}

	small := apitest.NewWith(t, apitest.Options{SendsPerHour: 3})
	for i := range 3 {
		newPhone(t, small, fmt.Sprintf("+25491123450%d", i), fmt.Sprintf("192.0.2.%d", i+1)).mustStart()
	}
	if res, _ := newPhone(t, small, "+254911234509", "192.0.2.9").start(); res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("a send past the overall limit: %d", res.StatusCode)
	}
}

// TestDemoReturnsTheCode: in demo mode the code comes back in the response
// and is not texted.
func TestDemoReturnsTheCode(t *testing.T) {
	t.Parallel()
	e := apitest.NewWith(t, apitest.Options{Demo: true})
	p := newPhone(t, e, ethiopia, "198.51.100.1")
	_, c := p.start()
	if len(c.DevCode) != 6 || len(e.SMS.Sent()) != 0 {
		t.Fatalf("demo: devCode %q, %d texts", c.DevCode, len(e.SMS.Sent()))
	}
	if res, _, _ := p.verify(c, c.DevCode); res.StatusCode != http.StatusOK {
		t.Fatalf("signing in with the devCode: %d", res.StatusCode)
	}
}
