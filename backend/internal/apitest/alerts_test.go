package apitest_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestAlertLevels: a device's levels are the platform defaults until a
// head changes one, which copies the parameter's others into the
// device's layer; the head sets the classroom's levels for a sensor type,
// which current conditions then raise; a community scientist changes
// nothing; a default is never deleted; a reset goes back a layer.
func TestAlertLevels(t *testing.T) {
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
	scientist := signedIn(t, e, apitest.Person{Sub: "g-sara", Email: "sara@trinity.example", Name: "Sara Tadesse"})
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(head.id), false); err != nil {
		t.Fatal(err)
	}
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"sara@trinity.example"}, "role": "community_scientist"}, nil)
	call(scientist, "POST", "/invitations/by-token/"+tokenFor(t, e, "sara@trinity.example")+"/accept", nil, nil)
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var station labs.DeviceView
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	deliver(t, e, "")
	path := "/devices/" + station.ID.String() + "/alerts"
	of := func(d labs.DeviceAlerts, param string) []labs.Rule {
		var out []labs.Rule
		for _, r := range d.Rules {
			if r.ParameterID == param {
				out = append(out, r)
			}
		}
		return out
	}

	var d labs.DeviceAlerts
	if status, _ := call(head, "GET", path, nil, &d); status != 200 || !d.CanEditDevice || !d.CanEditType || d.DeviceType != "sen0658" ||
		len(of(d, "uv")) != 2 || of(d, "uv")[0].Source != labs.ScopeDefault {
		t.Fatalf("the defaults: %d %+v", status, d)
	}
	var seen labs.DeviceAlerts
	if call(scientist, "GET", path, nil, &seen); seen.CanEditDevice || seen.CanEditType || len(seen.Rules) != len(d.Rules) {
		t.Errorf("a community scientist's view: %+v", seen)
	}
	if status, _ := call(stranger, "GET", path, nil, nil); status != 404 {
		t.Errorf("someone else: %d", status)
	}

	// Changing the default UV warning copies the UV caution too.
	rule := map[string]any{"parameterId": "uv", "direction": "above", "value": 9, "level": "warning", "title": "Very high UV", "enabled": true}
	if status, no := call(head, "PUT", path+"/alr_default_uv_warning?scope=device", rule, &d); status != 200 {
		t.Fatalf("changed: %d %+v", status, no)
	}
	uv := of(d, "uv")
	if len(uv) != 2 || uv[0].Source != labs.ScopeDevice || uv[0].Value != 9 || uv[1].Source != labs.ScopeDevice || uv[1].Value != 6 ||
		uv[0].UpdatedBy == nil || *uv[0].UpdatedBy != "Hana Bekele" {
		t.Errorf("copied on the first change: %+v", uv)
	}
	if status, _ := call(scientist, "POST", path+"?scope=device", rule, nil); status != 403 {
		t.Errorf("a community scientist's change: %d", status)
	}
	if status, no := call(head, "PUT", path+"/"+uv[0].ID+"?scope=device", map[string]any{"parameterId": "air_temperature", "direction": "above", "value": 30,
		"level": "warning", "title": "Hot", "enabled": true}, nil); status != 400 || no.Fields["parameterId"] == "" {
		t.Errorf("moved to another value: %d %+v", status, no)
	}
	if status, no := call(head, "POST", path+"?scope=type", map[string]any{"parameterId": "air_temperature", "direction": "above", "value": 80,
		"level": "caution", "title": " ", "enabled": true}, nil); status != 400 || no.Fields["value"] == "" || no.Fields["title"] == "" {
		t.Errorf("a bad level: %d %+v", status, no)
	}

	// The classroom's level for the type, then a reading that trips it.
	if status, no := call(head, "POST", path+"?scope=type", map[string]any{"parameterId": "air_temperature", "direction": "above", "value": 25,
		"level": "caution", "title": "Warm", "enabled": true}, &d); status != 200 {
		t.Fatalf("a type level: %d %+v", status, no)
	}
	if temp := of(d, "air_temperature"); len(temp) != 2 || temp[0].Source != labs.ScopeType || temp[0].Value != 25 || temp[1].Value != 32 {
		t.Errorf("the type's levels: %+v", temp)
	}
	at := time.Now().Add(-time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339)
	call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": false, "readings": []map[string]any{
		{"observedAt": at, "values": map[string]any{"air_temperature": 30}, "location": nil}}}, nil)
	var conditions []labs.Conditions
	if call(head, "GET", "/me/conditions", nil, &conditions); len(conditions) != 1 || len(conditions[0].Alerts) != 1 ||
		conditions[0].Alerts[0].Title != "Warm" || !strings.Contains(conditions[0].Alerts[0].Message, "the classroom's caution level of 25") {
		t.Errorf("the alerts: %+v", conditions)
	}

	// A default is never deleted; a device rule is; a reset goes back.
	if status, no := call(head, "DELETE", path+"/alr_default_heat_index_warning", nil, nil); status != 404 || !strings.Contains(no.Message, "Turn one off") {
		t.Errorf("a default deleted: %d %+v", status, no)
	}
	if status, _ := call(head, "DELETE", path+"/"+uv[1].ID, nil, &d); status != 200 || len(of(d, "uv")) != 1 {
		t.Errorf("a device rule deleted: %d %+v", status, of(d, "uv"))
	}
	if status, _ := call(head, "DELETE", path+"?scope=device&parameter=uv", nil, &d); status != 200 || len(of(d, "uv")) != 2 || of(d, "uv")[0].Source != labs.ScopeDefault {
		t.Errorf("reset: %d %+v", status, of(d, "uv"))
	}
}
