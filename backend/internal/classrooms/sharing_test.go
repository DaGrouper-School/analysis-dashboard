package classrooms

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

func TestCheckSharing(t *testing.T) {
	t.Parallel()
	https, three := "https://creativecommons.org/licenses/by/4.0/", 3
	good := SharingInput{Sharing: roles.DefaultSharing, Terms: settings.DataTerms{Licence: "CC BY 4.0", LicenceURL: &https, Attribution: "Trinity", CoordinateDecimals: &three}}
	if f := CheckSharing(good); len(f) != 0 {
		t.Fatalf("good: %v", f)
	}
	plain, seven := "http://example.org", 7
	bad := SharingInput{Sharing: roles.Sharing{Review: roles.ReviewAuto, AutoReleaseDays: 31, Releasers: "anyone"},
		Terms: settings.DataTerms{Licence: " ", LicenceURL: &plain, CoordinateDecimals: &seven}}
	f := CheckSharing(bad)
	for _, field := range []string{"autoReleaseDays", "releasers", "coordinateDecimals", "licence", "attribution", "licenceUrl"} {
		if f[field] == "" {
			t.Errorf("no problem for %s: %v", field, f)
		}
	}
	empty := ""
	if f := CheckSharing(SharingInput{Sharing: roles.DefaultSharing, Terms: settings.DataTerms{Licence: "CC0", LicenceURL: &empty, Attribution: "x"}}); len(f) != 0 {
		t.Errorf("an empty address is none: %v", f)
	}
	if got := cleanTerms(settings.DataTerms{Licence: " CC0 ", LicenceURL: &empty, Attribution: " x ", Restrictions: []string{" a ", " "}}); got.Licence != "CC0" ||
		got.LicenceURL != nil || got.Attribution != "x" || len(got.Restrictions) != 1 || got.Restrictions[0] != "a" {
		t.Errorf("cleaned: %+v", got)
	}
}

func TestNarrows(t *testing.T) {
	t.Parallel()
	d := roles.DefaultSharing
	with := func(edit func(*roles.Sharing)) roles.Sharing {
		s := d
		edit(&s)
		return s
	}
	for name, c := range map[string]struct {
		now  roles.Sharing
		want bool
	}{
		"unchanged":            {d, false},
		"hidden":               {with(func(s *roles.Sharing) { s.ShowPublic = false }), true},
		"only the head":        {with(func(s *roles.Sharing) { s.Releasers = roles.ReleasersHead }), true},
		"no device management": {with(func(s *roles.Sharing) { s.FacilitatorsManageDevices = false }), true},
		"no inviting":          {with(func(s *roles.Sharing) { s.FacilitatorsInvite = false }), true},
		"automatic review":     {with(func(s *roles.Sharing) { s.Review = roles.ReviewAuto }), false},
	} {
		if got := narrows(d, c.now); got != c.want {
			t.Errorf("%s: %v", name, got)
		}
	}
	auto := with(func(s *roles.Sharing) { s.Review = roles.ReviewAuto })
	longer := auto
	longer.AutoReleaseDays++
	if !narrows(auto, longer) || !narrows(auto, d) || narrows(longer, auto) {
		t.Error("a longer review period, or manual review, narrows what visitors read")
	}
}

func TestReleaseCutoff(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	manual := Classroom{Sharing: roles.DefaultSharing}
	if ReleaseCutoff(manual, now) != nil {
		t.Error("nothing released")
	}
	at := now.Add(-5 * 24 * time.Hour)
	manual.ReleasedThrough = &at
	if got := ReleaseCutoff(manual, now); !got.Equal(at) {
		t.Errorf("by hand: %v", got)
	}
	auto := manual
	auto.Sharing.Review, auto.Sharing.AutoReleaseDays = roles.ReviewAuto, 2
	if got := ReleaseCutoff(auto, now); !got.Equal(now.Add(-48 * time.Hour)) {
		t.Errorf("aged: %v", got)
	}
	recent := now.Add(-time.Hour)
	auto.ReleasedThrough = &recent
	if got := ReleaseCutoff(auto, now); !got.Equal(recent) {
		t.Errorf("released by hand past the review period: %v", got)
	}
}

// TestRelease: a release moves the cutoff and queues one publish; one in
// the future is taken as now; one at or before the cutoff changes and
// queues nothing.
func TestRelease(t *testing.T) {
	t.Parallel()
	ctx, pool, c := configured(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	by := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, created_at, updated_at) VALUES ($1, now(), now())`, by); err != nil {
		t.Fatal(err)
	}
	through := now.Add(-time.Hour)
	c, queued, err := Release(ctx, pool, now, c, by, through)
	if err != nil || !queued || !c.ReleasedThrough.Equal(through) {
		t.Fatalf("released: %v %v %v", c.ReleasedThrough, queued, err)
	}
	if _, queued, _ := Release(ctx, pool, now, c, by, through); queued {
		t.Error("released again")
	}
	if _, queued, _ := Release(ctx, pool, now, c, by, through.Add(-time.Hour)); queued {
		t.Error("the cutoff moved back")
	}
	c, _, _ = Release(ctx, pool, now, c, by, now.Add(time.Hour))
	if !c.ReleasedThrough.Equal(now) {
		t.Errorf("a cutoff in the future: %v", c.ReleasedThrough)
	}
	stored, _ := Get(ctx, pool, c.ID)
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sam_outbox WHERE entity = $1`, releaseEntity(c.ID)).Scan(&n); err != nil || n != 2 || !stored.ReleasedThrough.Equal(now) {
		t.Errorf("publishes queued: %d, cutoff %v, %v", n, stored.ReleasedThrough, err)
	}
}
