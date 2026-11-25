package registry

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// seedFile is the overlay's seed, built into the binary: the front end's
// seed (frontend/src/lib/data/seed.ts) keyed by SAM's IDs.
//
//go:embed seed.json
var seedFile []byte

// What an overlay row is, as the seed reports it.
const (
	KindDomain     = "registry domain"
	KindParameter  = "registry parameter"
	KindDeviceType = "registry device type"
	KindTemplate   = "registry template"
)

// SeedOverlay brings the overlay in line with the seed, as
// settings.SeedList does. The overlay's seed is not logged with the
// platform's configuration.
func SeedOverlay(ctx context.Context, q db.Querier, now time.Time) ([]settings.Seeded, error) {
	o, err := seedOverlay()
	if err != nil {
		return nil, err
	}
	var out []settings.Seeded
	for _, seed := range []func() ([]settings.Seeded, error){
		func() ([]settings.Seeded, error) { return settings.SeedList(ctx, q, now, domainList, o.Domains) },
		func() ([]settings.Seeded, error) { return settings.SeedList(ctx, q, now, parameterList, o.Parameters) },
		func() ([]settings.Seeded, error) {
			return settings.SeedList(ctx, q, now, deviceTypeList, o.DeviceTypes)
		},
		func() ([]settings.Seeded, error) { return settings.SeedList(ctx, q, now, templateList, o.Templates) },
	} {
		more, err := seed()
		out = append(out, more...)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func seedOverlay() (Overlay, error) {
	o, problems := jsoncheck.Decode[Overlay](seedFile)
	if len(problems) == 0 {
		problems = CheckOverlay(o)
	}
	return o, jsoncheck.Refuse("the registry's seed", problems)
}

// samID is an ID in SAM's forms: pm2_5, AIR, or dgs:soil_ec under a
// tenant's namespace.
var samID = regexp.MustCompile(`^(?:[a-z][a-z0-9_]*:)?[A-Za-z0-9_]+$`)

var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

var (
	sources = []string{SourceWeatherStation, SourceDevice, SourceManual}
	charts  = []string{"daily_bars", "line_band", "line"}
)

// CheckOverlay reports what is wrong with an overlay.
func CheckOverlay(o Overlay) []string {
	p := jsoncheck.IDs("domains", o.Domains, func(d DomainOverlay) string { return d.ID }, samID)
	p = append(p, jsoncheck.IDs("parameters", o.Parameters, func(x ParameterOverlay) string { return x.ID }, samID)...)
	p = append(p, jsoncheck.IDs("deviceTypes", o.DeviceTypes, func(t DeviceTypeOverlay) string { return t.ID }, samID)...)
	p = append(p, jsoncheck.IDs("templates", o.Templates, func(t TemplateOverlay) string { return t.ID }, samID)...)
	for i, x := range o.Parameters {
		at := fmt.Sprintf("parameters[%d]", i)
		if !slices.Contains(sources, x.Source) {
			p = append(p, fmt.Sprintf("%s.source: %q is not weather_station, device or manual", at, x.Source))
		}
		if x.Chart != nil && !slices.Contains(charts, *x.Chart) {
			p = append(p, fmt.Sprintf("%s.chart: %q is not daily_bars, line_band or line", at, *x.Chart))
		}
		if x.Noise != nil && *x.Noise < 0 {
			p = append(p, fmt.Sprintf("%s.noise: %v is below zero", at, *x.Noise))
		}
	}
	for i, t := range o.DeviceTypes {
		if !hexColor.MatchString(t.Color) {
			p = append(p, fmt.Sprintf("deviceTypes[%d].color: %q is not a color such as #f6921e", i, t.Color))
		}
	}
	return append(p, jsoncheck.Blank(o, "", "plain", "unit", "explanation", "definition")...)
}

var domainList = settings.List[DomainOverlay]{
	Kind: KindDomain, Unlogged: true,
	ID:    func(d DomainOverlay) string { return d.ID },
	Count: counter("registry_domains"),
	Current: current("registry_domains", `id, label, icon, description, hidden`, func(r scanner, d *DomainOverlay) error {
		return r.Scan(&d.ID, &d.Label, &d.Icon, &d.Description, &d.Hidden)
	}),
	Insert: func(ctx context.Context, q db.Querier, now time.Time, d DomainOverlay, position int) error {
		_, err := q.Exec(ctx, `INSERT INTO registry_domains (id, label, icon, description, hidden, position, seeded, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, true, $7)`, d.ID, d.Label, d.Icon, d.Description, d.Hidden, position, now)
		return err
	},
	Update: func(ctx context.Context, q db.Querier, now time.Time, d DomainOverlay, position int) error {
		_, err := q.Exec(ctx, `UPDATE registry_domains SET label = $2, icon = $3, description = $4, hidden = $5, position = $6,
			updated_at = $7 WHERE id = $1 AND seeded`, d.ID, d.Label, d.Icon, d.Description, d.Hidden, position, now)
		return err
	},
}

var parameterList = settings.List[ParameterOverlay]{
	Kind: KindParameter, Unlogged: true,
	ID:    func(p ParameterOverlay) string { return p.ID },
	Count: counter("registry_parameters"),
	Current: current("registry_parameters", parameterColumns, func(r scanner, p *ParameterOverlay) error {
		var err error
		*p, err = scanParameter(r)
		return err
	}),
	Insert: func(ctx context.Context, q db.Querier, now time.Time, p ParameterOverlay, position int) error {
		_, err := q.Exec(ctx, `INSERT INTO registry_parameters (`+parameterColumns+`, position, seeded, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true, $12)`,
			p.ID, p.Label, p.Definition, p.Plain, p.Explanation, p.Domain, p.Unit, p.Source, p.Noise, p.Chart, position, now)
		return err
	},
	Update: func(ctx context.Context, q db.Querier, now time.Time, p ParameterOverlay, position int) error {
		_, err := q.Exec(ctx, `UPDATE registry_parameters SET label = $2, definition = $3, plain = $4, explanation = $5,
			domain = $6, unit = $7, source = $8, noise = $9, chart = $10, position = $11, updated_at = $12 WHERE id = $1 AND seeded`,
			p.ID, p.Label, p.Definition, p.Plain, p.Explanation, p.Domain, p.Unit, p.Source, p.Noise, p.Chart, position, now)
		return err
	},
}

var deviceTypeList = settings.List[DeviceTypeOverlay]{
	Kind: KindDeviceType, Unlogged: true,
	ID:    func(t DeviceTypeOverlay) string { return t.ID },
	Count: counter("registry_device_types"),
	Current: current("registry_device_types", deviceTypeColumns, func(r scanner, t *DeviceTypeOverlay) error {
		var err error
		*t, err = scanDeviceType(r)
		return err
	}),
	Insert: func(ctx context.Context, q db.Querier, now time.Time, t DeviceTypeOverlay, position int) error {
		_, err := q.Exec(ctx, `INSERT INTO registry_device_types (`+deviceTypeColumns+`, position, seeded, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, $9)`,
			t.ID, t.Label, t.Description, t.Color, t.Icon, t.Tiles, t.Cards, position, now)
		return err
	},
	Update: func(ctx context.Context, q db.Querier, now time.Time, t DeviceTypeOverlay, position int) error {
		_, err := q.Exec(ctx, `UPDATE registry_device_types SET label = $2, description = $3, color = $4, icon = $5,
			tiles = $6, cards = $7, position = $8, updated_at = $9 WHERE id = $1 AND seeded`,
			t.ID, t.Label, t.Description, t.Color, t.Icon, t.Tiles, t.Cards, position, now)
		return err
	},
}

var templateList = settings.List[TemplateOverlay]{
	Kind: KindTemplate, Unlogged: true,
	ID:    func(t TemplateOverlay) string { return t.ID },
	Count: counter("registry_templates"),
	Current: current("registry_templates", templateColumns, func(r scanner, t *TemplateOverlay) error {
		var err error
		*t, err = scanTemplate(r)
		return err
	}),
	Insert: func(ctx context.Context, q db.Querier, now time.Time, t TemplateOverlay, position int) error {
		_, err := q.Exec(ctx, `INSERT INTO registry_templates (`+templateColumns+`, position, seeded, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, true, $7)`, t.ID, t.Label, t.Description, t.Domain, t.DeviceType, position, now)
		return err
	},
	Update: func(ctx context.Context, q db.Querier, now time.Time, t TemplateOverlay, position int) error {
		_, err := q.Exec(ctx, `UPDATE registry_templates SET label = $2, description = $3, domain = $4, device_type = $5,
			position = $6, updated_at = $7 WHERE id = $1 AND seeded`, t.ID, t.Label, t.Description, t.Domain, t.DeviceType, position, now)
		return err
	},
}
