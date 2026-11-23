package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// Sessions (docs/design/accounts-and-sign-in.md, "Sessions and cookies").
const (
	// CookieName is the session cookie.
	CookieName = "session"
	// extendEvery is how often at most a session's use moves its end on.
	extendEvery = time.Hour
	// pruneEvery is how often ended sessions are removed.
	pruneEvery = time.Hour
	// tokenBytes is a session token's length before encoding.
	tokenBytes = 32
)

// Sessions finds who a request is from, by the session cookie.
type Sessions struct {
	db     db.Querier
	now    func() time.Time
	ttl    time.Duration
	secure bool
}

type userKey struct{}

// CurrentUser is the signed-in user RequireUser found for the request.
func CurrentUser(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey{}).(User)
	return u, ok
}

// RequireUser serves next only for a request with a live session, with the
// user in its context; any other request gets 401 unauthenticated. A use
// of the session moves its end on, at most once an hour, and sends the
// cookie again with the new expiry so the browser keeps it as long as the
// database does.
func (s *Sessions) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.authenticate(w, r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

// authenticate is the request's user, extending their session when due.
func (s *Sessions) authenticate(w http.ResponseWriter, r *http.Request) (User, error) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return User{}, httpx.Unauthenticated()
	}
	hash := hashToken(c.Value)
	now := s.now()
	found, ok, err := liveSession(r.Context(), s.db, hash, now)
	if err != nil {
		return User{}, err
	}
	if !ok {
		s.clear(w)
		return User{}, httpx.Unauthenticated()
	}
	if now.Sub(found.extendedAt) >= extendEvery {
		expires := now.Add(s.ttl)
		if err := extendSession(r.Context(), s.db, hash, now, expires); err != nil {
			return User{}, err
		}
		s.set(w, c.Value, now, expires)
	}
	return found.user, nil
}

// start opens a session for user in q's transaction and sets its cookie.
func (s *Sessions) start(ctx context.Context, q db.Querier, w http.ResponseWriter, user uuid.UUID) error {
	token, err := newToken()
	if err != nil {
		return err
	}
	now := s.now()
	expires := now.Add(s.ttl)
	if err := insertSession(ctx, q, hashToken(token), user, now, expires); err != nil {
		return err
	}
	s.set(w, token, now, expires)
	return nil
}

// end deletes the request's session and clears its cookie.
func (s *Sessions) end(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		if err := deleteSession(r.Context(), s.db, hashToken(c.Value)); err != nil {
			return err
		}
	}
	s.clear(w)
	return nil
}

func (s *Sessions) set(w http.ResponseWriter, token string, now, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", Expires: expires,
		MaxAge: int(expires.Sub(now).Seconds()), HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
}

func (s *Sessions) clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure,
		SameSite: http.SameSiteLaxMode})
}

// newToken is a random session token.
func newToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken is what the database keeps of a token. The token is random and
// long, so a plain hash is enough: there is nothing to guess.
func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
