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
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/publicsite"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/views"
)

// TestPublicStation: a member reads the station page as themselves, with
// values not yet released (the core strip and cards saying so), the last
// sync, the device tiles and cards and
// "What's happening now?" with its last 24 hours and trend; a visitor
// reads released values only, sees every device with no values before a
// release, and sees a classroom that stops showing its data with no
// values at once.
func TestPublicStation(t *testing.T) {
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
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.031234, "lng": 38.741234}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var station, gauge labs.DeviceView
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Rain gauge", "type": "manual_rain_gauge", "identifier": ""}, &gauge)
	deliver(t, e, "")

	latest := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Hour)
	var readings []map[string]any
	for i, ago := range []time.Duration{26, 3, 1, 0} {
		readings = append(readings, map[string]any{"observedAt": latest.Add(-ago * time.Hour).Format(time.RFC3339),
			"values": map[string]any{"air_temperature": 18 + 2*float64(i), "wind_avg": 2.5, "wind_direction": 90.0}, "location": nil})
	}
	if status, no := call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": false, "readings": readings}, nil); status != 200 {
		t.Fatalf("readings: %d %+v", status, no)
	}
	if status, no := call(head, "POST", "/labs/"+lab.ID.String()+"/observations", map[string]any{"credit": false, "templateId": "rain_gauge_reading",
		"labId": lab.ID, "deviceId": gauge.ID, "observedAt": latest.Add(-time.Hour).Format(time.RFC3339),
		"values": map[string]any{"rain_gauge_daily_accumulation": 6.2}, "location": nil}, nil); status != 200 {
		t.Fatalf("a gauge reading: %d %+v", status, no)
	}

	page := func(p *profile) publicsite.StationPage {
		t.Helper()
		var out publicsite.StationPage
		if status, no := call(p, "GET", "/public/classrooms/"+c.ID, nil, &out); status != 200 {
			t.Fatalf("the station page: %d %+v", status, no)
		}
		return out
	}
	device := func(s publicsite.StationPage, id uuid.UUID) publicsite.PublicDevice {
		t.Helper()
		for _, d := range s.Devices {
			if d.ID == id {
				return d
			}
		}
		t.Fatalf("no device %v in %+v", id, s.Devices)
		return publicsite.PublicDevice{}
	}
	tile := func(tiles []views.NowTile, id string) views.NowTile {
		t.Helper()
		for _, tl := range tiles {
			if tl.ID == id {
				return tl
			}
		}
		t.Fatalf("no tile %s in %+v", id, tiles)
		return views.NowTile{}
	}

	before := page(nil)
	if len(before.Domains) != 0 || before.Classroom.Core.Review != views.NotReleased || len(before.Devices) != 2 || before.Can.SeeUnreleased ||
		before.Institution != nil || before.Core.Rain.Review != views.NotReleased || before.Completeness != nil || len(before.Places) != 1 {
		t.Fatalf("a visitor before a release: %+v", before)
	}
	if d := device(before, station.ID); len(d.Tiles) == 0 || d.Tiles[0].Value != nil || d.Color == "" || len(d.Summary) == 0 || d.Summary[0].Latest != nil {
		t.Errorf("a device with nothing released: %+v", d)
	}

	mine := page(head)
	if !mine.Can.SeeUnreleased || mine.Classroom.Core.Review != views.NotReleased || mine.Classroom.Core.LastSync == nil || len(mine.Domains) == 0 ||
		mine.Institution == nil || mine.Institution.Name != "Trinity School" || len(mine.Parameters) == 0 {
		t.Fatalf("a member before a release: %+v", mine)
	}
	temp := tile(mine.Now, "temperature")
	if temp.Value != 24.0 || temp.Unit == "" || len(temp.Last24Hours) != 24 || temp.Last24Hours[23] == nil || *temp.Last24Hours[23] != 24 ||
		temp.Trend == nil || temp.Trend.Direction != "rising" {
		t.Errorf("the temperature tile: %+v", temp)
	}
	if wind := tile(mine.Now, "wind"); wind.Caption == nil || *wind.Caption != "From the east" {
		t.Errorf("the wind tile: %+v", wind)
	}
	st := device(mine, station.ID)
	if st.Tiles[0].ParameterID == nil || *st.Tiles[0].ParameterID != "air_temperature" || st.Tiles[0].Value != 24.0 ||
		st.Summary[0].ID != "air_temperature" || st.Summary[0].Review != views.NotReleased || len(st.Summary[0].Week) != 7 {
		t.Errorf("the station's tiles and cards: %+v", st)
	}
	var air *publicsite.PublicValue
	for _, card := range mine.Domains {
		for i, v := range card.Values {
			if v.ParameterID == "air_temperature" {
				air = &card.Values[i]
			}
		}
	}
	if air == nil || air.Value != 24.0 || len(air.Last7Days) != 7 || air.Last7Days[6] == nil || *air.Last7Days[6] != 24 || air.Source.ID != station.ID.String() {
		t.Errorf("the air temperature card value: %+v", air)
	}

	call(head, "POST", "/classrooms/"+c.ID+"/release", map[string]any{"through": time.Now().Format(time.RFC3339)}, nil)
	deliver(t, e, "release:")
	e.Clock.Add(publicsite.CacheFor + time.Second)
	after := page(nil)
	e.Clock.Add(-publicsite.CacheFor - time.Second)
	if len(after.Domains) == 0 || after.Classroom.Core.Review != views.Released || after.Core.Rain.Review != views.Released ||
		after.Core.Rain.Sources != 1 || len(after.Core.Rain.BySource) != 1 || after.Core.Rain.BySource[0].SourceID != gauge.ID.String() ||
		after.Core.Temperature.High == nil || *after.Core.Temperature.High != 24 {
		t.Fatalf("a visitor after the release: %+v %+v", after.Classroom.Core, after.Core)
	}
	if d := device(after, station.ID); d.Tiles[0].Value != 24.0 || d.Summary[0].Review != views.Released {
		t.Errorf("the released station: %+v", d)
	}

	// A reading after the release: the member sees it at once, in review;
	// visitors keep the released one.
	if status, no := call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": false, "readings": []map[string]any{
		{"observedAt": latest.Add(time.Hour).Format(time.RFC3339), "values": map[string]any{"air_temperature": 26.0}, "location": nil}}}, nil); status != 200 {
		t.Fatalf("a newer reading: %d %+v", status, no)
	}
	if newer := page(head); newer.Classroom.Core.Review != views.InReview || tile(newer.Now, "temperature").Value != 26.0 ||
		device(newer, station.ID).Summary[0].Review != views.InReview {
		t.Errorf("a member after a newer reading: %+v", newer.Classroom.Core)
	}
	if theirs := page(nil); tile(theirs.Now, "temperature").Value != 24.0 {
		t.Errorf("a visitor after a newer reading: %+v", tile(theirs.Now, "temperature"))
	}

	terms := map[string]any{"licence": "CC BY 4.0", "licenceUrl": nil, "attribution": "Trinity Weather", "restrictions": []string{}, "coordinateDecimals": nil}
	off := map[string]any{"showPublic": false, "review": "manual", "autoReleaseDays": 2, "releasers": "head", "facilitatorsInvite": true, "facilitatorsManageDevices": true}
	if status, no := call(head, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": off, "terms": terms}, nil); status != 200 {
		t.Fatalf("hidden: %d %+v", status, no)
	}
	if hidden := page(nil); hidden.Shared || len(hidden.Domains) != 0 || device(hidden, station.ID).Tiles[0].Value != nil {
		t.Errorf("a hidden classroom to visitors: %+v", hidden)
	}
	if status, _ := call(nil, "GET", "/public/classrooms/"+uuid.NewString(), nil, nil); status != 404 {
		t.Errorf("an unknown classroom: %d", status)
	}
}
