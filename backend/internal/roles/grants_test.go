package roles

import (
	"slices"
	"testing"
)

// sharings are the settings that change a role's grants.
var sharings = []Sharing{
	DefaultSharing,
	{Releasers: ReleasersHead},
	{FacilitatorsManageDevices: true, Releasers: ReleasersHead},
	{FacilitatorsInvite: true, Releasers: ReleasersHead},
	{Releasers: ReleasersHeadAndFacils},
}

// TestHandingOn: under any sharing settings, a role can hand on a group's
// grants exactly where the grid lets it invite: for each grant the group
// holds, one of the role's delegates every action it uses and hands on,
// with more depth, and with no rule the held grant lacks. The service adds
// heads from its tenant grant, so every group's grants fit under it with
// depth to spare.
func TestHandingOn(t *testing.T) {
	t.Parallel()
	for _, s := range sharings {
		for _, adder := range All {
			invite := PermissionsFor(&Viewer{}, adder, s).Invite
			for _, role := range All {
				if got, want := HandsOn(adder, role, s), slices.Contains(invite, role); got != want {
					t.Errorf("under %+v, %s hands on %s's grants: %v, and the grid says %v", s, adder, role, got, want)
				}
			}
		}
		for _, role := range All {
			for _, g := range Grants(role, s) {
				if g.Depth >= TenantDepth-1 {
					t.Errorf("%s's %s has depth %d, with none to spare under the tenant grant's %d", role, g.Name, g.Depth, TenantDepth)
				}
			}
		}
	}
	for _, g := range AdminGrants() {
		if g.Depth >= TenantDepth || len(g.Delegate) > 0 {
			t.Errorf("the admins' %s: %+v", g.Name, g)
		}
	}
}

// TestGrants: what each group reads, records, manages and releases, and
// the grants that carry rules.
func TestGrants(t *testing.T) {
	t.Parallel()
	uses := func(r Role, s Sharing) []string {
		var out []string
		for _, g := range Grants(r, s) {
			out = union(out, g.Use)
		}
		return out
	}
	narrow := Sharing{Releasers: ReleasersHead}
	for _, c := range []struct {
		role Role
		s    Sharing
		want []string
	}{
		{Researcher, DefaultSharing, []string{ObservationRead, ExportCreate}},
		{CommunityScientist, DefaultSharing, []string{ObservationRead, ObservationSubmit, DeviceConnect, ExportCreate}},
		{Facilitator, narrow, []string{ObservationRead, ObservationSubmit, DeviceConnect, ExportCreate, AttributeAssert}},
		{Facilitator, DefaultSharing, []string{ObservationRead, ObservationSubmit, DeviceConnect, DeviceRegister, DeviceMaintain, DatasetPublish, ExportCreate, AttributeAssert}},
		{Head, narrow, []string{ObservationRead, ObservationSubmit, DeviceConnect, DeviceRegister, DeviceMaintain, DatasetPublish, ExportCreate, AttributeAssert}},
	} {
		if got := uses(c.role, c.s); !slices.Equal(got, union(c.want)) {
			t.Errorf("%s under %+v uses %v", c.role, c.s, got)
		}
	}
	for _, role := range All {
		for _, g := range Grants(role, DefaultSharing) {
			for _, r := range g.Rules {
				switch {
				case r.Type == "subject_types":
				case len(g.Use) != 1 || len(g.Delegate) > 0:
					t.Errorf("%s's %s carries %s beside other actions, or hands on: %+v", role, g.Name, r.Type, g)
				}
			}
		}
	}
	if got := AdminGrants()[0].Use; !slices.Equal(got, union([]string{DeviceRegister, DeviceMaintain, DatasetSetPolicy})) {
		t.Errorf("the admins use %v", got)
	}
}
