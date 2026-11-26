package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Platform admins in SAM: one group at the tenant root holding the admins'
// grants (docs/design/accounts-and-sign-in.md#platform-admins), its members
// kept in step with users.platform_admin. Both are sent as this service:
// an operator decides who is an admin.

// The outbox kinds of platform admins.
const (
	AdminsGroupKind = "admins_group"
	AdminKind       = "admin_membership"
)

// adminsGroup names the admins' group in tenant_groups and the outbox.
const adminsGroup = "admins"

func adminEntity(user uuid.UUID) string { return "admin:" + user.String() }

// ErrNoAccount means no account has the email or ID named.
var ErrNoAccount = errors.New("no such account")

// Admins are the platform admins, by name.
func Admins(ctx context.Context, q db.Querier) ([]User, error) {
	rows, err := q.Query(ctx, `SELECT `+userColumns+` FROM users WHERE platform_admin ORDER BY full_name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// FindAccount is the account with ID who, or with who as its verified
// email.
func FindAccount(ctx context.Context, q db.Querier, who string) (User, error) {
	var u User
	var err error
	if id, perr := uuid.Parse(who); perr == nil {
		u, err = getUser(ctx, q, id)
	} else {
		u, err = scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1) AND email_verified`, strings.TrimSpace(who)))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNoAccount
	}
	return u, err
}

// SetAdmin makes the account who names a platform admin, or no longer one,
// as by decided. It takes effect here at once, is logged, and reaches SAM
// through the outbox: the group first, then the membership. It reports
// whether anything changed.
func SetAdmin(ctx context.Context, q db.Querier, now time.Time, by settings.Actor, who string, admin bool) (User, bool, error) {
	var u User
	var changed bool
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		found, err := FindAccount(ctx, tx, who)
		if err != nil {
			return err
		}
		if u, err = lockUser(ctx, tx, found.ID); err != nil {
			return err
		}
		if u.PlatformAdmin == admin {
			return nil
		}
		changed, u.PlatformAdmin = true, admin
		if _, err := tx.Exec(ctx, `UPDATE users SET platform_admin = $2, updated_at = $3 WHERE id = $1`, u.ID, admin, now); err != nil {
			return err
		}
		if admin {
			if err := queueGroup(ctx, tx, now); err != nil {
				return err
			}
		}
		if _, err := samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: adminEntity(u.ID), Kind: AdminKind,
			Payload: adminState{Admin: admin}, TakesAccessAway: !admin}); err != nil {
			return err
		}
		summary := "Made " + name(u) + " a platform admin"
		if !admin {
			summary = name(u) + " is no longer a platform admin"
		}
		return settings.Record(ctx, tx, now, by, settings.Change{Kind: settings.KindPlatformAdmin, Subject: u.ID.String(), Summary: summary})
	})
	return u, changed, err
}

func name(u User) string {
	if u.Email != nil {
		return u.FullName + " (" + *u.Email + ")"
	}
	return u.FullName
}

// queueGroup queues the admins' group until SAM has it.
func queueGroup(ctx context.Context, q db.Querier, now time.Time) error {
	if _, found, err := groupID(ctx, q); err != nil || found {
		return err
	}
	if waiting, err := samaccess.Waiting(ctx, q, adminsGroup); err != nil || waiting {
		return err
	}
	_, err := samaccess.Enqueue(ctx, q, now, samaccess.Change{Entity: adminsGroup, Kind: AdminsGroupKind, Payload: struct{}{}})
	return err
}

