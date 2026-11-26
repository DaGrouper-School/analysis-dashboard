package labs

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/testutil"
)

func good() Input {
	return Input{Name: "Trinity garden", PlaceName: "Trinity School", Country: "Ethiopia", Location: Point{Lat: 9.03, Lng: 38.74},
		SiteProfile: SiteProfile{Setting: "urban", Description: "A garden behind the school.", ResearchQuestions: []string{"  How much rain falls?  ", " "},
			ParameterNotes: map[string]string{"dgs:soil_ec": " Salty here. ", "pm2_5": " "}}}
}

func TestCheck(t *testing.T) {
	t.Parallel()
	if f := Check(good()); len(f) != 0 {
		t.Fatalf("a good lab: %v", f)
	}
	bad := Input{Name: "ab", Location: Point{Lat: 91, Lng: math.NaN()},
		SiteProfile: SiteProfile{Setting: "desert", Description: "short", ResearchQuestions: []string{" "}, ParameterNotes: map[string]string{"Soil EC": "x"}}}
	f := Check(bad)
	for _, field := range []string{"name", "placeName", "country", "location", "setting", "description", "researchQuestions", "parameterNotes"} {
		if f[field] == "" {
			t.Errorf("no problem for %s: %v", field, f)
		}
	}
}

func TestClean(t *testing.T) {
	t.Parallel()
	c := clean(good())
	if len(c.SiteProfile.ResearchQuestions) != 1 || c.SiteProfile.ResearchQuestions[0] != "How much rain falls?" ||
		len(c.SiteProfile.ParameterNotes) != 1 || c.SiteProfile.ParameterNotes["dgs:soil_ec"] != "Salty here." {
		t.Errorf("cleaned: %+v", c.SiteProfile)
	}
}

func TestSite(t *testing.T) {
	t.Parallel()
	if got := SiteCode("DGC-HG2W5K", 1); got != "DGC-HG2W5K-01" {
		t.Errorf("a site code: %s", got)
	}
	g := point(Point{Lat: 9.03, Lng: 38.74})
	if string(g) != `{"type":"Point","coordinates":[38.74,9.03]}` || !sameGeometry(g, []byte(`{"coordinates": [38.74, 9.03], "type": "Point"}`)) ||
		sameGeometry(g, point(Point{Lat: 9.03, Lng: 38.75})) {
		t.Errorf("geometry: %s", g)
	}
}

// TestLabs: labs are numbered in their classroom, and a change of
// location, and only that, sends the site to SAM again.
func TestLabs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.DB(t)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if _, err := settings.SeedSettings(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	c, err := classrooms.Create(ctx, pool, now, uuid.Nil, classrooms.CreateInput{Name: "Trinity Weather", InstitutionName: "Trinity", HeadEmail: "h@t.example"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := Create(ctx, pool, now, c.ID, c.Code, good())
	if err != nil || first.Number != 1 {
		t.Fatalf("a first lab: %+v, %v", first, err)
	}
	second, err := Create(ctx, pool, now, c.ID, c.Code, good())
	if err != nil || second.Number != 2 {
		t.Fatalf("a second lab: %+v, %v", second, err)
	}
	queued := func(l Lab) int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM sam_outbox WHERE entity = $1`, entity(l.ID)).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	renamed := good()
	renamed.Name = "Trinity rooftop"
	if _, err := Update(ctx, pool, now, first, c.Code, renamed); err != nil || queued(first) != 1 {
		t.Fatalf("renamed: %d, %v", queued(first), err)
	}
	moved := renamed
	moved.Location = Point{Lat: 9.04, Lng: 38.75}
	if _, err := Update(ctx, pool, now, first, c.Code, moved); err != nil || queued(first) != 2 {
		t.Fatalf("moved: %d, %v", queued(first), err)
	}
	list, err := ForClassroom(ctx, pool, c.ID)
	if err != nil || len(list) != 2 || list[0].Name != "Trinity rooftop" || list[0].Location.Lng != 38.75 || list[1].ID != second.ID {
		t.Fatalf("listed: %+v, %v", list, err)
	}
}
