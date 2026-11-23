package accounts

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The module's store: the only queries on users and the participant ID
// sequence. Each works on the pool or inside a transaction.

const userColumns = `users.id, participant_id, sam_principal_id, minor, consent, sam_minor, sam_consent,
	auth_provider, full_name, email, email_verified, phone, phone_verified, avatar_url, role, platform_admin,
	address, onboarding_completed, public_profile, users.created_at, updated_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.ParticipantID, &u.Principal, &u.Minor, &u.Consent, &u.samMinor, &u.samConsent,
		&u.AuthProvider, &u.FullName, &u.Email, &u.EmailVerified, &u.Phone, &u.PhoneVerified, &u.AvatarURL, &u.Role,
		&u.PlatformAdmin, &u.Address, &u.OnboardingCompleted, &u.PublicProfile, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// insertUser makes a user, with the next participant ID.
func insertUser(ctx context.Context, q db.Querier, id uuid.UUID, now time.Time, n NewUser) (User, error) {
	return scanUser(q.QueryRow(ctx, `
		INSERT INTO users (id, created_at, updated_at, auth_provider, full_name, email, email_verified, phone, phone_verified, avatar_url)
		VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING `+userColumns,
		id, now, n.AuthProvider, n.FullName, n.Email, n.EmailVerified, n.Phone, n.PhoneVerified, n.AvatarURL))
}

// userWithEmail is the user a verified email belongs to, if any.
func userWithEmail(ctx context.Context, q db.Querier, email string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT id FROM users WHERE email_verified AND lower(email) = lower($1)`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

// googleUser is the user a Google subject is linked to, if any.
func googleUser(ctx context.Context, q db.Querier, subject string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT user_id FROM google_identities WHERE subject = $1`, subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func linkGoogle(ctx context.Context, q db.Querier, subject string, id uuid.UUID, email string, now time.Time) error {
	_, err := q.Exec(ctx, `INSERT INTO google_identities (subject, user_id, email, created_at) VALUES ($1, $2, nullif($3, ''), $4)`,
		subject, id, email, now)
	return err
}

// noteGoogleEmail keeps the email Google last gave for a subject.
func noteGoogleEmail(ctx context.Context, q db.Querier, subject, email string) error {
	_, err := q.Exec(ctx, `UPDATE google_identities SET email = nullif($2, '') WHERE subject = $1`, subject, email)
	return err
}

// DemoAccount is a Google account demo mode offers.
type DemoAccount struct {
	Subject   string
	Name      string
	Email     string
	Type      string
	AvatarURL *string
}

func demoAccounts(ctx context.Context, q db.Querier) ([]DemoAccount, error) {
	rows, err := q.Query(ctx, `SELECT subject, name, email, type, avatar_url FROM demo_accounts ORDER BY name, subject`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (DemoAccount, error) {
		var d DemoAccount
		err := r.Scan(&d.Subject, &d.Name, &d.Email, &d.Type, &d.AvatarURL)
		return d, err
	})
}

func demoAccount(ctx context.Context, q db.Querier, subject string) (DemoAccount, bool, error) {
	var d DemoAccount
	err := q.QueryRow(ctx, `SELECT subject, name, email, type, avatar_url FROM demo_accounts WHERE subject = $1`, subject).
		Scan(&d.Subject, &d.Name, &d.Email, &d.Type, &d.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return DemoAccount{}, false, nil
	}
	return d, err == nil, err
}

// session is a session found by its token's hash, with its user.
type session struct {
	user       User
	extendedAt time.Time
	expiresAt  time.Time
}

func insertSession(ctx context.Context, q db.Querier, hash []byte, user uuid.UUID, now, expires time.Time) error {
	_, err := q.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, extended_at, expires_at) VALUES ($1, $2, $3, $3, $4)`,
		hash, user, now, expires)
	return err
}

