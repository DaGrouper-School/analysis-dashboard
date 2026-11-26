package apitest_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestSharingAndRelease: visitors read nothing until a facilitator
// releases, then exactly what was released, generalised and stripped; a
// retried release publishes once and a cutoff never moves back; what
// waits is counted by source; the head's terms reach SAM as the space's
// data policy, set as the head; turning "show public" off hides the
// classroom from visitors, and only the head releasing takes it from the
// facilitator, here and in SAM; the reconciler puts back a public grant
// SAM lost.
func TestSharingAndRelease(t *testing.T) {
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
	facilitator := signedIn(t, e, apitest.Person{Sub: "g-dawit", Email: "dawit@trinity.example", Name: "Dawit Alemu"})
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	for _, p := range []*profile{head, facilitator} {
		if _, err := accounts.SetMinor(ctx, e.Pool, now, uuid.MustParse(p.id), false); err != nil {
			t.Fatal(err)
		}
	}
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"dawit@trinity.example"}, "role": "facilitator"}, nil)
	call(facilitator, "POST", "/invitations/by-token/"+tokenFor(t, e, "dawit@trinity.example")+"/accept", nil, nil)
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var station labs.DeviceView
	call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station)
	deliver(t, e, "")
	stored, _ := classrooms.Get(ctx, e.Pool, uuid.MustParse(c.ID))
	space := *stored.SpaceID

	base := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)
	var readings []map[string]any
	for i := range 3 {
		readings = append(readings, map[string]any{"observedAt": base.Add(time.Duration(i) * 24 * time.Hour).Format(time.RFC3339),
			"values": map[string]any{"air_temperature": 20.0 + float64(i)}, "location": map[string]any{"lat": 9.031234, "lng": 38.741234}})
	}
	var sent []labs.Reading
	call(head, "POST", "/devices/"+station.ID.String()+"/readings", map[string]any{"credit": false, "readings": readings}, &sent)
	public := func() []client.ObservationRecord {
		t.Helper()
		page, err := e.Access.Anonymous().Observations(ctx, client.ObservationQuery{SpaceID: space}, client.PageQuery{Limit: 50})
		var ce *client.Error
		if errors.As(err, &ce) && ce.Status == 403 {
			return nil
		}
		if err != nil {
			t.Fatal(err)
		}
		return page.Items
	}
	if got := public(); len(got) != 0 {
		t.Fatalf("visitors before a release: %+v", got)
	}
	var review labs.ReleaseReview
	if status, no := call(head, "GET", "/classrooms/"+c.ID+"/release", nil, &review); status != 200 || review.Pending != 3 || len(review.Sources) != 1 ||
		*review.Sources[0].Name != "Pilot station" || review.ReleasedThrough != nil || !review.ShowPublic || review.Auto {
		t.Fatalf("waiting: %d %+v %+v", status, review, no)
	}
	if status, _ := call(stranger, "GET", "/classrooms/"+c.ID+"/release", nil, nil); status != 404 {
		t.Errorf("someone else's review: %d", status)
	}

	// The facilitator releases through the second reading.
	through := base.Add(24 * time.Hour)
	body := map[string]any{"through": through.Format(time.RFC3339)}
	if status, no := call(facilitator, "POST", "/classrooms/"+c.ID+"/release", body, &review); status != 200 || review.ReleasedThrough == nil ||
		!review.ReleasedThrough.Equal(through) {
		t.Fatalf("released: %d %+v %+v", status, review, no)
	}
	call(facilitator, "POST", "/classrooms/"+c.ID+"/release", body, nil)
	call(facilitator, "POST", "/classrooms/"+c.ID+"/release", map[string]any{"through": base.Format(time.RFC3339)}, &review)
	if !review.ReleasedThrough.Equal(through) {
		t.Errorf("the cutoff moved back: %v", review.ReleasedThrough)
	}
	var releases int
	if err := e.Pool.QueryRow(ctx, `SELECT count(*) FROM sam_outbox WHERE entity = $1`, "release:"+c.ID).Scan(&releases); err != nil || releases != 1 {
		t.Errorf("releases queued: %d %v", releases, err)
	}
	deliver(t, e, "release:")
	got := public()
	if len(got) != 2 || got[0].Location == nil {
		t.Fatalf("visitors after the release: %+v", got)
	}
	for _, r := range got {
		if r.ObservedAt.After(through) || r.SubmissionID != nil || !slices.Contains(r.Obligations, "strip_identity") {
			t.Errorf("a public record: %+v", r)
		}
	}
	if call(head, "GET", "/classrooms/"+c.ID+"/release", nil, &review); review.Pending != 1 {
		t.Errorf("waiting after the release: %+v", review)
	}

	// The head's terms and settings.
	licence := "https://creativecommons.org/licenses/by-nc/4.0/"
	terms := map[string]any{"licence": "CC BY-NC 4.0", "licenceUrl": licence, "attribution": "Trinity Weather, Trinity School",
		"restrictions": []string{"No commercial use", " "}, "coordinateDecimals": 3}
	off := map[string]any{"showPublic": false, "review": "manual", "autoReleaseDays": 2, "releasers": "head", "facilitatorsInvite": true, "facilitatorsManageDevices": true}
	bad := map[string]any{"showPublic": true, "review": "auto", "autoReleaseDays": 0, "releasers": "head", "facilitatorsInvite": true, "facilitatorsManageDevices": true}
	if status, no := call(head, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": bad, "terms": map[string]any{"licence": "", "licenceUrl": "http://x.example",
		"attribution": "x", "restrictions": []string{}, "coordinateDecimals": nil}}, nil); status != 400 || no.Fields["autoReleaseDays"] == "" ||
		no.Fields["licence"] == "" || no.Fields["licenceUrl"] == "" {
		t.Errorf("bad settings: %d %+v", status, no)
	}
	if status, _ := call(facilitator, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": off, "terms": terms}, nil); status != 403 {
		t.Errorf("a facilitator's settings: %d", status)
	}
	var saved classrooms.SharingInput
	if status, no := call(head, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": off, "terms": terms}, &saved); status != 200 ||
		saved.Sharing.ShowPublic || len(saved.Terms.Restrictions) != 1 {
		t.Fatalf("the head's settings: %d %+v %+v", status, saved, no)
	}
	// SAM shows the data until the revocations reach it; the public pages
	// check the setting here first.
	var revoked bool
	if err := e.Pool.QueryRow(ctx, `SELECT takes_access_away FROM sam_outbox WHERE entity = $1 ORDER BY version DESC LIMIT 1`, "classroom:"+c.ID).
		Scan(&revoked); err != nil || !revoked {
		t.Errorf("hiding the classroom takes access away: %v %v", revoked, err)
	}
	deliver(t, e, "")
	if got := public(); len(got) != 0 {
		t.Errorf("visitors after showing stopped: %+v", got)
	}
	if status, _ := call(facilitator, "POST", "/classrooms/"+c.ID+"/release", body, nil); status != 403 {
		t.Errorf("a facilitator releasing once only the head does: %d", status)
	}
	if got := samAllows(t, e, facilitator, space, roles.DatasetPublish, roles.DeviceRegister); got[roles.DatasetPublish] || !got[roles.DeviceRegister] {
		t.Errorf("the facilitator in SAM: %v", got)
	}
	policy, err := e.SAM.Service().Policy(ctx, space)
	if err != nil || policy.SpaceID != space || policy.Licence != "CC BY-NC 4.0" || policy.LicenceURL == nil || *policy.LicenceURL != licence ||
		!slices.Equal(policy.Restrictions, []string{"No commercial use"}) {
		t.Errorf("the space's policy: %+v %v", policy, err)
	}

	// Shown again; SAM loses a public grant; the reconciler puts it back.
	on := map[string]any{"showPublic": true, "review": "auto", "autoReleaseDays": 2, "releasers": "head", "facilitatorsInvite": true, "facilitatorsManageDevices": true}
	call(head, "PUT", "/classrooms/"+c.ID+"/sharing", map[string]any{"sharing": on, "terms": terms}, nil)
	deliver(t, e, "")
	if got := public(); len(got) != 2 {
		t.Errorf("visitors when shown again: %d", len(got))
	}
	anonymous := e.Access.Tenant().AnonymousID
	grants, err := e.SAM.Service().Grants(ctx, client.GrantQuery{SubjectID: &anonymous, SpaceID: &space}, client.PageQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var live []client.Grant
	for _, g := range grants.Items {
		if g.RevokedAt == nil {
			live = append(live, g)
		}
	}
	if len(live) != 2 {
		t.Fatalf("the public grants: %+v", live)
	}
	if _, err := e.SAM.Service().Revoke(ctx, live[0].ID, client.RevokeInput{}); err != nil {
		t.Fatal(err)
	}
	for _, check := range e.App.Checks() {
		if check.Name == "public" {
			if err := check.Run(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	deliver(t, e, "classroom:")
	grants, _ = e.SAM.Service().Grants(ctx, client.GrantQuery{SubjectID: &anonymous, SpaceID: &space}, client.PageQuery{Limit: 50})
	n := 0
	for _, g := range grants.Items {
		if g.RevokedAt == nil {
			n++
		}
	}
	if n != 2 {
		t.Errorf("the public grants after the reconciler: %d", n)
	}
}
