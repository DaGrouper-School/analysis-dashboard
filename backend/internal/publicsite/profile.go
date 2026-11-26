package publicsite

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// A classroom's public profile and contributors' pages. A released value
// is linked to a person only through the credit they asked for, matched
// to what SAM lets the public read by reading ID: SAM strips who recorded
// a value for visitors, so nothing here compares times with the cutoff.

// perRequest is how many readings one SAM read names, as SAM takes.
const perRequest = 100

// contributorRecent is how many of a contributor's latest values their
// page lists.
const contributorRecent = 20

const msgNoContributor = "Contributor not found."

// ProfileFacts are what a classroom says about itself.
type ProfileFacts struct {
	AreaCode    string   `json:"areaCode"`
	Language    string   `json:"language"`
	Description string   `json:"description"`
	Ecosystems  []string `json:"ecosystems"`
}

// ProfileSite is a project's site with its readings by registry label.
type ProfileSite struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Ecosystem      string   `json:"ecosystem"`
	Readings       []string `json:"readings"`
	Frequency      string   `json:"frequency"`
	ReadingsPerDay float64  `json:"readingsPerDay"`
	YearsPlanned   float64  `json:"yearsPlanned"`
}

// ProfileProject is a project and its sites.
type ProfileProject struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Question string        `json:"question"`
	Sites    []ProfileSite `json:"sites"`
}

// KitType is a device type as the kit names it.
type KitType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// KitItem is the front end's KitItem.
type KitItem struct {
	DeviceType KitType  `json:"deviceType"`
	Count      int      `json:"count"`
	Measures   []string `json:"measures"`
	Serials    []string `json:"serials"`
}

// Contributor is one person credited on a classroom's released values.
type Contributor struct {
	ParticipantID int64   `json:"participantId"`
	Name          *string `json:"name"`
}

// ProfilePage is the front end's PublicClassroomProfile.
type ProfilePage struct {
	Content       Content              `json:"content"`
	Classroom     Classroom            `json:"classroom"`
	Profile       ProfileFacts         `json:"profile"`
	Projects      []ProfileProject     `json:"projects"`
	Kit           []KitItem            `json:"kit"`
	Institution   *Institution         `json:"institution"`
	CustodianRole *roles.Role          `json:"custodianRole"`
	Sessions      []classrooms.Session `json:"sessions"`
	Contributors  []Contributor        `json:"contributors"`
	Member        bool                 `json:"member"`
	CanManage     bool                 `json:"canManage"`
}