// liveSession is the session hash names, unless it has ended.
func liveSession(ctx context.Context, q db.Querier, hash []byte, now time.Time) (session, bool, error) {
	row := q.QueryRow(ctx, `SELECT `+userColumns+`, sessions.extended_at, sessions.expires_at
		FROM sessions JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = $1 AND sessions.expires_at > $2`, hash, now)
	var s session
	u := &s.user
	err := row.Scan(&u.ID, &u.ParticipantID, &u.Principal, &u.Minor, &u.Consent, &u.samMinor, &u.samConsent,
		&u.AuthProvider, &u.FullName, &u.Email, &u.EmailVerified, &u.Phone, &u.PhoneVerified, &u.AvatarURL, &u.Role,
		&u.PlatformAdmin, &u.Address, &u.OnboardingCompleted, &u.PublicProfile, &u.CreatedAt, &u.UpdatedAt,
		&s.extendedAt, &s.expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return session{}, false, nil
	}
	return s, err == nil, err
}

// extendSession moves the session's end on, once an hour at most.
func extendSession(ctx context.Context, q db.Querier, hash []byte, now, expires time.Time) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET extended_at = $2, expires_at = $3 WHERE token_hash = $1`, hash, now, expires)
	return err
}

func deleteSession(ctx context.Context, q db.Querier, hash []byte) error {
	_, err := q.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash)
	return err
}

// deleteEndedSessions removes sessions that ended before now.
func deleteEndedSessions(ctx context.Context, q db.Querier, now time.Time) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	return tag.RowsAffected(), err
}

func getUser(ctx context.Context, q db.Querier, id uuid.UUID) (User, error) {
	return scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// lockUser reads a user and locks their row until tx ends, so two changes
// to them queue their outbox versions in the order they commit.
func lockUser(ctx context.Context, tx db.Querier, id uuid.UUID) (User, error) {
	return scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1 FOR UPDATE`, id))
}

