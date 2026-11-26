package publicsite

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/views"
)

// domainCards are the values by domain, in the registry's order, values
// with no domain last, each card's values by the registry's order of
// parameters and then by source; each value with its last reading of
// each of the seven days up to its own. The parameters are those shown.
func domainCards(v *registry.View, values []views.Value, days []views.DayStat, sources map[string]Source) ([]DomainCard, []registry.Parameter) {
	order := map[string]int{}
	for i, p := range v.Parameters {
		order[p.ID] = i
	}
	shown := map[string]bool{}
	byDomain := map[string][]PublicValue{}
	for _, val := range values {
		p := v.Parameter(val.ParameterID)
		pv := PublicValue{ParameterID: val.ParameterID, Value: val.Value, ObservedAt: val.ObservedAt, Source: sources[val.Source], Last7Days: []*float64{}}
		if _, ok := val.Value.(float64); ok {
			pv.Last7Days = views.LastDays(days, val.Source, val.ParameterID, val.ObservedAt.UTC().Format("2006-01-02"))
		}
		d := ""
		if p.Domain != nil {
			d = *p.Domain
		}
		byDomain[d] = append(byDomain[d], pv)
		shown[val.ParameterID] = true
	}
	sortValues := func(list []PublicValue) []PublicValue {
		name := func(s Source) string {
			if s.Name == nil {
				return ""
			}
			return *s.Name
		}
		slices.SortFunc(list, func(a, b PublicValue) int {
			if d := order[a.ParameterID] - order[b.ParameterID]; d != 0 {
				return d
			}
			return strings.Compare(name(a.Source), name(b.Source))
		})
		return list
	}
	cards := []DomainCard{}
	for i, d := range v.Domains {
		if list := byDomain[d.ID]; len(list) > 0 {
			cards = append(cards, DomainCard{Domain: &v.Domains[i], Values: sortValues(list)})
			delete(byDomain, d.ID)
		}
	}
	var loose []PublicValue
	for _, list := range byDomain {
		loose = append(loose, list...)
	}
	if len(loose) > 0 {
		cards = append(cards, DomainCard{Values: sortValues(loose)})
	}
	params := []registry.Parameter{}
	for _, p := range v.Parameters {
		if shown[p.ID] {
			params = append(params, p)
		}
	}
	return cards, params
}

// nowTiles are "What's happening now?", each shown value with its last 24
// hours and which way it went.
func (m *Module) nowTiles(ctx context.Context, values []views.Value, param views.Lookup, core views.CoreReadings, hours []views.HourStat) ([]views.NowTile, error) {
	e, err := settings.Get(ctx, m.db, settings.KeyNowTiles)
	if err != nil {
		return nil, err
	}
	var config settings.NowTiles
	if err := json.Unmarshal(e.Value, &config); err != nil {
		return nil, err
	}
	tiles, shown := views.NowTiles(config.Tiles, values, param, &core)
	for _, s := range shown {
		t := &tiles[s.Tile]
		if _, ok := s.Value.Value.(float64); !ok {
			continue
		}
		t.Last24Hours = views.Last24Hours(hours, s.Value.Source, []string{s.Value.ParameterID}, s.Value.ObservedAt)
		t.Trend = views.TrendOf(hours, s.Value.Source, s.Value.ParameterID, s.Value.ObservedAt, param(s.Value.ParameterID))
	}
	return tiles, nil
}

