package apitest_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestLabs: those who manage a classroom's devices add and change its
// labs; each lab is a space under its classroom's in SAM, made once the
// classroom's is, with a site of the classroom's code and the lab's number
// at its point; a change of location replaces the site; only those who see
// the classroom see its labs.
func TestLabs(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := settings.SeedSettings(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	a := admin(t, e)
	someone := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	call := func(p *profile, method, path string, body, out any) (int, refusal) {
		t.Helper()
		raw := ""
		if body != nil {
			b, _ := json.Marshal(body)
			raw = string(b)
		}
		res, got := e.DoWith(t, p.client, method, "/api"+path, raw)
		var no refusal
		if res.StatusCode >= 400 {
			_ = json.Unmarshal([]byte(got), &no)
		} else if out != nil {
			if err := json.Unmarshal([]byte(got), out); err != nil {
				t.Fatalf("%s %s: %s", method, path, got)
			}
		}
		return res.StatusCode, no
	}
	var c classrooms.Summary
	if status, _ := call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "h@t.example"}, &c); status != 200 {
		t.Fatalf("a classroom: %d", status)
	}
	in := map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia", "location": map[string]any{"lat": 9.03, "lng": 38.74},
		"siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.", "researchQuestions": []string{"How much rain falls?"}}}
	var lab labs.Lab
	if status, no := call(a, "POST", "/classrooms/"+c.ID+"/labs", in, &lab); status != 200 || lab.Name != "Trinity garden" {
		t.Fatalf("a lab: %d %+v %+v", status, lab, no)
	}
	if status, no := call(a, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "x", "placeName": "", "country": "", "location": map[string]any{"lat": 99, "lng": 0},
		"siteProfile": map[string]any{"setting": "urban", "description": "", "researchQuestions": []string{}}}, nil); status != 400 || no.Fields["location"] == "" {
		t.Errorf("a bad lab: %d %+v", status, no)
	}
	for _, path := range []string{"/classrooms/" + c.ID + "/labs", "/labs/" + lab.ID.String()} {
		if status, _ := call(someone, "GET", path, nil, nil); status != 404 {
			t.Errorf("%s seen by someone else: %d", path, status)
		}
	}
	if status, _ := call(someone, "POST", "/classrooms/"+c.ID+"/labs", in, nil); status != 404 {
		t.Errorf("a lab added by someone else: %d", status)
	}

	// The lab waits for its classroom's space, then has its own and a site.
	deliver(t, e, "lab:")
	cls, _ := classrooms.Get(ctx, e.Pool, uuid.MustParse(c.ID))
	stored, err := labs.Get(ctx, e.Pool, lab.ID)
	if err != nil || stored.SpaceID == nil {
		t.Fatalf("the lab's space: %+v, %v", stored, err)
	}
	svc := e.SAM.Service()
	space, err := svc.Space(ctx, *stored.SpaceID)
	if err != nil || *space.ParentID != *cls.SpaceID {
		t.Fatalf("the space under the classroom's: %+v, %v", space, err)
	}
	site, err := svc.Site(ctx, *stored.SpaceID)
	if err != nil || site.Code != *c.Code+"-01" || site.Version != 1 {
		t.Fatalf("the site: %+v, %v", site, err)
	}
	var g struct {
		Coordinates []float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(site.Geometry, &g); err != nil || len(g.Coordinates) != 2 || g.Coordinates[0] != 38.74 || g.Coordinates[1] != 9.03 {
		t.Fatalf("the site's point: %s, %v", site.Geometry, err)
	}

	in["location"] = map[string]any{"lat": 9.05, "lng": 38.76}
	if status, no := call(a, "PUT", "/labs/"+lab.ID.String(), in, &lab); status != 200 || lab.Location.Lat != 9.05 {
		t.Fatalf("moved: %d %+v %+v", status, lab, no)
	}
	deliver(t, e, "lab:")
	if site, err = svc.Site(ctx, *stored.SpaceID); err != nil || site.Version != 2 {
		t.Fatalf("the site replaced: %+v, %v", site, err)
	}
	if err := json.Unmarshal(site.Geometry, &g); err != nil || g.Coordinates[0] != 38.76 || g.Coordinates[1] != 9.05 {
		t.Errorf("the new point: %s", site.Geometry)
	}

	var list []labs.Lab
	if status, _ := call(a, "GET", "/classrooms/"+c.ID+"/labs", nil, &list); status != 200 || len(list) != 1 {
		t.Errorf("the labs: %d %+v", status, list)
	}
	var d labs.Detail
	if status, _ := call(a, "GET", "/labs/"+lab.ID.String(), nil, &d); status != 200 || d.ClassroomName != "Trinity Weather" || !d.CanManage || d.CanRecord {
		t.Errorf("the lab: %d %+v", status, d)
	}
	if status, _ := call(a, "GET", "/labs/"+uuid.NewString(), nil, nil); status != 404 {
		t.Errorf("an unknown lab: %d", status)
	}
}
