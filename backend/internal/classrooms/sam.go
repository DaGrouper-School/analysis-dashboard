package classrooms

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
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
	}
}

// applyClassroom brings a classroom in SAM to its desired state: a space
// under the tenant's root, its four role groups homed there, and each
// group's grants from the role templates. Each step is recorded here once
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
		if err := issueGrants(ctx, a, group, *c.SpaceID, role, want.Sharing); err != nil {
			return err
		}
	}
	return nil
}

// issueGrants issues the grants of role's template that group does not
// hold yet, each named by its label.
func issueGrants(ctx context.Context, a samaccess.Attempt, group, space uuid.UUID, role roles.Role, sharing roles.Sharing) error {
	held, err := a.SAM.Grants(ctx, client.GrantQuery{SubjectID: &group, SpaceID: &space}, client.PageQuery{Limit: 100})
	if err != nil {
		return err
	}
	for _, g := range roles.Grants(role, sharing) {
		if slices.ContainsFunc(held.Items, func(h client.Grant) bool { return h.RevokedAt == nil && slices.Contains(h.Labels, g.Name) }) {
			continue
		}
		if _, err := a.SAM.Issue(a.WithKey(ctx, "grant:"+string(role)+":"+g.Name), g.Input(group, space)); err != nil {
			return err
		}
	}
	return nil
}