// completeness is how complete the core day's month is against the
// classroom's protocol, from the values the reader may read: a visitor
// counts released values only. Nil with no core day or no protocol.
func completeness(ctx context.Context, as *client.Client, space uuid.UUID, c classrooms.Classroom, v *registry.View, d *string) (*views.Completeness, error) {
	var expect []views.Expectation
	var ids []string
	for _, p := range c.Profile.Projects {
		for _, s := range p.Sites {
			for _, id := range s.ParameterIDs {
				expect = append(expect, views.Expectation{ParameterID: id, ReadingsPerDay: s.ReadingsPerDay})
				if !slices.Contains(ids, id) {
					ids = append(ids, id)
				}
			}
		}
	}
	if d == nil || len(expect) == 0 {
		return nil, nil
	}
	start, _ := time.Parse("2006-01-02", (*d)[:7]+"-01")
	last, _ := time.Parse("2006-01-02", *d)
	end := last.AddDate(0, 0, 1)
	var hours []views.HourStat
	err := spans(start, end, time.Hour, len(ids), func(from, to time.Time) error {
		agg, err := as.Aggregate(ctx, client.Aggregation{ObservationQuery: client.ObservationQuery{SpaceID: space, ParameterIDs: ids, From: &from, To: &to},
			Bucket: "hour", TimeZone: "UTC", GroupBy: []string{"parameter_id"}, Functions: []string{"count", "last"}})
		if err != nil {
			return samaccess.Translate(ctx, err)
		}
		for _, g := range agg.Groups {
			if g.Bucket != nil && g.ParameterID != nil && g.Count != nil && g.Last != nil {
				hours = append(hours, views.HourStat{ParameterID: *g.ParameterID, Hour: *g.Bucket, LastAt: g.Last.At, Count: *g.Count})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := views.MonthCompleteness(hours, *d, expect, func(id string) *views.DomainRef { return domainRef(v, id) })
	return &out, nil
}

// domainRef is the domain of a parameter, nil for one with none.
func domainRef(v *registry.View, id string) *views.DomainRef {
	p := v.Parameter(id)
	if p == nil || p.Domain == nil {
		return nil
	}
	for _, d := range v.Domains {
		if d.ID == *p.Domain {
			return &views.DomainRef{ID: d.ID, Label: d.Label, Icon: d.Icon}
		}
	}
	return nil
}

// places are every classroom with a lab, by name, for the location tabs.
func (m *Module) places(ctx context.Context) ([]Place, error) {
	listed, err := classrooms.List(ctx, m.db, uuid.Nil, true)
	if err != nil {
		return nil, err
	}
	out := []Place{}
	for _, l := range listed {
		c, err := m.shell(ctx, l.Classroom)
		if err != nil {
			return nil, err
		}
		if c.Location != nil {
			out = append(out, Place{ID: c.ID, Name: c.Name, PlaceName: c.PlaceName, Country: c.Country, Location: c.Location, StageID: c.StageID, RegionID: c.RegionID})
		}
	}
	slices.SortFunc(out, func(a, b Place) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, nil
}

// devices are the classroom's devices, each with its tiles and cards from
// its newest reading the reader may see; one with none still lists its
// registry values, with no data.
func (m *Module) devices(ctx context.Context, c classrooms.Classroom, v *registry.View, data stationData) ([]PublicDevice, error) {
	types := map[string]registry.DeviceType{}
	for _, t := range v.DeviceTypes {
		types[t.ID] = t
	}
	icons := map[string]string{}
	for _, d := range v.Domains {
		icons[d.ID] = d.Icon
	}
	param := views.Lookup(v.Parameter)
	out := []PublicDevice{}
	err := labs.AllDevices(ctx, m.db, c.ID, func(d labs.Device) {
		if d.Removed {
			return
		}
		t := types[d.Type]
		source := ""
		if d.SAMDeviceID != nil {
			source = d.SAMDeviceID.String()
		}
		newest, values := newestReading(data.latest, d.SAMDeviceID)
		tileValues, cardValues := values, values
		if newest == nil {
			tileValues, cardValues = none(t.TileParameterIDs), none(t.CardParameterIDs)
		}
		review := data.review(source)
		out = append(out, PublicDevice{ID: d.ID, Name: d.Name, Type: d.Type, Color: t.Color, Icon: t.Icon,
			Tiles: views.ReadingTiles(views.KeyValues(t.TileParameterIDs, tileValues, param, icons, true), newest, t.Icon, data.hours, source, param),
			Summary: views.DeviceSummary(views.KeyValues(t.CardParameterIDs, cardValues, param, icons, false), newest, m.now(), data.days, source,
				func(id string) string { return views.ChartOf(param(id)) }, func(id string) string { return review([]string{id}) })})
	})
	slices.SortStableFunc(out, func(a, b PublicDevice) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, err
}

// none is a reading holding each of ids with no value.
func none(ids []string) map[string]any {
	out := map[string]any{}
	for _, id := range ids {
		out[id] = nil
	}
	return out
}

// newestReading is a device's newest reading among the latest values:
// when it was observed, and the values of it; nil with none.
func newestReading(latest []client.ObservationRecord, device *uuid.UUID) (*time.Time, map[string]any) {
	if device == nil {
		return nil, nil
	}
	var newest *client.ObservationRecord
	for i, l := range latest {
		if l.DeviceID != nil && *l.DeviceID == *device && (newest == nil || l.ObservedAt.After(newest.ObservedAt)) {
			newest = &latest[i]
		}
	}
	if newest == nil {
		return nil, nil
	}
	values := map[string]any{}
	for _, l := range latest {
		if l.DeviceID != nil && *l.DeviceID == *device && l.ReadingID == newest.ReadingID {
			values[l.ParameterID] = l.Value
		}
	}
	at := newest.ObservedAt
	return &at, values
}

// institution is the institution card: shown to everyone once the host
// agreement is signed, and to the classroom's members before, so its head
// can fill it in. Its device types are those of the classroom's kit, in
// the registry's order.
func institution(c classrooms.Classroom, shown Classroom, v *registry.View, devices []PublicDevice, member bool) *Institution {
	name := shown.InstitutionName
	if name == nil && member {
		name = &c.InstitutionName
	}
	if name == nil {
		return nil
	}
	out := &Institution{Institution: c.Profile.Institution, Name: *name, Location: shown.Location, DeviceTypes: []InstitutionDeviceType{}}
	for _, t := range v.DeviceTypes {
		if slices.ContainsFunc(devices, func(d PublicDevice) bool { return d.Type == t.ID }) {
			out.DeviceTypes = append(out.DeviceTypes, InstitutionDeviceType{ID: t.ID, Label: t.Label, Icon: t.Icon, Color: t.Color})
		}
	}
	return out
}
