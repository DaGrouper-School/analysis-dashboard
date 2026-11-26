package classrooms

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
)

// InvitationView is the front end's Invitation.
type InvitationView struct {
	ID            string     `json:"id"`
	ClassroomID   string     `json:"classroomId"`
	Email         string     `json:"email"`
	Role          roles.Role `json:"role"`
	Status        string     `json:"status"`
	InvitedByName string     `json:"invitedByName"`
	CreatedAt     time.Time  `json:"createdAt"`
	// Token is the link's secret: the invitee gets it by email, and the
	// client only in demo mode.
	Token string `json:"token"`
}

// Preview is the front end's InvitationPreview.
type Preview struct {
	Token           string     `json:"token"`
	ClassroomName   string     `json:"classroomName"`
	InstitutionName string     `json:"institutionName"`
	Role            roles.Role `json:"role"`
	Email           string     `json:"email"`
	Status          string     `json:"status"`
	InvitedByName   string     `json:"invitedByName"`
}

func (m *Module) view(i Invitation) InvitationView {
	v := InvitationView{ID: i.ID.String(), ClassroomID: i.ClassroomID.String(), Email: i.Email, Role: i.Role, Status: i.Status,
		InvitedByName: i.InvitedByName, CreatedAt: i.CreatedAt}
	if m.cfg.ShowTokens {
		v.Token = i.Token
	}
	return v
}

const (
	msgInvalidInvitation = "This invitation is no longer valid."
	msgExpired           = "This invitation has expired. Ask for a new one."
	msgNoMember          = "Member not found."
)

// link is where an invitation's email points.
func (m *Module) link(token string) string {
	return strings.TrimRight(m.cfg.FrontEnd, "/") + "/invite/" + token
}

// send emails each invitation. A failure is logged: the invitation stays
// open and inviting again resends it.
func (m *Module) send(ctx context.Context, c Classroom, list []Invitation) {
	for _, inv := range list {
		subject, text := InvitationText(c, inv, m.link(inv.Token))
		if err := m.cfg.Email.Send(ctx, notify.Mail{To: inv.Email, Subject: subject, Text: text}); err != nil {
			slog.ErrorContext(ctx, "sending an invitation failed", "invitation", inv.ID, "err", err)
		}
	}
}

// inviter is who an invitation from u, holding role, is sent as.
func inviter(u accounts.User, role roles.Role) Inviter {
	if role == "" && u.PlatformAdmin {
		return Inviter{UserID: &u.ID, Name: u.FullName, Service: true}
	}
	return Inviter{UserID: &u.ID, Name: u.FullName}
}

