package classrooms

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
)

// Invitation is an invitation to a classroom, as stored. Token is set only
// on one just made: the store keeps its hash.
type Invitation struct {
	ID            uuid.UUID
	ClassroomID   uuid.UUID
	Email         string
	Role          roles.Role
	InvitedBy     *uuid.UUID
	InvitedByName string
	// ByService is whether accepting is sent to SAM as this service: an
	// invitation from a platform admin with no role in the classroom, or
	// the head's when it was created.
	ByService bool
	Status    string
	CreatedAt time.Time
	ExpiresAt time.Time
	Token     string
}

// Statuses of an invitation.
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRevoked  = "revoked"
)

// Inviter is who sends an invitation: a member, by their own rights, or a
// platform admin with no role, as this service.
type Inviter struct {
	UserID  *uuid.UUID
	Name    string
	Service bool
}

// Errors of invitations.
var (
	ErrNoInvitation  = errors.New("no such invitation")
	ErrExpired       = errors.New("the invitation has expired")
	ErrOrphaned      = errors.New("the invitation's inviter is gone")
	ErrUsed          = errors.New("the invitation has been used")
	ErrNotInvitee    = errors.New("the invitation was sent to someone else")
	ErrAlreadyMember = errors.New("already a member")
)

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Invite invites each email to role in c, open for ttl. Inviting an email
// again replaces its pending invitation, like a resend.
func Invite(ctx context.Context, q db.Querier, now time.Time, ttl time.Duration, c Classroom, emails []string, role roles.Role, by Inviter) ([]Invitation, error) {
	var out []Invitation
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		for _, email := range emails {
			if _, err := tx.Exec(ctx, `UPDATE invitations SET status = 'revoked', decided_at = $3
				WHERE classroom_id = $1 AND email = $2 AND status = 'pending'`, c.ID, email, now); err != nil {
				return err
			}
			inv := Invitation{ID: uuid.New(), ClassroomID: c.ID, Email: email, Role: role, InvitedBy: by.UserID, InvitedByName: by.Name,
				ByService: by.Service, Status: StatusPending, CreatedAt: now, ExpiresAt: now.Add(ttl), Token: newToken()}
			if _, err := tx.Exec(ctx, `
				INSERT INTO invitations (id, classroom_id, email, role, token_hash, invited_by, invited_by_name, by_service, status, created_at, expires_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9, $10)`,
				inv.ID, c.ID, email, role, hashToken(inv.Token), by.UserID, by.Name, by.Service, now, inv.ExpiresAt); err != nil {
				return err
			}
			out = append(out, inv)
		}
		return nil
	})
	return out, err
}

const invitationColumns = `id, classroom_id, email, role, invited_by, invited_by_name, by_service, status, created_at, expires_at`

func scanInvitation(r pgx.Row) (Invitation, error) {
	var i Invitation
	err := r.Scan(&i.ID, &i.ClassroomID, &i.Email, &i.Role, &i.InvitedBy, &i.InvitedByName, &i.ByService, &i.Status, &i.CreatedAt, &i.ExpiresAt)
	return i, err
}

func invitations(ctx context.Context, q db.Querier, sql string, args ...any) ([]Invitation, error) {
	rows, err := q.Query(ctx, `SELECT `+invitationColumns+` FROM invitations `+sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Invitation, error) { return scanInvitation(r) })
}

// usable reports why an invitation can no longer be accepted, or nil.
func (i Invitation) usable(now time.Time) error {
	switch {
	case i.Status == StatusRevoked:
		return ErrNoInvitation
	case i.Status == StatusAccepted:
		return ErrUsed
	case !now.Before(i.ExpiresAt):
		return ErrExpired
	case !i.ByService && i.InvitedBy == nil:
		return ErrOrphaned
	}
	return nil
}

// ByToken is the invitation token opens, or ErrNoInvitation.
func ByToken(ctx context.Context, q db.Querier, token string) (Invitation, error) {
	i, err := scanInvitation(q.QueryRow(ctx, `SELECT `+invitationColumns+` FROM invitations WHERE token_hash = $1`, hashToken(token)))
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNoInvitation
	}
	return i, err
}

// PendingFor are the invitations open to email, newest first.
func PendingFor(ctx context.Context, q db.Querier, now time.Time, email string) ([]Invitation, error) {
	return invitations(ctx, q, `WHERE email = $1 AND status = 'pending' AND expires_at > $2 AND (by_service OR invited_by IS NOT NULL)
		ORDER BY created_at DESC, id`, strings.ToLower(email), now)
}

