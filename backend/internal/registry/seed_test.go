package registry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestSeedFile: the built-in seed passes its checks.
func TestSeedFile(t *testing.T) {
	t.Parallel()
	o, err := seedOverlay()
	if err != nil || len(o.Domains) == 0 || len(o.Parameters) == 0 || len(o.DeviceTypes) == 0 || len(o.Templates) == 0 {
		t.Fatalf("%v", err)
	}
}

func TestCheckOverlay(t *testing.T) {
	t.Parallel()
	got := CheckOverlay(Overlay{
		Domains: []DomainOverlay{{ID: "AIR", Label: "Air", Icon: "tree", Description: "d"}, {ID: "AIR", Label: "Again", Icon: "x", Description: "d"}},
		Parameters: []ParameterOverlay{
			{ID: "dgs:soil_ec", Label: "Soil EC", Source: SourceDevice},
			{ID: "bad id", Label: " ", Source: "sensor", Noise: ptr(-1.0), Chart: ptr("pie")},
		},
		DeviceTypes: []DeviceTypeOverlay{{ID: "probe", Label: "Probe", Description: "d", Color: "blue", Icon: "sensor", Tiles: []string{}, Cards: []string{}}},
	})
	want := []string{
		`domains[1]: "AIR" repeats domains[0]`,
		`parameters[1]: "bad id" is not an ID`,
		`parameters[1].source: "sensor" is not weather_station, device or manual`,
		`parameters[1].chart: "pie" is not daily_bars, line_band or line`,
		"parameters[1].noise: -1 is below zero",
		`deviceTypes[0].color: "blue" is not a color`,
		"parameters[1].label: blank",
	}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("problem %d: %q lacks %q", i, got[i], w)
		}
	}
}

// TestSeedOverlay: the seed fills the overlay and the default alert
// levels, a second run changes
// nothing, and a row an admin changed is kept; what is stored reads back
// as the seed wrote it.
func TestSeedOverlay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	all, _ := readSeed()
	seed := all.Overlay
	count := func(seeded []settings.Seeded, o settings.Outcome) int {
		n := 0
		for _, s := range seeded {
			if s.Outcome == o {
				n++
			}
		}
		return n
	}
	total := len(seed.Domains) + len(seed.Parameters) + len(seed.DeviceTypes) + len(seed.Templates) + len(all.AlertDefaults)
	seeded, err := SeedOverlay(ctx, pool, now)
	if err != nil || count(seeded, settings.Added) != total || seeded[0].What != KindDomain+" AIR" {
		t.Fatalf("the seed: %v", err)
	}
	if seeded, err = SeedOverlay(ctx, pool, now); err != nil || count(seeded, settings.Unchanged) != total {
		t.Fatalf("the seed again: %v", err)
	}
	stored, err := LoadOverlay(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Parameters) != len(seed.Parameters) || stored.Parameters[0].ID != seed.Parameters[0].ID ||
		len(stored.DeviceTypes[0].Tiles) != len(seed.DeviceTypes[0].Tiles) || *stored.Templates[0].DeviceType != *seed.Templates[0].DeviceType {
		t.Fatalf("read back: %+v", stored)
	}

	if _, err := pool.Exec(ctx, `UPDATE registry_device_types SET label = 'Station', seeded = false WHERE id = 'weather_station'`); err != nil {
		t.Fatal(err)
	}
	seeded, err = SeedOverlay(ctx, pool, now.Add(time.Hour))
	if err != nil || count(seeded, settings.Kept) != 1 {
		t.Fatalf("the seed after an admin's change: %v", err)
	}
	if stored, _ = LoadOverlay(ctx, pool); stored.DeviceTypes[0].Label != "Station" {
		t.Fatalf("an admin's change was not kept: %+v", stored.DeviceTypes[0])
	}
	var logged int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM configuration_changes`).Scan(&logged); err != nil || logged != 0 {
		t.Fatalf("the overlay's seed was logged with the configuration: %d, %v", logged, err)
	}
}
