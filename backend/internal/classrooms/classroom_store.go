package classrooms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
)

// The store's queries on classrooms and classroom_members.

const classroomColumns = `id, code, name, institution_name, setup, stage_id, region_id, sharing, terms, profile, head_email, created_at,
	space_id, heads_group, facilitators_group, scientists_group, researchers_group, released_through, sam_policy_version`

// groupColumns are the role groups' columns, by role.
var groupColumns = map[roles.Role]string{
	roles.Head:               "heads_group",
	roles.Facilitator:        "facilitators_group",
	roles.CommunityScientist: "scientists_group",
	roles.Researcher:         "researchers_group",
}

func scanClassroom(r pgx.Row) (Classroom, error) {
	var c Classroom
	var sharing, terms, profile []byte
	groups := make([]*uuid.UUID, len(roles.All))
	if err := r.Scan(&c.ID, &c.Code, &c.Name, &c.InstitutionName, &c.Setup, &c.StageID, &c.RegionID, &sharing, &terms, &profile,
		&c.HeadEmail, &c.CreatedAt, &c.SpaceID, &groups[0], &groups[1], &groups[2], &groups[3], &c.ReleasedThrough, &c.SAMPolicyVersion); err != nil {
		return c, err
	}
	c.Groups = map[roles.Role]uuid.UUID{}
	for i, role := range roles.All {
		if groups[i] != nil {
			c.Groups[role] = *groups[i]
		}
	}
	return c, errors.Join(json.Unmarshal(sharing, &c.Sharing), json.Unmarshal(terms, &c.Terms), json.Unmarshal(profile, &c.Profile))
}

func insert(ctx context.Context, q db.Querier, c Classroom, by uuid.UUID) error {
	sharing, terms, profile, err := marshal(c)
	if err != nil {
		return err
	}
	var creator *uuid.UUID
	if by != uuid.Nil {
		creator = &by
	}
	_, err = q.Exec(ctx, `
		INSERT INTO classrooms (id, code, name, institution_name, setup, stage_id, region_id, sharing, terms, profile, head_email,
			created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)`,
		c.ID, c.Code, c.Name, c.InstitutionName, c.Setup, c.StageID, c.RegionID, sharing, terms, profile, c.HeadEmail, creator, c.CreatedAt)
	return err
}

func marshal(c Classroom) (sharing, terms, profile []byte, err error) {
	if sharing, err = json.Marshal(c.Sharing); err != nil {
		return
	}
	if terms, err = json.Marshal(c.Terms); err != nil {
		return
	}
	profile, err = json.Marshal(c.Profile)
	return
}

