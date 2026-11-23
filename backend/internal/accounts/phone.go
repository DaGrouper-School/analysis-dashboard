package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// The phone code rules (docs/design/accounts-and-sign-in.md#phone-codes).
const (
	codeDigits  = 6
	codeLife    = 5 * time.Minute
	resendAfter = 30 * time.Second
	// maxAttempts is a number's attempts while its codes live; a new code
	// does not reset them.
	maxAttempts = 5
	// limitWindow is the sliding window every send and verify limit counts
	// over.
	limitWindow = time.Hour
	// The limits per hour, besides the overall send limit, which each
	// deployment sets (DGS_SMS_SENDS_PER_HOUR).
	sendsPerNumber      = 5
	sendsPerAddress     = 20
	verifiesPerNumber   = 10
	verifiesPerAddress  = 50
	DefaultSendsPerHour = 200
)

// The purposes a code is sent for.
const (
	PurposeSignIn      = "sign_in"
	PurposeVerifyPhone = "verify_phone"
)

// The answers a phone code can be refused with.
var (
	errBadNumber   = httpx.FieldError(httpx.CodeInvalidInput, "phone", "Enter a valid phone number with country code.")
	errCodeExpired = httpx.FieldError(httpx.CodeCodeExpired, "code", "This code has expired. Request a new one.")
	errCodeInvalid = httpx.FieldError(httpx.CodeCodeInvalid, "code", "That code is not right. Check the SMS and try again.")
)

const (
	msgWaitToResend  = "Please wait before requesting another code."
	msgTooManySends  = "Too many codes requested. Please try again later."
	msgTooManyTries  = "Too many attempts. Request a new code later."
	msgTooManyChecks = "Too many attempts. Please try again later."
)

// e164 is a number in E.164 form: a plus and 8 to 15 digits, the first not
// zero.
var e164 = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)

// NormalizePhone is raw in E.164 form, or "" when it is not a number with
// its country code. It accepts what the front end's normalizePhone does: a
// leading + or 00, then digits with any spacing or punctuation.
func NormalizePhone(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(trimmed, "00") {
		return ""
	}
	var digits strings.Builder
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	national := digits.String()
	if strings.HasPrefix(trimmed, "00") {
		national = national[2:]
	}
	if n := "+" + national; e164.MatchString(n) {
		return n
	}
	return ""
}

