package apitest_test

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/apitest"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// samCells are the grid's cells SAM enforces, each with the actions SAM
// decides it by on the classroom's space. Whoever reads the classroom may
// also export it.
var samCells = []struct {
	name    string
	allowed func(roles.Permissions) bool
	actions []string
}{
	{"seeUnreleased", func(p roles.Permissions) bool { return p.SeeUnreleased }, []string{roles.ObservationRead, roles.ExportCreate}},
	{"openValuePage", func(p roles.Permissions) bool { return p.OpenValuePage }, []string{roles.ObservationRead}},
	{"manageConsent", func(p roles.Permissions) bool { return p.ManageConsent }, []string{roles.AttributeAssert}},
	{"manageDevices", func(p roles.Permissions) bool { return p.ManageDevices }, []string{roles.DeviceRegister, roles.DeviceMaintain}},
	{"recordReadings", func(p roles.Permissions) bool { return p.RecordReadings }, []string{roles.ObservationSubmit, roles.DeviceConnect}},
	{"releaseData", func(p roles.Permissions) bool { return p.ReleaseData }, []string{roles.DatasetPublish}},
}

// samPerson is a user registered in SAM as this service's, with minor=false
// asserted, and a client acting as them.
type samPerson struct {
	id     uuid.UUID
	client *client.Client
}

func newSAMPerson(t *testing.T, e *apitest.Env, participant int64) samPerson {
	t.Helper()
	ctx := context.Background()
	tn := e.Access.Tenant()
	issuer := tn.IssuerID()
	reg, err := e.Access.Service().Register(ctx, client.PrincipalInput{Type: "user", HomeSpaceID: tn.RootSpaceID, IssuerID: &issuer,
		Subject: strconv.FormatInt(participant, 10)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Access.Service().Assert(ctx, reg.Principal.ID, client.AssertionInput{Name: "minor", Value: "false"}); err != nil {
		t.Fatal(err)
	}
	c, err := e.Access.User(samaccess.Person{ParticipantID: participant, Registered: true})
	if err != nil {
		t.Fatal(err)
	}
	return samPerson{id: reg.Principal.ID, client: c}
}

// group registers a group homed in space, holding grants there, with
// members, and returns its ID.
func group(t *testing.T, e *apitest.Env, space uuid.UUID, grants []roles.Grant, members ...uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	svc := e.Access.Service()
	reg, err := svc.Register(ctx, client.PrincipalInput{Type: "group", HomeSpaceID: space})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range grants {
		if _, err := svc.Issue(ctx, g.Input(reg.Principal.ID, space)); err != nil {
			t.Fatalf("issuing %s: %v", g.Name, err)
		}
	}
	for _, m := range members {
		if err := svc.AddMember(ctx, reg.Principal.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	return reg.Principal.ID
}

// TestRoleGrantsInSAM: for each role, under the default and the narrowest
// sharing settings, every cell the grid marks as SAM's is what SAM decides
// for a member of that role's group, export included; a platform admin in
// the admins' group manages devices and nothing else SAM decides; each
// role adds members in SAM exactly where the grid lets it invite. The
// grid's recordReadings for a non-member admin is this service's
// difference with the front end: SAM refuses it.
func TestRoleGrantsInSAM(t *testing.T) {
	t.Parallel()
	e := apitest.New(t)
	ctx := context.Background()
	root := e.Access.Tenant().RootSpaceID
	admin := newSAMPerson(t, e, 990001)
	group(t, e, root, roles.AdminGrants(), admin.id)

	participant := int64(990100)
	for _, sharing := range []roles.Sharing{roles.DefaultSharing, {Releasers: roles.ReleasersHead}} {
		space, err := e.Access.Service().CreateSpace(ctx, client.SpaceInput{ParentID: root})
		if err != nil {
			t.Fatal(err)
		}
		type viewer struct {
			name   string
			person samPerson
			perms  roles.Permissions
		}
		viewers := []viewer{{"a platform admin", admin, roles.PermissionsFor(&roles.Viewer{PlatformAdmin: true}, "", sharing)}}
		groups, members := map[roles.Role]uuid.UUID{}, map[roles.Role]samPerson{}
		for _, role := range roles.All {
			participant++
			p := newSAMPerson(t, e, participant)
			groups[role], members[role] = group(t, e, space.ID, roles.Grants(role, sharing), p.id), p
			viewers = append(viewers, viewer{string(role), p, roles.PermissionsFor(&roles.Viewer{}, role, sharing)})
		}
		// A researcher's grant hands nothing on, to anyone.
		for _, to := range []uuid.UUID{admin.id, e.Access.Tenant().AnonymousID} {
			in := client.GrantInput{SubjectID: to, SpaceID: space.ID, Use: []string{roles.ObservationRead}}
			if _, err := members[roles.Researcher].client.Issue(ctx, in); err == nil {
				t.Errorf("a researcher issued a read grant to %s", to)
			}
		}
		// SAM lets a member add members to a group exactly where the grid
		// lets them invite.
		for _, adder := range roles.All {
			invite := roles.PermissionsFor(&roles.Viewer{}, adder, sharing).Invite
			for _, role := range roles.All {
				participant++
				newcomer := newSAMPerson(t, e, participant)
				err := members[adder].client.AddMember(ctx, groups[role], newcomer.id)
				if want := slices.Contains(invite, role); (err == nil) != want {
					t.Errorf("under %+v, %s adding a %s: %v", sharing, adder, role, err)
				}
			}
		}
		for _, v := range viewers {
			var checks []client.Check
			for _, cell := range samCells {
				for _, a := range cell.actions {
					checks = append(checks, client.Check{Action: a, SpaceID: space.ID})
				}
			}
			results, err := v.person.client.Check(ctx, checks)
			if err != nil {
				t.Fatal(err)
			}
			i := 0
			for _, cell := range samCells {
				for _, a := range cell.actions {
					got, want := results[i].Allowed, cell.allowed(v.perms)
					i++
					if v.name == "a platform admin" && cell.name == "recordReadings" {
						want = false
					}
					if got != want {
						t.Errorf("%s under %+v: %s (%s) is %v in SAM, %v in the grid: %s", v.name, sharing, cell.name, a, got, want, results[i-1].Reason)
					}
				}
			}
		}
	}
}
