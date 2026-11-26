package apitest_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestRawTable: a classroom's values come from SAM a page at a time,
// newest first, with the total, the classroom's terms and the filters
// the values offer; the filters narrow by site, source, field
// observations, parameter and days; a sort by parameter pages through
// every value once; a page past SAM's offset is refused; only members
// read the table.
func TestRawTable(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	e.RunBackground()
	ctx := context.Background()
	now := time.Now()
	if _, err := settings.SeedSettings(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
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
	a := admin(t, e)
	head := signedIn(t, e, apitest.Person{Sub: "g-hana", Email: "hana@trinity.example", Name: "Hana Bekele"})
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(head.id), false); err != nil {
		t.Fatal(err)
	}
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	newLab := func(name string, lat float64) labs.Lab {
		var l labs.Lab
		call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": name, "placeName": "Trinity School", "country": "Ethiopia",
			"location": map[string]any{"lat": lat, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A place at the school.",
				"researchQuestions": []string{"How much rain falls?"}}}, &l)
		return l
	}
	garden, roof := newLab("Trinity garden", 9.03), newLab("Trinity roof", 9.04)
	var station labs.DeviceView
	call(head, "POST", "/labs/"+garden.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	deliver(t, e, "")

	base := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Hour)
	var readings []map[string]any
	for i := range 3 {
		readings = append(readings, map[string]any{"observedAt": base.Add(time.Duration(i) * 24 * time.Hour).Format(time.RFC3339),
			"values": map[string]any{"air_temperature": 20.0 + float64(i)}, "location": nil})
	}
	if status, no := call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": false, "readings": readings}, nil); status != 200 {
		t.Fatalf("readings: %d %+v", status, no)
	}
	if status, no := call(head, "POST", "/labs/"+garden.ID.String()+"/observations", map[string]any{"credit": false, "templateId": "species_sighting",
		"labId": garden.ID, "deviceId": nil, "observedAt": base.Add(12 * time.Hour).Format(time.RFC3339),
		"values": map[string]any{"species_name": "Ibis", "species_count": 2}, "location": map[string]any{"lat": 9.031234, "lng": 38.741234}}, nil); status != 200 {
		t.Fatalf("a sighting: %d %+v", status, no)
	}

	raw := func(q map[string]any) labs.RawPage {
		t.Helper()
		body := map[string]any{"classroomId": c.ID, "page": 1, "pageSize": 25}
		for k, v := range q {
			body[k] = v
		}
		var p labs.RawPage
		if status, no := call(head, "POST", "/exports/raw", body, &p); status != 200 {
			t.Fatalf("raw %v: %d %+v", q, status, no)
		}
		return p
	}
	all := raw(nil)
	if all.Total != 5 || len(all.Rows) != 5 || all.ClassroomName != "Trinity Weather" || all.Terms.Licence == "" {
		t.Fatalf("every value: %+v", all)
	}
	if all.Rows[0].ParameterID != "air_temperature" || all.Rows[0].Value != 22.0 || all.Rows[0].SourceID != station.ID.String() ||
		all.Rows[0].SiteName != "Trinity garden" || all.Rows[0].LocationSource != labs.FromLab || all.Rows[0].ContributorID == nil || *all.Rows[0].ContributorID != head.pid {
		t.Errorf("the newest row: %+v", all.Rows[0])
	}
	o := all.Options
	if len(o.Sites) != 1 || o.Sites[0].ID != garden.ID || len(o.Sources) != 2 || o.Sources[0].Kind != "field" || o.Sources[1].ID != station.ID.String() ||
		len(o.ParameterIDs) != 3 || o.Days == nil || o.Days.First != base.Format("2006-01-02") || o.Days.Last != base.Add(48*time.Hour).Format("2006-01-02") {
		t.Errorf("the options: %+v", o)
	}

	for name, c := range map[string]struct {
		q     map[string]any
		total int64
	}{
		"field observations": {map[string]any{"sourceId": "field"}, 2},
		"the station":        {map[string]any{"sourceId": station.ID.String()}, 3},
		"a parameter":        {map[string]any{"parameterId": "air_temperature"}, 3},
		"one day":            {map[string]any{"from": base.Add(24 * time.Hour).Format("2006-01-02"), "to": base.Add(24 * time.Hour).Format("2006-01-02")}, 1},
		"the empty site":     {map[string]any{"labId": roof.ID.String()}, 0},
	} {
		if p := raw(c.q); p.Total != c.total || len(p.Rows) != int(c.total) {
			t.Errorf("%s: %d rows of %d", name, len(p.Rows), p.Total)
		}
	}
	if p := raw(map[string]any{"sourceId": "field"}); len(p.Options.ParameterIDs) != 2 || len(p.Options.Sources) != 2 {
		t.Errorf("a source narrows the parameters, not the sources: %+v", p.Options)
	}
	if p := raw(map[string]any{"sourceId": "field"}); p.Rows[0].SourceKind != "field" || p.Rows[0].Location.Lat != 9.031234 {
		t.Errorf("a field row: %+v", p.Rows[0])
	}

	// Sorted by parameter, two a page: every value once.
	seen := map[string]bool{}
	var order []string
	for page := 1; page <= 3; page++ {
		for _, row := range raw(map[string]any{"sort": "parameter", "page": page, "pageSize": 2}).Rows {
			seen[row.ID] = true
			order = append(order, row.ParameterID)
		}
	}
	if len(seen) != 5 || !slices.IsSorted(order) {
		t.Errorf("paged by parameter: %d %v", len(seen), order)
	}

	if status, no := call(head, "POST", "/exports/raw", map[string]any{"classroomId": c.ID, "page": 402, "pageSize": 25}, nil); status != 400 || no.Fields["page"] == "" {
		t.Errorf("past SAM's offset: %d %+v", status, no)
	}
	for name, p := range map[string]*profile{"a platform admin": a, "a stranger": stranger} {
		if status, _ := call(p, "POST", "/exports/raw", map[string]any{"classroomId": c.ID, "page": 1, "pageSize": 25}, nil); status != 404 {
			t.Errorf("%s: %d", name, status)
		}
	}
}
