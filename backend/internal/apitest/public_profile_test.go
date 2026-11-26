package apitest_test

import (
	"context"
	"encoding/json"
	"strconv"
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

// TestPublicProfileAndContributors: a classroom's profile lists the
// contributors credited on its released values only, never one who did
// not ask for credit or whose values wait for release, named only with a
// public profile; kit serials are for members; a contributor's page shows
// their released credited values, and answers 404, as an unknown ID does,
// for anyone without a public profile or a minor; a classroom that stops
// showing its data lists no contributors and drops out of their pages.
func TestPublicProfileAndContributors(t *testing.T) {
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
	quiet := signedIn(t, e, apitest.Person{Sub: "g-abel", Email: "abel@trinity.example", Name: "Abel Tesfaye"})
	young := signedIn(t, e, apitest.Person{Sub: "g-sara", Email: "sara@trinity.example", Name: "Sara Alemu"})
	for _, p := range []*profile{head, quiet, young} {
		if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(p.id), false); err != nil {
			t.Fatal(err)
		}
	}
	call(head, "PUT", "/me/public-profile", map[string]any{"show": true}, nil)
	call(young, "PUT", "/me/public-profile", map[string]any{"show": true}, nil)
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	for _, p := range []struct {
		who   *profile
		email string
	}{{quiet, "abel@trinity.example"}, {young, "sara@trinity.example"}} {
		if status, no := call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{p.email}, "role": "community_scientist"}, nil); status != 201 && status != 200 {
			t.Fatalf("inviting %s: %d %+v", p.email, status, no)
		}
		call(p.who, "POST", "/invitations/by-token/"+tokenFor(t, e, p.email)+"/accept", nil, nil)
	}
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.031234, "lng": 38.741234}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var station labs.DeviceView
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	deliver(t, e, "")

	at := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Hour)
	sighting := func(p *profile, credit bool, observed time.Time, count int) {
		t.Helper()
		if status, no := call(p, "POST", "/labs/"+lab.ID.String()+"/observations", map[string]any{"credit": credit, "templateId": "species_sighting",
			"labId": lab.ID, "deviceId": nil, "observedAt": observed.Format(time.RFC3339), "values": map[string]any{"species_name": "Ibis", "species_count": count},
			"location": nil}, nil); status != 200 {
			t.Fatalf("a sighting: %d %+v", status, no)
		}
	}
	sighting(head, true, at, 2)
	sighting(quiet, true, at, 3)
	sighting(quiet, false, at, 4)
	sighting(young, true, at, 5)
	// Found to be a minor after recording: never listed or given a page.
	if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(young.id), true); err != nil {
		t.Fatal(err)
	}

	profilePage := func(p *profile) publicsite.ProfilePage {
		t.Helper()
		var out publicsite.ProfilePage
		if status, no := call(p, "GET", "/public/classrooms/"+c.ID+"/profile", nil, &out); status != 200 {
			t.Fatalf("the profile: %d %+v", status, no)
		}
		return out
	}
	contributor := func(pid int64) (int, publicsite.ContributorPage) {
		t.Helper()
		var out publicsite.ContributorPage
		status, _ := call(nil, "GET", "/public/contributors/"+strconv.FormatInt(pid, 10), nil, &out)
		return status, out
	}

	before := profilePage(nil)
	if len(before.Contributors) != 0 || len(before.Kit) != 1 || before.Kit[0].Count != 1 || len(before.Kit[0].Serials) != 0 || before.Member ||
		before.CustodianRole != nil || before.CanManage {
		t.Fatalf("a visitor before a release: %+v", before)
	}
	if mine := profilePage(head); !mine.Member || !mine.CanManage || len(mine.Kit[0].Serials) != 1 || mine.Kit[0].Serials[0] != "PILOT-01" ||
		len(mine.Kit[0].Measures) == 0 || mine.Institution == nil {
		t.Errorf("a member's profile: %+v", mine)
	}
	if status, _ := contributor(head.pid); status != 200 {
		t.Errorf("a contributor with a public profile before a release: %d", status)
	} else if _, page := contributor(head.pid); page.Total != 0 || len(page.Classrooms) != 0 {
		t.Errorf("before a release: %+v", page)
	}

	call(head, "POST", "/classrooms/"+c.ID+"/release", map[string]any{"through": time.Now().Format(time.RFC3339)}, nil)
	deliver(t, e, "release:")
	// A credited sighting after the release, observed before it: it waits.
	sighting(head, true, at.Add(-time.Hour), 9)

	after := profilePage(nil)
	if len(after.Contributors) != 2 || after.Contributors[0].Name == nil || *after.Contributors[0].Name != "Hana Bekele" ||
		after.Contributors[0].ParticipantID != head.pid || after.Contributors[1].Name != nil || after.Contributors[1].ParticipantID != quiet.pid {
		t.Fatalf("the contributors after the release: %+v", after.Contributors)
	}
	status, page := contributor(head.pid)
	if status != 200 || page.Profile == nil || page.Profile.FullName != "Hana Bekele" || page.Profile.Email != nil || page.Total != 2 ||
		len(page.Recent) != 2 || len(page.Classrooms) != 1 || page.Classrooms[0].Name != "Trinity Weather" {
		t.Fatalf("the head's page: %d %+v", status, page)
	}
	for _, r := range page.Recent {
		if r.Value == 9.0 || r.ClassroomName != "Trinity Weather" || r.ParameterLabel == "" {
			t.Errorf("a contribution: %+v", r)
		}
	}
	for name, pid := range map[string]int64{"no public profile": quiet.pid, "a minor": young.pid, "unknown": 999999999} {
		if status, _ := contributor(pid); status != 404 {
			t.Errorf("%s: %d", name, status)
		}
	}
	if status, _ := call(nil, "GET", "/public/contributors/x", nil, nil); status != 404 {
		t.Errorf("not an ID: %d", status)
	}

	terms := map[string]any{"licence": "CC BY 4.0", "licenceUrl": nil, "attribution": "Trinity Weather", "restrictions": []string{}, "coordinateDecimals": nil}
	off := map[string]any{"showPublic": false, "review": "manual", "autoReleaseDays": 2, "releasers": "head", "facilitatorsInvite": true, "facilitatorsManageDevices": true}
	if status, no := call(head, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": off, "terms": terms}, nil); status != 200 {
		t.Fatalf("hidden: %d %+v", status, no)
	}
	if hidden := profilePage(nil); len(hidden.Contributors) != 0 {
		t.Errorf("a hidden classroom's contributors: %+v", hidden.Contributors)
	}
	if _, page := contributor(head.pid); page.Total != 0 || len(page.Classrooms) != 0 {
		t.Errorf("the head's page after hiding: %+v", page)
	}
}
