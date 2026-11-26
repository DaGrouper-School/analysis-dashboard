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
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/publicsite"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestPublicParameterPage: a member reads a value's page as themselves,
// with values still in review; a visitor reads released values only, and
// nothing before a release; each source is a series, oldest first, hourly
// beyond the last day; rain shows every rain source together with the
// manual gauge as the reference; other public classrooms with the value
// are offered to compare; a classroom that stops showing its data is not
// found by visitors at once.
func TestPublicParameterPage(t *testing.T) {
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
		client := e.NewClient()
		if p != nil {
			client = p.client
		}
		res, got := e.DoWith(t, client, method, "/api"+path, raw)
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
	if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(head.id), false); err != nil {
		t.Fatal(err)
	}
	type setup struct {
		id             string
		lab            labs.Lab
		station, gauge labs.DeviceView
	}
	classroom := func(name, identifier string) setup {
		t.Helper()
		var c classrooms.Summary
		call(a, "POST", "/classrooms", map[string]any{"name": name, "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
		call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
		s := setup{id: c.ID}
		call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": name + " garden", "placeName": "Trinity School", "country": "Ethiopia",
			"location": map[string]any{"lat": 9.031234, "lng": 38.741234}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
				"researchQuestions": []string{"How much rain falls?"}, "parameterNotes": map[string]string{"air_temperature": "In the shade of the old fig."}}}, &s.lab)
		if status, no := call(head, "POST", "/labs/"+s.lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": identifier}, &s.station); status != 200 {
			t.Fatalf("the station: %d %+v", status, no)
		}
		call(head, "POST", "/labs/"+s.lab.ID.String()+"/devices", map[string]any{"name": "Rain gauge", "type": "manual_rain_gauge", "identifier": ""}, &s.gauge)
		return s
	}
	trinity, beside := classroom("Trinity Weather", "pilot-01"), classroom("Bay Street", "pilot-02")
	deliver(t, e, "")

	latest := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Hour)
	var readings []map[string]any
	for i, ago := range []time.Duration{50, 30, 0} {
		readings = append(readings, map[string]any{"observedAt": latest.Add(-ago * time.Hour).Format(time.RFC3339),
			"values": map[string]any{"air_temperature": 20.5 + float64(i)}, "location": nil})
	}
	for _, s := range []setup{trinity, beside} {
		if status, no := call(head, "POST", "/devices/"+s.station.ID.String()+"/readings", map[string]any{"credit": false, "readings": readings}, nil); status != 200 {
			t.Fatalf("readings: %d %+v", status, no)
		}
	}
	if status, no := call(head, "POST", "/labs/"+trinity.lab.ID.String()+"/observations", map[string]any{"credit": false, "templateId": "rain_gauge_reading",
		"labId": trinity.lab.ID, "deviceId": trinity.gauge.ID, "observedAt": latest.Add(-time.Hour).Format(time.RFC3339),
		"values": map[string]any{"rain_gauge_daily_accumulation": 6.2}, "location": nil}, nil); status != 200 {
		t.Fatalf("a gauge reading: %d %+v", status, no)
	}

	page := func(p *profile, classroom, parameter string) (int, publicsite.ParameterPage) {
		t.Helper()
		var out publicsite.ParameterPage
		status, _ := call(p, "GET", "/public/classrooms/"+classroom+"/parameters/"+parameter, nil, &out)
		return status, out
	}
	if status, _ := page(nil, trinity.id, "air_temperature"); status != 404 {
		t.Errorf("a visitor before a release: %d", status)
	}
	status, mine := page(head, trinity.id, "air_temperature")
	if status != 200 || !mine.Member || !mine.InReview || mine.ReleasedThrough != nil || len(mine.Series) != 1 || len(mine.Series[0].Points) != 3 {
		t.Fatalf("a member before a release: %d %+v", status, mine)
	}
	s := mine.Series[0]
	if s.Source.ID != trinity.station.ID.String() || s.Source.Kind != "device" || s.Source.Name == nil || *s.Source.Name != "Pilot station" ||
		s.Source.Color == "" || s.ParameterID != "air_temperature" || !s.Points[0].ObservedAt.Equal(latest.Add(-50*time.Hour)) || s.Points[2].Value != 22.5 {
		t.Errorf("the station's series: %+v", s)
	}
	if mine.Note == nil || *mine.Note != "In the shade of the old fig." || mine.Domain == nil || len(mine.DeviceTypes) != 1 || mine.Rain != nil ||
		mine.Terms.Licence == "" || mine.Parameter.ID != "air_temperature" || len(mine.Content.Stages) == 0 {
		t.Errorf("the page around the values: %+v", mine)
	}

	for _, s := range []setup{trinity, beside} {
		call(head, "POST", "/classrooms/"+s.id+"/release", map[string]any{"through": time.Now().Format(time.RFC3339)}, nil)
	}
	deliver(t, e, "release:")
	// Past the cached "nothing released" from before; the clock comes
	// back after, as SAM checks a member's token against its own.
	e.Clock.Add(publicsite.CacheFor + time.Second)
	status, theirs := page(nil, trinity.id, "air_temperature")
	e.Clock.Add(-publicsite.CacheFor - time.Second)
	if status != 200 || theirs.Member || theirs.InReview || theirs.ReleasedThrough == nil || len(theirs.Series) != 1 || len(theirs.Series[0].Points) != 3 {
		t.Fatalf("a visitor after the release: %d %+v", status, theirs)
	}
	if len(theirs.Compare) != 1 || theirs.Compare[0].Name != "Bay Street" {
		t.Errorf("to compare: %+v", theirs.Compare)
	}
	status, rain := page(nil, trinity.id, "rain_gauge_daily_accumulation")
	if status != 200 || rain.Rain == nil || rain.Rain.ReferenceSourceID == nil || *rain.Rain.ReferenceSourceID != trinity.gauge.ID.String() ||
		rain.Rain.Tolerance <= 0 || len(rain.Series) != 1 || rain.Series[0].Points[0].Value != 6.2 {
		t.Errorf("the rain page: %d %+v", status, rain)
	}
	if !slices.Contains(theirs.DeviceTypes, mine.DeviceTypes[0]) {
		t.Errorf("device types: %v", theirs.DeviceTypes)
	}

	for name, path := range map[string][2]string{
		"an unknown value":     {trinity.id, "no_such_value"},
		"an unknown classroom": {uuid.NewString(), "air_temperature"},
		"a value with none":    {trinity.id, "soil_moisture"},
	} {
		if status, _ := page(nil, path[0], path[1]); status != 404 {
			t.Errorf("%s: %d", name, status)
		}
	}

	terms := map[string]any{"licence": "CC BY 4.0", "licenceUrl": nil, "attribution": "Trinity Weather", "restrictions": []string{}, "coordinateDecimals": nil}
	off := map[string]any{"showPublic": false, "review": "manual", "autoReleaseDays": 2, "releasers": "head", "facilitatorsInvite": true, "facilitatorsManageDevices": true}
	if status, no := call(head, "PUT", "/classrooms/"+trinity.id+"/sharing", map[string]any{"sharing": off, "terms": terms}, nil); status != 200 {
		t.Fatalf("hidden: %d %+v", status, no)
	}
	if status, _ := page(nil, trinity.id, "air_temperature"); status != 404 {
		t.Errorf("a hidden classroom's page: %d", status)
	}
	if status, _ := page(head, trinity.id, "air_temperature"); status != 200 {
		t.Errorf("a member of a hidden classroom: %d", status)
	}
}
