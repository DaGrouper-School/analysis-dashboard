package classrooms

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The package's store: the only queries on stages and regions.

func stages(ctx context.Context, q db.Querier) ([]Stage, error) {
	rows, err := q.Query(ctx, `SELECT id, label, description, color FROM stages ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Stage])
}

func regions(ctx context.Context, q db.Querier) ([]Region, error) {
	rows, err := q.Query(ctx, `SELECT id, label FROM regions ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Region])
}

func countStages(ctx context.Context, q db.Querier) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM stages`).Scan(&n)
	return n, err
}

func countRegions(ctx context.Context, q db.Querier) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM regions`).Scan(&n)
	return n, err
}

func stage(ctx context.Context, q db.Querier, id string) (s Stage, position int, seeded, found bool, err error) {
	err = q.QueryRow(ctx, `SELECT id, label, description, color, position, seeded FROM stages WHERE id = $1 FOR UPDATE`, id).
		Scan(&s.ID, &s.Label, &s.Description, &s.Color, &position, &seeded)
	if errors.Is(err, pgx.ErrNoRows) {
		return Stage{}, 0, false, false, nil
	}
	return s, position, seeded, err == nil, err
}

func region(ctx context.Context, q db.Querier, id string) (r Region, position int, seeded, found bool, err error) {
	err = q.QueryRow(ctx, `SELECT id, label, position, seeded FROM regions WHERE id = $1 FOR UPDATE`, id).
		Scan(&r.ID, &r.Label, &position, &seeded)
	if errors.Is(err, pgx.ErrNoRows) {
		return Region{}, 0, false, false, nil
	}
	return r, position, seeded, err == nil, err
}

func insertStage(ctx context.Context, q db.Querier, now time.Time, s Stage, position int) error {
	_, err := q.Exec(ctx, `
		INSERT INTO stages (id, label, description, color, position, seeded, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, $6)`, s.ID, s.Label, s.Description, s.Color, position, now)
	return err
}

func updateStage(ctx context.Context, q db.Querier, now time.Time, s Stage, position int) error {
	_, err := q.Exec(ctx, `
		UPDATE stages SET label = $2, description = $3, color = $4, position = $5, updated_at = $6
		WHERE id = $1 AND seeded`, s.ID, s.Label, s.Description, s.Color, position, now)
	return err
}

func insertRegion(ctx context.Context, q db.Querier, now time.Time, r Region, position int) error {
	_, err := q.Exec(ctx, `INSERT INTO regions (id, label, position, seeded, updated_at) VALUES ($1, $2, $3, true, $4)`,
		r.ID, r.Label, position, now)
	return err
}

func updateRegion(ctx context.Context, q db.Querier, now time.Time, r Region, position int) error {
	_, err := q.Exec(ctx, `UPDATE regions SET label = $2, position = $3, updated_at = $4 WHERE id = $1 AND seeded`,
		r.ID, r.Label, position, now)
	return err
}