func (m *Module) profile(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	c, _, member, err := m.shownClassroom(r)
	if err != nil {
		return err
	}
	v, err := m.reg.View(ctx)
	if err != nil {
		return err
	}
	params, err := coreParameters(ctx, m.db)
	if err != nil {
		return err
	}
	content, err := m.siteContent(ctx)
	if err != nil {
		return err
	}
	shown, err := m.publicClassroom(ctx, c, params)
	if err != nil {
		return err
	}
	can, err := m.permissions(r, c)
	if err != nil {
		return err
	}
	p := c.Profile
	out := ProfilePage{Content: content, Classroom: shown, Member: member,
		Profile:   ProfileFacts{AreaCode: p.AreaCode, Language: p.Language, Description: p.Description, Ecosystems: append([]string{}, p.Ecosystems...)},
		Projects:  projects(p.Projects, v),
		Sessions:  upcoming(p.Sessions, m.now()),
		CanManage: can.ManageMembers || can.ManageDevices || len(can.Invite) > 0}
	if member && p.CustodianRole != "" {
		role := p.CustodianRole
		out.CustodianRole = &role
	}
	var types []string
	if out.Kit, types, err = m.kit(ctx, c, v, member); err != nil {
		return err
	}
	out.Institution = institution(c, shown, v, types, member)
	if out.Contributors, err = m.contributors(ctx, c); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// projects are the classroom's projects, each site's readings by registry
// label.
func projects(list []classrooms.Project, v *registry.View) []ProfileProject {
	out := []ProfileProject{}
	for _, p := range list {
		pp := ProfileProject{ID: p.ID, Title: p.Title, Question: p.Question, Sites: []ProfileSite{}}
		for _, s := range p.Sites {
			site := ProfileSite{Code: s.Code, Name: s.Name, Ecosystem: s.Ecosystem, Frequency: s.Frequency, ReadingsPerDay: s.ReadingsPerDay,
				YearsPlanned: s.YearsPlanned, Readings: []string{}}
			for _, id := range s.ParameterIDs {
				site.Readings = append(site.Readings, labelOf(v, id))
			}
			pp.Sites = append(pp.Sites, site)
		}
		out = append(out, pp)
	}
	return out
}

func labelOf(v *registry.View, id string) string {
	if p := v.Parameter(id); p != nil {
		return p.Label
	}
	return id
}

// upcoming are the sessions starting from now, soonest first; one whose
// time cannot be read is left out.
func upcoming(sessions []classrooms.Session, now time.Time) []classrooms.Session {
	out := []classrooms.Session{}
	at := map[string]time.Time{}
	for _, s := range sessions {
		t, err := time.Parse(time.RFC3339, s.StartsAt)
		if err == nil && !t.Before(now) {
			out, at[s.ID] = append(out, s), t
		}
	}
	slices.SortStableFunc(out, func(a, b classrooms.Session) int { return at[a.ID].Compare(at[b.ID]) })
	return out
}

// kit is the classroom's Lab in a Bag, from the devices registered to it,
// by the registry's order of types: each type's count, what it measures,
// its domains then any values with no domain, and its serials, for
// members only. types are the kit's device types.
func (m *Module) kit(ctx context.Context, c classrooms.Classroom, v *registry.View, member bool) ([]KitItem, []string, error) {
	byType := map[string][]labs.Device{}
	err := labs.AllDevices(ctx, m.db, c.ID, func(d labs.Device) {
		if !d.Removed {
			byType[d.Type] = append(byType[d.Type], d)
		}
	})
	if err != nil {
		return nil, nil, err
	}
	out, types := []KitItem{}, []string{}
	for _, t := range v.DeviceTypes {
		devices := byType[t.ID]
		if len(devices) == 0 {
			continue
		}
		item := KitItem{DeviceType: KitType{ID: t.ID, Label: t.Label, Icon: t.Icon}, Count: len(devices), Measures: []string{}, Serials: []string{}}
		for _, d := range v.Domains {
			if slices.ContainsFunc(t.ParameterIDs, func(id string) bool { p := v.Parameter(id); return p != nil && p.Domain != nil && *p.Domain == d.ID }) {
				item.Measures = append(item.Measures, d.Label)
			}
		}
		for _, id := range t.ParameterIDs {
			if p := v.Parameter(id); p == nil || p.Domain == nil {
				item.Measures = append(item.Measures, labelOf(v, id))
			}
		}
		if member {
			for _, d := range devices {
				item.Serials = append(item.Serials, d.Identifier)
			}
		}
		out, types = append(out, item), append(types, t.ID)
	}
	return out, types, nil
}

// shownPerson is whether a person may be named on the public site: they
// turned on their public profile and are known not to be a minor.
func shownPerson(u accounts.User) bool {
	return u.PublicProfile && u.Minor != nil && !*u.Minor
}

// contributors are the people credited on the classroom's values the
// public may read, never a minor: named when they turned on their public
// profile, by participant ID otherwise; named ones first, by name. A
// classroom that does not show its data publicly has none.
func (m *Module) contributors(ctx context.Context, c classrooms.Classroom) ([]Contributor, error) {
	out := []Contributor{}
	if !c.Sharing.ShowPublic || c.SpaceID == nil {
		return out, nil
	}
	credited, err := labs.CreditedIn(ctx, m.db, c.ID)
	if err != nil {
		return nil, err
	}
	for user, readings := range credited {
		u, err := accounts.Get(ctx, m.db, user)
		if err != nil {
			return nil, err
		}
		if u.Minor == nil || *u.Minor {
			continue
		}
		has, err := m.publicValues(ctx, *c.SpaceID, readings)
		if err != nil {
			return nil, err
		}
		if !has {
			continue
		}
		con := Contributor{ParticipantID: u.ParticipantID}
		if shownPerson(u) {
			name := u.FullName
			con.Name = &name
		}
		out = append(out, con)
	}
	slices.SortFunc(out, func(a, b Contributor) int {
		switch {
		case (a.Name == nil) != (b.Name == nil):
			if a.Name != nil {
				return -1
			}
			return 1
		case a.Name != nil && *a.Name != *b.Name:
			return strings.Compare(*a.Name, *b.Name)
		}
		return int(a.ParticipantID - b.ParticipantID)
	})
	return out, nil
}

// chunks are readings in groups SAM takes in one read.
func chunks(readings []uuid.UUID) [][]uuid.UUID {
	var out [][]uuid.UUID
	for start := 0; start < len(readings); start += perRequest {
		out = append(out, readings[start:min(start+perRequest, len(readings))])
	}
	return out
}

// publicValues reports whether the public may read any value of readings
// in space.
func (m *Module) publicValues(ctx context.Context, space uuid.UUID, readings []uuid.UUID) (bool, error) {
	for _, part := range chunks(readings) {
		n, err := m.publicCount(ctx, space, part)
		if err != nil || n > 0 {
			return n > 0, err
		}
	}
	return false, nil
}

// publicCount is how many values of readings in space the public may
// read.
func (m *Module) publicCount(ctx context.Context, space uuid.UUID, readings []uuid.UUID) (int64, error) {
	sum, err := m.sam.Anonymous().Summarise(ctx, client.ObservationQuery{SpaceID: space, ReadingIDs: readings})
	switch {
	case notReadable(err):
		return 0, nil
	case err != nil:
		return 0, samaccess.Translate(ctx, err)
	}
	return sum.Count, nil
}

// ContributorProfile is who a contributor is, by their own choice.
type ContributorProfile struct {
	FullName  string  `json:"fullName"`
	Email     *string `json:"email"`
	Role      string  `json:"role"`
	AvatarURL *string `json:"avatarUrl"`
}

// ContributorClassroom is a classroom a contributor has released values
// in.
type ContributorClassroom struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Contribution is the front end's PublicContribution.
type Contribution struct {
	ClassroomID    uuid.UUID `json:"classroomId"`
	ClassroomName  string    `json:"classroomName"`
	ParameterID    string    `json:"parameterId"`
	ParameterLabel string    `json:"parameterLabel"`
	Unit           string    `json:"unit"`
	ValueType      string    `json:"valueType,omitempty"`
	Value          any       `json:"value"`
	ObservedAt     time.Time `json:"observedAt"`
}

// ContributorPage is the front end's PublicContributor.
type ContributorPage struct {
	Content       Content                `json:"content"`
	ParticipantID int64                  `json:"participantId"`
	Profile       *ContributorProfile    `json:"profile"`
	Classrooms    []ContributorClassroom `json:"classrooms"`
	Total         int64                  `json:"total"`
	Recent        []Contribution         `json:"recent"`
}

// contributor is a contributor's page: the values the public may read
// that they took credit for, in the classrooms that show their data, and
// who they are. Only a person who turned on their public profile and is
// not a minor has a page; anyone else, as an unknown ID, is not found, so
// participant IDs cannot be counted through. Their email is never shown.
func (m *Module) contributor(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	pid, err := strconv.ParseInt(r.PathValue("participantId"), 10, 64)
	if err != nil {
		return httpx.NotFound(msgNoContributor)
	}
	u, err := accounts.ByParticipant(ctx, m.db, pid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !shownPerson(u)) {
		return httpx.NotFound(msgNoContributor)
	}
	if err != nil {
		return err
	}
	content, err := m.siteContent(ctx)
	if err != nil {
		return err
	}
	v, err := m.reg.View(ctx)
	if err != nil {
		return err
	}
	out := ContributorPage{Content: content, ParticipantID: u.ParticipantID, Classrooms: []ContributorClassroom{}, Recent: []Contribution{},
		Profile: &ContributorProfile{FullName: u.FullName, Role: u.Role, AvatarURL: u.AvatarURL}}
	credited, err := labs.CreditedTo(ctx, m.db, u.ID)
	if err != nil {
		return err
	}
	for id, readings := range credited {
		c, err := classrooms.Get(ctx, m.db, id)
		if err != nil {
			return err
		}
		if !c.Sharing.ShowPublic || c.SpaceID == nil {
			continue
		}
		n, recent, err := m.contributions(ctx, c, readings, v)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		out.Total += n
		out.Recent = append(out.Recent, recent...)
		out.Classrooms = append(out.Classrooms, ContributorClassroom{ID: c.ID, Name: c.Name})
	}
	slices.SortFunc(out.Recent, func(a, b Contribution) int { return b.ObservedAt.Compare(a.ObservedAt) })
	out.Recent = out.Recent[:min(len(out.Recent), contributorRecent)]
	slices.SortFunc(out.Classrooms, func(a, b ContributorClassroom) int { return strings.Compare(a.Name, b.Name) })
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// contributions are how many values of readings in c the public may read,
// and the newest of them, up to contributorRecent, each named by its
// registry entry; a value the registry does not hold is left out.
func (m *Module) contributions(ctx context.Context, c classrooms.Classroom, readings []uuid.UUID, v *registry.View) (int64, []Contribution, error) {
	var total int64
	var recent []Contribution
	for _, part := range chunks(readings) {
		n, err := m.publicCount(ctx, *c.SpaceID, part)
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			continue
		}
		total += n
		page, err := m.sam.Anonymous().Observations(ctx, client.ObservationQuery{SpaceID: *c.SpaceID, ReadingIDs: part, Sort: "observed_at", Order: "desc"},
			client.PageQuery{Limit: contributorRecent})
		if err != nil {
			return 0, nil, samaccess.Translate(ctx, err)
		}
		for _, rec := range page.Items {
			p := v.Parameter(rec.ParameterID)
			if p == nil {
				continue
			}
			recent = append(recent, Contribution{ClassroomID: c.ID, ClassroomName: c.Name, ParameterID: rec.ParameterID, ParameterLabel: p.Label,
				Unit: p.Unit, ValueType: p.ValueType, Value: rec.Value, ObservedAt: rec.ObservedAt})
		}
	}
	return total, recent, nil
}