// usersAfter is a page of users by ID, after the given one.
func usersAfter(ctx context.Context, q db.Querier, after uuid.UUID, limit int) ([]User, error) {
	rows, err := q.Query(ctx, `SELECT `+userColumns+` FROM users WHERE id > $1 ORDER BY id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
}

// newParticipantID gives the user the next participant ID.
func newParticipantID(ctx context.Context, q db.Querier, id uuid.UUID) (int64, error) {
	var n int64
	err := q.QueryRow(ctx, `UPDATE users SET participant_id = nextval('participant_ids') WHERE id = $1 RETURNING participant_id`, id).
		Scan(&n)
	return n, err
}

func setMinor(ctx context.Context, q db.Querier, id uuid.UUID, minor bool) error {
	_, err := q.Exec(ctx, `UPDATE users SET minor = $2 WHERE id = $1`, id, minor)
	return err
}

func setConsent(ctx context.Context, q db.Querier, id uuid.UUID, c Consent) error {
	_, err := q.Exec(ctx, `UPDATE users SET consent = $2 WHERE id = $1`, id, c)
	return err
}

// setPrincipal records the user's SAM principal, registered under
// participant ID n, unless they have been renumbered since.
func setPrincipal(ctx context.Context, q db.Querier, id, principal uuid.UUID, n int64) error {
	_, err := q.Exec(ctx, `UPDATE users SET sam_principal_id = $2 WHERE id = $1 AND participant_id = $3`, id, principal, n)
	return err
}

// setSAMMinor and setSAMConsent record what SAM holds about the user, once
// it has confirmed an assertion, or once the reconciler has read it.
func setSAMMinor(ctx context.Context, q db.Querier, id uuid.UUID, v *bool) error {
	_, err := q.Exec(ctx, `UPDATE users SET sam_minor = $2 WHERE id = $1`, id, v)
	return err
}

func setSAMConsent(ctx context.Context, q db.Querier, id uuid.UUID, v *Consent) error {
	_, err := q.Exec(ctx, `UPDATE users SET sam_consent = $2 WHERE id = $1`, id, v)
	return err
}

// nextParticipantID is the ID the sequence gives next, without taking it.
func nextParticipantID(ctx context.Context, q db.Querier) (int64, error) {
	var n int64
	err := q.QueryRow(ctx, `SELECT CASE WHEN is_called THEN last_value + 1 ELSE last_value END FROM participant_ids`).Scan(&n)
	return n, err
}

// moveParticipantIDsPast makes the sequence give IDs above last from now
// on, unless it already does.
func moveParticipantIDsPast(ctx context.Context, q db.Querier, last int64) error {
	_, err := q.Exec(ctx, `SELECT setval('participant_ids', greatest($1::bigint,
		(SELECT CASE WHEN is_called THEN last_value ELSE last_value - 1 END FROM participant_ids)))`, last)
	return err
}

// takeLimits takes one use of every limit in tx, or none: when any key is
// at its limit, it reports how long until that key has room, and takes
// nothing. Each key's row is locked while it is counted, so two requests
// never both take its last place; keys are locked in order, so two
// requests never wait on each other.
func takeLimits(ctx context.Context, tx db.Querier, now time.Time, window time.Duration, limits []limit) (time.Duration, error) {
	slices.SortFunc(limits, func(a, b limit) int { return strings.Compare(a.key, b.key) })
	since := now.Add(-window)
	for _, l := range limits {
		if _, err := tx.Exec(ctx, `INSERT INTO rate_limits (key) VALUES ($1) ON CONFLICT (key) DO NOTHING`, l.key); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `SELECT key FROM rate_limits WHERE key = $1 FOR UPDATE`, l.key); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM rate_events WHERE key = $1 AND at <= $2`, l.key, since); err != nil {
			return 0, err
		}
		var used int
		var oldest *time.Time
		if err := tx.QueryRow(ctx, `SELECT count(*), min(at) FROM rate_events WHERE key = $1`, l.key).Scan(&used, &oldest); err != nil {
			return 0, err
		}
		if used >= l.max {
			return oldest.Add(window).Sub(now), nil
		}
	}
	for _, l := range limits {
		if _, err := tx.Exec(ctx, `INSERT INTO rate_events (key, at) VALUES ($1, $2)`, l.key, now); err != nil {
			return 0, err
		}
	}
	return 0, nil
}

// challenge is a code sent to a number.
type challenge struct {
	id        uuid.UUID
	phone     string
	purpose   string
	user      *uuid.UUID
	codeHash  []byte
	resendAt  time.Time
	expiresAt time.Time
	endedAt   *time.Time
}

// resendAt is when the number may next be sent a code for purpose: the
// live code's resend time, or zero when it has none.
func resendAt(ctx context.Context, q db.Querier, phone, purpose string, now time.Time) (time.Time, error) {
	var at time.Time
	err := q.QueryRow(ctx, `SELECT resend_at FROM phone_challenges WHERE phone = $1 AND purpose = $2 AND ended_at IS NULL
		AND expires_at > $3 ORDER BY created_at DESC LIMIT 1`, phone, purpose, now).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return at, err
}

// replaceChallenge ends the number's live codes for purpose and stores c in
// their place. The number's attempts carry over while any of its codes has
// lived, and start again only once every code has expired.
func replaceChallenge(ctx context.Context, tx db.Querier, c challenge, now time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE phone_challenges SET ended_at = $3 WHERE phone = $1 AND purpose = $2 AND ended_at IS NULL`,
		c.phone, c.purpose, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO phone_challenges (id, phone, purpose, user_id, code_hash, created_at, resend_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, c.id, c.phone, c.purpose, c.user, c.codeHash, now, c.resendAt, c.expiresAt); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO phone_attempts (phone, purpose, attempts, window_ends_at) VALUES ($1, $2, 0, $4)
		ON CONFLICT (phone, purpose) DO UPDATE SET
			attempts = CASE WHEN phone_attempts.window_ends_at <= $3 THEN 0 ELSE phone_attempts.attempts END,
			window_ends_at = $4`, c.phone, c.purpose, now, c.expiresAt)
	return err
}

