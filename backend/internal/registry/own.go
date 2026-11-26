package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Namespace is this tenant's in SAM's registry: its own entries are
// <Namespace>:<id>.
const Namespace = "dgs"

// Kinds of this tenant's own entries.
const (
	OwnDomain     = "domain"
	OwnParameter  = "parameter"
	OwnDeviceType = "device_type"
	OwnTemplate   = "template"
)

// Own is one of this tenant's own entries, as sent to SAM. A nil
// definition is one removed. The registry shows it until SAM has it.
type Own struct {
	Kind      string
	ID        string
	Domain    *client.DomainInput
	Parameter *client.ParameterInput
	Type      *client.DeviceTypeInput
	Template  *client.TemplateInput
	UpdatedAt time.Time
}

func (o Own) definition() any {
	switch {
	case o.Domain != nil:
		return o.Domain
	case o.Parameter != nil:
		return o.Parameter
	case o.Type != nil:
		return o.Type
	case o.Template != nil:
		return o.Template
	}
	return nil
}

// entity is the entry's name in the outbox.
func entity(kind, id string) string { return "registry:" + kind + ":" + id }

// Outbox kinds, one per kind of entry.
var outboxKinds = map[string]string{
	OwnDomain:     "registry_domain",
	OwnParameter:  "registry_parameter",
	OwnDeviceType: "registry_device_type",
	OwnTemplate:   "registry_template",
}

// loadOwn reads every own entry.
func loadOwn(ctx context.Context, q db.Querier) ([]Own, error) {
	return rows(ctx, q, `SELECT kind, id, definition, updated_at FROM registry_own_entries ORDER BY kind, id`, scanOwn)
}

func scanOwn(r scanner) (Own, error) {
	var o Own
	var raw []byte
	if err := r.Scan(&o.Kind, &o.ID, &raw, &o.UpdatedAt); err != nil {
		return o, err
	}
	return o, o.decode(raw)
}

// decode sets the definition of o's kind from raw, JSON or null.
func (o *Own) decode(raw []byte) error {
	if raw == nil || string(raw) == "null" {
		return nil
	}
	var target any
	switch o.Kind {
	case OwnDomain:
		o.Domain = &client.DomainInput{}
		target = o.Domain
	case OwnParameter:
		o.Parameter = &client.ParameterInput{}
		target = o.Parameter
	case OwnDeviceType:
		o.Type = &client.DeviceTypeInput{}
		target = o.Type
	case OwnTemplate:
		o.Template = &client.TemplateInput{}
		target = o.Template
	default:
		return fmt.Errorf("an own entry of kind %q", o.Kind)
	}
	return json.Unmarshal(raw, target)
}

