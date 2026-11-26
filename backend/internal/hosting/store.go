package hosting

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// The package's store: the only queries on host_questions,
// host_applications and host_answers.

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

func insertApplication(ctx context.Context, q db.Querier, a Application, tokenHash []byte) error {
	if _, err := q.Exec(ctx, `
		INSERT INTO host_applications (id, institution_name, contact_name, contact_role, contact_email, contact_phone,
			place_name, country, ecosystems, learners, study_goals, token_hash, status, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		a.ID, a.InstitutionName, a.ContactName, a.ContactRole, a.ContactEmail, a.ContactPhone,
		a.Site.PlaceName, a.Site.Country, a.Site.Ecosystems, a.Site.Learners, a.Site.StudyGoals, tokenHash, a.Status, a.SubmittedAt); err != nil {
		return err
	}
	for id, answer := range a.Answers {
		if _, err := q.Exec(ctx, `INSERT INTO host_answers (application_id, question_id, answer) VALUES ($1, $2, $3)`, a.ID, id, answer); err != nil {
			return err
		}
	}
	return nil
}

const applicationColumns = `id, institution_name, contact_name, contact_role, contact_email, contact_phone,
	place_name, country, ecosystems, learners, study_goals, status, submitted_at, decided_at, classroom_id,
	coalesce((SELECT jsonb_object_agg(question_id, answer) FROM host_answers WHERE application_id = host_applications.id), '{}')`

func scanApplication(r pgx.Row) (Application, error) {
	var a Application
	err := r.Scan(&a.ID, &a.InstitutionName, &a.ContactName, &a.ContactRole, &a.ContactEmail, &a.ContactPhone,
		&a.Site.PlaceName, &a.Site.Country, &a.Site.Ecosystems, &a.Site.Learners, &a.Site.StudyGoals,
		&a.Status, &a.SubmittedAt, &a.DecidedAt, &a.ClassroomID, &a.Answers)
	return a, err
}

func oneApplication(ctx context.Context, q db.Querier, where string, arg any) (Application, error) {
	a, err := scanApplication(q.QueryRow(ctx, `SELECT `+applicationColumns+` FROM host_applications WHERE `+where, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNoApplication
	}
	return a, err
}

func lockApplication(ctx context.Context, q db.Querier, id uuid.UUID) (Application, error) {
	return oneApplication(ctx, q, `id = $1 FOR UPDATE OF host_applications`, id)
}

func applications(ctx context.Context, q db.Querier, where string) ([]Application, error) {
	rows, err := q.Query(ctx, `SELECT `+applicationColumns+` FROM host_applications WHERE `+where)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Application, error) { return scanApplication(r) })
}
