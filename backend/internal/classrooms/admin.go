package classrooms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// ErrNotFound means there is no such stage or region.
var ErrNotFound = errors.New("not found")

// ErrInUse means a classroom refers to the stage or region.
var ErrInUse = errors.New("in use")

// StageInput is what an admin sets of a stage, and where it goes in the
// list, from 0.
type StageInput struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Position    int    `json:"position"`
}

// RegionInput is what an admin sets of a region, and where it goes.
type RegionInput struct {
	Label    string `json:"label"`
	Position int    `json:"position"`
}

// list is one of the package's ordered lists, as admins change it.
type list struct {
	table, kind string
}

var (
	stageTable  = list{table: "stages", kind: settings.KindStage}
	regionTable = list{table: "regions", kind: settings.KindRegion}
)

// checkPlaced reports what is wrong with an ID and a position.
func checkPlaced(id string, position int) []string {
	var p []string
	if !jsoncheck.Slug.MatchString(id) {
		p = append(p, fmt.Sprintf("id: %q is not an ID of lower-case letters, digits, - and _", id))
	}
	if position < 0 {
		p = append(p, fmt.Sprintf("position: %d is below zero", position))
	}
	return p
}

// PutStage adds the stage id or changes it, from raw, placed at its
// position, and logs it as by's. It reports whether it was added.
func PutStage(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string, raw []byte) (bool, error) {
	in, problems := jsoncheck.Decode[StageInput](raw)
	if len(problems) == 0 {
		problems = checkPlaced(id, in.Position)
		if !hexColor.MatchString(in.Color) {
			problems = append(problems, fmt.Sprintf("color: %q is not a color such as #f6921e", in.Color))
		}
		problems = append(problems, jsoncheck.Blank(in, "")...)
	}
	if err := jsoncheck.Refuse("the stage", problems); err != nil {
		return false, err
	}
	return stageTable.put(ctx, q, now, by, id, in.Position, `
		INSERT INTO stages (id, label, description, color, position, seeded, updated_at) VALUES ($1, $2, $3, $4, $5, false, $6)
		ON CONFLICT (id) DO UPDATE SET label = $2, description = $3, color = $4, seeded = false, updated_at = $6
		RETURNING xmax = 0`, in.Label, in.Description, in.Color)
}

// PutRegion adds the region id or changes it, as PutStage does.
func PutRegion(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string, raw []byte) (bool, error) {
	in, problems := jsoncheck.Decode[RegionInput](raw)
	if len(problems) == 0 {
		problems = append(checkPlaced(id, in.Position), jsoncheck.Blank(in, "")...)
	}
	if err := jsoncheck.Refuse("the region", problems); err != nil {
		return false, err
	}
	return regionTable.put(ctx, q, now, by, id, in.Position, `
		INSERT INTO regions (id, label, position, seeded, updated_at) VALUES ($1, $2, $3, false, $4)
		ON CONFLICT (id) DO UPDATE SET label = $2, seeded = false, updated_at = $4
		RETURNING xmax = 0`, in.Label)
}

// DeleteStage deletes a stage no classroom refers to, and logs it.
func DeleteStage(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string) error {
	return stageTable.delete(ctx, q, now, by, id)
}

// DeleteRegion deletes a region no classroom refers to, and logs it.
func DeleteRegion(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string) error {
	return regionTable.delete(ctx, q, now, by, id)
}

// put writes a row with upsert, whose arguments are id, fields, then the
// position and the time, and moves it to position: every other row whose
// place changes is an admin's change too, which the seed keeps.
func (l list) put(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string, position int, upsert string, fields ...any) (bool, error) {
	var added bool
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM `+l.table+` ORDER BY position, id FOR UPDATE`)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		order := settings.Place(ids, id, position)
		args := append(append([]any{id}, fields...), len(ids), now)
		if err := tx.QueryRow(ctx, upsert, args...).Scan(&added); err != nil {
			return err
		}
		for i, x := range order {
			if _, err := tx.Exec(ctx, `UPDATE `+l.table+` SET position = $2, seeded = false, updated_at = $3 WHERE id = $1 AND position <> $2`,
				x, i, now); err != nil {
				return err
			}
		}
		what := "Changed"
		if added {
			what = "Added"
		}
		return settings.Record(ctx, tx, now, by, settings.Change{Kind: l.kind, Subject: id, Summary: fmt.Sprintf("%s %s %s", what, l.kind, id)})
	})
	return added, err
}

func (l list) delete(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, id string) error {
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM `+l.table+` WHERE id = $1`, id)
		var pe *pgconn.PgError
		switch {
		case errors.As(err, &pe) && pe.Code == "23503":
			return ErrInUse
		case err != nil:
			return err
		case tag.RowsAffected() == 0:
			return ErrNotFound
		}
		return settings.Record(ctx, tx, now, by, settings.Change{Kind: l.kind, Subject: id, Summary: fmt.Sprintf("Deleted %s %s", l.kind, id)})
	})
}
