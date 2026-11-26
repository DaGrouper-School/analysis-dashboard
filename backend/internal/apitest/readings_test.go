package apitest_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/consent"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestReadingUpload: a head uploads a device's readings, which SAM stores
// unsigned as the head, one with a GPS fix and one at the lab's site; a
// resend stores nothing twice; a value the device type does not send, a
// date too far ahead and those who may not record are refused; with
// future_observations at refuse, a reading dated ahead is refused; a minor
// is refused until their guardian gives consent, and their GPS fix
// reaches SAM rounded.
func TestReadingUpload(t *testing.T) {
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
	if _, err := classrooms.SeedConsent(ctx, e.Pool, now); err != nil {
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
	minor := signedIn(t, e, apitest.Person{Sub: "g-sara", Email: "sara@trinity.example", Name: "Sara Tadesse"})
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"sara@trinity.example"}, "role": "community_scientist"}, nil)
	call(minor, "POST", "/invitations/by-token/"+tokenFor(t, e, "sara@trinity.example")+"/accept", nil, nil)
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)
	var station labs.DeviceView
	if status, no := call(head, "POST", "/labs/"+lab.ID.String()+"/devices", map[string]any{"name": "Pilot station", "type": "sen0658", "identifier": "pilot-01"}, &station); status != 200 {
		t.Fatalf("a device: %d %+v", status, no)
	}
	path := "/devices/" + station.ID.String() + "/readings"
	at := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)
	readings := []map[string]any{
		{"observedAt": at.Format(time.RFC3339), "values": map[string]any{"air_temperature": 21.5}, "location": map[string]any{"lat": 9.031, "lng": 38.741}},
		{"observedAt": at.Add(time.Minute).Format(time.RFC3339), "values": map[string]any{"air_temperature": 22}, "location": nil},
	}
	if status, no := call(head, "POST", path, map[string]any{"readings": readings, "credit": true}, nil); status != 409 {
		t.Errorf("before the device is in SAM: %d %+v", status, no)
	}
	// The head is an adult, as onboarding tells SAM; SAM takes a person it
	// holds no minor for as a minor.
	if _, err := accounts.SetMinor(ctx, e.Pool, time.Now(), uuid.MustParse(head.id), false); err != nil {
		t.Fatal(err)
	}
	deliver(t, e, "")

	var got []labs.Reading
	if status, no := call(head, "POST", path, map[string]any{"readings": readings, "credit": true}, &got); status != 200 || len(got) != 2 ||
		got[0].LocationSource != labs.FromDevice || got[1].LocationSource != labs.FromLab || got[1].Location.Lat != 9.03 ||
		got[0].CreditedTo == nil || *got[0].CreditedTo != "Hana Bekele" || got[0].RecordedByParticipantID == nil || *got[0].RecordedByParticipantID != head.pid ||
		got[0].ClassroomID.String() != c.ID {
		t.Fatalf("uploaded: %d %+v %+v", status, got, no)
	}
	var again []labs.Reading
	if call(head, "POST", path, map[string]any{"readings": readings, "credit": true}, &again); len(again) != 2 || again[0].ID != got[0].ID || again[1].ID != got[1].ID {
		t.Errorf("resent: %+v", again)
	}
	var kept int
	if err := e.Pool.QueryRow(ctx, `SELECT count(*) FROM relayed_records WHERE form = 'upload' AND credit_user_id IS NOT NULL`).Scan(&kept); err != nil || kept != 2 {
		t.Errorf("relayed records: %d %v", kept, err)
	}
	stored, _ := labs.Get(ctx, e.Pool, lab.ID)
	samReadings := func(p *profile) map[uuid.UUID]client.Reading {
		t.Helper()
		as, err := e.Access.User(samaccess.Person{ParticipantID: p.pid, Registered: true})
		if err != nil {
			t.Fatal(err)
		}
		page, err := as.Readings(ctx, client.ObservationQuery{SpaceID: *stored.SpaceID}, client.PageQuery{})
		if err != nil {
			t.Fatal(err)
		}
		out := map[uuid.UUID]client.Reading{}
		for _, r := range page.Items {
			out[r.ReadingID] = r
		}
		return out
	}
	held := samReadings(head)
	first, ok := held[got[0].ID]
	if !ok || len(held) != 2 || first.DeviceID == nil || first.RecordedBy == nil || !strings.Contains(string(first.Location), "38.741") {
		t.Fatalf("in SAM: %+v", held)
	}
	if v := first.Values["air_temperature"]; v.Value != 21.5 || !strings.Contains(strings.Join(v.Flags, ","), "unsigned") {
		t.Errorf("the value in SAM: %+v", v)
	}
	if v := held[got[1].ID].Values["air_temperature"]; !strings.Contains(strings.Join(v.Flags, ","), "location_from_site") {
		t.Errorf("the lab's site in SAM: %+v", v)
	}

	// Refusals.
	one := func(values map[string]any, at time.Time) map[string]any {
		return map[string]any{"readings": []map[string]any{{"observedAt": at.Format(time.RFC3339), "values": values, "location": nil}}, "credit": false}
	}
	if status, no := call(head, "POST", path, one(map[string]any{"soil_moisture": 3}, at), nil); status != 400 || no.Fields["values.soil_moisture"] == "" ||
		!strings.HasPrefix(no.Message, "Reading 1: ") {
		t.Errorf("a value the type does not send: %d %+v", status, no)
	}
	if status, no := call(head, "POST", path, one(map[string]any{"air_temperature": 20}, time.Now().Add(48*time.Hour)), nil); status != 400 || no.Fields["observedAt"] == "" {
		t.Errorf("two days ahead: %d %+v", status, no)
	}
	if status, _ := call(head, "POST", path, one(map[string]any{"air_temperature": 20}, time.Now().Add(30*time.Minute)), nil); status != 200 {
		t.Errorf("half an hour ahead, flagged: %d", status)
	}
	if _, err := settings.Put(ctx, e.Pool, now, settings.Operator, settings.KeyFuture, []byte(`{"action": "refuse"}`)); err != nil {
		t.Fatal(err)
	}
	if status, no := call(head, "POST", path, one(map[string]any{"air_temperature": 20}, time.Now().Add(40*time.Minute)), nil); status != 400 || no.Fields["observedAt"] != "It is dated in the future." {
		t.Errorf("ahead, refused: %d %+v", status, no)
	}
	if status, _ := call(stranger, "POST", path, one(map[string]any{"air_temperature": 20}, at), nil); status != 404 {
		t.Errorf("someone else: %d", status)
	}
	if status, no := call(a, "POST", path, one(map[string]any{"air_temperature": 20}, at), nil); status != 403 {
		t.Errorf("a platform admin: %d %+v", status, no)
	}

	// A minor: refused, then rounded once their guardian consents.
	if err := pgx.BeginFunc(ctx, e.Pool, func(tx pgx.Tx) error {
		_, _, err := consent.Ask(ctx, tx, time.Now(), uuid.MustParse(minor.id), consent.GuardianInput{GuardianName: "Almaz", GuardianContact: "almaz@home.example"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	fix := map[string]any{"readings": []map[string]any{{"observedAt": at.Add(2 * time.Minute).Format(time.RFC3339),
		"values": map[string]any{"air_temperature": 23}, "location": map[string]any{"lat": 9.03456, "lng": 38.74321}}}, "credit": true}
	if status, no := call(minor, "POST", path, fix, nil); status != 403 || !strings.Contains(no.Message, "guardian needs to give consent") {
		t.Errorf("a minor without consent: %d %+v", status, no)
	}
	// Decided on the link by a token set here, since it was asked for without the endpoint.
	if _, err := e.Pool.Exec(ctx, `UPDATE guardian_links SET token_hash = sha256('tok') WHERE user_id = $1 AND voided_at IS NULL`, minor.id); err != nil {
		t.Fatal(err)
	}
	if _, err := consent.Decide(ctx, e.Pool, time.Now(), "tok", true, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	deliver(t, e, "user:")
	var mine []labs.Reading
	if status, no := call(minor, "POST", path, fix, &mine); status != 200 || len(mine) != 1 || mine[0].Location.Lat != 9.03 || mine[0].Location.Lng != 38.74 ||
		mine[0].CreditedTo != nil {
		t.Fatalf("a minor with consent: %d %+v %+v", status, mine, no)
	}
	if r := samReadings(head)[mine[0].ID]; !strings.Contains(string(r.Location), "38.74") || strings.Contains(string(r.Location), "38.743") {
		t.Errorf("the minor's position in SAM: %s", r.Location)
	}
}