// clientAddress is the key the request's client is limited under: the last
// address in header when a trusted proxy sets it, otherwise the
// connection's, with IPv6 addresses keyed by their /64.
func clientAddress(r *http.Request, header string) string {
	raw := ""
	if header != "" {
		if v := r.Header.Get(header); v != "" {
			parts := strings.Split(v, ",")
			raw = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	if raw == "" {
		raw = r.RemoteAddr
		if host, _, err := net.SplitHostPort(raw); err == nil {
			raw = host
		}
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return raw
	}
	addr = addr.Unmap()
	if addr.Is6() {
		p, _ := addr.Prefix(64)
		return p.String()
	}
	return addr.String()
}

// limit is one sliding-window limit: a key and how many uses it allows.
type limit struct {
	key string
	max int
}

// take takes one use of every limit, or answers rate_limited with when to
// try again.
func (m *Module) take(ctx context.Context, msg string, limits ...limit) error {
	var wait time.Duration
	err := db.InTx(ctx, m.db, func(tx pgx.Tx) error {
		var err error
		wait, err = takeLimits(ctx, tx, m.now(), limitWindow, limits)
		if err == nil && wait > 0 {
			return errLimited
		}
		return err
	})
	if errors.Is(err, errLimited) {
		return httpx.RateLimited(msg, wait.Round(time.Second)+time.Second)
	}
	return err
}

// errLimited ends the limits' transaction when one is reached, so none is
// taken.
var errLimited = errors.New("a limit is reached")

// PhoneChallenge is a code sent, as the front end's PhoneChallenge shows it.
type PhoneChallenge struct {
	RequestID string `json:"requestId"`
	Phone     string `json:"phone"`
	ResendAt  string `json:"resendAvailableAt"`
	ExpiresAt string `json:"expiresAt"`
	// DevCode is the code itself, in demo mode only.
	DevCode string `json:"devCode,omitempty"`
}

// RequestCode sends a code for purpose to raw, normalised to E.164, from
// the client at addr. It is refused while the number's last code may not
// yet be resent, and past the send limits per number, per client address
// and overall. user is the signed-in user a verify_phone code is for.
func (m *Module) RequestCode(ctx context.Context, raw, purpose string, user *uuid.UUID, addr string) (PhoneChallenge, error) {
	phone := NormalizePhone(raw)
	if phone == "" {
		return PhoneChallenge{}, errBadNumber
	}
	now := m.now()
	at, err := resendAt(ctx, m.db, phone, purpose, now)
	if err != nil {
		return PhoneChallenge{}, err
	}
	if at.After(now) {
		return PhoneChallenge{}, httpx.RateLimited(msgWaitToResend, at.Sub(now).Round(time.Second)+time.Second)
	}
	if err := m.take(ctx, msgTooManySends, limit{"send:phone:" + phone, sendsPerNumber}, limit{"send:addr:" + addr, sendsPerAddress},
		limit{"send:all", m.cfg.SMSSendsPerHour}); err != nil {
		return PhoneChallenge{}, err
	}
	code, err := newCode()
	if err != nil {
		return PhoneChallenge{}, err
	}
	c := challenge{id: uuid.New(), phone: phone, purpose: purpose, user: user, resendAt: now.Add(resendAfter), expiresAt: now.Add(codeLife)}
	c.codeHash = hashCode(c.id, code)
	if err := db.InTx(ctx, m.db, func(tx pgx.Tx) error { return replaceChallenge(ctx, tx, c, now) }); err != nil {
		return PhoneChallenge{}, err
	}
	out := PhoneChallenge{RequestID: c.id.String(), Phone: phone, ResendAt: timestamp(c.resendAt), ExpiresAt: timestamp(c.expiresAt)}
	if m.cfg.Demo {
		// Demo mode returns the code instead of texting it.
		out.DevCode = code
		return out, nil
	}
	text := fmt.Sprintf("Your DaGrouper School code is %s. It expires in %d minutes. Never share it.", code, int(codeLife.Minutes()))
	if err := m.cfg.SMS.Send(ctx, phone, text); err != nil {
		return PhoneChallenge{}, fmt.Errorf("sending a code: %w", err)
	}
	return out, nil
}

// CheckCode uses the code requestID names, for purpose, from the client at
// addr, and returns the challenge it answered.
//
// Past the verify limits per number and per client address it is refused.
// Otherwise one of the number's attempts is spent, in one statement that
// increments the count and reads it, and only then is the code compared:
// a wrong guess is never free, and parallel guesses spend the budget
// exactly. A right code is used once.
func (m *Module) CheckCode(ctx context.Context, requestID, code, purpose, addr string) (challenge, error) {
	id, err := uuid.Parse(requestID)
	if err != nil {
		return challenge{}, errCodeExpired
	}
	c, ok, err := getChallenge(ctx, m.db, id)
	if err != nil {
		return challenge{}, err
	}
	now := m.now()
	if !ok || c.purpose != purpose || c.endedAt != nil || !c.expiresAt.After(now) {
		return challenge{}, errCodeExpired
	}
	if err := m.take(ctx, msgTooManyChecks, limit{"verify:phone:" + c.phone, verifiesPerNumber},
		limit{"verify:addr:" + addr, verifiesPerAddress}); err != nil {
		return challenge{}, err
	}
	spent, resets, err := spendAttempt(ctx, m.db, c.phone, purpose, maxAttempts, now)
	if err != nil {
		return challenge{}, err
	}
	if !spent {
		return challenge{}, httpx.RateLimited(msgTooManyTries, max(resets.Sub(now), 0).Round(time.Second)+time.Second)
	}
	if subtle.ConstantTimeCompare(hashCode(id, code), c.codeHash) != 1 {
		return challenge{}, errCodeInvalid
	}
	used, err := useChallenge(ctx, m.db, id, now)
	if err != nil {
		return challenge{}, err
	}
	if !used {
		return challenge{}, errCodeExpired
	}
	return c, nil
}

// SignInWithPhone signs in the person who holds the number a sign-in code
// was sent to, creating their account on first sign-in, and starts a
// session, setting its cookie on w. It reports whether the account is new.
// The account, its outbox row and the session are written in one
// transaction; nothing waits on SAM.
func (m *Module) SignInWithPhone(ctx context.Context, w http.ResponseWriter, requestID, code, addr string) (User, bool, error) {
	c, err := m.CheckCode(ctx, requestID, code, PurposeSignIn, addr)
	if err != nil {
		return User{}, false, err
	}
	var u User
	var isNew bool
	signIn := func(tx pgx.Tx) error {
		id, found, err := userWithPhone(ctx, tx, c.phone)
		if err != nil {
			return err
		}
		if found {
			u, err = getUser(ctx, tx, id)
			isNew = false
		} else {
			phone := c.phone
			u, err = Create(ctx, tx, m.now(), NewUser{AuthProvider: "phone", Phone: &phone, PhoneVerified: true})
			isNew = true
		}
		if err != nil {
			return err
		}
		return m.sessions.start(ctx, tx, w, u.ID)
	}
	err = db.InTx(ctx, m.db, signIn)
	if isUniqueViolation(err) {
		err = db.InTx(ctx, m.db, signIn)
	}
	if err != nil {
		slog.ErrorContext(ctx, "phone sign-in after a right code", "err", err)
	}
	return u, isNew, err
}

// newCode is a code of codeDigits digits from a secure random source.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", codeDigits, n.Int64()), nil
}

// hashCode is what is stored of a code: its hash with the challenge's ID,
// so equal codes sent to two numbers are stored differently.
func hashCode(id uuid.UUID, code string) []byte {
	h := sha256.New()
	h.Write(id[:])
	h.Write([]byte(code))
	return h.Sum(nil)
}
