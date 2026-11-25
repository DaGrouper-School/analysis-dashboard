package hosting

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The package's store: the only queries on host_questions.

func questions(ctx context.Context, q db.Querier) ([]Question, error) {
	rows, err := q.Query(ctx, `
		SELECT id, label, help, kind, required FROM host_questions WHERE retired_at IS NULL ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Question])
}

func countQuestions(ctx context.Context, q db.Querier) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM host_questions`).Scan(&n)
	return n, err
}

// question is the question with id; a retired one counts as someone
// else's change, which the seed keeps.
func question(ctx context.Context, q db.Querier, id string) (qn Question, position int, seeded, found bool, err error) {
	err = q.QueryRow(ctx, `
		SELECT id, label, help, kind, required, position, seeded AND retired_at IS NULL
		FROM host_questions WHERE id = $1 FOR UPDATE`, id).
		Scan(&qn.ID, &qn.Label, &qn.Help, &qn.Kind, &qn.Required, &position, &seeded)
	if errors.Is(err, pgx.ErrNoRows) {
		return Question{}, 0, false, false, nil
	}
	return qn, position, seeded, err == nil, err
}

func insertQuestion(ctx context.Context, q db.Querier, now time.Time, qn Question, position int) error {
	_, err := q.Exec(ctx, `
		INSERT INTO host_questions (id, label, help, kind, required, position, seeded, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7)`, qn.ID, qn.Label, qn.Help, qn.Kind, qn.Required, position, now)
	return err
}

func updateQuestion(ctx context.Context, q db.Querier, now time.Time, qn Question, position int) error {
	_, err := q.Exec(ctx, `
		UPDATE host_questions SET label = $2, help = $3, kind = $4, required = $5, position = $6, updated_at = $7
		WHERE id = $1 AND seeded AND retired_at IS NULL`, qn.ID, qn.Label, qn.Help, qn.Kind, qn.Required, position, now)
	return err
}
