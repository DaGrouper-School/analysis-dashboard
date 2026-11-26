package roles

import (
	"encoding/json"
	"reflect"
	"slices"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"
)

// SAM's actions the grants name.
const (
	ObservationRead   = "observation:read"
	ObservationSubmit = "observation:submit"
	DeviceConnect     = "device:connect"
	DeviceRegister    = "device:register"
	DeviceMaintain    = "device:maintain"
	DatasetPublish    = "dataset:publish"
	DatasetSetPolicy  = "dataset:set_policy"
	ExportCreate      = "export:create"
	AttributeAssert   = "attribute:assert"
)

// TenantDepth is the depth of the tenant grant this service issues every
// group's grants from. A grant it issues has less.
const TenantDepth = 4

// ExportDestination is the destination this service names when it exports
// a classroom's readings: the only one the export grants allow.
const ExportDestination = "dgs"

// ConsentAttribute is the attribute heads and facilitators assert.
const ConsentAttribute = "consent"

// Grant is a grant a group holds: what it uses, what it hands on and how
// many hand-offs deep, and its rules. Its name labels it in SAM.
type Grant struct {
	Name     string
	Use      []string
	Delegate []string
	Depth    int
	Rules    []client.Rule
}

// Input is the grant to issue to group on space.
func (g Grant) Input(group, space uuid.UUID) client.GrantInput {
	return client.GrantInput{SubjectID: group, SpaceID: space, Use: g.Use, Delegate: g.Delegate, Depth: g.Depth,
		Rules: g.Rules, Labels: []string{g.Name}}
}

// Actions are what the grant uses and hands on.
func (g Grant) Actions() []string { return union(g.Use, g.Delegate) }

func rule(name string, params any) client.Rule {
	raw, _ := json.Marshal(params)
	return client.Rule{Type: name, Version: 1, Params: raw}
}

// What the groups use.
var (
	read    = []string{ObservationRead}
	record  = []string{ObservationSubmit, DeviceConnect}
	devices = []string{DeviceRegister, DeviceMaintain}
	release = []string{DatasetPublish}
)

// export lets whoever reads the classroom download it, through this
// service only. Its rule is on a grant of its own, so it is never on the
// grants that hand others theirs.
func export() Grant {
	return Grant{Name: "export", Use: []string{ExportCreate}, Rules: []client.Rule{rule("export_to", map[string]any{"destinations": []string{ExportDestination}})}}
}

// toNoOne limits whom a grant hands authority to, to anonymous, which can
// join no group and gets a grant only from an issuer holding
// dataset:publish: a grant carrying it hands nothing on to anyone.
func toNoOne() client.Rule {
	return rule("subject_types", map[string]any{"types": []string{"anonymous"}})
}

// consent is the right to record minors' guardian consent, and nothing
// else: a grant of its own, so its rule is never on a grant that hands
// others theirs.
func consent(name string) Grant {
	return Grant{Name: name, Use: []string{AttributeAssert}, Rules: []client.Rule{rule("attribute_names", map[string]any{"names": []string{ConsentAttribute}})}}
}

// Grants are the grants role's group holds in a classroom with sharing,
// so that SAM allows exactly what the grid does: who reads, records,
// manages devices, releases, records consent and adds whose members.
// Actions that reach different depths are separate grants, and the grants
// that hand others theirs carry no rules. The export and consent rules are
// on grants of their own, which hand nothing on. The
// submit grants carry no consent rule: SAM refuses a minor's submission
// without consent itself.
func Grants(role Role, sharing Sharing) []Grant {
	switch role {
	case Researcher:
		// One hand-off deeper than facilitators hand on, so only heads add
		// researchers, though their reading is a community scientist's;
		// handing on to no one.
		return []Grant{{Name: "researcher", Use: read, Delegate: read, Depth: depthFor(CommunityScientist), Rules: []client.Rule{toNoOne()}}, export()}
	case CommunityScientist:
		return []Grant{{Name: "community-scientist", Use: union(read, record)}, export()}
	case Facilitator:
		main := Grant{Name: "facilitator", Use: union(read, record)}
		if sharing.FacilitatorsManageDevices {
			main.Use = union(main.Use, devices)
		}
		if sharing.Releasers == ReleasersHeadAndFacils {
			main.Use = union(main.Use, release)
		}
		if sharing.FacilitatorsInvite {
			main.Delegate, main.Depth = handsOn(CommunityScientist), depthFor(CommunityScientist)
		}
		return []Grant{main, export(), consent("facilitator-consent")}
	case Head:
		return []Grant{
			{Name: "head", Use: union(read, record, devices, release),
				Delegate: handsOn(Facilitator, CommunityScientist, Researcher),
				Depth:    depthFor(Facilitator, CommunityScientist, Researcher)},
			export(), consent("head-consent"),
		}
	}
	return nil
}

// AdminGrants are what the platform admins' group holds at the tenant
// root: they set up any classroom's devices as themselves and set its terms
// before it has a head. They read nothing and never release.
func AdminGrants() []Grant {
	return []Grant{{Name: "admins", Use: union(devices, []string{DatasetSetPolicy})}}
}

// HandsOn reports whether a member of adder's group can add members to
// role's group in SAM: SAM lets them when, for every grant role's group
// holds, one of adder's grants delegates each action it uses and hands on,
// with more depth, and carries no rule it lacks. It is the grid's invite.
// A platform admin adds heads as this service, from its tenant grant.
func HandsOn(adder, role Role, sharing Sharing) bool {
	for _, held := range Grants(role, sharing) {
		if !slices.ContainsFunc(Grants(adder, sharing), func(p Grant) bool { return handsOnTo(p, held) }) {
			return false
		}
	}
	return true
}

func handsOnTo(p, g Grant) bool {
	if p.Depth <= g.Depth {
		return false
	}
	for _, a := range g.Actions() {
		if !slices.Contains(p.Delegate, a) {
			return false
		}
	}
	for _, r := range p.Rules {
		if !slices.ContainsFunc(g.Rules, func(x client.Rule) bool { return reflect.DeepEqual(x, r) }) {
			return false
		}
	}
	return true
}

// handsOn is every action the groups of roles hold, under any sharing
// settings, for the grant that adds their members.
func handsOn(roles ...Role) []string {
	var out []string
	for _, r := range roles {
		for _, g := range widest(r) {
			out = union(out, g.Actions())
		}
	}
	return out
}

// depthFor is one more than the deepest grant the groups of roles hold.
func depthFor(roles ...Role) int {
	d := 0
	for _, r := range roles {
		for _, g := range widest(r) {
			d = max(d, g.Depth)
		}
	}
	return d + 1
}

// widest are a role's grants under the sharing settings that give it the
// most.
func widest(r Role) []Grant {
	return Grants(r, Sharing{FacilitatorsInvite: true, FacilitatorsManageDevices: true, Releasers: ReleasersHeadAndFacils})
}

// union is the actions of lists, each once, in a fixed order.
func union(lists ...[]string) []string {
	out := []string{}
	for _, l := range lists {
		for _, a := range l {
			if !slices.Contains(out, a) {
				out = append(out, a)
			}
		}
	}
	slices.Sort(out)
	return out
}
