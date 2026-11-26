package hosting

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// ErrNotFound means there is no such question.
var ErrNotFound = errors.New("not found")

// ErrRetired means the question is retired, so it no longer changes.
var ErrRetired = errors.New("retired")

// KeptQuestion is a question as admins keep it: retired ones included,
// with when they were.
type KeptQuestion struct {
	Question
	RetiredAt *time.Time `json:"retiredAt"`
}

// QuestionInput is what an admin sets of a question, and where it goes in
// the list, from 0.
type QuestionInput struct {
	Label    string `json:"label"`
	Help     string `json:"help"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
	Position int    `json:"position"`
}

// AllQuestions are every question, retired ones included, in order.
func AllQuestions(ctx context.Context, q db.Querier) ([]KeptQuestion, error) {
	rows, err := q.Query(ctx, `SELECT id, label, help, kind, required, retired_at FROM host_questions ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (k KeptQuestion, err error) {
		return k, r.Scan(&k.ID, &k.Label, &k.Help, &k.Kind, &k.Required, &k.RetiredAt)
	})
}

// PutQuestion adds the question id or changes it, from raw, placed at its
// position, and logs it as by's; every other question whose place changes
// is an admin's change too. A retired question no longer changes. It
// reports whether it was added.
func PutQuestion(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string, raw []byte) (bool, error) {
	in, problems := jsoncheck.Decode[QuestionInput](raw)
	if len(problems) == 0 {
		problems = checkInput(id, in)
	}
	if err := jsoncheck.Refuse("the question", problems); err != nil {
		return false, err
	}
	var added bool
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		all, err := AllQuestions(ctx, tx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(all, func(k KeptQuestion) bool { return k.ID == id })
		if i >= 0 && all[i].RetiredAt != nil {
			return ErrRetired
		}
		added = i < 0
		ids := make([]string, len(all))
		for j, k := range all {
			ids[j] = k.ID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO host_questions (id, label, help, kind, required, position, seeded, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, false, $7)
			ON CONFLICT (id) DO UPDATE SET label = $2, help = $3, kind = $4, required = $5, seeded = false, updated_at = $7`,
			id, in.Label, in.Help, in.Kind, in.Required, len(ids), now); err != nil {
			return err
		}
		for j, x := range settings.Place(ids, id, in.Position) {
			if _, err := tx.Exec(ctx, `UPDATE host_questions SET position = $2, seeded = false, updated_at = $3 WHERE id = $1 AND position <> $2`,
				x, j, now); err != nil {
				return err
			}
		}
		what := "Changed"
		if added {
			what = "Added"
		}
		return settings.Record(ctx, tx, now, by, settings.Change{Kind: settings.KindHostQuestion, Subject: id,
			Summary: fmt.Sprintf("%s host question %s", what, id)})
	})
	return added, err
}

// RetireQuestion retires a question: applicants are no longer asked it,
// and earlier applications keep their answers. Retiring one twice changes
// nothing.
func RetireQuestion(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string) error {
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		var retired *time.Time
		err := tx.QueryRow(ctx, `SELECT retired_at FROM host_questions WHERE id = $1 FOR UPDATE`, id).Scan(&retired)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrNotFound
		case err != nil || retired != nil:
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE host_questions SET retired_at = $2, seeded = false, updated_at = $2 WHERE id = $1`, id, now); err != nil {
			return err
		}
		return settings.Record(ctx, tx, now, by, settings.Change{Kind: settings.KindHostQuestion, Subject: id,
			Summary: "Retired host question " + id})
	})
}

// checkInput reports what is wrong with a question an admin sets, by
// field, by the rules CheckQuestions holds a list to.
func checkInput(id string, in QuestionInput) []string {
	var p []string
	if !questionID.MatchString(id) {
		p = append(p, fmt.Sprintf("id: %q is not an ID of lower-case letters, digits and _", id))
	}
	switch in.Kind {
	case KindYesNo, KindNumber, KindText:
	default:
		p = append(p, fmt.Sprintf("kind: %q is not yes_no, number or text", in.Kind))
	}
	if in.Position < 0 {
		p = append(p, fmt.Sprintf("position: %d is below zero", in.Position))
	}
	return append(p, jsoncheck.Blank(in, "", "help")...)
}
