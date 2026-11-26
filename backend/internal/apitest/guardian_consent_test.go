package apitest_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/consent"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestGuardianConsentByLink: a minor in a classroom names a guardian, who
// is emailed a link; the guardian's page shows the current consent text,
// filled in for the classroom, and a decision on an older version is
// refused; SAM refuses the minor's submissions until the guardian gives
// consent, takes them after, and refuses them again once the minor asks
// again, which voids the earlier link.
func TestGuardianConsentByLink(t *testing.T) {
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
	linkFor := func(address string) string {
		t.Helper()
		sent := e.Email.Sent()
		for i := len(sent) - 1; i >= 0; i-- {
			if _, after, ok := strings.Cut(sent[i].Text, apitest.FrontEnd+"/consent/"); ok && sent[i].To == address {
				token, _, _ := strings.Cut(after, "\n")
				return token
			}
		}
		t.Fatalf("no consent link emailed to %s", address)
		return ""
	}

	// A classroom with a lab, its head, and a community scientist under 18.
	a := admin(t, e)
	head := signedIn(t, e, apitest.Person{Sub: "g-hana", Email: "hana@trinity.example", Name: "Hana Bekele"})
	minor := signedIn(t, e, apitest.Person{Sub: "g-sara", Email: "sara@trinity.example", Name: "Sara Tadesse"})
	var c classrooms.Summary
	call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "hana@trinity.example"}, &c)
	call(head, "POST", "/invitations/by-token/"+tokenFor(t, e, "hana@trinity.example")+"/accept", nil, nil)
	call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"sara@trinity.example"}, "role": "community_scientist"}, nil)
	if status, _ := call(minor, "POST", "/invitations/by-token/"+tokenFor(t, e, "sara@trinity.example")+"/accept", nil, nil); status != 200 {
		t.Fatalf("the minor joins: %d", status)
	}
	var lab labs.Lab
	call(head, "POST", "/classrooms/"+c.ID+"/labs", map[string]any{"name": "Trinity garden", "placeName": "Trinity School", "country": "Ethiopia",
		"location": map[string]any{"lat": 9.03, "lng": 38.74}, "siteProfile": map[string]any{"setting": "urban", "description": "A garden behind the school.",
			"researchQuestions": []string{"How much rain falls?"}}}, &lab)

	if status, no := call(minor, "POST", "/me/guardian-consent", map[string]any{"guardianName": "Almaz", "guardianContact": "nope"}, nil); status != 400 ||
		no.Fields["guardianContact"] == "" {
		t.Errorf("a bad guardian: %d %+v", status, no)
	}
	var mine *consent.View
	if call(minor, "GET", "/me/guardian-consent", nil, &mine); mine != nil {
		t.Errorf("before asking: %+v", mine)
	}
	var asked consent.View
	if status, no := call(minor, "POST", "/me/guardian-consent", map[string]any{"guardianName": " Almaz Tadesse ", "guardianContact": "Almaz@Home.example"}, &asked); status != 200 ||
		asked.Status != "pending" || asked.GuardianContact != "almaz@home.example" || asked.DevToken != nil || asked.Method != "link" {
		t.Fatalf("asked: %d %+v %+v", status, asked, no)
	}
	token := linkFor("almaz@home.example")
	deliver(t, e, "")

	stored, _ := labs.Get(ctx, e.Pool, lab.ID)
	submit := func() bool {
		t.Helper()
		as, err := e.Access.User(samaccess.Person{ParticipantID: minor.pid, Registered: true})
		if err != nil {
			t.Fatal(err)
		}
		out, err := as.SubmitObservations(ctx, client.ObservationBatch{SpaceID: *stored.SpaceID, Observations: []client.Observation{{
			ID: uuid.New(), ParameterID: "air_temperature", Value: 21.5, ObservedAt: time.Now().Add(-time.Minute).UTC(), Procedure: "manual"}}})
		return err == nil && out.Stored == 1
	}
	if submit() {
		t.Error("SAM took a minor's submission before consent")
	}

	// The guardian's page, then their decision.
	var req consent.Request
	if status, _ := call(nil, "GET", "/consent/"+token+"?language=am", nil, &req); status != 200 || req.ContributorName != "Sara" ||
		req.GuardianName != "Almaz Tadesse" || len(req.ClassroomNames) != 1 || req.ClassroomNames[0] != "Trinity Weather" || req.Status != "pending" ||
		req.Version != 1 || !strings.Contains(req.Text.Intro, "Trinity School runs Trinity Weather") || strings.Contains(req.Text.Intro, "{") {
		t.Fatalf("the guardian's page: %d %+v", status, req)
	}
	if status, no := call(nil, "POST", "/consent/"+token, map[string]any{"give": true, "version": 0}, nil); status != 409 || no.Code != "consent_text_changed" {
		t.Errorf("an older text: %d %+v", status, no)
	}
	if status, _ := call(nil, "POST", "/consent/"+token, map[string]any{"give": true}, nil); status != 400 {
		t.Errorf("no version: %d", status)
	}
	if status, _ := call(nil, "POST", "/consent/"+token, map[string]any{"give": true, "version": req.Version}, &req); status != 200 || req.Status != "given" || req.DecidedAt == nil {
		t.Fatalf("given: %d %+v", status, req)
	}
	if call(minor, "GET", "/me/guardian-consent", nil, &mine); mine == nil || mine.Status != "given" || mine.DecidedAt == nil {
		t.Errorf("the minor's view: %+v", mine)
	}
	deliver(t, e, "user:")
	if got := held(t, e, minor.pid); got["minor"] != "true" || got["consent"] != "given" {
		t.Errorf("SAM holds %v", got)
	}
	if !submit() {
		t.Error("SAM refused a minor's submission with consent")
	}

	// Asking again voids the link; consent is pending here and withdrawn in SAM.
	call(minor, "POST", "/me/guardian-consent", map[string]any{"guardianName": "Almaz Tadesse", "guardianContact": "+251 911 234 567"}, &asked)
	if asked.Status != "pending" || asked.GuardianContact != "+251911234567" {
		t.Errorf("asked again: %+v", asked)
	}
	if sent := e.SMS.Sent(); len(sent) == 0 || sent[len(sent)-1].To != "+251911234567" || !strings.Contains(sent[len(sent)-1].Text, apitest.FrontEnd+"/consent/") {
		t.Errorf("the text: %+v", sent)
	}
	if status, _ := call(nil, "GET", "/consent/"+token, nil, nil); status != 404 {
		t.Errorf("the voided link: %d", status)
	}
	deliver(t, e, "user:")
	if got := held(t, e, minor.pid); got["consent"] != "withdrawn" {
		t.Errorf("SAM holds %v after asking again", got)
	}
	if submit() {
		t.Error("SAM took a submission after asking again")
	}
}
