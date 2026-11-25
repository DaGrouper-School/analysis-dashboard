package registry

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
)

// Overlay is how this service presents SAM's entries, keyed by SAM's IDs,
// each list in display order. An entry without a row is shown as SAM
// defines it.
type Overlay struct {
	Domains     []DomainOverlay     `json:"domains"`
	Parameters  []ParameterOverlay  `json:"parameters"`
	DeviceTypes []DeviceTypeOverlay `json:"deviceTypes"`
	Templates   []TemplateOverlay   `json:"templates"`
}

// DomainOverlay is a domain's presentation. A hidden domain is not shown.
type DomainOverlay struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Hidden      bool   `json:"hidden"`
}

// ParameterOverlay is a parameter's presentation. A nil field shows SAM's
// own.
type ParameterOverlay struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Definition *string `json:"definition"`
	// Plain is what the value means, in one plain sentence; empty for none.
	Plain string `json:"plain"`
	// Explanation is how a measured or entered value is taken; nil for the
	// wording of its source.
	Explanation *string `json:"explanation"`
	// Domain is the domain it is shown under, instead of SAM's.
	Domain *string `json:"domain"`
	// Unit is the unit as shown, instead of SAM's symbol.
	Unit   *string  `json:"unit"`
	Source string   `json:"source"`
	Noise  *float64 `json:"noise"`
	Chart  *string  `json:"chart"`
}

// DeviceTypeOverlay is a device type's presentation, and the values shown
// as tiles and charted as cards when it is picked.
type DeviceTypeOverlay struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Color       string   `json:"color"`
	Icon        string   `json:"icon"`
	Tiles       []string `json:"tiles"`
	Cards       []string `json:"cards"`
}

// TemplateOverlay is an observation template's presentation, the domain
// it is listed under and the device type it is read off.
type TemplateOverlay struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Domain      *string `json:"domain"`
	DeviceType  *string `json:"deviceType"`
}

// LoadOverlay reads the whole overlay.
func LoadOverlay(ctx context.Context, q db.Querier) (Overlay, error) {
	var o Overlay
	var err error
	if o.Domains, err = rows(ctx, q, `SELECT id, label, icon, description, hidden
		FROM registry_domains ORDER BY position, id`, func(r pgx.Row) (d DomainOverlay, err error) {
		return d, r.Scan(&d.ID, &d.Label, &d.Icon, &d.Description, &d.Hidden)
	}); err != nil {
		return o, err
	}
	if o.Parameters, err = rows(ctx, q, `SELECT `+parameterColumns+`
		FROM registry_parameters ORDER BY position, id`, scanParameter); err != nil {
		return o, err
	}
	if o.DeviceTypes, err = rows(ctx, q, `SELECT `+deviceTypeColumns+`
		FROM registry_device_types ORDER BY position, id`, scanDeviceType); err != nil {
		return o, err
	}
	o.Templates, err = rows(ctx, q, `SELECT `+templateColumns+`
		FROM registry_templates ORDER BY position, id`, scanTemplate)
	return o, err
}

const (
	parameterColumns  = `id, label, definition, plain, explanation, domain, unit, source, noise, chart`
	deviceTypeColumns = `id, label, description, color, icon, tiles, cards`
	templateColumns   = `id, label, description, domain, device_type`
)

func scanParameter(r pgx.Row) (p ParameterOverlay, err error) {
	return p, r.Scan(&p.ID, &p.Label, &p.Definition, &p.Plain, &p.Explanation, &p.Domain, &p.Unit, &p.Source, &p.Noise, &p.Chart)
}

func scanDeviceType(r pgx.Row) (t DeviceTypeOverlay, err error) {
	err = r.Scan(&t.ID, &t.Label, &t.Description, &t.Color, &t.Icon, &t.Tiles, &t.Cards)
	t.Tiles, t.Cards = nonNil(t.Tiles), nonNil(t.Cards)
	return t, err
}

func scanTemplate(r pgx.Row) (t TemplateOverlay, err error) {
	return t, r.Scan(&t.ID, &t.Label, &t.Description, &t.Domain, &t.DeviceType)
}

// rows runs a query and scans each row.
func rows[T any](ctx context.Context, q db.Querier, sql string, scan func(pgx.Row) (T, error), args ...any) ([]T, error) {
	rs, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []T
	for rs.Next() {
		v, err := scan(rs)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rs.Err()
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// scanner is a row to scan.
type scanner = pgx.Row

// counter counts the rows of an overlay table.
func counter(table string) func(context.Context, db.Querier) (int, error) {
	return func(ctx context.Context, q db.Querier) (int, error) {
		var n int
		err := q.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n)
		return n, err
	}
}

// current reads an overlay row as settings.List needs it, locking it.
func current[T any](table, columns string, scan func(scanner, *T) error) func(context.Context, db.Querier, string) (T, int, bool, bool, error) {
	return func(ctx context.Context, q db.Querier, id string) (row T, position int, seeded, found bool, err error) {
		err = q.QueryRow(ctx, `SELECT position, seeded FROM `+table+` WHERE id = $1 FOR UPDATE`, id).Scan(&position, &seeded)
		if errors.Is(err, pgx.ErrNoRows) {
			return row, 0, false, false, nil
		}
		if err != nil {
			return row, 0, false, false, err
		}
		err = scan(q.QueryRow(ctx, `SELECT `+columns+` FROM `+table+` WHERE id = $1`, id), &row)
		return row, position, seeded, err == nil, err
	}
}
