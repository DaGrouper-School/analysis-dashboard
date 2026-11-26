package labs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// Credit is who asked to be credited for the readings this service
// relayed: the only link between a released value and a person, matched
// to what SAM lets the public read by reading ID.

// CreditedIn is, for each person credited for a reading relayed into the
// classroom's labs, the readings they are credited for, newest first.
func CreditedIn(ctx context.Context, q db.Querier, classroom uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := q.Query(ctx, `
		SELECT r.credit_user_id, r.reading_id FROM relayed_records r JOIN labs l ON l.id = r.lab_id
		WHERE l.classroom_id = $1 AND r.credit_user_id IS NOT NULL ORDER BY r.received_at DESC, r.reading_id`, classroom)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]uuid.UUID{}
	var user, reading uuid.UUID
	_, err = pgx.ForEachRow(rows, []any{&user, &reading}, func() error {
		out[user] = append(out[user], reading)
		return nil
	})
	return out, err
}

// CreditedTo is, for each classroom, the readings user is credited for,
// newest first.
func CreditedTo(ctx context.Context, q db.Querier, user uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := q.Query(ctx, `
		SELECT l.classroom_id, r.reading_id FROM relayed_records r JOIN labs l ON l.id = r.lab_id
		WHERE r.credit_user_id = $1 ORDER BY r.received_at DESC, r.reading_id`, user)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]uuid.UUID{}
	var classroom, reading uuid.UUID
	_, err = pgx.ForEachRow(rows, []any{&classroom, &reading}, func() error {
		out[classroom] = append(out[classroom], reading)
		return nil
	})
	return out, err
}
