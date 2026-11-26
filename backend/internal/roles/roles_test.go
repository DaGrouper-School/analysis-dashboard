package roles

import (
	"encoding/json"
	"slices"
	"testing"
)

// TestPermissionsFor: every cell of the grid, as the front end has it, for
// each viewer under the default sharing settings and under the narrowest.
func TestPermissionsFor(t *testing.T) {
	t.Parallel()
	admin, person := &Viewer{PlatformAdmin: true}, &Viewer{}
	narrow := Sharing{Releasers: ReleasersHead}
	type cells = map[string]bool
	for _, c := range []struct {
		name    string
		viewer  *Viewer
		role    Role
		sharing Sharing
		yes     []string
		invite  []Role
	}{
		{"a visitor", nil, "", DefaultSharing, nil, nil},
		{"a visitor named as a head", nil, Head, DefaultSharing, nil, nil},
		{"someone with no role", person, "", DefaultSharing, nil, nil},
		{"a platform admin", admin, "", DefaultSharing,
			[]string{"view", "seeCode", "seeEmails", "manageMembers", "edit", "manageSharing", "viewSharing", "manageDevices", "recordReadings"}, All},
		{"a head", person, Head, narrow,
			[]string{"view", "seeUnreleased", "openValuePage", "seeCode", "seeSiteCodes", "seeEmails", "manageMembers", "edit", "manageSharing",
				"viewSharing", "manageConsent", "manageDeviceAlerts", "manageTypeAlerts", "manageDevices", "recordReadings", "releaseData"},
			[]Role{Facilitator, CommunityScientist, Researcher}},
		{"a facilitator", person, Facilitator, DefaultSharing,
			[]string{"view", "seeUnreleased", "openValuePage", "seeCode", "seeSiteCodes", "seeEmails", "viewSharing", "manageConsent",
				"manageDeviceAlerts", "manageDevices", "recordReadings", "releaseData"}, []Role{CommunityScientist}},
		{"a facilitator, narrowly", person, Facilitator, narrow,
			[]string{"view", "seeUnreleased", "openValuePage", "seeCode", "seeSiteCodes", "seeEmails", "viewSharing", "manageConsent",
				"manageDeviceAlerts", "recordReadings"}, nil},
		{"a community scientist", person, CommunityScientist, DefaultSharing,
			[]string{"view", "seeUnreleased", "openValuePage", "seeSiteCodes", "recordReadings"}, nil},
		{"a researcher", person, Researcher, DefaultSharing, []string{"view", "seeUnreleased", "openValuePage", "seeSiteCodes"}, nil},
		{"a platform admin who is a researcher", admin, Researcher, narrow,
			[]string{"view", "seeUnreleased", "openValuePage", "seeCode", "seeSiteCodes", "seeEmails", "manageMembers", "edit", "manageSharing",
				"viewSharing", "manageDevices", "recordReadings"}, All},
	} {
		p := PermissionsFor(c.viewer, c.role, c.sharing)
		raw, _ := json.Marshal(p)
		var got map[string]any
		_ = json.Unmarshal(raw, &got)
		want := cells{}
		for _, y := range c.yes {
			want[y] = true
		}
		for name, v := range got {
			if name == "invite" {
				continue
			}
			if v != want[name] {
				t.Errorf("%s: %s is %v", c.name, name, v)
			}
		}
		if len(got) != 17 {
			t.Errorf("%s: %d cells", c.name, len(got))
		}
		if !slices.Equal(p.Invite, c.invite) && (len(p.Invite) != 0 || len(c.invite) != 0) {
			t.Errorf("%s: invites %v", c.name, p.Invite)
		}
		if p.Invite == nil {
			t.Errorf("%s: invite is null", c.name)
		}
	}
}
