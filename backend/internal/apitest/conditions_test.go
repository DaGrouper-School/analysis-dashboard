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

// TestConditionsAndHistory: a device's readings come back from SAM newest
// first, each once, with values by parameter, a lab-site position as
// locationSource lab, and credit; current conditions show each member the
// latest reading of their classrooms' devices and nobody else's, platform
// admins included.
func TestConditionsAndHistory(t *testing.T) {
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
	other := signedIn(t, e, apitest.Person{Sub: "g-abebe", Email: "abebe@gondar.example", Name: "Abebe Kebede"})
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	for _, p := range []*profile{head, other} {
		if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(p.id), false); err != nil {
			t.Fatal(err)
		}
	}
	var c, gondar classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(a, "POST", "/classrooms", map[string]any{"name": "Gondar Weather", "institutionName": "Gondar School", "headEmail": "abebe@gondar.example"}, &gondar)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	call(other, "POST", "/invitations/by-token/"+tokenFor(t, e, "abebe@gondar.example")+"/accept", nil, nil)
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var station labs.DeviceView
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	deliver(t, e, "")

	at := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)
	var sent []labs.Reading
	if status, no := call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": true, "readings": []map[string]any{
		{"observedAt": at.Format(time.RFC3339), "values": map[string]any{"air_temperature": 21.5}, "location": map[string]any{"lat": 9.031, "lng": 38.741}},
		{"observedAt": at.Add(time.Minute).Format(time.RFC3339), "values": map[string]any{"air_temperature": 22.5}, "location": nil},
	}}, &sent); status != 200 || len(sent) != 2 {
		t.Fatalf("uploaded: %d %+v", status, no)
	}

	var history []labs.Reading
	if status, no := call(head, "GET", "/devices/"+station.ID.String()+"/readings", nil, &history); status != 200 || len(history) != 2 {
		t.Fatalf("the history: %d %+v %+v", status, history, no)
	}
	newest, oldest := history[0], history[1]
	if newest.ID != sent[1].ID || oldest.ID != sent[0].ID || newest.Values["air_temperature"] != 22.5 || newest.LocationSource != labs.FromLab ||
		newest.Location.Lat != 9.03 || oldest.LocationSource != labs.FromDevice || oldest.Location.Lng != 38.741 ||
		oldest.CreditedTo == nil || *oldest.CreditedTo != "Hana Bekele" || oldest.RecordedByParticipantID == nil || *oldest.RecordedByParticipantID != head.pid ||
		!oldest.ReceivedAt.Equal(sent[0].ReceivedAt) || newest.ClassroomID.String() != c.ID {
		t.Errorf("the history: %+v", history)
	}
	if call(head, "GET", "/devices/"+station.ID.String()+"/readings?limit=1", nil, &history); len(history) != 1 || history[0].ID != sent[1].ID {
		t.Errorf("one reading: %+v", history)
	}
	if status, _ := call(stranger, "GET", "/devices/"+station.ID.String()+"/readings", nil, nil); status != 404 {
		t.Errorf("someone else's device: %d", status)
	}
	if call(head, "GET", "/devices/"+station.ID.String()+"/readings?limit=0", nil, &history); len(history) != 2 {
		t.Errorf("a limit of 0 is the default: %+v", history)
	}

	var conditions []labs.Conditions
	if status, no := call(head, "GET", "/me/conditions", nil, &conditions); status != 200 || len(conditions) != 1 ||
		conditions[0].Device.ID != station.ID || conditions[0].ClassroomName != "Trinity Weather" || conditions[0].Reading.ID != sent[1].ID ||
		conditions[0].Reading.Values["air_temperature"] != 22.5 || conditions[0].Reading.LocationSource != labs.FromLab || len(conditions[0].Alerts) != 0 {
		t.Fatalf("the head's conditions: %d %+v %+v", status, conditions, no)
	}
	for name, p := range map[string]*profile{"a platform admin": a, "another classroom's head": other, "a stranger": stranger} {
		if status, _ := call(p, "GET", "/me/conditions", nil, &conditions); status != 200 || len(conditions) != 0 {
			t.Errorf("%s's conditions: %d %+v", name, status, conditions)
		}
	}
}
