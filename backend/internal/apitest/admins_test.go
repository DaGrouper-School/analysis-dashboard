package apitest_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// TestPlatformAdmins: an operator makes someone a platform admin, who then
// holds the admins' grants at the tenant root in SAM, managing any
// classroom's devices and terms and reading nothing; unmaking them takes
// it away here at once and in SAM; each change is logged; the reconciler
// puts back what SAM lost.
func TestPlatformAdmins(t *testing.T) {
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
	p := signedIn(t, e, apitest.Person{Sub: "g-amina", Email: "amina@dagrouper.example", Name: "Amina Yusuf"})
	if _, _, err := accounts.SetAdmin(ctx, e.Pool, now, settings.Operator, "nobody@dagrouper.example", true); err == nil {
		t.Fatal("an unknown account was made an admin")
	}
	u, changed, err := accounts.SetAdmin(ctx, e.Pool, now, settings.Operator, "Amina@DaGrouper.example", true)
	if err != nil || !changed || !u.PlatformAdmin {
		t.Fatalf("made an admin: %+v %v %v", u, changed, err)
	}
	if _, changed, _ := accounts.SetAdmin(ctx, e.Pool, now, settings.Operator, p.id, true); changed {
		t.Error("made an admin twice")
	}

	// The admin creates a classroom, whose space SAM has.
	if res, _ := e.DoWith(t, p.client, "POST", "/api/classrooms", `{"name": "Trinity Weather", "institutionName": "Trinity School", "headEmail": "h@t.example"}`); res.StatusCode != 200 {
		t.Fatalf("a classroom by the admin: %d", res.StatusCode)
	}
	deliver(t, e, "admin")
	deliver(t, e, "classroom:")
	var id uuid.UUID
	if err := e.Pool.QueryRow(ctx, `SELECT id FROM classrooms`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	stored, _ := classrooms.Get(ctx, e.Pool, id)
	root := e.Access.Tenant().RootSpaceID
	actions := []string{roles.DeviceRegister, roles.DeviceMaintain, roles.DatasetSetPolicy, roles.ObservationRead, roles.ObservationSubmit, roles.DatasetPublish}
	want := map[string]bool{roles.DeviceRegister: true, roles.DeviceMaintain: true, roles.DatasetSetPolicy: true}
	for name, space := range map[string]uuid.UUID{"the tenant root": root, "a classroom": *stored.SpaceID} {
		got := samAllows(t, e, p, space, actions...)
		for _, a := range actions {
			if got[a] != want[a] {
				t.Errorf("an admin on %s: %s is %v in SAM", name, a, got[a])
			}
		}
	}

	// SAM loses the membership; the reconciler puts it back.
	var group uuid.UUID
	if err := e.Pool.QueryRow(ctx, `SELECT group_id FROM tenant_groups WHERE name = 'admins'`).Scan(&group); err != nil {
		t.Fatal(err)
	}
	admin, _ := accounts.Get(ctx, e.Pool, uuid.MustParse(p.id))
	if !admin.SAMAdmin {
		t.Error("SAM's membership was not recorded")
	}
	if err := e.SAM.Service().RemoveMember(ctx, group, *admin.Principal); err != nil {
		t.Fatal(err)
	}
	for _, check := range e.App.Checks() {
		if check.Name == "admins" {
			if err := check.Run(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	deliver(t, e, "admin:")
	if got := samAllows(t, e, p, root, roles.DeviceRegister); !got[roles.DeviceRegister] {
		t.Error("the reconciler did not put the admin back")
	}

	// Unmade: here at once, then in SAM.
	if _, changed, err := accounts.SetAdmin(ctx, e.Pool, now, settings.Operator, p.id, false); err != nil || !changed {
		t.Fatalf("unmade: %v %v", changed, err)
	}
	if res, _ := e.DoWith(t, p.client, "GET", "/api/admin/stages", ""); res.StatusCode != 403 {
		t.Errorf("a former admin's admin page: %d", res.StatusCode)
	}
	var takesAway bool
	if err := e.Pool.QueryRow(ctx, `SELECT takes_access_away FROM sam_outbox WHERE entity = $1 ORDER BY version DESC LIMIT 1`, "admin:"+p.id).
		Scan(&takesAway); err != nil || !takesAway {
		t.Errorf("unmaking takes access away: %v, %v", takesAway, err)
	}
	deliver(t, e, "admin:")
	if got := samAllows(t, e, p, root, roles.DeviceRegister); got[roles.DeviceRegister] {
		t.Error("a former admin still registers devices in SAM")
	}
	log, err := settings.Changes(ctx, e.Pool, 5)
	if err != nil || len(log) < 2 || log[0].Summary != "Amina Yusuf (amina@dagrouper.example) is no longer a platform admin" ||
		log[1].Summary != "Made Amina Yusuf (amina@dagrouper.example) a platform admin" || log[0].Actor != settings.Operator {
		t.Errorf("the log: %+v, %v", log, err)
	}
}
