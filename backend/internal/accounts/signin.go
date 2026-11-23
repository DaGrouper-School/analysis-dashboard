package accounts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// The answers a Google sign-in can be refused with.
var (
	errSignInFailed = httpx.FieldError(httpx.CodeInvalidInput, "idToken", "Google sign-in failed. Please try again.")
	errEmailTaken   = httpx.New(httpx.CodeConflict,
		"This email belongs to another account. Sign in the way you did before.")
)

// SignInWithGoogle signs in the person an ID token names, creating their
// account on their first sign-in, and starts a session, setting its
// cookie on w. It reports whether the account is new. In demo mode, a demo
// account's subject stands in for a token.
//
// The account, its outbox row, its Google identity and the session are
// written in one transaction. Nothing waits on SAM: the account reaches it
// through the outbox.
func (m *Module) SignInWithGoogle(ctx context.Context, w http.ResponseWriter, idToken string) (User, bool, error) {
	claims, err := m.googleClaims(ctx, idToken)
	if err != nil {
		return User{}, false, err
	}
	var u User
	var isNew bool
	signIn := func(tx pgx.Tx) error {
		var err error
		if u, isNew, err = m.googleAccount(ctx, tx, claims); err != nil {
			return err
		}
		return m.sessions.start(ctx, tx, w, u.ID)
	}
	err = db.InTx(ctx, m.db, signIn)
	if isUniqueViolation(err) {
		// Two first sign-ins of one person at once: the second finds the
		// account the first made.
		err = db.InTx(ctx, m.db, signIn)
	}
	return u, isNew, err
}

// googleClaims is who idToken names: a verified Google ID token, or in demo
// mode a demo account's subject.
func (m *Module) googleClaims(ctx context.Context, idToken string) (GoogleClaims, error) {
	if m.cfg.Demo {
		d, ok, err := demoAccount(ctx, m.db, idToken)
		if err != nil {
			return GoogleClaims{}, err
		}
		if ok {
			c := GoogleClaims{Email: d.Email, EmailVerified: true, Name: d.Name}
			c.Subject = d.Subject
			if d.AvatarURL != nil {
				c.Picture = *d.AvatarURL
			}
			return c, nil
		}
	}
	c, err := m.google.verify(ctx, idToken)
	if err != nil {
		slog.InfoContext(ctx, "Google sign-in refused", "err", err)
		return GoogleClaims{}, errSignInFailed
	}
	return c, nil
}

// googleAccount is the account linked to the Google subject, or a new one
// for it. A new account whose verified email belongs to another account is
// refused: the person signs in the way they did before.
func (m *Module) googleAccount(ctx context.Context, tx pgx.Tx, c GoogleClaims) (User, bool, error) {
	id, linked, err := googleUser(ctx, tx, c.Subject)
	if err != nil {
		return User{}, false, err
	}
	if linked {
		if err := noteGoogleEmail(ctx, tx, c.Subject, c.Email); err != nil {
			return User{}, false, err
		}
		u, err := getUser(ctx, tx, id)
		return u, false, err
	}
	n := NewUser{AuthProvider: "google", FullName: c.Name, EmailVerified: c.Email != "" && c.EmailVerified}
	if c.Email != "" {
		n.Email = &c.Email
	}
	if c.Picture != "" {
		n.AvatarURL = &c.Picture
	}
	if n.EmailVerified {
		_, taken, err := userWithEmail(ctx, tx, c.Email)
		if err != nil {
			return User{}, false, err
		}
		if taken {
			return User{}, false, errEmailTaken
		}
	}
	u, err := Create(ctx, tx, m.now(), n)
	if err != nil {
		return User{}, false, err
	}
	return u, true, linkGoogle(ctx, tx, c.Subject, u.ID, c.Email, m.now())
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
