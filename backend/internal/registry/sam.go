package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"
)

// SAM is SAM's registry as effective in this tenant's root: the core's
// entries and this tenant's own, with the unit catalogue and the
// derivations, as read at FetchedAt.
type SAM struct {
	Domains     []client.Domain
	Parameters  []client.Parameter
	DeviceTypes []client.DeviceType
	Templates   []client.Template
	Units       []client.Unit
	Derivations []client.Derivation
	FetchedAt   time.Time
}

// pageSize is the most SAM gives in one page.
const pageSize = 200

// Fetch reads SAM's registry through c, the entries effective in root.
// The registry is open to every caller.
func Fetch(ctx context.Context, c *client.Client, root uuid.UUID, now time.Time) (*SAM, error) {
	in := client.RegistryQuery{SpaceID: &root}
	s := &SAM{FetchedAt: now}
	var err error
	if s.Domains, err = all(ctx, "domains", func(q client.PageQuery) (client.Page[client.Domain], error) {
		return c.Domains(ctx, in, q)
	}); err != nil {
		return nil, err
	}
	if s.Parameters, err = all(ctx, "parameters", func(q client.PageQuery) (client.Page[client.Parameter], error) {
		return c.Parameters(ctx, in, q)
	}); err != nil {
		return nil, err
	}
	if s.DeviceTypes, err = all(ctx, "device types", func(q client.PageQuery) (client.Page[client.DeviceType], error) {
		return c.DeviceTypes(ctx, in, q)
	}); err != nil {
		return nil, err
	}
	if s.Templates, err = all(ctx, "templates", func(q client.PageQuery) (client.Page[client.Template], error) {
		return c.Templates(ctx, in, q)
	}); err != nil {
		return nil, err
	}
	if s.Derivations, err = all(ctx, "derivations", func(q client.PageQuery) (client.Page[client.Derivation], error) {
		return c.Derivations(ctx, q)
	}); err != nil {
		return nil, err
	}
	if s.Units, err = c.Units(ctx); err != nil {
		return nil, fmt.Errorf("reading SAM's units: %w", err)
	}
	return s, nil
}

// all reads every page of a list.
func all[T any](ctx context.Context, what string, page func(client.PageQuery) (client.Page[T], error)) ([]T, error) {
	var out []T
	q := client.PageQuery{Limit: pageSize}
	for {
		p, err := page(q)
		if err != nil {
			return nil, fmt.Errorf("reading SAM's %s: %w", what, err)
		}
		out = append(out, p.Items...)
		if p.NextCursor == "" {
			return out, ctx.Err()
		}
		q.Cursor = p.NextCursor
	}
}
