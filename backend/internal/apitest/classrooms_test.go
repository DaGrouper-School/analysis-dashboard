package apitest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestClassrooms: platform admins create classrooms, each a space under
// the tenant root in SAM with its four role groups and their grants,
// whose IDs the classroom records once SAM confirms them; only those who
// may see a classroom see it; admins edit and configure it.
func TestClassrooms(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := settings.SeedSettings(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	if _, err := classrooms.SeedLists(ctx, e.Pool, now); err != nil {
		t.Fatal(err)
	}
	someone := signedIn(t, e, apitest.Person{Sub: "g-kofi", Email: "kofi@example.org", Name: "Kofi Mensah"})
	in := map[string]any{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "head@trinity.example"}
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
	if status, _ := call(someone, "POST", "/classrooms", in, nil); status != http.StatusForbidden {
		t.Fatalf("created by someone else: %d", status)
	}
	a := admin(t, e)
	if status, no := call(a, "POST", "/classrooms", map[string]any{"name": "ab", "institutionName": "x", "headEmail": "no"}, nil); status != 400 ||
		no.Fields["name"] == "" || no.Fields["headEmail"] == "" {
		t.Errorf("a bad classroom: %d %+v", status, no)
	}
	var c classrooms.Summary
	if status, no := call(a, "POST", "/classrooms", in, &c); status != 200 || c.Code == nil || c.Setup != "admin_assisted" || c.MyRole != nil ||
		c.CanRecord || c.MemberCount != 0 {
		t.Fatalf("a classroom: %d %+v %+v", status, c, no)
	}

	deliver(t, e, "classroom:")
	stored, err := classrooms.Get(ctx, e.Pool, uuid.MustParse(c.ID))
	if err != nil || stored.SpaceID == nil || len(stored.Groups) != 4 {
		t.Fatalf("the IDs SAM confirmed: %+v, %v", stored, err)
	}
	svc := e.SAM.Service()
	if space, err := svc.Space(ctx, *stored.SpaceID); err != nil || *space.ParentID != e.Access.Tenant().RootSpaceID {
		t.Fatalf("the space: %+v, %v", space, err)
	}
	for _, role := range roles.All {
		group := stored.Groups[role]
		held, err := svc.Grants(ctx, client.GrantQuery{SubjectID: &group}, client.PageQuery{Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		var labels []string
		for _, g := range held.Items {
			labels = append(labels, g.Labels...)
		}
		for _, g := range roles.Grants(role, roles.DefaultSharing) {
			if !slices.Contains(labels, g.Name) {
				t.Errorf("%s's group lacks %s: %v", role, g.Name, labels)
			}
		}
		if len(held.Items) != len(roles.Grants(role, roles.DefaultSharing)) {
			t.Errorf("%s's group holds %d grants", role, len(held.Items))
		}
	}

	var mine []classrooms.Summary
	if call(a, "GET", "/classrooms", nil, &mine); len(mine) != 1 || mine[0].ID != c.ID {
		t.Errorf("an admin's classrooms: %+v", mine)
	}
	if call(someone, "GET", "/classrooms", nil, &mine); len(mine) != 0 {
		t.Errorf("someone else's classrooms: %+v", mine)
	}
	if status, _ := call(someone, "GET", "/classrooms/"+c.ID, nil, nil); status != 404 {
		t.Errorf("seen by someone else: %d", status)
	}
	var d classrooms.Detail
	if status, _ := call(a, "GET", "/classrooms/"+c.ID, nil, &d); status != 200 || !d.Permissions.Edit || d.Permissions.SeeUnreleased ||
		d.Sharing == nil || d.Sharing.Terms.Attribution != "Trinity Weather, Trinity School. DaGrouper Global Classrooms." || len(d.Members) != 0 {
		t.Errorf("seen by an admin: %d %+v", status, d)
	}
	if status, _ := call(a, "PATCH", "/classrooms/"+c.ID, map[string]any{"name": "Trinity Rain", "institutionName": "Trinity School", "history": "Since 1950."}, &c); status != 200 ||
		c.Name != "Trinity Rain" {
		t.Errorf("an edit: %d %+v", status, c)
	}
	if status, _ := call(someone, "PATCH", "/classrooms/"+c.ID, map[string]any{"name": "Mine", "institutionName": "Mine"}, nil); status != 404 {
		t.Errorf("an edit by someone else: %d", status)
	}

	var conf classrooms.Configuration
	if status, no := call(a, "PATCH", "/admin/classrooms/"+c.ID, map[string]any{"stageId": "pilot", "profile": map[string]any{"hostAgreementSigned": true}}, &conf); status != 200 ||
		conf.StageID != "pilot" || !conf.Profile.HostAgreementSigned || conf.Profile.Institution.History != "Since 1950." {
		t.Errorf("configured: %d %+v %+v", status, conf, no)
	}
	if status, no := call(a, "PATCH", "/admin/classrooms/"+c.ID, map[string]any{"regionId": "atlantis"}, nil); status != 400 || no.Fields["regionId"] == "" {
		t.Errorf("an unknown region: %d %+v", status, no)
	}
	if status, _ := call(someone, "PATCH", "/admin/classrooms/"+c.ID, map[string]any{}, nil); status != 403 {
		t.Errorf("configured by someone else: %d", status)
	}
	if status, _ := call(a, "DELETE", "/admin/stages/pilot", nil, nil); status != 409 {
		t.Errorf("the classroom's stage deleted: %d", status)
	}
}