// putOwn stores o and sends it to SAM through the outbox, in q's
// transaction.
func putOwn(ctx context.Context, q db.Querier, now time.Time, o Own) error {
	def := o.definition()
	raw, err := json.Marshal(def)
	if err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO registry_own_entries (kind, id, definition, updated_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (kind, id) DO UPDATE SET definition = $3, updated_at = $4`, o.Kind, o.ID, raw, now); err != nil {
		return err
	}
	_, err = samaccess.Enqueue(ctx, q, now, samaccess.Change{Entity: entity(o.Kind, o.ID), Kind: outboxKinds[o.Kind], Payload: def})
	return err
}

// withOwn is SAM's registry with this tenant's own entries as this service
// defines them: added, replaced or removed.
func withOwn(s *SAM, own []Own) *SAM {
	out := *s
	out.Domains, out.Parameters = slices.Clone(s.Domains), slices.Clone(s.Parameters)
	out.DeviceTypes, out.Templates = slices.Clone(s.DeviceTypes), slices.Clone(s.Templates)
	for _, o := range own {
		switch o.Kind {
		case OwnDomain:
			out.Domains = put(out.Domains, o.ID, func(d client.Domain) string { return d.ID }, o.Domain != nil, func(d client.Domain) client.Domain {
				d.ID, d.Name, d.Description = o.ID, o.Domain.Name, o.Domain.Description
				d.UpdatedAt = since(d.UpdatedAt, o.UpdatedAt)
				return d
			})
		case OwnParameter:
			out.Parameters = put(out.Parameters, o.ID, func(p client.Parameter) string { return p.ID }, o.Parameter != nil, func(p client.Parameter) client.Parameter {
				in := o.Parameter
				p.ID, p.Name, p.Definition, p.Unit, p.ValueType, p.DomainID = o.ID, in.Name, in.Definition, in.Unit, in.ValueType, in.DomainID
				p.VariantOf, p.Min, p.Max, p.CategoryValues = in.VariantOf, in.Min, in.Max, in.CategoryValues
				p.UpdatedAt = since(p.UpdatedAt, o.UpdatedAt)
				return p
			})
		case OwnDeviceType:
			out.DeviceTypes = put(out.DeviceTypes, o.ID, func(t client.DeviceType) string { return t.ID }, o.Type != nil, func(t client.DeviceType) client.DeviceType {
				t.ID, t.Name, t.Description, t.Manual, t.Keys = o.ID, o.Type.Name, o.Type.Description, o.Type.Manual, o.Type.Keys
				t.UpdatedAt = since(t.UpdatedAt, o.UpdatedAt)
				return t
			})
		case OwnTemplate:
			out.Templates = put(out.Templates, o.ID, func(t client.Template) string { return t.ID }, o.Template != nil, func(t client.Template) client.Template {
				t.ID, t.Name, t.Description, t.Fields = o.ID, o.Template.Name, o.Template.Description, o.Template.Fields
				t.UpdatedAt = since(t.UpdatedAt, o.UpdatedAt)
				return t
			})
		}
	}
	return &out
}

// put replaces the entry with id in list by set applied to it, adds it if
// there is none, or removes it when it is not kept.
func put[E any](list []E, id string, idOf func(E) string, keep bool, set func(E) E) []E {
	i := slices.IndexFunc(list, func(e E) bool { return idOf(e) == id })
	switch {
	case !keep && i >= 0:
		return slices.Delete(list, i, i+1)
	case !keep:
		return list
	case i >= 0:
		list[i] = set(list[i])
		return list
	}
	var zero E
	return append(list, set(zero))
}

// since is when SAM last changed an entry, or, before SAM has it, when
// this service did.
func since(sam, changed time.Time) time.Time {
	if sam.IsZero() {
		return changed
	}
	return sam
}

// OutboxKinds are the kinds of this tenant's own entries, sent as this
// service, which holds registry:manage on the tenant's root.
func (m *Module) OutboxKinds() []samaccess.Kind {
	return []samaccess.Kind{
		{Name: outboxKinds[OwnDomain], Apply: m.applier(OwnDomain)},
		{Name: outboxKinds[OwnParameter], Apply: m.applier(OwnParameter)},
		{Name: outboxKinds[OwnDeviceType], Apply: m.applier(OwnDeviceType)},
		{Name: outboxKinds[OwnTemplate], Apply: m.applier(OwnTemplate)},
	}
}

// applier sends an own entry of kind to SAM: created or replaced, naming
// the version SAM holds, 0 to create, or removed. It waits for the own
// entries the entry names. Once SAM has it, the cached registry is read
// again.
func (m *Module) applier(kind string) func(context.Context, samaccess.Attempt) error {
	return func(ctx context.Context, a samaccess.Attempt) error {
		o := Own{Kind: kind}
		if err := o.decode(a.Payload); err != nil {
			return samaccess.Hold(err)
		}
		o.ID = strings.TrimPrefix(a.Entity, entity(kind, ""))
		deps := o.needs()
		if kind == OwnDomain && o.Domain == nil {
			// SAM removes a domain no value is under: this tenant's own values
			// move out of it first.
			own, err := loadOwn(ctx, m.db)
			if err != nil {
				return err
			}
			for _, p := range own {
				if p.Kind == OwnParameter {
					deps = append(deps, entity(OwnParameter, p.ID))
				}
			}
		}
		for _, dep := range deps {
			if waiting, err := samaccess.Waiting(ctx, m.db, dep); err != nil || waiting {
				return errors.Join(samaccess.ErrNotYet, err)
			}
		}
		if err := send(a.WithKey(ctx, "put"), a.SAM, o); err != nil {
			return err
		}
		m.cache.Refresh()
		return nil
	}
}

// needs are the outbox entities of the own entries o names, which reach
// SAM first.
func (o Own) needs() []string {
	var out []string
	own := func(kind, id string) {
		if isOwn(id) {
			out = append(out, entity(kind, id))
		}
	}
	switch {
	case o.Parameter != nil:
		own(OwnDomain, o.Parameter.DomainID)
	case o.Type != nil:
		for _, k := range o.Type.Keys {
			if k.ParameterID != nil {
				own(OwnParameter, *k.ParameterID)
			}
		}
	case o.Template != nil:
		for _, f := range o.Template.Fields {
			if f.ParameterID != nil {
				own(OwnParameter, *f.ParameterID)
			}
		}
	}
	return out
}

// isOwn reports whether id is one of this tenant's own entries.
func isOwn(id string) bool { return strings.HasPrefix(id, Namespace+":") }

// send brings SAM's entry to o's definition.
func send(ctx context.Context, c *client.Client, o Own) error {
	switch o.Kind {
	case OwnDomain:
		held, err := c.Domain(ctx, o.ID)
		version, found, err := versionOf(held.Version, err)
		switch {
		case err != nil:
			return err
		case o.Domain == nil && found:
			return c.DeleteDomain(ctx, o.ID, version)
		case o.Domain == nil:
			return nil
		}
		in := *o.Domain
		in.Version = version
		_, err = c.PutDomain(ctx, o.ID, in)
		return err
	case OwnParameter:
		held, err := c.Parameter(ctx, o.ID)
		version, _, err := versionOf(held.Version, err)
		if err != nil || o.Parameter == nil {
			return err
		}
		in := *o.Parameter
		in.Version = version
		_, err = c.PutParameter(ctx, o.ID, in)
		return err
	case OwnDeviceType:
		held, err := c.DeviceType(ctx, o.ID)
		version, _, err := versionOf(held.Version, err)
		if err != nil || o.Type == nil {
			return err
		}
		in := *o.Type
		in.Version = version
		_, err = c.PutDeviceType(ctx, o.ID, in)
		return err
	case OwnTemplate:
		held, err := c.Template(ctx, o.ID)
		version, _, err := versionOf(held.Version, err)
		if err != nil || o.Template == nil {
			return err
		}
		in := *o.Template
		in.Version = version
		_, err = c.PutTemplate(ctx, o.ID, in)
		return err
	}
	return samaccess.Hold(fmt.Errorf("an own entry of kind %q", o.Kind))
}

// versionOf is the version SAM holds of an entry it answered with, 0 when
// it has none.
func versionOf(version int, err error) (int, bool, error) {
	var e *client.Error
	if errors.As(err, &e) && e.Status == 404 {
		return 0, false, nil
	}
	return version, err == nil, err
}