func (m *Module) invite(w http.ResponseWriter, r *http.Request) error {
	c, role, can, err := m.visible(r)
	if err != nil {
		return err
	}
	var in struct {
		Emails []string   `json:"emails"`
		Role   roles.Role `json:"role"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if !slices.Contains(can.Invite, in.Role) {
		return httpx.Forbidden("You cannot invite people with that role.")
	}
	good, bad := cleanEmails(in.Emails)
	switch {
	case len(bad) > 0:
		return httpx.FieldError(httpx.CodeInvalidInput, "emails", "Not valid: "+strings.Join(bad, ", "))
	case len(good) == 0:
		return httpx.FieldError(httpx.CodeInvalidInput, "emails", "Add at least one email address.")
	}
	members, err := Members(r.Context(), m.db, c.ID)
	if err != nil {
		return err
	}
	var already []string
	for _, mem := range members {
		if mem.Email != nil && slices.Contains(good, strings.ToLower(*mem.Email)) {
			already = append(already, strings.ToLower(*mem.Email))
		}
	}
	if len(already) > 0 {
		return httpx.FieldError(httpx.CodeConflict, "emails", "Already in this classroom: "+strings.Join(already, ", "))
	}
	u, _ := accounts.CurrentUser(r.Context())
	list, err := Invite(r.Context(), m.db, m.now(), m.cfg.InvitationTTL, c, good, in.Role, inviter(u, role))
	if err != nil {
		return err
	}
	m.send(r.Context(), c, list)
	out := make([]InvitationView, len(list))
	for i, inv := range list {
		out[i] = m.view(inv)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) revokeInvitation(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("invitationId"))
	if err != nil {
		return httpx.NotFound("Invitation not found.")
	}
	inv, err := InvitationByID(r.Context(), m.db, id)
	if errors.Is(err, ErrNoInvitation) {
		return httpx.NotFound("Invitation not found.")
	}
	if err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	_, _, can, err := Visible(r.Context(), m.db, inv.ClassroomID, u)
	if errors.Is(err, ErrNoClassroom) {
		return httpx.NotFound("Invitation not found.")
	}
	if err != nil {
		return err
	}
	if !slices.Contains(can.Invite, inv.Role) {
		return httpx.Forbidden("You cannot revoke this invitation.")
	}
	if err := RevokeInvitation(r.Context(), m.db, m.now(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// preview is what an invitation shows, for its token.
func (m *Module) preview(ctx context.Context, inv Invitation, token string) (Preview, error) {
	c, err := Get(ctx, m.db, inv.ClassroomID)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Token: token, ClassroomName: c.Name, InstitutionName: c.InstitutionName, Role: inv.Role, Email: inv.Email,
		Status: inv.Status, InvitedByName: inv.InvitedByName}, nil
}

// refusal is the answer to an invitation that cannot be used.
func refusal(err error) error {
	switch {
	case errors.Is(err, ErrNoInvitation), errors.Is(err, ErrOrphaned), errors.Is(err, ErrNoClassroom):
		return httpx.NotFound(msgInvalidInvitation)
	case errors.Is(err, ErrExpired):
		return httpx.NotFound(msgExpired)
	case errors.Is(err, ErrUsed):
		return httpx.New(httpx.CodeConflict, "This invitation has already been used.")
	}
	return err
}

func (m *Module) getInvitation(w http.ResponseWriter, r *http.Request) error {
	token := r.PathValue("token")
	inv, err := ByToken(r.Context(), m.db, token)
	if err == nil {
		if err = inv.usable(m.now()); errors.Is(err, ErrUsed) {
			err = nil
		}
	}
	if err != nil {
		return refusal(err)
	}
	p, err := m.preview(r.Context(), inv, token)
	if err != nil {
		return refusal(err)
	}
	httpx.WriteJSON(w, http.StatusOK, p)
	return nil
}

func (m *Module) listMyInvitations(w http.ResponseWriter, r *http.Request) error {
	u, _ := accounts.CurrentUser(r.Context())
	out := []Preview{}
	if u.Email != nil {
		list, err := PendingFor(r.Context(), m.db, m.now(), *u.Email)
		if err != nil {
			return err
		}
		for _, inv := range list {
			p, err := m.preview(r.Context(), inv, "")
			if err != nil {
				return err
			}
			out = append(out, p)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) acceptInvitation(w http.ResponseWriter, r *http.Request) error {
	u, _ := accounts.CurrentUser(r.Context())
	token := r.PathValue("token")
	c, err := Accept(r.Context(), m.db, m.now(), token, u.ID, u.Email)
	if errors.Is(err, ErrNotInvitee) {
		inv, _ := ByToken(r.Context(), m.db, token)
		return httpx.Forbidden("This invitation was sent to " + inv.Email + ". Sign in with that email to accept it.")
	}
	if err != nil {
		return refusal(err)
	}
	role, err := RoleOf(r.Context(), m.db, c.ID, u.ID)
	if err != nil {
		return err
	}
	members, err := Members(r.Context(), m.db, c.ID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, summary(c, u, role, len(members)))
	return nil
}

// target is the member the path names and their role, or 404.
func (m *Module) target(r *http.Request, c Classroom) (uuid.UUID, roles.Role, error) {
	id, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		return id, "", httpx.NotFound(msgNoMember)
	}
	role, err := RoleOf(r.Context(), m.db, c.ID, id)
	if err == nil && role == "" {
		err = httpx.NotFound(msgNoMember)
	}
	return id, role, err
}

// setMemberRoleHandler changes a member's role: a head for any role but
// head, sent as the head; a platform admin with no role in the classroom
// for any, as this service.
func (m *Module) setMemberRole(w http.ResponseWriter, r *http.Request) error {
	c, role, _, err := m.visible(r)
	if err != nil {
		return err
	}
	user, current, err := m.target(r, c)
	if err != nil {
		return err
	}
	var in struct {
		Role roles.Role `json:"role"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if !slices.Contains(roles.All, in.Role) {
		return httpx.FieldError(httpx.CodeInvalidInput, "role", "Pick a role.")
	}
	u, _ := accounts.CurrentUser(r.Context())
	var by *uuid.UUID
	switch {
	case role == "" && u.PlatformAdmin:
	case role == roles.Head && user != u.ID && current != roles.Head && in.Role != roles.Head:
		by = &u.ID
	default:
		return httpx.Forbidden("You cannot change this member's role.")
	}
	if err := SetMemberRole(r.Context(), m.db, m.now(), c.ID, user, in.Role, by); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// removeMember takes a member out: a member leaving, but a head; a head
// removing anyone but a head; a facilitator removing a community scientist
// while they may invite them; each sent as themselves. A platform admin
// with no role removes anyone, as this service.
func (m *Module) removeMember(w http.ResponseWriter, r *http.Request) error {
	c, role, can, err := m.visible(r)
	if err != nil {
		return err
	}
	user, current, err := m.target(r, c)
	if err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	var by *uuid.UUID
	switch {
	case role == "" && u.PlatformAdmin:
	case user == u.ID && current != roles.Head,
		role == roles.Head && current != roles.Head,
		role == roles.Facilitator && current == roles.CommunityScientist && slices.Contains(can.Invite, roles.CommunityScientist):
		by = &u.ID
	default:
		return httpx.Forbidden("You cannot remove this member.")
	}
	if err := RemoveMember(r.Context(), m.db, m.now(), c.ID, user, by); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
