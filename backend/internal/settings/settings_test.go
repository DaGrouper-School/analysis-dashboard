package settings_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

// TestCheck: each setting refuses a value that breaks its shape or rules,
// naming every problem, and stores a good one in one form.
func TestCheck(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		key, raw string
		want     []string
	}{
		{settings.KeyPrivacy, `{"minorCoordinateDecimals": 7, "extra": true}`,
			[]string{"extra: not a field here"}},
		{settings.KeyPrivacy, `{"minorCoordinateDecimals": 7}`, []string{"minorCoordinateDecimals: 7 is not from 0 to 4"}},
		{settings.KeyCoreParameters, `{"rainDaily": [], "airTemperature": ["Air Temp"], "rainTolerance": -1, "rainReference": "x"}`,
			[]string{"rainDaily: empty", "airTemperature[0]: \"Air Temp\" is not an ID", `rainReference: "x" is not one of rainDaily`, "rainTolerance: -1 is below zero"}},
		{settings.KeyDefaultTerms, `{"licence": " ", "licenceUrl": "http://x", "attribution": "{school}", "restrictions": [""], "coordinateDecimals": 5}`,
			[]string{`licenceUrl: "http://x" is not an https URL`, "coordinateDecimals: 5 is not from 0 to 4", "{school} is not filled in", "licence: blank", "restrictions[0]: blank"}},
		{settings.KeyNowTiles, `{"tiles": [{"id": "a", "label": "A", "icon": "sun", "parameterIds": [], "caption": {"parameterIds": ["uv"], "text": "UV", "format": "dial"}},
			{"id": "a", "label": "B", "icon": "sun", "parameterIds": ["uv"], "core": {"value": "uv"}}]}`,
			[]string{"tiles[0].parameterIds: name at least one value", "tiles[0].caption.text: holds {value}", "tiles[0].caption.format: compass, or none",
				"tiles[1].id: used by another tile", "tiles[1].core.value: rainToday or airTemperature"}},
	} {
		_, err := settings.Check(c.key, []byte(c.raw))
		var invalid *jsoncheck.Invalid
		if !errors.As(err, &invalid) || len(invalid.Problems) != len(c.want) {
			t.Errorf("%s %s: %v", c.key, c.raw, err)
			continue
		}
		for i, w := range c.want {
			if !strings.Contains(invalid.Problems[i], w) {
				t.Errorf("%s problem %d: %q lacks %q", c.key, i, invalid.Problems[i], w)
			}
		}
	}
	got, err := settings.Check(settings.KeyPrivacy, []byte(" { \"minorCoordinateDecimals\" : 3 } "))
	if err != nil || string(got) != `{"minorCoordinateDecimals":3}` {
		t.Fatalf("a good value: %s, %v", got, err)
	}
	if _, err := settings.Check("theme", []byte(`{}`)); !errors.Is(err, settings.ErrUnknownKey) {
		t.Fatalf("an unknown key: %v", err)
	}
}

// TestPutAndSeed: the seed adds every setting and a second run changes
// nothing; a value put since is kept by the next seed; every change is
// logged, newest first, with who made it.
func TestPutAndSeed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)

	if _, err := settings.Get(ctx, pool, settings.KeyPrivacy); !errors.Is(err, settings.ErrNotSet) {
		t.Fatalf("before the seed: %v", err)
	}
	outcomes := func(seeded []settings.Seeded) string {
		var b strings.Builder
		for _, s := range seeded {
			b.WriteString(s.What + "=" + string(s.Outcome) + " ")
		}
		return strings.TrimSpace(b.String())
	}
	seeded, err := settings.SeedSettings(ctx, pool, now)
	if err != nil || outcomes(seeded) != "core_parameters=added default_terms=added privacy=added future_observations=added now_tiles=added" {
		t.Fatalf("the seed: %s, %v", outcomes(seeded), err)
	}
	seeded, err = settings.SeedSettings(ctx, pool, now.Add(time.Hour))
	if err != nil || outcomes(seeded) != "core_parameters=unchanged default_terms=unchanged privacy=unchanged future_observations=unchanged now_tiles=unchanged" {
		t.Fatalf("the seed again: %s, %v", outcomes(seeded), err)
	}

	admin := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO users (id, created_at, updated_at) VALUES ($1, now(), now())`, admin)
	if err != nil {
		t.Fatal(err)
	}
	by := settings.Actor{UserID: &admin, Name: "Amara Okafor"}
	if _, err := settings.Put(ctx, pool, now, by, settings.KeyPrivacy, []byte(`{"minorCoordinateDecimals": 9}`)); err == nil {
		t.Fatal("a bad value was stored")
	}
	if changed, err := settings.Put(ctx, pool, now.Add(2*time.Hour), by, settings.KeyPrivacy, []byte(`{"minorCoordinateDecimals": 1}`)); err != nil || !changed {
		t.Fatalf("a good value: %v, %v", changed, err)
	}
	if changed, err := settings.Put(ctx, pool, now.Add(3*time.Hour), by, settings.KeyPrivacy, []byte(`{"minorCoordinateDecimals": 1}`)); err != nil || changed {
		t.Fatalf("the same again: %v, %v", changed, err)
	}
	seeded, err = settings.SeedSettings(ctx, pool, now.Add(4*time.Hour))
	if err != nil || !strings.Contains(outcomes(seeded), "privacy=kept") {
		t.Fatalf("the seed after a put: %s, %v", outcomes(seeded), err)
	}
	e, err := settings.Get(ctx, pool, settings.KeyPrivacy)
	if err != nil || string(e.Value) != `{"minorCoordinateDecimals": 1}` || e.Seeded {
		t.Fatalf("stored: %s seeded %v, %v", e.Value, e.Seeded, err)
	}

	log, err := settings.Changes(ctx, pool, 10)
	if err != nil || len(log) != 6 {
		t.Fatalf("the log: %+v, %v", log, err)
	}
	if l := log[0]; l.Actor.Name != "Amara Okafor" || l.Actor.UserID == nil || *l.Actor.UserID != admin || l.Subject != settings.KeyPrivacy || l.Summary != "Changed privacy" {
		t.Fatalf("the newest entry: %+v", l)
	}
	if l := log[3]; l.Actor != settings.Seed || l.Kind != settings.KindSetting || l.Summary != "added by the seed" {
		t.Fatalf("the oldest entry: %+v", l)
	}
}

func TestPlace(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		ids      []string
		id       string
		position int
		want     string
	}{
		{[]string{"a", "b", "c"}, "c", 0, "c a b"},
		{[]string{"a", "b", "c"}, "a", 1, "b a c"},
		{[]string{"a", "b"}, "x", 1, "a x b"},
		{[]string{"a", "b"}, "x", 9, "a b x"},
		{[]string{"a", "b"}, "b", -1, "b a"},
		{nil, "x", 0, "x"},
	} {
		if got := strings.Join(settings.Place(c.ids, c.id, c.position), " "); got != c.want {
			t.Errorf("%v %s at %d: %q, want %q", c.ids, c.id, c.position, got, c.want)
		}
	}
}
