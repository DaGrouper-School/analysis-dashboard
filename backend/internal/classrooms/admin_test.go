package classrooms

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestPutStage: an admin adds a stage at a place in the list and changes
// one; every row whose place changes, and the one changed, is the admin's,
// which the seed keeps; each change is logged.
func TestPutStage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := SeedLists(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	ids := func() string {
		list, err := Stages(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(list))
		for i, s := range list {
			out[i] = s.ID
		}
		return strings.Join(out, " ")
	}
	added, err := PutStage(ctx, pool, now, settings.Operator, "planning", []byte(`{"label": "Planning", "description": "Finding a place.", "color": "#a77bf3", "position": 0}`))
	if err != nil || !added || ids() != "planning preparing pilot operating" {
		t.Fatalf("a stage added first: %v %v %q", added, err, ids())
	}
	if added, err = PutStage(ctx, pool, now, settings.Operator, "operating", []byte(`{"label": "Running", "description": "d", "color": "#3cc04b", "position": 1}`)); err != nil || added ||
		ids() != "planning operating preparing pilot" {
		t.Fatalf("a stage changed and moved: %v %v %q", added, err, ids())
	}
	seeded, err := SeedLists(ctx, pool, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range seeded {
		if strings.HasPrefix(s.What, "stage") && s.Outcome != settings.Kept {
			t.Errorf("after an admin's reordering, the seed %s %s", s.Outcome, s.What)
		}
	}
	var invalid *jsoncheck.Invalid
	_, err = PutStage(ctx, pool, now, settings.Operator, "Bad ID", []byte(`{"label": " ", "description": "d", "color": "blue", "position": -1}`))
	if !errors.As(err, &invalid) {
		t.Fatalf("a bad stage: %v", err)
	}
	f := invalid.Fields()
	if f["id"] == "" || f["position"] == "" || f["color"] == "" || f["label"] != "blank" {
		t.Errorf("fields: %v", f)
	}
	log, err := settings.Changes(ctx, pool, 3)
	if err != nil || log[0].Summary != "Changed stage operating" || log[1].Summary != "Added stage planning" {
		t.Fatalf("the log: %+v, %v", log, err)
	}
}

// TestDeleteStage: a stage nothing refers to is deleted and logged; one a
// row refers to is in use; one that is not there is not found. Classrooms
// refer to stages through a foreign key; a table stands in for them here.
func TestDeleteStage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := SeedLists(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE refers (stage_id text REFERENCES stages, region_id text REFERENCES regions);
		INSERT INTO refers VALUES ('pilot', 'caribbean')`); err != nil {
		t.Fatal(err)
	}
	if err := DeleteStage(ctx, pool, now, settings.Operator, "pilot"); !errors.Is(err, ErrInUse) {
		t.Errorf("a stage in use: %v", err)
	}
	if err := DeleteRegion(ctx, pool, now, settings.Operator, "caribbean"); !errors.Is(err, ErrInUse) {
		t.Errorf("a region in use: %v", err)
	}
	if err := DeleteStage(ctx, pool, now, settings.Operator, "preparing"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteStage(ctx, pool, now, settings.Operator, "preparing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted again: %v", err)
	}
	if list, _ := Stages(ctx, pool); len(list) != 2 {
		t.Errorf("stages: %+v", list)
	}
	if added, err := PutRegion(ctx, pool, now, settings.Operator, "south_asia", []byte(`{"label": "South Asia", "position": 9}`)); err != nil || !added {
		t.Fatalf("a region added at the end: %v", err)
	}
	if list, _ := Regions(ctx, pool); list[len(list)-1].ID != "south_asia" {
		t.Errorf("regions: %+v", list)
	}
	log, _ := settings.Changes(ctx, pool, 2)
	if log[0].Summary != "Added region south_asia" || log[1].Summary != "Deleted stage preparing" {
		t.Errorf("the log: %+v", log)
	}
}
