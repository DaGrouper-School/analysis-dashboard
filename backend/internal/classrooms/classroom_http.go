package classrooms

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/hosting"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Summary is the front end's ClassroomSummary, for one viewer.
type Summary struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	InstitutionName string      `json:"institutionName"`
	Code            *string     `json:"code"`
	Setup           string      `json:"setup"`
	CreatedAt       time.Time   `json:"createdAt"`
	MyRole          *roles.Role `json:"myRole"`
	CanRecord       bool        `json:"canRecord"`
	MemberCount     int         `json:"memberCount"`
}

// summary is c as viewer sees it, holding role there ("" for none).
func summary(c Classroom, viewer accounts.User, role roles.Role, members int) Summary {
	can := roles.PermissionsFor(&roles.Viewer{PlatformAdmin: viewer.PlatformAdmin}, role, c.Sharing)
	s := Summary{ID: c.ID.String(), Name: c.Name, InstitutionName: c.InstitutionName, Setup: c.Setup, CreatedAt: c.CreatedAt,
		CanRecord: role != "" && role != roles.Researcher, MemberCount: members}
	if can.SeeCode {
		s.Code = &c.Code
	}
	if role != "" {
		s.MyRole = &role
	}
	return s
}

// MemberView is the front end's ClassroomMember.
type MemberView struct {
	UserID   string     `json:"userId"`
	FullName string     `json:"fullName"`
	Email    *string    `json:"email"`
	Role     roles.Role `json:"role"`
	JoinedAt time.Time  `json:"joinedAt"`
}

// Detail is the front end's ClassroomDetail.
type Detail struct {
	Classroom   Summary           `json:"classroom"`
	Members     []MemberView      `json:"members"`
	Invitations []InvitationView  `json:"invitations"`
	Permissions roles.Permissions `json:"permissions"`
	Sharing     *SharingView      `json:"sharing"`
}

// SharingView is the front end's SharingInput.
type SharingView struct {
	Sharing roles.Sharing      `json:"sharing"`
	Terms   settings.DataTerms `json:"terms"`
}

const msgNoClassroom = "That classroom no longer exists."

