package apitest_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestManualObservations: the labs a member may record in are listed with
// their devices; a rain gauge entry read off a device and a species
// sighting as a field observation are stored in SAM, a resent entry once;
// an entry through a device type's plain template joins the device's
// readings; the template's checks, the device rules and an unknown form
// are refused.
func TestManualObservations(t *testing.T) {
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
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var gauge, station labs.DeviceView
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Rain gauge", "type": "manual_rain_gauge", "identifier": ""}, &gauge)
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	deliver(t, e, "")

	var targets []labs.Target
	if status, _ := call(head, "GET", "/me/observation-targets", nil, &targets); status != 200 || len(targets) != 1 || targets[0].Lab.ID != lab.ID ||
		targets[0].ClassroomName != "Trinity Weather" || len(targets[0].Devices) != 2 {
		t.Fatalf("the head's targets: %d %+v", status, targets)
	}
	for name, p := range map[string]*profile{"a platform admin": a, "a stranger": stranger} {
		if call(p, "GET", "/me/observation-targets", nil, &targets); len(targets) != 0 {
			t.Errorf("%s's targets: %+v", name, targets)
		}
	}

	path := "/labs/" + lab.ID.String() + "/observations"
	at := time.Now().Add(-time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	rain := map[string]any{"credit": false, "templateId": "rain_gauge_reading", "labId": lab.ID, "deviceId": gauge.ID, "observedAt": at,
		"values": map[string]any{"rain_gauge_daily_accumulation": 12.5}, "location": nil}
	var o, again labs.Observation
	if status, no := call(head, "POST", path, rain, &o); status != 200 || o.DeviceID == nil || *o.DeviceID != gauge.ID || o.LocationSource != labs.FromLab ||
		o.TemplateID != "rain_gauge_reading" || o.Values["rain_gauge_daily_accumulation"] != 12.5 {
		t.Fatalf("a rain gauge entry: %d %+v %+v", status, o, no)
	}
	if call(head, "POST", path, rain, &again); again.ID != o.ID {
		t.Errorf("resent: %+v", again)
	}
	var history []labs.Reading
	if call(head, "GET", "/devices/"+gauge.ID.String()+"/readings", nil, &history); len(history) != 1 || history[0].ID != o.ID ||
		history[0].Values["rain_gauge_daily_accumulation"] != 12.5 {
		t.Errorf("the gauge's readings: %+v", history)
	}

	sighting := map[string]any{"credit": true, "templateId": "species_sighting", "labId": lab.ID, "deviceId": nil, "observedAt": at,
		"values": map[string]any{"species_name": "Grevy's zebra", "species_count": 3}, "location": map[string]any{"lat": 9.04, "lng": 38.75}}
	var seen labs.Observation
	if status, no := call(head, "POST", path, sighting, &seen); status != 200 || seen.DeviceID != nil || seen.LocationSource != labs.FromDevice ||
		seen.Location.Lat != 9.04 || seen.CreditedTo == nil || *seen.CreditedTo != "Hana Bekele" || seen.Values["species_count"] != 3.0 {
		t.Fatalf("a sighting: %d %+v %+v", status, seen, no)
	}

	plain := map[string]any{"credit": false, "templateId": "device:sen0658", "labId": lab.ID, "deviceId": station.ID, "observedAt": at,
		"values": map[string]any{"wind_avg": 2.5, "air_temperature": 20}, "location": nil}
	var p labs.Observation
	if status, no := call(head, "POST", path, plain, &p); status != 200 || p.DeviceID == nil || *p.DeviceID != station.ID {
		t.Fatalf("a plain device entry: %d %+v %+v", status, p, no)
	}
	history = nil
	if call(head, "GET", "/devices/"+station.ID.String()+"/readings", nil, &history); len(history) != 1 || history[0].ID != p.ID {
		t.Errorf("the station's readings: %+v", history)
	}
	var kept int
	if err := e.Pool.QueryRow(ctx, `SELECT count(*) FROM relayed_records WHERE form = 'observation'`).Scan(&kept); err != nil || kept != 3 {
		t.Errorf("relayed records: %d %v", kept, err)
	}

	// Refusals.
	with := func(base map[string]any, k string, v any) map[string]any {
		out := map[string]any{}
		for key, val := range base {
			out[key] = val
		}
		out[k] = v
		return out
	}
	for name, c := range map[string]struct {
		body  map[string]any
		field string
	}{
		"too much rain":            {with(rain, "values", map[string]any{"rain_gauge_daily_accumulation": 600}), "rain_gauge_daily_accumulation"},
		"a value off the form":     {with(rain, "values", map[string]any{"rain_gauge_daily_accumulation": 1, "wind_avg": 2}), "wind_avg"},
		"no device":                {with(rain, "deviceId", nil), "deviceId"},
		"a device of another type": {with(rain, "deviceId", station.ID), "deviceId"},
		"a sighting off a device":  {with(sighting, "deviceId", gauge.ID), "deviceId"},
		"no time":                  {with(rain, "observedAt", "yesterday"), "observedAt"},
		"a bad position":           {with(sighting, "location", map[string]any{"lat": 91, "lng": 0}), "location"},
	} {
		if status, no := call(head, "POST", path, c.body, nil); status != 400 || no.Fields[c.field] == "" {
			t.Errorf("%s: %d %+v", name, status, no)
		}
	}
	if status, _ := call(head, "POST", path, with(rain, "templateId", "nothing"), nil); status != 404 {
		t.Errorf("an unknown form: %d", status)
	}
	if status, _ := call(stranger, "POST", path, rain, nil); status != 404 {
		t.Errorf("someone else's lab: %d", status)
	}
}
