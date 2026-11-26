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
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// tokenFor is the token of the newest invitation emailed to address.
func tokenFor(t *testing.T, e *apitest.Env, address string) string {
	t.Helper()
	sent := e.Email.Sent()
	for i := len(sent) - 1; i >= 0; i-- {
		if sent[i].To != address {
			continue
		}
		_, after, ok := strings.Cut(sent[i].Text, apitest.FrontEnd+"/invite/")
		if !ok {
			t.Fatalf("no link in %q", sent[i].Text)
		}
		token, _, _ := strings.Cut(after, "\n")
		return token
	}
	t.Fatalf("no invitation emailed to %s", address)
	return ""
}

// samAllows reports what SAM allows p on space, by action.
func samAllows(t *testing.T, e *apitest.Env, p *profile, space uuid.UUID, actions ...string) map[string]bool {
	t.Helper()
	c, err := e.Access.User(samaccess.Person{ParticipantID: p.pid, Registered: true})
	if err != nil {
		t.Fatal(err)
	}
	checks := make([]client.Check, len(actions))
	for i, a := range actions {
		checks[i] = client.Check{Action: a, SpaceID: space}
	}
	results, err := c.Check(context.Background(), checks)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range results {
		out[r.Action] = r.Allowed
	}
	return out
}

// TestInvitationsAndMembers: a new classroom invites its head as this
// service; the head invites a facilitator, who invites a community
// scientist, each joining SAM's group as the one who invited them; a role
// change and leaving change only that member's group; an expired or
// orphaned invitation is refused.
func TestInvitationsAndMembers(t *testing.T) {
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
	scientist := signedIn(t, e, apitest.Person{Sub: "g-sara", Email: "sara@trinity.example", Name: "Sara Tadesse"})
	stranger := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})

	var c classrooms.Summary
	if status, _ := call(a, "POST", "/classrooms", map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "Hana@Trinity.example"}, &c); status != 200 {
		t.Fatalf("a classroom: %d", status)
	}
	token := tokenFor(t, e, "hana@trinity.example")
	var preview classrooms.Preview
	if status, _ := call(stranger, "GET", "/invitations/by-token/"+token, nil, &preview); status != 200 || preview.Role != roles.Head ||
		preview.ClassroomName != "Trinity Weather" {
		t.Fatalf("the head's invitation: %d %+v", status, preview)
	}
	if status, no := call(stranger, "POST", "/invitations/by-token/"+token+"/accept", nil, nil); status != 403 || !strings.Contains(no.Message, "hana@trinity.example") {
		t.Errorf("accepted by someone else: %d %+v", status, no)
	}
	var mine []classrooms.Preview
	if call(head, "GET", "/me/invitations", nil, &mine); len(mine) != 1 || mine[0].Token != "" {
		t.Errorf("the head's invitations: %+v", mine)
	}
	if status, _ := call(head, "POST", "/invitations/by-token/"+token+"/accept", nil, &c); status != 200 || *c.MyRole != roles.Head {
		t.Fatalf("the head joins: %d %+v", status, c)
	}
	if status, _ := call(head, "POST", "/invitations/by-token/"+token+"/accept", nil, nil); status != 409 {
		t.Errorf("accepted twice: %d", status)
	}

	// The head invites a facilitator, who invites a community scientist.
	var sent []classrooms.InvitationView
	if status, no := call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"dawit@trinity.example"}, "role": "facilitator"}, &sent); status != 200 ||
		len(sent) != 1 || sent[0].Token != "" {
		t.Fatalf("a facilitator invited: %d %+v %+v", status, sent, no)
	}
	if status, _ := call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"x@y.example"}, "role": "institution_head"}, nil); status != 403 {
		t.Errorf("a head invited by a head: %d", status)
	}
	if status, _ := call(facilitator, "POST", "/invitations/by-token/"+tokenFor(t, e, "dawit@trinity.example")+"/accept", nil, nil); status != 200 {
		t.Fatalf("the facilitator joins: %d", status)
	}
	if status, _ := call(facilitator, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"sara@trinity.example"}, "role": "community_scientist"}, nil); status != 200 {
		t.Fatalf("a community scientist invited: %d", status)
	}
	if status, _ := call(facilitator, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"r@y.example"}, "role": "researcher"}, nil); status != 403 {
		t.Errorf("a researcher invited by a facilitator: %d", status)
	}
	if status, _ := call(scientist, "POST", "/invitations/by-token/"+tokenFor(t, e, "sara@trinity.example")+"/accept", nil, nil); status != 200 {
		t.Fatalf("the community scientist joins: %d", status)
	}

	deliver(t, e, "member:")
	stored, _ := classrooms.Get(ctx, e.Pool, uuid.MustParse(c.ID))
	space := *stored.SpaceID
	actions := []string{roles.ObservationRead, roles.DeviceConnect, roles.AttributeAssert, roles.DatasetPublish}
	for _, w := range []struct {
		name string
		p    *profile
		want map[string]bool
	}{
		{"the head", head, map[string]bool{roles.ObservationRead: true, roles.DeviceConnect: true, roles.AttributeAssert: true, roles.DatasetPublish: true}},
		{"the facilitator", facilitator, map[string]bool{roles.ObservationRead: true, roles.DeviceConnect: true, roles.AttributeAssert: true, roles.DatasetPublish: true}},
		{"the community scientist", scientist, map[string]bool{roles.ObservationRead: true, roles.DeviceConnect: true}},
		// Anyone reads the classroom's published records, through its
		// public grant; nothing else.
		{"a stranger", stranger, map[string]bool{roles.ObservationRead: true}},
	} {
		got := samAllows(t, e, w.p, space, actions...)
		for _, act := range actions {
			if got[act] != w.want[act] {
				t.Errorf("%s: %s is %v in SAM", w.name, act, got[act])
			}
		}
	}
	// The community scientist's membership was sent as the facilitator.
	var actor *uuid.UUID
	if err := e.Pool.QueryRow(ctx, `SELECT actor_id FROM sam_outbox WHERE entity = $1 ORDER BY version DESC LIMIT 1`,
		"member:"+c.ID+":"+scientist.id).Scan(&actor); err != nil || actor == nil || actor.String() != facilitator.id {
		t.Fatalf("the community scientist's membership was sent as %v: %v", actor, err)
	}

	// A role change moves them to one group only; leaving takes them out.
	if status, _ := call(facilitator, "PATCH", "/classrooms/"+c.ID+"/members/"+scientist.id, map[string]any{"role": "researcher"}, nil); status != 403 {
		t.Errorf("a role changed by a facilitator: %d", status)
	}
	if status, _ := call(head, "PATCH", "/classrooms/"+c.ID+"/members/"+scientist.id, map[string]any{"role": "researcher"}, nil); status != 204 {
		t.Fatalf("a role changed by the head: %d", status)
	}
	deliver(t, e, "member:")
	if got := samAllows(t, e, scientist, space, roles.ObservationRead, roles.DeviceConnect); !got[roles.ObservationRead] || got[roles.DeviceConnect] {
		t.Errorf("a researcher now: %v", got)
	}
	if status, _ := call(scientist, "DELETE", "/classrooms/"+c.ID+"/members/"+scientist.id, nil, nil); status != 204 {
		t.Fatalf("leaving: %d", status)
	}
	if status, _ := call(scientist, "GET", "/classrooms/"+c.ID, nil, nil); status != 404 {
		t.Errorf("a classroom after leaving it: %d", status)
	}
	deliver(t, e, "member:")
	// A researcher who left exports nothing; reading published records
	// stays open to anyone.
	if got := samAllows(t, e, scientist, space, roles.ExportCreate); got[roles.ExportCreate] {
		t.Errorf("export after leaving: %v", got)
	}
	var d classrooms.Detail
	if call(head, "GET", "/classrooms/"+c.ID, nil, &d); len(d.Members) != 2 || d.Members[0].Role != roles.Head || d.Members[1].Role != roles.Facilitator {
		t.Errorf("the members: %+v", d.Members)
	}

	// An expired invitation, then one whose inviter is gone.
	call(head, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"late@trinity.example"}, "role": "researcher"}, nil)
	late := tokenFor(t, e, "late@trinity.example")
	e.Clock.Add(classrooms.DefaultInvitationTTL + time.Hour)
	if status, no := call(stranger, "GET", "/invitations/by-token/"+late, nil, nil); status != 404 || !strings.Contains(no.Message, "expired") {
		t.Errorf("an expired invitation: %d %+v", status, no)
	}
	call(facilitator, "POST", "/classrooms/"+c.ID+"/invitations", map[string]any{"emails": []string{"kofi@example.org"}, "role": "community_scientist"}, nil)
	orphan := tokenFor(t, e, "kofi@example.org")
	if _, err := e.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, facilitator.id); err != nil {
		t.Fatal(err)
	}
	if status, _ := call(stranger, "POST", "/invitations/by-token/"+orphan+"/accept", nil, nil); status != 404 {
		t.Errorf("an orphaned invitation: %d", status)
	}
}