func (m *Module) listMine(w http.ResponseWriter, r *http.Request) error {
	u, _ := accounts.CurrentUser(r.Context())
	list, err := List(r.Context(), m.db, u.ID, u.PlatformAdmin)
	if err != nil {
		return err
	}
	out := make([]Summary, 0, len(list))
	for _, l := range list {
		out = append(out, summary(l.Classroom, u, l.Role, l.Members))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) create(w http.ResponseWriter, r *http.Request) error {
	u, _ := accounts.CurrentUser(r.Context())
	if !u.PlatformAdmin {
		return httpx.Forbidden("Only platform admins can create a classroom.")
	}
	var in CreateInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(CheckCreate(in)); err != nil {
		return err
	}
	c, err := m.createFrom(r.Context(), u.ID, in)
	switch {
	case errors.Is(err, ErrNoStage):
		return httpx.New(httpx.CodeConflict, "Add a stage before creating classrooms.")
	case errors.Is(err, hosting.ErrNoApplication):
		return httpx.NotFound("Host application not found.")
	case errors.Is(err, hosting.ErrHasClassroom):
		return httpx.New(httpx.CodeConflict, "A classroom was already created from this application.")
	case errors.Is(err, hosting.ErrNotAccepted):
		return httpx.New(httpx.CodeConflict, "Accept the application before creating its classroom.")
	case err != nil:
		return err
	}
	// The head joins by accepting an invitation sent as this service.
	list, err := Invite(r.Context(), m.db, m.now(), m.cfg.InvitationTTL, c, []string{*c.HeadEmail}, roles.Head,
		Inviter{UserID: &u.ID, Name: u.FullName, Service: true})
	if err != nil {
		return err
	}
	m.send(r.Context(), c, list)
	httpx.WriteJSON(w, http.StatusOK, summary(c, u, "", 0))
	return nil
}

// createFrom creates a classroom, set up from the host application in
// it names, which must be accepted and have no classroom yet.
func (m *Module) createFrom(ctx context.Context, by uuid.UUID, in CreateInput) (Classroom, error) {
	if in.ApplicationID == nil {
		return Create(ctx, m.db, m.now(), by, in)
	}
	id, err := uuid.Parse(*in.ApplicationID)
	if err != nil {
		return Classroom{}, hosting.ErrNoApplication
	}
	var c Classroom
	err = hosting.CreateClassroom(ctx, m.db, id, func(tx pgx.Tx) (uuid.UUID, error) {
		var err error
		c, err = Create(ctx, tx, m.now(), by, in)
		return c.ID, err
	})
	return c, err
}

// visible is the classroom in the path as the viewer sees it, with their
// role and permissions, or 404 for one they may not see.
func (m *Module) visible(r *http.Request) (Classroom, roles.Role, roles.Permissions, error) {
	u, _ := accounts.CurrentUser(r.Context())
	id, err := uuid.Parse(r.PathValue("classroomId"))
	if err != nil {
		return Classroom{}, "", roles.Permissions{}, httpx.NotFound(msgNoClassroom)
	}
	c, role, can, err := Visible(r.Context(), m.db, id, u)
	if errors.Is(err, ErrNoClassroom) {
		return c, "", can, httpx.NotFound(msgNoClassroom)
	}
	return c, role, can, err
}

// Visible is the classroom with id as u sees it, with their role there and
// their permissions, or ErrNoClassroom for one they may not see.
func Visible(ctx context.Context, q db.Querier, id uuid.UUID, u accounts.User) (Classroom, roles.Role, roles.Permissions, error) {
	c, err := Get(ctx, q, id)
	if err != nil {
		return c, "", roles.Permissions{}, err
	}
	role, err := RoleOf(ctx, q, c.ID, u.ID)
	if err != nil {
		return c, "", roles.Permissions{}, err
	}
	can := roles.PermissionsFor(&roles.Viewer{PlatformAdmin: u.PlatformAdmin}, role, c.Sharing)
	if !can.View {
		return c, "", can, ErrNoClassroom
	}
	return c, role, can, nil
}

func (m *Module) get(w http.ResponseWriter, r *http.Request) error {
	c, role, can, err := m.visible(r)
	if err != nil {
		return err
	}
	members, err := Members(r.Context(), m.db, c.ID)
	if err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	out := Detail{Classroom: summary(c, u, role, len(members)), Members: []MemberView{}, Invitations: []InvitationView{}, Permissions: can}
	if len(can.Invite) > 0 {
		pending, err := PendingIn(r.Context(), m.db, m.now(), c.ID)
		if err != nil {
			return err
		}
		for _, inv := range pending {
			out.Invitations = append(out.Invitations, m.view(inv))
		}
	}
	for _, mem := range members {
		v := MemberView{UserID: mem.UserID.String(), FullName: mem.FullName, Role: mem.Role, JoinedAt: mem.JoinedAt}
		if can.SeeEmails {
			v.Email = mem.Email
		}
		out.Members = append(out.Members, v)
	}
	if can.ViewSharing {
		out.Sharing = &SharingView{Sharing: c.Sharing, Terms: c.Terms}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) update(w http.ResponseWriter, r *http.Request) error {
	c, role, can, err := m.visible(r)
	if err != nil {
		return err
	}
	if !can.Edit {
		return httpx.Forbidden("Only the institution head can edit the classroom.")
	}
	var in UpdateInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(CheckUpdate(in)); err != nil {
		return err
	}
	if err := Update(r.Context(), m.db, m.now(), c.ID, in); err != nil {
		return err
	}
	if c, err = Get(r.Context(), m.db, c.ID); err != nil {
		return err
	}
	members, err := Members(r.Context(), m.db, c.ID)
	if err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	httpx.WriteJSON(w, http.StatusOK, summary(c, u, role, len(members)))
	return nil
}

// Configuration is what platform admins configure of a classroom.
type Configuration struct {
	StageID  string  `json:"stageId"`
	RegionID *string `json:"regionId"`
	Profile  Profile `json:"profile"`
}

func (m *Module) configure(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("classroomId"))
	if err != nil {
		return httpx.NotFound(msgNoClassroom)
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBody))
	if err != nil {
		return httpx.New(httpx.CodeInvalidInput, "The request could not be read.")
	}
	err = Configure(r.Context(), m.db, m.now(), actor(r), id, raw)
	var invalid *jsoncheck.Invalid
	switch {
	case errors.Is(err, ErrNoClassroom):
		return httpx.NotFound(msgNoClassroom)
	case errors.As(err, &invalid):
		return httpx.Invalid(invalid.Fields())
	case err != nil:
		return err
	}
	c, err := Get(r.Context(), m.db, id)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, Configuration{StageID: c.StageID, RegionID: c.RegionID, Profile: c.Profile})
	return nil
}
