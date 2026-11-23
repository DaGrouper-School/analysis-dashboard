package accounts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The module's store: the only queries on users and the participant ID
// sequence. Each works on the pool or inside a transaction.

const userColumns = `id, participant_id, sam_principal_id, minor, consent, sam_minor, sam_consent`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.ParticipantID, &u.Principal, &u.Minor, &u.Consent, &u.samMinor, &u.samConsent)
	return u, err
}

// insertUser makes a user, with the next participant ID.
func insertUser(ctx context.Context, q db.Querier, id uuid.UUID, now time.Time) (User, error) {
	return scanUser(q.QueryRow(ctx, `INSERT INTO users (id, created_at) VALUES ($1, $2) RETURNING `+userColumns, id, now))
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
