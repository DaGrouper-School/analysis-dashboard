package roles

import (
	"slices"
	"testing"

	"github.com/dagrouper-school/sam/client"
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
		{Head, narrow, []string{ObservationRead, ObservationSubmit, DeviceConnect, DeviceRegister, DeviceMaintain, DatasetPublish, DatasetSetPolicy, ExportCreate, AttributeAssert}},
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

// TestPublicGrants: none while a classroom does not show its data; its
// published records, generalised and stripped, while it does; and those
// past the review period too under automatic review; never finer than two
// decimal places.
func TestPublicGrants(t *testing.T) {
	t.Parallel()
	if got := PublicGrants(Sharing{ShowPublic: false}, nil); len(got) != 0 {
		t.Errorf("not shown: %+v", got)
	}
	manual := PublicGrants(Sharing{ShowPublic: true, Review: ReviewManual}, nil)
	if len(manual) != 1 || manual[0].Name != "public:published" || !slices.Equal(manual[0].Use, []string{ObservationRead}) ||
		!slices.Contains(manual[0].Labels, PublicLabel) || len(manual[0].Rules) != 3 || manual[0].Rules[0].Type != "record_attr" ||
		string(manual[0].Rules[1].Params) != `{"grid_m":1113}` || manual[0].Rules[2].Type != "strip_identity" {
		t.Errorf("manual review: %+v", manual)
	}
	one, four := 1, 4
	if got := PublicGrants(Sharing{ShowPublic: true}, &one); string(got[0].Rules[1].Params) != `{"grid_m":11132}` {
		t.Errorf("one decimal: %s", got[0].Rules[1].Params)
	}
	if got := PublicGrants(Sharing{ShowPublic: true}, &four); string(got[0].Rules[1].Params) != `{"grid_m":1113}` {
		t.Errorf("finer than two: %s", got[0].Rules[1].Params)
	}
	auto := PublicGrants(Sharing{ShowPublic: true, Review: ReviewAuto, AutoReleaseDays: 3}, nil)
	if len(auto) != 2 || auto[1].Name != "public:embargo" || auto[1].Rules[0].Type != "embargo" || string(auto[1].Rules[0].Params) != `{"duration":"3d"}` {
		t.Errorf("automatic review: %+v", auto)
	}
}

// TestSame: a held grant is the same as a template whatever the order of
// its actions and the spacing of its rules' parameters, and differs when
// an action or a rule's parameter does.
func TestSame(t *testing.T) {
	t.Parallel()
	g := Grants(Facilitator, DefaultSharing)[0]
	held := client.Grant{Use: slices.Clone(g.Use), Delegate: g.Delegate, Depth: g.Depth, Rules: g.Rules}
	slices.Reverse(held.Use)
	if !g.same(held) {
		t.Error("the same grant differs")
	}
	narrowed := Grants(Facilitator, Sharing{Releasers: ReleasersHead, FacilitatorsInvite: true, FacilitatorsManageDevices: true})[0]
	if narrowed.same(held) {
		t.Error("a facilitator who no longer releases holds the same grant")
	}
	p := PublicGrants(Sharing{ShowPublic: true}, nil)[0]
	spaced := client.Grant{Use: p.Use, Rules: slices.Clone(p.Rules)}
	spaced.Rules[1] = client.Rule{Type: "generalise_location", Version: 1, Params: []byte(`{ "grid_m" : 1113 }`)}
	if !p.same(spaced) {
		t.Error("spacing makes a rule differ")
	}
	spaced.Rules[1] = client.Rule{Type: "generalise_location", Version: 1, Params: []byte(`{"grid_m":11132}`)}
	if p.same(spaced) {
		t.Error("another grid is the same")
	}
}