// PendingIn are a classroom's open invitations, newest first.
func PendingIn(ctx context.Context, q db.Querier, now time.Time, classroom uuid.UUID) ([]Invitation, error) {
	return invitations(ctx, q, `WHERE classroom_id = $1 AND status = 'pending' AND expires_at > $2 AND (by_service OR invited_by IS NOT NULL)
		ORDER BY created_at DESC, id`, classroom, now)
}

// InvitationByID is the invitation with id, or ErrNoInvitation.
func InvitationByID(ctx context.Context, q db.Querier, id uuid.UUID) (Invitation, error) {
	i, err := scanInvitation(q.QueryRow(ctx, `SELECT `+invitationColumns+` FROM invitations WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNoInvitation
	}
	return i, err
}

// RevokeInvitation revokes an invitation.
func RevokeInvitation(ctx context.Context, q db.Querier, now time.Time, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE invitations SET status = 'revoked', decided_at = $2 WHERE id = $1 AND status = 'pending'`, id, now)
	return err
}

// revokeOutgrown revokes the pending invitations user sent in classroom
// that a member of role can no longer hand on; all of them when role is
// "", as when they leave. An invitation follows its inviter.
func revokeOutgrown(ctx context.Context, q db.Querier, now time.Time, c Classroom, user uuid.UUID, role roles.Role) error {
	keep := []string{}
	if role != "" {
		for _, r := range roles.PermissionsFor(&roles.Viewer{}, role, c.Sharing).Invite {
			keep = append(keep, string(r))
		}
	}
	_, err := q.Exec(ctx, `UPDATE invitations SET status = 'revoked', decided_at = $4
		WHERE classroom_id = $1 AND invited_by = $2 AND NOT by_service AND status = 'pending' AND NOT role = ANY($3)`, c.ID, user, keep, now)
	return err
}

// rank orders the roles by what they may do, for a role that only grows
// when an invitation is accepted.
var rank = map[roles.Role]int{roles.Researcher: 1, roles.CommunityScientist: 2, roles.Facilitator: 3, roles.Head: 4}

// Accept joins user, whose email the invitation was sent to, to the
// classroom: in its role, or the role they hold if that is above it. The
// membership is sent to SAM as the inviter, or this service.
func Accept(ctx context.Context, q db.Querier, now time.Time, token string, user uuid.UUID, email *string) (Classroom, error) {
	var c Classroom
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		inv, err := ByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM invitations WHERE id = $1 FOR UPDATE`, inv.ID); err != nil {
			return err
		}
		if err := inv.usable(now); err != nil {
			return err
		}
		if email == nil || !strings.EqualFold(*email, inv.Email) {
			return ErrNotInvitee
		}
		if c, err = lock(ctx, tx, inv.ClassroomID); err != nil {
			return err
		}
		current, err := memberRole(ctx, tx, c.ID, user)
		if err != nil {
			return err
		}
		if current == "" || rank[inv.Role] > rank[current] {
			var by *uuid.UUID
			if !inv.ByService {
				by = inv.InvitedBy
			}
			if err := setRole(ctx, tx, now, c, user, inv.Role, by, false); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE invitations SET status = 'accepted', decided_at = $2 WHERE id = $1`, inv.ID, now)
		return err
	})
	return c, err
}

// RoleLabels are the roles as people read them.
var RoleLabels = map[roles.Role]string{
	roles.Head: "institution head", roles.Facilitator: "facilitator", roles.CommunityScientist: "community scientist", roles.Researcher: "researcher",
}

// InvitationText is the email an invitation is sent as.
func InvitationText(c Classroom, inv Invitation, link string) (subject, text string) {
	subject = fmt.Sprintf("You're invited to %s", c.Name)
	text = fmt.Sprintf("%s invited you to join %s, a Global Classroom at %s, as %s.\n\nOpen this link to accept:\n%s\n\n"+
		"The link works until %s, for %s only. If you did not expect it, you can ignore this email.\n",
		inv.InvitedByName, c.Name, c.InstitutionName, RoleLabels[inv.Role], link, inv.ExpiresAt.UTC().Format("2 January 2006"), inv.Email)
	return subject, text
}

// cleanEmails are emails trimmed, lower-cased, blanks and repeats left
// out, and those that are not addresses.
func cleanEmails(emails []string) (good, bad []string) {
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		switch {
		case e == "" || slices.Contains(good, e) || slices.Contains(bad, e):
		case validEmail(e):
			good = append(good, e)
		default:
			bad = append(bad, e)
		}
	}
	return good, bad
}
