package classrooms

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func TestNewCode(t *testing.T) {
	t.Parallel()
	form := regexp.MustCompile(`^DGC-[` + codeAlphabet + `]{6}$`)
	seen := map[string]bool{}
	for range 200 {
		c := newCode()
		if !form.MatchString(c) || strings.ContainsAny(c[4:], "01OIL") {
			t.Fatalf("%q", c)
		}
		seen[c] = true
	}
	if len(seen) < 190 {
		t.Errorf("only %d codes of 200 differ", len(seen))
	}
}

func TestChecks(t *testing.T) {
	t.Parallel()
	if f := CheckCreate(CreateInput{Name: "Trinity Weather", InstitutionName: "Trinity School", HeadEmail: "head@trinity.example"}); len(f) != 0 {
		t.Fatalf("a good classroom: %v", f)
	}
	f := CheckCreate(CreateInput{Name: "ab", InstitutionName: " ", HeadEmail: "not an email"})
	if f["name"] == "" || f["institutionName"] == "" || f["headEmail"] != "Enter a valid email address." {
		t.Errorf("a bad classroom: %v", f)
	}
	if f := CheckCreate(CreateInput{Name: "abc", InstitutionName: "xy"}); f["headEmail"] != "Enter the institution head's email." {
		t.Errorf("no head: %v", f)
	}
	long := strings.Repeat("a", MaxHistory+1)
	if f := CheckUpdate(UpdateInput{Name: "abc", InstitutionName: "xy", History: &long}); f["history"] == "" {
		t.Errorf("a long history: %v", f)
	}
}

func TestUnder(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"items[0].x: not a field here": "profile.projects.items[0].x: not a field here",
		"[1].title: blank":             "profile.projects[1].title: blank",
		"the value is null":            "profile.projects: the value is null",
		"not JSON: the value ends":     "profile.projects: not JSON: the value ends",
	} {
		if got := under("profile.projects", in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// configured is a database with the platform's configuration, and a
// classroom created on it.
func configured(t *testing.T) (context.Context, *pgxpool.Pool, Classroom) {
	t.Helper()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := settings.SeedSettings(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedLists(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	c, err := Create(ctx, pool, now, uuid.Nil, CreateInput{Name: "Trinity Weather", InstitutionName: "Trinity School", HeadEmail: " Head@Trinity.example "})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, pool, c
}

// TestCreate: a new classroom starts at the first stage with the default
// sharing settings and the platform's terms filled in for it, and is sent
// to SAM.
func TestCreate(t *testing.T) {
	t.Parallel()
	ctx, pool, c := configured(t)
	got, err := Get(ctx, pool, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.StageID != "preparing" || got.RegionID != nil || got.Setup != SetupAdminAssisted || got.Sharing != roles.DefaultSharing ||
		*got.HeadEmail != "head@trinity.example" || got.SpaceID != nil || len(got.Groups) != 0 || got.Profile.CustodianRole != roles.Head {
		t.Fatalf("stored: %+v", got)
	}
	if got.Terms.Attribution != "Trinity Weather, Trinity School. DaGrouper Global Classrooms." || got.Terms.Licence != "CC BY 4.0" {
		t.Errorf("terms: %+v", got.Terms)
	}
	if byCode, err := ByCode(ctx, pool, c.Code); err != nil || byCode.ID != c.ID {
		t.Errorf("by code: %v", err)
	}
	var queued int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sam_outbox WHERE entity = $1 AND kind = $2`, entity(c.ID), Kind).Scan(&queued); err != nil || queued != 1 {
		t.Errorf("queued for SAM: %d, %v", queued, err)
	}
	if _, err := Get(ctx, pool, uuid.New()); !errors.Is(err, ErrNoClassroom) {
		t.Errorf("an unknown classroom: %v", err)
	}
	// A stage a classroom is at is in use.
	if err := DeleteStage(ctx, pool, time.Now(), settings.Operator, "preparing"); !errors.Is(err, ErrInUse) {
		t.Errorf("the classroom's stage deleted: %v", err)
	}
}

// TestConfigure: admins set a classroom's stage, region and profile
// fields; a field left out stays as it is; every value is checked first,
// and the change is logged.
func TestConfigure(t *testing.T) {
	t.Parallel()
	ctx, pool, c := configured(t)
	now := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	err := Configure(ctx, pool, now, settings.Operator, c.ID, []byte(`{"stageId": "pilot", "regionId": "east_africa",
		"profile": {"areaCode": "ET-AA", "hostAgreementSigned": true,
			"sessions": [{"id": "s1", "title": "First session", "startsAt": "2026-12-01T09:00:00Z", "place": "The lab"}],
			"institution": {"history": "Founded in 1950.", "hub": null, "website": "https://trinity.example"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Get(ctx, pool, c.ID)
	p := got.Profile
	if got.StageID != "pilot" || *got.RegionID != "east_africa" || p.AreaCode != "ET-AA" || !p.HostAgreementSigned || len(p.Sessions) != 1 ||
		p.Institution.History != "Founded in 1950." || *p.Institution.Website != "https://trinity.example" || p.CustodianRole != roles.Head {
		t.Fatalf("configured: %+v", got)
	}
	if err := Configure(ctx, pool, now, settings.Operator, c.ID, []byte(`{"regionId": null}`)); err != nil {
		t.Fatal(err)
	}
	if got, _ = Get(ctx, pool, c.ID); got.RegionID != nil || got.StageID != "pilot" || !got.Profile.HostAgreementSigned {
		t.Fatalf("the region cleared: %+v", got)
	}

	var invalid *jsoncheck.Invalid
	err = Configure(ctx, pool, now, settings.Operator, c.ID, []byte(`{"stageId": "done", "colour": 1,
		"profile": {"language": "English", "nope": 1, "custodianRole": "teacher",
			"sessions": [{"id": "s1", "title": "", "startsAt": "tomorrow", "place": "x"}],
			"projects": [{"id": "p1", "title": "Rain", "question": "", "sites": [{"code": "S1", "name": "Field", "ecosystem": "T",
				"parameterIds": [], "frequency": "Daily", "readingsPerDay": 0, "yearsPlanned": 1}]}],
			"institution": {"history": "", "hub": null, "website": "http://x"}}}`))
	if !errors.As(err, &invalid) {
		t.Fatalf("a bad configuration: %v", err)
	}
	f := invalid.Fields()
	for _, field := range []string{"stageId", "colour", "profile.language", "profile.nope", "profile.custodianRole", "profile.sessions[0].startsAt",
		"profile.sessions[0].title", "profile.projects[0].sites[0]", "profile.institution.website"} {
		if f[field] == "" {
			t.Errorf("no problem for %s: %v", field, f)
		}
	}
	if got, _ := Get(ctx, pool, c.ID); got.StageID != "pilot" || got.Profile.Language != "" {
		t.Errorf("a refused configuration was stored: %+v", got)
	}
	if err := Configure(ctx, pool, now, settings.Operator, uuid.New(), []byte(`{}`)); !errors.Is(err, ErrNoClassroom) {
		t.Errorf("an unknown classroom: %v", err)
	}
	log, _ := settings.Changes(ctx, pool, 2)
	if log[0].Summary != "Configured "+c.Code+": regionId" || log[1].Summary != "Configured "+c.Code+": profile, regionId, stageId" {
		t.Errorf("the log: %+v", log)
	}
}
