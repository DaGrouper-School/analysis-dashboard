package accounts

import (
	"context"
	"errors"
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
