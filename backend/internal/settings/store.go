package settings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The package's store: the only queries on platform_settings and
// configuration_changes.

func get(ctx context.Context, q db.Querier, key string) (Entry, error) {
	e := Entry{Key: key}
	err := q.QueryRow(ctx, `SELECT value, seeded, updated_at FROM platform_settings WHERE key = $1`, key).
		Scan(&e.Value, &e.Seeded, &e.UpdatedAt)
	return e, err
}

func all(ctx context.Context, q db.Querier) (map[string]Entry, error) {
	rows, err := q.Query(ctx, `SELECT key, value, seeded, updated_at FROM platform_settings`)
	if err != nil {
		return nil, err
	}
	out := map[string]Entry{}
	var e Entry
	_, err = pgx.ForEachRow(rows, []any{&e.Key, &e.Value, &e.Seeded, &e.UpdatedAt}, func() error {
		out[e.Key] = Entry{Key: e.Key, Value: append(json.RawMessage(nil), e.Value...), Seeded: e.Seeded, UpdatedAt: e.UpdatedAt}
		return nil
	})
	return out, err
}

// put stores a value written by someone other than the seed, and reports
// whether anything changed: the value, or that the seed wrote it last.
func put(ctx context.Context, q db.Querier, now time.Time, key string, value json.RawMessage) (bool, error) {
	err := q.QueryRow(ctx, `
		INSERT INTO platform_settings (key, value, seeded, updated_at) VALUES ($1, $2::jsonb, false, $3)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, seeded = false, updated_at = excluded.updated_at
		WHERE platform_settings.value <> excluded.value OR platform_settings.seeded
		RETURNING key`, key, string(value), now).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// putSeed stores the seed's value for key unless someone else wrote the
// one there, and says what it did.
func putSeed(ctx context.Context, q db.Querier, now time.Time, key string, value json.RawMessage) (Outcome, error) {
	var seeded bool
	var same bool
	err := q.QueryRow(ctx, `SELECT seeded, value = $2::jsonb FROM platform_settings WHERE key = $1 FOR UPDATE`,
		key, string(value)).Scan(&seeded, &same)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = q.Exec(ctx, `INSERT INTO platform_settings (key, value, seeded, updated_at) VALUES ($1, $2::jsonb, true, $3)`,
			key, string(value), now)
		return Added, err
	case err != nil:
		return "", err
	case !seeded:
		return Kept, nil
	case same:
		return Unchanged, nil
	}
	_, err = q.Exec(ctx, `UPDATE platform_settings SET value = $2::jsonb, updated_at = $3 WHERE key = $1`, key, string(value), now)
	return Updated, err
}

func record(ctx context.Context, q db.Querier, now time.Time, actor Actor, c Change) error {
	_, err := q.Exec(ctx, `
		INSERT INTO configuration_changes (at, actor_id, actor_name, kind, subject, summary)
		VALUES ($1, $2, $3, $4, $5, $6)`, now, actor.UserID, actor.Name, c.Kind, c.Subject, c.Summary)
	return err
}

// changes are the newest changes, before the change with ID before when
// it is not zero.
func changes(ctx context.Context, q db.Querier, before int64, limit int) ([]Logged, error) {
	rows, err := q.Query(ctx, `
		SELECT id, at, actor_id, actor_name, kind, subject, summary FROM configuration_changes
		WHERE $1 = 0 OR id < $1 ORDER BY id DESC LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Logged, error) {
		var l Logged
		err := row.Scan(&l.ID, &l.At, &l.Actor.UserID, &l.Actor.Name, &l.Kind, &l.Subject, &l.Summary)
		return l, err
	})
}
