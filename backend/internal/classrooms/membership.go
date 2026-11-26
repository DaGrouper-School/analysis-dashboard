package classrooms

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// The outbox kinds of membership: one sent as the person whose decision it
// carries out, one as this service, for a platform admin with no role in
// the classroom.
const (
	MemberKind        = "membership"
	MemberServiceKind = "membership_by_service"
)

func memberEntity(classroom, user uuid.UUID) string {
	return "member:" + classroom.String() + ":" + user.String()
}

// memberChange is what a membership change sends: who it is sent as, nil
// for this service. The role wanted is the member's row here.
type memberChange struct {
	By *uuid.UUID `json:"by"`
}

// ErrNoMember means the user is not a member of the classroom.
var ErrNoMember = errors.New("not a member")

// memberRole is user's role in a classroom, "" for none, locking the row.
func memberRole(ctx context.Context, q db.Querier, classroom, user uuid.UUID) (roles.Role, error) {
	var role roles.Role
	err := q.QueryRow(ctx, `SELECT role FROM classroom_members WHERE classroom_id = $1 AND user_id = $2 AND NOT removed FOR UPDATE`,
		classroom, user).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// setRole makes user a member of c in role, and sends it to SAM as by, or
// this service. takesAway marks a change after which they may do less.
func setRole(ctx context.Context, q db.Querier, now time.Time, c Classroom, user uuid.UUID, role roles.Role, by *uuid.UUID, takesAway bool) error {
	if _, err := q.Exec(ctx, `
		INSERT INTO classroom_members (classroom_id, user_id, role, joined_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (classroom_id, user_id) DO UPDATE SET role = $3, removed = false,
			joined_at = CASE WHEN classroom_members.removed THEN $4 ELSE classroom_members.joined_at END`, c.ID, user, role, now); err != nil {
		return err
	}
	if err := revokeOutgrown(ctx, q, now, c, user, role); err != nil {
		return err
	}
	return sendMember(ctx, q, now, c.ID, user, by, takesAway)
}

// removeMember takes user out of c here at once, and from SAM as by, or
// this service. Their pending invitations are revoked.
func removeMember(ctx context.Context, q db.Querier, now time.Time, c Classroom, user uuid.UUID, by *uuid.UUID) error {
	if _, err := q.Exec(ctx, `UPDATE classroom_members SET removed = true WHERE classroom_id = $1 AND user_id = $2`, c.ID, user); err != nil {
		return err
	}
	if err := revokeOutgrown(ctx, q, now, c, user, ""); err != nil {
		return err
	}
	return sendMember(ctx, q, now, c.ID, user, by, true)
}

func sendMember(ctx context.Context, q db.Querier, now time.Time, classroom, user uuid.UUID, by *uuid.UUID, takesAway bool) error {
	kind := MemberKind
	if by == nil {
		kind = MemberServiceKind
	}
	_, err := samaccess.Enqueue(ctx, q, now, samaccess.Change{Entity: memberEntity(classroom, user), Kind: kind, Actor: by,
		Payload: memberChange{By: by}, TakesAccessAway: takesAway})
	return err
}

// SetMemberRole changes a member's role, sent as by, or this service.
// Narrowing it takes effect here at once.
func SetMemberRole(ctx context.Context, q db.Querier, now time.Time, classroom, user uuid.UUID, role roles.Role, by *uuid.UUID) error {
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		c, err := lock(ctx, tx, classroom)
		if err != nil {
			return err
		}
		current, err := memberRole(ctx, tx, classroom, user)
		switch {
		case err != nil:
			return err
		case current == "":
			return ErrNoMember
		case current == role:
			return nil
		}
		return setRole(ctx, tx, now, c, user, role, by, rank[role] < rank[current])
	})
}

// RemoveMember takes a member out of a classroom, sent as by, or this
// service. It takes effect here at once.
func RemoveMember(ctx context.Context, q db.Querier, now time.Time, classroom, user uuid.UUID, by *uuid.UUID) error {
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		c, err := lock(ctx, tx, classroom)
		if err != nil {
			return err
		}
		current, err := memberRole(ctx, tx, classroom, user)
		if err != nil {
			return err
		}
		if current == "" {
			return ErrNoMember
		}
		return removeMember(ctx, tx, now, c, user, by)
	})
}

// memberRow is a member as SAM is to hold them.
type memberRow struct {
	role    roles.Role
	samRole *roles.Role
	removed bool
}

