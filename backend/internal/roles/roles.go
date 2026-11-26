// Package roles holds who may do what in a classroom: the permission grid,
// cell for cell the same as the front end's (frontend/src/lib/domain/
// permissions.ts), and the grants each classroom's role groups hold in SAM
// (docs/design/classrooms-and-permissions.md).
package roles

// Role is a member's role in a classroom.
type Role string

// The roles.
const (
	Head               Role = "institution_head"
	Facilitator        Role = "facilitator"
	CommunityScientist Role = "community_scientist"
	Researcher         Role = "researcher"
)

// All are the roles, each with a group in every classroom.
var All = []Role{Head, Facilitator, CommunityScientist, Researcher}

// Who may release a classroom's data.
const (
	ReleasersHead          = "head"
	ReleasersHeadAndFacils = "head_and_facilitators"
)

// Sharing is the front end's ClassroomSharing: the settings a head
// chooses.
type Sharing struct {
	ShowPublic                bool   `json:"showPublic"`
	Review                    string `json:"review"`
	AutoReleaseDays           int    `json:"autoReleaseDays"`
	Releasers                 string `json:"releasers"`
	FacilitatorsInvite        bool   `json:"facilitatorsInvite"`
	FacilitatorsManageDevices bool   `json:"facilitatorsManageDevices"`
}

// DefaultSharing is a new classroom's: public once released by hand, by
// the head or a facilitator.
var DefaultSharing = Sharing{
	ShowPublic:                true,
	Review:                    "manual",
	AutoReleaseDays:           2,
	Releasers:                 ReleasersHeadAndFacils,
	FacilitatorsInvite:        true,
	FacilitatorsManageDevices: true,
}

// Permissions is the front end's ClassroomPermissions.
type Permissions struct {
	View               bool   `json:"view"`
	SeeUnreleased      bool   `json:"seeUnreleased"`
	OpenValuePage      bool   `json:"openValuePage"`
	SeeCode            bool   `json:"seeCode"`
	SeeSiteCodes       bool   `json:"seeSiteCodes"`
	SeeEmails          bool   `json:"seeEmails"`
	Invite             []Role `json:"invite"`
	ManageMembers      bool   `json:"manageMembers"`
	Edit               bool   `json:"edit"`
	ManageSharing      bool   `json:"manageSharing"`
	ViewSharing        bool   `json:"viewSharing"`
	ManageConsent      bool   `json:"manageConsent"`
	ManageDeviceAlerts bool   `json:"manageDeviceAlerts"`
	ManageTypeAlerts   bool   `json:"manageTypeAlerts"`
	ManageDevices      bool   `json:"manageDevices"`
	RecordReadings     bool   `json:"recordReadings"`
	ReleaseData        bool   `json:"releaseData"`
}

// Viewer is who looks at a classroom; nil is a visitor, who gets nothing
// here.
type Viewer struct {
	PlatformAdmin bool
}

// PermissionsFor is what viewer may do in a classroom where they hold role
// ("" for none), under its sharing settings.
func PermissionsFor(viewer *Viewer, role Role, sharing Sharing) Permissions {
	admin := viewer != nil && viewer.PlatformAdmin
	member := viewer != nil && role != ""
	head := member && role == Head
	facilitator := member && role == Facilitator
	focal := head || facilitator
	p := Permissions{
		View:          admin || member,
		SeeUnreleased: member,
		OpenValuePage: member,
		SeeCode:       admin || focal,
		SeeSiteCodes:  member,
		SeeEmails:     admin || focal,
		Invite:        []Role{},
		ManageMembers: admin || head,
		Edit:          admin || head,
		ManageSharing: admin || head,
		ViewSharing:   admin || focal,
		// The people who hand out and collect the paper sheets; not platform
		// admins, who are not at the classroom.
		ManageConsent:      focal,
		ManageDeviceAlerts: focal,
		ManageTypeAlerts:   head,
		ManageDevices:      admin || head || (facilitator && sharing.FacilitatorsManageDevices),
		// Researchers only read.
		RecordReadings: admin || focal || (member && role == CommunityScientist),
		ReleaseData:    head || (facilitator && sharing.Releasers == ReleasersHeadAndFacils),
	}
	switch {
	case admin:
		p.Invite = append(p.Invite, All...)
	case head:
		p.Invite = append(p.Invite, Facilitator, CommunityScientist, Researcher)
	case facilitator && sharing.FacilitatorsInvite:
		p.Invite = append(p.Invite, CommunityScientist)
	}
	return p
}