func getChallenge(ctx context.Context, q db.Querier, id uuid.UUID) (challenge, bool, error) {
	c := challenge{id: id}
	err := q.QueryRow(ctx, `SELECT phone, purpose, user_id, code_hash, resend_at, expires_at, ended_at FROM phone_challenges WHERE id = $1`, id).
		Scan(&c.phone, &c.purpose, &c.user, &c.codeHash, &c.resendAt, &c.expiresAt, &c.endedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return challenge{}, false, nil
	}
	return c, err == nil, err
}

// spendAttempt spends one of the number's attempts for purpose, in one
// statement that increments the count and reads it, so parallel guesses
// spend the budget exactly. It reports false, with when the budget starts
// again, when none is left.
func spendAttempt(ctx context.Context, q db.Querier, phone, purpose string, max int, now time.Time) (bool, time.Time, error) {
	var n int
	err := q.QueryRow(ctx, `UPDATE phone_attempts SET attempts = attempts + 1
		WHERE phone = $1 AND purpose = $2 AND attempts < $3 AND window_ends_at > $4 RETURNING attempts`, phone, purpose, max, now).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		var ends time.Time
		if err := q.QueryRow(ctx, `SELECT window_ends_at FROM phone_attempts WHERE phone = $1 AND purpose = $2`, phone, purpose).
			Scan(&ends); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return false, time.Time{}, err
		}
		return false, ends, nil
	}
	return err == nil, time.Time{}, err
}

// useChallenge ends a live code as used, once: of two requests with the
// right code, one uses it.
func useChallenge(ctx context.Context, q db.Querier, id uuid.UUID, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE phone_challenges SET ended_at = $2 WHERE id = $1 AND ended_at IS NULL AND expires_at > $2`, id, now)
	return tag.RowsAffected() == 1, err
}

// userWithPhone is the user a verified number belongs to, if any.
func userWithPhone(ctx context.Context, q db.Querier, phone string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT id FROM users WHERE phone_verified AND phone = $1`, phone).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

// deleteSpent removes what no limit or code needs any more: limit events
// older than the window, codes ended or expired a day ago, and attempt
// budgets whose window ended a day ago.
func deleteSpent(ctx context.Context, q db.Querier, now time.Time, window time.Duration) error {
	day := now.Add(-24 * time.Hour)
	for _, sql := range []struct {
		q   string
		arg time.Time
	}{
		{`DELETE FROM rate_events WHERE at <= $1`, now.Add(-window)},
		{`DELETE FROM phone_challenges WHERE coalesce(ended_at, expires_at) <= $1`, day},
		{`DELETE FROM phone_attempts WHERE window_ends_at <= $1`, day},
	} {
		if _, err := q.Exec(ctx, sql.q, sql.arg); err != nil {
			return err
		}
	}
	return nil
}

// The profile's writes. Each moves updated_at on.

func setProfile(ctx context.Context, q db.Querier, id uuid.UUID, name, email string, verified bool, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET full_name = $2, email = $3, email_verified = $4, updated_at = $5 WHERE id = $1`,
		id, name, email, verified, now)
	return err
}

func setAddress(ctx context.Context, q db.Querier, id uuid.UUID, address []byte, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET address = $2, updated_at = $3 WHERE id = $1`, id, address, now)
	return err
}

func setVerifiedPhone(ctx context.Context, q db.Querier, id uuid.UUID, phone string, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET phone = $2, phone_verified = true, updated_at = $3 WHERE id = $1`, id, phone, now)
	return err
}

func setPublicProfile(ctx context.Context, q db.Querier, id uuid.UUID, show bool, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET public_profile = $2, updated_at = $3 WHERE id = $1`, id, show, now)
	return err
}

func setOnboardingCompleted(ctx context.Context, q db.Querier, id uuid.UUID, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET onboarding_completed = true, updated_at = $2 WHERE id = $1`, id, now)
	return err
}
