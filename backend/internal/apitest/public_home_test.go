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

// TestPublicHome: anyone reads the stages and regions, and every
// classroom with its stage and core readings as SAM lets anonymous read
// them: no values before a release, the released ones after, with the
// registry's units; positions rounded and the host unnamed before its
// agreement; a classroom that stops showing its data has no values at
// once, while SAM's anonymous grants are still live.
func TestPublicHome(t *testing.T) {
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

	day := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Hour)
	call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": false, "readings": []map[string]any{
		{"observedAt": day.Format(time.RFC3339), "values": map[string]any{"air_temperature": 24.5}, "location": nil}}}, nil)
	if status, no := call(head, "POST", "/labs/"+lab.ID.String()+"/observations", map[string]any{"credit": false, "templateId": "rain_gauge_reading",
		"labId": lab.ID, "deviceId": gauge.ID, "observedAt": day.Add(-time.Hour).Format(time.RFC3339),
		"values": map[string]any{"rain_gauge_daily_accumulation": 6.2}, "location": nil}, nil); status != 200 {
		t.Fatalf("a gauge reading: %d %+v", status, no)
	}

	home := func() (publicsite.Home, publicsite.Classroom) {
		t.Helper()
		var h publicsite.Home
		if status, _ := call(nil, "GET", "/public/home", nil, &h); status != 200 || len(h.Classrooms) != 1 {
			t.Fatalf("the home page: %d %+v", status, h)
		}
		return h, h.Classrooms[0]
	}
	h, got := home()
	if got.Core.Review != views.NotReleased || got.Core.Day != nil || got.Core.AirTemperature != nil || got.InstitutionName != nil ||
		got.Location == nil || got.Location.Lat != 9.03 || got.Location.Lng != 38.74 || *got.PlaceName != "Trinity School" ||
		len(got.Themes) != 1 || got.StageID == "" || len(h.Content.Stages) == 0 || len(h.Content.Regions) == 0 {
		t.Fatalf("before a release: %+v %+v", got, h.Content)
	}
	if h.Units.Temperature == "" || h.Units.Rain == "" {
		t.Errorf("the units: %+v", h.Units)
	}

	// Released; the cached answer goes stale.
	call(head, "POST", "/classrooms/"+c.ID+"/release", map[string]any{"through": time.Now().Format(time.RFC3339)}, nil)
	deliver(t, e, "release:")
	if _, got = home(); got.Core.Review != views.NotReleased {
		t.Errorf("the cached answer before it goes stale: %+v", got.Core)
	}
	e.Clock.Add(publicsite.CacheFor + time.Second)
	_, got = home()
	if got.Core.Review != views.Released || got.Core.Day == nil || *got.Core.Day != day.Format("2006-01-02") || got.Core.AirTemperature == nil ||
		*got.Core.AirTemperature != 24.5 || got.Core.RainToday == nil || *got.Core.RainToday != 6.2 || got.Core.RainDays != 1 || got.Core.LastSync == nil {
		t.Fatalf("after the release: %+v", got.Core)
	}

	// Showing stops: no values at once, before SAM revokes anything.
	terms := map[string]any{"licence": "CC BY 4.0", "licenceUrl": nil, "attribution": "Trinity Weather", "restrictions": []string{}, "coordinateDecimals": nil}
	off := map[string]any{"showPublic": false, "review": "manual", "autoReleaseDays": 2, "releasers": "head", "facilitatorsInvite": true, "facilitatorsManageDevices": true}
	if status, no := call(head, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": off, "terms": terms}, nil); status != 200 {
		t.Fatalf("hidden: %d %+v", status, no)
	}
	if _, got = home(); got.Core.Review != views.NotReleased || got.Core.AirTemperature != nil {
		t.Errorf("a hidden classroom's values: %+v", got.Core)
	}

	var content publicsite.Content
	if status, _ := call(nil, "GET", "/public/content", nil, &content); status != 200 || len(content.Stages) != len(h.Content.Stages) || len(content.Regions) == 0 {
		t.Errorf("the site content: %d %+v", status, content)
	}
}
