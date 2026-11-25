package classrooms

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestSeedFile: the built-in seed passes its checks.
func TestSeedFile(t *testing.T) {
	t.Parallel()
	s, err := seedLists()
	if err != nil || len(s.Stages) == 0 || len(s.Regions) == 0 {
		t.Fatalf("%+v, %v", s, err)
	}
}

func TestCheckStages(t *testing.T) {
	t.Parallel()
	got := CheckStages("stages", []Stage{
		{ID: "pilot", Label: "Pilot", Description: "d", Color: "#00bdff"},
		{ID: "Pilot 2", Label: " ", Description: "d", Color: "blue"},
		{ID: "pilot", Label: "Again", Description: "d", Color: "#abc"},
	})
	want := []string{`stages[1]: "Pilot 2" is not an ID`, "stages[2]: \"pilot\" repeats stages[0]", `stages[1].color: "blue" is not a color`, "stages[1].label: blank"}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("problem %d: %q lacks %q", i, got[i], w)
		}
	}
	if got := CheckRegions("regions", []Region{{ID: "east_africa", Label: ""}}); !slices.Equal(got, []string{"regions[0].label: blank"}) {
		t.Errorf("regions: %q", got)
	}
}

// TestSeedLists: an empty list gets every row; a second run changes
// nothing; a row the seed wrote is updated to the seed's, one an admin
// changed is kept, and one an admin deleted stays deleted.
func TestSeedLists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	count := func(seeded []settings.Seeded, o settings.Outcome) int {
		n := 0
		for _, s := range seeded {
			if s.Outcome == o {
				n++
			}
		}
		return n
	}
	seeded, err := SeedLists(ctx, pool, now)
	if err != nil || count(seeded, settings.Added) != 5 || seeded[0].What != "stage preparing" {
		t.Fatalf("the seed: %+v, %v", seeded, err)
	}
	if seeded, err = SeedLists(ctx, pool, now); err != nil || count(seeded, settings.Unchanged) != 5 {
		t.Fatalf("the seed again: %+v, %v", seeded, err)
	}
	got, err := Stages(ctx, pool)
	if err != nil || len(got) != 3 || got[0].ID != "preparing" || got[2].ID != "operating" {
		t.Fatalf("stages: %+v, %v", got, err)
	}
	regions, err := Regions(ctx, pool)
	if err != nil || len(regions) != 2 || regions[1] != (Region{ID: "east_africa", Label: "East Africa"}) {
		t.Fatalf("regions: %+v, %v", regions, err)
	}

	// An older release's seed wrote pilot; an admin changed operating and
	// deleted the Caribbean.
	for _, sql := range []string{
		`UPDATE stages SET label = 'Old pilot' WHERE id = 'pilot'`,
		`UPDATE stages SET label = 'Running', seeded = false WHERE id = 'operating'`,
		`DELETE FROM regions WHERE id = 'caribbean'`,
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	seeded, err = SeedLists(ctx, pool, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	outcome := map[string]settings.Outcome{}
	for _, s := range seeded {
		outcome[s.What] = s.Outcome
	}
	if outcome["stage pilot"] != settings.Updated || outcome["stage operating"] != settings.Kept ||
		outcome["region caribbean"] != settings.LeftOut || outcome["stage preparing"] != settings.Unchanged {
		t.Fatalf("outcomes: %v", outcome)
	}
	got, _ = Stages(ctx, pool)
	if got[1].Label != "Pilot" || got[2].Label != "Running" {
		t.Fatalf("stages after: %+v", got)
	}
	if regions, _ = Regions(ctx, pool); len(regions) != 1 {
		t.Fatalf("regions after: %+v", regions)
	}
	log, err := settings.Changes(ctx, pool, 20)
	if err != nil || len(log) != 6 || log[0].Subject != "pilot" || log[0].Summary != "updated by the seed" {
		t.Fatalf("the log: %+v, %v", log, err)
	}
}