func groupID(ctx context.Context, q db.Querier) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT group_id FROM tenant_groups WHERE name = $1`, adminsGroup).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, false, nil
	}
	return id, err == nil, err
}

// adminState is what SAM is to hold of an admin: in the group or not. The
// applier follows the flag here, which may be newer.
type adminState struct {
	Admin bool `json:"admin"`
}

// applyAdminsGroup creates the admins' group at the tenant root, records
// it once SAM confirms it, and issues it the admins' grants there.
func (m *Module) applyAdminsGroup(ctx context.Context, a samaccess.Attempt) error {
	root := m.sam.Tenant().RootSpaceID
	id, found, err := groupID(ctx, m.db)
	if err != nil {
		return err
	}
	if !found {
		reg, err := a.SAM.Register(a.WithKey(ctx, "group"), client.PrincipalInput{Type: "group", HomeSpaceID: root})
		if err != nil {
			return err
		}
		id = reg.Principal.ID
		if _, err := m.db.Exec(ctx, `INSERT INTO tenant_groups (name, group_id, created_at) VALUES ($1, $2, $3)
			ON CONFLICT (name) DO NOTHING`, adminsGroup, id, m.now()); err != nil {
			return err
		}
	}
	return roles.IssueMissing(ctx, a, id, root, roles.AdminGrants(), "grant")
}

// applyAdmin puts the user in the admins' group, or takes them out, as the
// flag here says, and records what SAM holds. It waits for the group and
// for the user's principal.
func (m *Module) applyAdmin(ctx context.Context, a samaccess.Attempt) error {
	var state adminState
	if err := json.Unmarshal(a.Payload, &state); err != nil {
		return samaccess.Hold(err)
	}
	user, err := uuid.Parse(strings.TrimPrefix(a.Entity, "admin:"))
	if err != nil {
		return samaccess.Hold(err)
	}
	group, found, err := groupID(ctx, m.db)
	if err != nil {
		return err
	}
	if !found {
		return samaccess.ErrNotYet
	}
	u, err := getUser(ctx, m.db, user)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.Principal == nil {
		return samaccess.ErrNotYet
	}
	if u.PlatformAdmin {
		err = a.SAM.AddMember(a.WithKey(ctx, "add"), group, *u.Principal)
	} else {
		err = a.SAM.RemoveMember(a.WithKey(ctx, "remove"), group, *u.Principal)
		var e *client.Error
		if errors.As(err, &e) && e.Status == 404 {
			err = nil
		}
	}
	if err != nil {
		return err
	}
	_, err = m.db.Exec(ctx, `UPDATE users SET sam_admin = $2 WHERE id = $1`, user, u.PlatformAdmin)
	return err
}

// reconcileAdmins asks SAM, as each admin and each user SAM holds as one,
// whether they may register devices at the tenant root, and queues a
// correction where SAM and the flag here differ. Heads hold that right
// only on their classroom's space.
func (m *Module) reconcileAdmins(ctx context.Context) error {
	if !m.guard.Done() {
		return nil
	}
	rows, err := m.db.Query(ctx, `SELECT `+userColumns+` FROM users WHERE (platform_admin OR sam_admin) AND sam_principal_id IS NOT NULL ORDER BY id`)
	if err != nil {
		return err
	}
	users, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
	if err != nil {
		return err
	}
	root := m.sam.Tenant().RootSpaceID
	for _, u := range users {
		if err := m.reconcileAdmin(ctx, u, root); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.WarnContext(ctx, "reconciler could not compare an admin with SAM", "user", u.ID, "err", err)
		}
	}
	return nil
}

func (m *Module) reconcileAdmin(ctx context.Context, u User, root uuid.UUID) error {
	if waiting, err := samaccess.Waiting(ctx, m.db, adminEntity(u.ID)); err != nil || waiting {
		return err
	}
	as, err := m.sam.User(samaccess.Person{ParticipantID: u.ParticipantID, Registered: true})
	if err != nil {
		return err
	}
	results, err := as.Check(ctx, []client.Check{{Action: roles.DeviceRegister, SpaceID: root}})
	if err != nil || len(results) != 1 || results[0].Allowed == u.PlatformAdmin {
		return err
	}
	slog.WarnContext(ctx, "reconciler found an admin SAM holds otherwise; correcting", "user", u.ID, "admin", u.PlatformAdmin)
	return pgx.BeginFunc(ctx, m.db, func(tx pgx.Tx) error {
		if u.PlatformAdmin {
			if err := queueGroup(ctx, tx, m.now()); err != nil {
				return err
			}
		}
		_, err := samaccess.Enqueue(ctx, tx, m.now(), samaccess.Change{Entity: adminEntity(u.ID), Kind: AdminKind,
			Payload: adminState{Admin: u.PlatformAdmin}, TakesAccessAway: !u.PlatformAdmin})
		return err
	})
}
