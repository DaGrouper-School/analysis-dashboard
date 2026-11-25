package settings

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// seedFile is the settings' seed: a value for every key, the same as the
// front end's seed (frontend/src/lib/data/seed.ts). It is built into the
// binary, so dgs seed needs no file beside it.
//
//go:embed seed.json
var seedFile []byte

// Outcome is what the seed did with one setting or row.
type Outcome string

// The outcomes of seeding.
const (
	// Added: it was not there.
	Added Outcome = "added"
	// Updated: the seed wrote it last, and the seed has changed since.
	Updated Outcome = "updated"
	// Unchanged: it is already the seed's.
	Unchanged Outcome = "unchanged"
	// Kept: someone else changed it since, and the seed leaves it alone.
	Kept Outcome = "kept"
	// LeftOut: a row the list does not have, which the seed adds only to
	// an empty list, so a row an admin deleted stays deleted.
	LeftOut Outcome = "left out"
)

// Seeded is one setting or row and what the seed did with it.
type Seeded struct {
	// What names it, such as privacy or stage pilot.
	What    string
	Outcome Outcome
}

// SeedSettings stores the seed's value for each setting, in the order of
// Keys: added where missing, updated where the seed wrote it last, and
// kept where someone else has written it since. Each addition and update
// is logged as the seed's. A second run changes nothing.
func SeedSettings(ctx context.Context, q db.Querier, now time.Time) ([]Seeded, error) {
	values, err := seedValues()
	if err != nil {
		return nil, err
	}
	out := make([]Seeded, 0, len(keys))
	for _, k := range keys {
		outcome, err := putSeed(ctx, q, now, k.Name, values[k.Name])
		if err != nil {
			return out, fmt.Errorf("seeding %s: %w", k.Name, err)
		}
		if err := RecordSeed(ctx, q, now, KindSetting, k.Name, outcome); err != nil {
			return out, err
		}
		out = append(out, Seeded{What: k.Name, Outcome: outcome})
	}
	return out, nil
}

// RecordSeed logs what the seed did with kind's subject, when it added or
// updated it.
func RecordSeed(ctx context.Context, q db.Querier, now time.Time, kind, subject string, outcome Outcome) error {
	if outcome != Added && outcome != Updated {
		return nil
	}
	return Record(ctx, q, now, Seed, Change{Kind: kind, Subject: subject, Summary: string(outcome) + " by the seed"})
}

// seedValues are the seed's values, each checked, for every key.
func seedValues() (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(seedFile, &raw); err != nil {
		return nil, fmt.Errorf("the settings' seed: %w", err)
	}
	if len(raw) != len(keys) {
		return nil, fmt.Errorf("the settings' seed has %d keys; there are %d settings", len(raw), len(keys))
	}
	values := map[string]json.RawMessage{}
	for _, k := range keys {
		v, ok := raw[k.Name]
		if !ok {
			return nil, fmt.Errorf("the settings' seed has no %s", k.Name)
		}
		checked, err := Check(k.Name, v)
		if err != nil {
			return nil, fmt.Errorf("the settings' seed: %w", err)
		}
		values[k.Name] = checked
	}
	return values, nil
}

// List is how SeedList reaches a list's rows. Its functions are the store
// functions of the module that owns the list.
type List[T comparable] struct {
	// Kind is what a row is, for the change log, such as stage.
	Kind string
	// ID is a row's ID.
	ID func(T) string
	// Count is how many rows the list has.
	Count func(ctx context.Context, q db.Querier) (int, error)
	// Current is the row with id as stored, its position, and whether the
	// seed wrote it last; found is false when there is none.
	Current func(ctx context.Context, q db.Querier, id string) (row T, position int, seeded, found bool, err error)
	// Insert adds row at position as the seed's; Update replaces the
	// seed's row with row at position.
	Insert func(ctx context.Context, q db.Querier, now time.Time, row T, position int) error
	Update func(ctx context.Context, q db.Querier, now time.Time, row T, position int) error
}

// SeedList brings a list in line with the seed's rows, in order: every
// row is added while the list is empty; otherwise a row the seed wrote
// last is updated, a row someone else changed is kept, and a row the list
// lacks is left out, so a row an admin deleted stays deleted. Each
// addition and update is logged as the seed's. The caller holds the seed
// lock, so two seeds never both find the list empty.
func SeedList[T comparable](ctx context.Context, q db.Querier, now time.Time, l List[T], rows []T) ([]Seeded, error) {
	n, err := l.Count(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]Seeded, 0, len(rows))
	for position, row := range rows {
		id := l.ID(row)
		outcome, err := seedRow(ctx, q, now, l, n == 0, row, position)
		if err != nil {
			return out, fmt.Errorf("seeding %s %s: %w", l.Kind, id, err)
		}
		if err := RecordSeed(ctx, q, now, l.Kind, id, outcome); err != nil {
			return out, err
		}
		out = append(out, Seeded{What: l.Kind + " " + id, Outcome: outcome})
	}
	return out, nil
}

func seedRow[T comparable](ctx context.Context, q db.Querier, now time.Time, l List[T], empty bool, row T, position int) (Outcome, error) {
	if empty {
		return Added, l.Insert(ctx, q, now, row, position)
	}
	current, at, seeded, found, err := l.Current(ctx, q, l.ID(row))
	switch {
	case err != nil:
		return "", err
	case !found:
		return LeftOut, nil
	case !seeded:
		return Kept, nil
	case current == row && at == position:
		return Unchanged, nil
	}
	return Updated, l.Update(ctx, q, now, row, position)
}
