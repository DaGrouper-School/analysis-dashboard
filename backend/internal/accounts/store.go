package accounts

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The module's store: the only queries on users and the participant ID
// sequence. Each works on the pool or inside a transaction.

// insertUser makes a user, with the next participant ID.
func insertUser(ctx context.Context, q db.Querier, id uuid.UUID, now time.Time) (User, error) {
	u := User{ID: id}
	err := q.QueryRow(ctx, `INSERT INTO users (id, created_at) VALUES ($1, $2) RETURNING participant_id`, id, now).
		Scan(&u.ParticipantID)
	return u, err
}

func getUser(ctx context.Context, q db.Querier, id uuid.UUID) (User, error) {
	u := User{ID: id}
	err := q.QueryRow(ctx, `SELECT participant_id, sam_principal_id FROM users WHERE id = $1`, id).Scan(&u.ParticipantID, &u.Principal)
	return u, err
}

// newParticipantID gives the user the next participant ID.
func newParticipantID(ctx context.Context, q db.Querier, id uuid.UUID) (int64, error) {
	var n int64
	err := q.QueryRow(ctx, `UPDATE users SET participant_id = nextval('participant_ids') WHERE id = $1 RETURNING participant_id`, id).
		Scan(&n)
	return n, err
}

// setPrincipal records the user's SAM principal, registered under
// participant ID n, unless they have been renumbered since.
func setPrincipal(ctx context.Context, q db.Querier, id, principal uuid.UUID, n int64) error {
	_, err := q.Exec(ctx, `UPDATE users SET sam_principal_id = $2 WHERE id = $1 AND participant_id = $3`, id, principal, n)
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
