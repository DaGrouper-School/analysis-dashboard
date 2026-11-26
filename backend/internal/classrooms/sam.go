package classrooms

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// OutboxKinds are the classroom kind, sent as this service, and the
// membership kinds, sent as the person whose decision a change carries
// out or as this service.
func (m *Module) OutboxKinds() []samaccess.Kind {
	return []samaccess.Kind{
		{Name: Kind, Apply: m.applyClassroom},
		{Name: MemberKind, ByPerson: true, Apply: m.applyMember},
		{Name: MemberServiceKind, Apply: m.applyMember},
		{Name: PolicyKind, ByPerson: true, Apply: m.applyPolicy},
		{Name: ReleaseKind, ByPerson: true, Apply: m.applyRelease},
	}
}

// applyClassroom brings a classroom in SAM to its desired state: a space
// under the tenant's root, its four role groups homed there, each group's
// grants from the role templates, replaced when its sharing changes them,
// and the public's grants while it shows its data. Each step is recorded here once
// SAM confirms it, and has its own idempotency key, so a retry resumes
// where an attempt stopped.
func (m *Module) applyClassroom(ctx context.Context, a samaccess.Attempt) error {
	var want desired
	if err := json.Unmarshal(a.Payload, &want); err != nil {
		return samaccess.Hold(err)
	}
	id, err := uuid.Parse(strings.TrimPrefix(a.Entity, "classroom:"))
	if err != nil {
		return samaccess.Hold(err)
	}
	c, err := Get(ctx, m.db, id)
	if errors.Is(err, ErrNoClassroom) {
		return samaccess.Hold(err)
	}
	if err != nil {
		return err
	}
	if c.SpaceID == nil {
		space, err := a.SAM.CreateSpace(a.WithKey(ctx, "space"), client.SpaceInput{ParentID: m.root})
		if err != nil {
			return err
		}
		if err := confirm(ctx, m.db, id, "space_id", space.ID); err != nil {
			return err
		}
		c.SpaceID = &space.ID
	}
	for _, role := range roles.All {
		group, ok := c.Groups[role]
		if !ok {
			reg, err := a.SAM.Register(a.WithKey(ctx, "group:"+string(role)), client.PrincipalInput{Type: "group", HomeSpaceID: *c.SpaceID})
			if err != nil {
				return err
			}
			if err := confirm(ctx, m.db, id, groupColumns[role], reg.Principal.ID); err != nil {
				return err
			}
			group = reg.Principal.ID
		}
		if err := roles.IssueMissing(ctx, a, group, *c.SpaceID, roles.Grants(role, want.Sharing), "grant:"+string(role)); err != nil {
			return err
		}
	}
	if m.cfg.SAM == nil {
		return nil
	}
	// The public's grants, as this service, which holds dataset:publish.
	return roles.Reconcile(ctx, a, m.cfg.SAM.Tenant().AnonymousID, *c.SpaceID, roles.PublicGrants(want.Sharing, want.Decimals), "public", roles.PublicLabel)
}