func readMember(ctx context.Context, q db.Querier, classroom, user uuid.UUID) (memberRow, bool, error) {
	var m memberRow
	err := q.QueryRow(ctx, `SELECT role, sam_role, removed FROM classroom_members WHERE classroom_id = $1 AND user_id = $2`, classroom, user).
		Scan(&m.role, &m.samRole, &m.removed)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, false, nil
	}
	return m, err == nil, err
}

func setSAMRole(ctx context.Context, q db.Querier, classroom, user uuid.UUID, role *roles.Role) error {
	_, err := q.Exec(ctx, `UPDATE classroom_members SET sam_role = $3 WHERE classroom_id = $1 AND user_id = $2`, classroom, user, role)
	return err
}

// applyMember brings a member's role groups in SAM in line with their row
// here: out of the group SAM holds them in, if it is not their role's,
// then into their role's, each step recorded once SAM confirms it, so they
// are never in two. A change sent as a member waits until that member's
// own membership has reached SAM.
func (m *Module) applyMember(ctx context.Context, a samaccess.Attempt) error {
	var change memberChange
	if err := json.Unmarshal(a.Payload, &change); err != nil {
		return samaccess.Hold(err)
	}
	classroom, user, err := parseMember(a.Entity)
	if err != nil {
		return samaccess.Hold(err)
	}
	row, found, err := readMember(ctx, m.db, classroom, user)
	if err != nil || !found {
		return err
	}
	c, principal, err := m.ready(ctx, classroom, user, change.By)
	if err != nil {
		return err
	}
	if row.samRole != nil && (row.removed || *row.samRole != row.role) {
		err := a.SAM.RemoveMember(a.WithKey(ctx, "remove:"+string(*row.samRole)), c.Groups[*row.samRole], principal)
		var e *client.Error
		if err != nil && (!errors.As(err, &e) || e.Status != 404) {
			return err
		}
		if err := setSAMRole(ctx, m.db, classroom, user, nil); err != nil {
			return err
		}
		row.samRole = nil
	}
	switch {
	case row.removed:
		_, err := m.db.Exec(ctx, `DELETE FROM classroom_members WHERE classroom_id = $1 AND user_id = $2 AND removed`, classroom, user)
		return err
	case row.samRole == nil:
		if err := a.SAM.AddMember(a.WithKey(ctx, "add:"+string(row.role)), c.Groups[row.role], principal); err != nil {
			return err
		}
		return setSAMRole(ctx, m.db, classroom, user, &row.role)
	}
	return nil
}

// ready is the classroom and the member's SAM principal once a membership
// change can be sent: SAM has the classroom's groups and the member, and,
// for a change sent as another member, that member's own membership.
func (m *Module) ready(ctx context.Context, classroom, user uuid.UUID, by *uuid.UUID) (Classroom, uuid.UUID, error) {
	c, err := Get(ctx, m.db, classroom)
	if err != nil {
		return c, uuid.Nil, err
	}
	if len(c.Groups) < len(roles.All) {
		return c, uuid.Nil, samaccess.ErrNotYet
	}
	if by != nil && *by != user {
		actor, found, err := readMember(ctx, m.db, classroom, *by)
		if err != nil {
			return c, uuid.Nil, err
		}
		if found && !actor.removed && (actor.samRole == nil || *actor.samRole != actor.role) {
			return c, uuid.Nil, samaccess.ErrNotYet
		}
	}
	u, err := accounts.Get(ctx, m.db, user)
	if err != nil {
		return c, uuid.Nil, err
	}
	if u.Principal == nil {
		return c, uuid.Nil, samaccess.ErrNotYet
	}
	return c, *u.Principal, nil
}

func parseMember(entity string) (classroom, user uuid.UUID, err error) {
	ids := strings.Split(strings.TrimPrefix(entity, "member:"), ":")
	if len(ids) != 2 {
		return uuid.Nil, uuid.Nil, errors.New("not a membership: " + entity)
	}
	if classroom, err = uuid.Parse(ids[0]); err != nil {
		return
	}
	user, err = uuid.Parse(ids[1])
	return
}

// InSAM reports whether SAM holds user as a member of a classroom in the
// role they hold here, so a change sent as them can rely on it.
func InSAM(ctx context.Context, q db.Querier, classroom, user uuid.UUID) (bool, error) {
	m, found, err := readMember(ctx, q, classroom, user)
	return found && !m.removed && m.samRole != nil && *m.samRole == m.role, err
}