// save stores what an admin or head changes of c.
func save(ctx context.Context, q db.Querier, now time.Time, c Classroom) error {
	sharing, terms, profile, err := marshal(c)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		UPDATE classrooms SET name = $2, institution_name = $3, stage_id = $4, region_id = $5, sharing = $6, terms = $7, profile = $8,
			updated_at = $9 WHERE id = $1`, c.ID, c.Name, c.InstitutionName, c.StageID, c.RegionID, sharing, terms, profile, now)
	return err
}

// Get is the classroom with id, or ErrNoClassroom.
func Get(ctx context.Context, q db.Querier, id uuid.UUID) (Classroom, error) {
	return one(ctx, q, `SELECT `+classroomColumns+` FROM classrooms WHERE id = $1`, id)
}

// ByCode is the classroom with code, or ErrNoClassroom.
func ByCode(ctx context.Context, q db.Querier, code string) (Classroom, error) {
	return one(ctx, q, `SELECT `+classroomColumns+` FROM classrooms WHERE code = $1`, code)
}

func lock(ctx context.Context, q db.Querier, id uuid.UUID) (Classroom, error) {
	return one(ctx, q, `SELECT `+classroomColumns+` FROM classrooms WHERE id = $1 FOR UPDATE`, id)
}

func one(ctx context.Context, q db.Querier, sql string, arg any) (Classroom, error) {
	c, err := scanClassroom(q.QueryRow(ctx, sql, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNoClassroom
	}
	return c, err
}

// Listed is a classroom with the viewer's role in it ("" for none) and
// how many members it has.
type Listed struct {
	Classroom
	Role    roles.Role
	Members int
}

// List is every classroom for a platform admin, and the classrooms user
// is a member of for anyone else, newest first.
func List(ctx context.Context, q db.Querier, user uuid.UUID, admin bool) ([]Listed, error) {
	rows, err := q.Query(ctx, `
		SELECT `+prefixed("c.", classroomColumns)+`, coalesce(m.role, ''),
			(SELECT count(*) FROM classroom_members WHERE classroom_id = c.id AND NOT removed)
		FROM classrooms c LEFT JOIN classroom_members m ON m.classroom_id = c.id AND m.user_id = $1 AND NOT m.removed
		WHERE $2 OR m.user_id IS NOT NULL ORDER BY c.created_at DESC, c.id`, user, admin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Listed
	for rows.Next() {
		var l Listed
		var sharing, terms, profile []byte
		groups := make([]*uuid.UUID, len(roles.All))
		if err := rows.Scan(&l.ID, &l.Code, &l.Name, &l.InstitutionName, &l.Setup, &l.StageID, &l.RegionID, &sharing, &terms, &profile,
			&l.HeadEmail, &l.CreatedAt, &l.SpaceID, &groups[0], &groups[1], &groups[2], &groups[3], &l.ReleasedThrough, &l.SAMPolicyVersion,
			&l.Role, &l.Members); err != nil {
			return nil, err
		}
		l.Groups = map[roles.Role]uuid.UUID{}
		for i, role := range roles.All {
			if groups[i] != nil {
				l.Groups[role] = *groups[i]
			}
		}
		if err := errors.Join(json.Unmarshal(sharing, &l.Sharing), json.Unmarshal(terms, &l.Terms), json.Unmarshal(profile, &l.Profile)); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// prefixed is columns, a comma-separated list, each with prefix p.
func prefixed(p, columns string) string {
	parts := strings.Split(columns, ",")
	for i, c := range parts {
		parts[i] = p + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}

// Member is one member of a classroom.
type Member struct {
	UserID   uuid.UUID
	FullName string
	Email    *string
	Role     roles.Role
	JoinedAt time.Time
}

// Members are a classroom's members, heads first, then by name.
func Members(ctx context.Context, q db.Querier, classroom uuid.UUID) ([]Member, error) {
	rows, err := q.Query(ctx, `
		SELECT m.user_id, coalesce(u.full_name, ''), u.email, m.role, m.joined_at
		FROM classroom_members m JOIN users u ON u.id = m.user_id WHERE m.classroom_id = $1 AND NOT m.removed
		ORDER BY array_position(ARRAY['institution_head', 'facilitator', 'community_scientist', 'researcher'], m.role), u.full_name, m.user_id`, classroom)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (m Member, err error) {
		return m, r.Scan(&m.UserID, &m.FullName, &m.Email, &m.Role, &m.JoinedAt)
	})
}

// RoleOf is user's role in a classroom, "" for none.
func RoleOf(ctx context.Context, q db.Querier, classroom, user uuid.UUID) (roles.Role, error) {
	var role roles.Role
	err := q.QueryRow(ctx, `SELECT role FROM classroom_members WHERE classroom_id = $1 AND user_id = $2 AND NOT removed`, classroom, user).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// exists reports a problem when table, stages or regions, has no row id.
func exists(ctx context.Context, q db.Querier, table, at, id string) []string {
	var found bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = $1)`, id).Scan(&found); err != nil {
		return []string{fmt.Sprintf("%s: %v", at, err)}
	}
	if !found {
		return []string{fmt.Sprintf("%s: %q is not one of the %s", at, id, table)}
	}
	return nil
}

// confirm records an ID SAM has confirmed: the space, or a role group's.
func confirm(ctx context.Context, q db.Querier, id uuid.UUID, column string, value uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE classrooms SET `+column+` = $2 WHERE id = $1`, id, value)
	return err
}
