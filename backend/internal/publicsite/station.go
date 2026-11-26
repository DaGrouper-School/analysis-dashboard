package publicsite

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/views"
)

// A station page: a classroom's overview, read from SAM as the member when
// the caller is one and as anonymous otherwise, from SAM's latest values
// and its daily and hourly aggregates.

// coreDays is the window of daily values a station page reads: the core
// day's month of air temperature, which also holds the week's rain and
// every device's seven days.
const coreDays = 31

// PublicValue is the front end's PublicValue.
type PublicValue struct {
	ParameterID string     `json:"parameterId"`
	Value       any        `json:"value"`
	ObservedAt  time.Time  `json:"observedAt"`
	Source      Source     `json:"source"`
	Last7Days   []*float64 `json:"last7Days"`
}

// DomainCard is the front end's PublicDomainCard.
type DomainCard struct {
	Domain *registry.Domain `json:"domain"`
	Values []PublicValue    `json:"values"`
}

// Place is a classroom on the location tabs and the globe.
type Place struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	PlaceName *string     `json:"placeName"`
	Country   *string     `json:"country"`
	Location  *labs.Point `json:"location"`
	StageID   string      `json:"stageId"`
	RegionID  *string     `json:"regionId"`
}

// PublicDevice is the front end's PublicDevice.
type PublicDevice struct {
	ID      uuid.UUID                 `json:"id"`
	Name    string                    `json:"name"`
	Type    string                    `json:"type"`
	Color   string                    `json:"color"`
	Icon    string                    `json:"icon"`
	Tiles   []views.NowTile           `json:"tiles"`
	Summary []views.DeviceSummaryCard `json:"summary"`
}

// InstitutionDeviceType is a kind of device on the institution card.
type InstitutionDeviceType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// Institution is the front end's InstitutionOverview.
type Institution struct {
	classrooms.Institution
	Name        string                  `json:"name"`
	Location    *labs.Point             `json:"location"`
	DeviceTypes []InstitutionDeviceType `json:"deviceTypes"`
}

// StationPage is the front end's PublicStation.
type StationPage struct {
	Content      Content              `json:"content"`
	Classroom    Classroom            `json:"classroom"`
	Units        Units                `json:"units"`
	Parameters   []registry.Parameter `json:"parameters"`
	Domains      []DomainCard         `json:"domains"`
	Core         views.StationCore    `json:"core"`
	Completeness *views.Completeness  `json:"completeness"`
	SiteCodes    []string             `json:"siteCodes"`
	Can          roles.Permissions    `json:"can"`
	Shared       bool                 `json:"shared"`
	Places       []Place              `json:"places"`
	Now          []views.NowTile      `json:"now"`
	Devices      []PublicDevice       `json:"devices"`
	Institution  *Institution         `json:"institution"`
}

// stationData is what a station page reads of SAM.
type stationData struct {
	latest []client.ObservationRecord
	// days are every value the reader sees by day, released the
	// released ones only, which the core detail is computed from.
	days, released []views.DayStat
	hours          []views.HourStat
	// published and unpublished say which values of which source there
	// are, by source and parameter.
	published, unpublished map[string]bool
}

func (m *Module) station(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	c, as, member, err := m.shownClassroom(r)
	if err != nil {
		return err
	}
	v, err := m.reg.View(ctx)
	if err != nil {
		return err
	}
	params, err := coreParameters(ctx, m.db)
	if err != nil {
		return err
	}
	content, err := m.siteContent(ctx)
	if err != nil {
		return err
	}
	shown, err := m.publicClassroom(ctx, c, params)
	if err != nil {
		return err
	}
	sources, _, err := m.sources(ctx, c, v)
	if err != nil {
		return err
	}
	var data stationData
	readable := c.SpaceID != nil && (member || c.Sharing.ShowPublic)
	if readable {
		if data, err = readStation(ctx, as, *c.SpaceID, member); err != nil {
			return err
		}
	}
	if member {
		memberCore(&shown.Core, data)
	}
	can, err := m.permissions(r, c)
	if err != nil {
		return err
	}
	out := StationPage{Content: content, Classroom: shown, Units: unitsOf(v, params), Shared: c.Sharing.ShowPublic,
		Can: can, SiteCodes: siteCodes(c, can)}
	param := views.Lookup(v.Parameter)
	values := publicValues(data, sources, param)
	out.Domains, out.Parameters = domainCards(v, values, data.days, sources)
	out.Core = stationCore(data, shown.Core.Day, params, sources)
	if out.Now, err = m.nowTiles(ctx, values, param, shown.Core, data.hours); err != nil {
		return err
	}
	if readable {
		if out.Completeness, err = completeness(ctx, as, *c.SpaceID, c, v, shown.Core.Day); err != nil {
			return err
		}
	}
	if out.Places, err = m.places(ctx); err != nil {
		return err
	}
	devices, err := m.devices(ctx, c, v, data)
	if err != nil {
		return err
	}
	out.Devices = devices
	out.Institution = institution(c, shown, v, devices, member)
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// siteCodes are the classroom's site codes, for those who may see them.
func siteCodes(c classrooms.Classroom, can roles.Permissions) []string {
	out := []string{}
	if !can.SeeSiteCodes {
		return out
	}
	for _, p := range c.Profile.Projects {
		for _, s := range p.Sites {
			out = append(out, s.Code)
		}
	}
	return out
}

// stationCore is the core detail from the released values, each source
// named by its ID here.
func stationCore(data stationData, d *string, params settings.CoreParameters, sources map[string]Source) views.StationCore {
	nameOf := func(source string) *string {
		if s, ok := sources[source]; ok {
			return s.Name
		}
		return nil
	}
	out := views.Station(data.released, d, data.review(""), params, nameOf)
	for i := range out.Rain.BySource {
		if s, ok := sources[out.Rain.BySource[i].SourceID]; ok {
			out.Rain.BySource[i].SourceID = s.ID
		}
	}
	return out
}

// permissions are what the caller may do in the classroom; a visitor may
// do nothing.
func (m *Module) permissions(r *http.Request, c classrooms.Classroom) (roles.Permissions, error) {
	u, ok := accounts.CurrentUser(r.Context())
	if !ok {
		return roles.PermissionsFor(nil, "", c.Sharing), nil
	}
	role, err := classrooms.RoleOf(r.Context(), m.db, c.ID, u.ID)
	return roles.PermissionsFor(&roles.Viewer{PlatformAdmin: u.PlatformAdmin}, role, c.Sharing), err
}

func unitsOf(v *registry.View, params settings.CoreParameters) Units {
	unit := func(ids []string) string {
		if len(ids) > 0 {
			if p := v.Parameter(ids[0]); p != nil {
				return p.Unit
			}
		}
		return ""
	}
	return Units{Rain: unit(params.RainDaily), Temperature: unit(params.AirTemperature)}
}

// memberCore is the core strip as a member sees it: the last sync of any
// device, and in review while values wait for release.
func memberCore(core *views.CoreReadings, data stationData) {
	for _, l := range data.latest {
		if core.LastSync == nil || l.IngestedAt.After(*core.LastSync) {
			at := l.IngestedAt
			core.LastSync = &at
		}
	}
	if len(data.unpublished) > 0 && core.Day != nil {
		core.Review = views.InReview
	}
}

// key names a source's parameter in stationData's sets.
func key(source, parameterID string) string { return source + "|" + parameterID }

// review says where ids stand in review from source, or from every
// source for "": not released while none of them is, in review while
// newer ones wait, else released.
func (d stationData) review(source string) func(ids []string) string {
	return func(ids []string) string {
		has := func(set map[string]bool) bool {
			for k := range set {
				src, id, _ := strings.Cut(k, "|")
				if (source == "" || src == source) && slices.Contains(ids, id) {
					return true
				}
			}
			return false
		}
		switch {
		case !has(d.published):
			return views.NotReleased
		case has(d.unpublished):
			return views.InReview
		}
		return views.Released
	}
}

// sourceKey is the source key of a value from device: SAM's device ID,
// or "" for the field.
func sourceKey(device *uuid.UUID) string {
	if device == nil {
		return ""
	}
	return device.String()
}

// readStation reads what a station page shows of space: its latest
// values; each source's values by day over the core day's month, all and
// released; by hour over the last day; and which values are released and
// which wait.
func readStation(ctx context.Context, as *client.Client, space uuid.UUID, member bool) (stationData, error) {
	var out stationData
	latest, err := as.Latest(ctx, client.ObservationQuery{SpaceID: space})
	if notReadable(err) {
		return out, nil
	}
	if err != nil {
		return out, samaccess.Translate(ctx, err)
	}
	out.latest = latest
	if len(latest) == 0 {
		return out, nil
	}
	newest := latest[0].ObservedAt
	for _, l := range latest {
		if l.ObservedAt.After(newest) {
			newest = l.ObservedAt
		}
	}
	d := newest.UTC()
	end := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	from := end.AddDate(0, 0, -coreDays)
	q := client.ObservationQuery{SpaceID: space}
	if out.days, err = dayStats(ctx, as, q, from, end, len(latest)); err != nil {
		return out, err
	}
	out.released = out.days
	if member {
		pq := q
		pq.Publication = "published"
		if out.released, err = dayStats(ctx, as, pq, from, end, len(latest)); err != nil {
			return out, err
		}
	}
	if out.hours, err = hourStats(ctx, as, q, newest.Add(-24*time.Hour), newest.Add(time.Microsecond), len(latest)); err != nil {
		return out, err
	}
	out.published, out.unpublished = map[string]bool{}, map[string]bool{}
	if !member {
		for _, l := range latest {
			out.published[key(sourceKey(l.DeviceID), l.ParameterID)] = true
		}
		return out, nil
	}
	for status, set := range map[string]map[string]bool{"published": out.published, "unpublished": out.unpublished} {
		sq := q
		sq.Publication = status
		epoch := time.Unix(0, 0)
		sq.From = &epoch
		agg, err := as.Aggregate(ctx, client.Aggregation{ObservationQuery: sq, GroupBy: []string{"device_id", "parameter_id"}, Functions: []string{"count"}})
		if err != nil {
			return out, samaccess.Translate(ctx, err)
		}
		for _, g := range agg.Groups {
			if g.ParameterID != nil && g.Count != nil && *g.Count > 0 {
				set[key(sourceKey(g.DeviceID), *g.ParameterID)] = true
			}
		}
	}
	return out, nil
}

// spans splits from to until into spans holding at most SAM's groups,
// series groups a bucket of size.
func spans(from, until time.Time, size time.Duration, series int, each func(start, end time.Time) error) error {
	const maxGroups = 10_000
	span := time.Duration(max(1, maxGroups/max(1, series))) * size
	for start := from; start.Before(until); start = start.Add(span) {
		end := start.Add(span)
		if end.After(until) {
			end = until
		}
		if err := each(start, end); err != nil {
			return err
		}
	}
	return nil
}

// dayStats are each source's values of each parameter by UTC day.
func dayStats(ctx context.Context, as *client.Client, q client.ObservationQuery, from, until time.Time, series int) ([]views.DayStat, error) {
	var out []views.DayStat
	err := spans(from, until, 24*time.Hour, series, func(start, end time.Time) error {
		q.From, q.To = &start, &end
		agg, err := as.Aggregate(ctx, client.Aggregation{ObservationQuery: q, Bucket: "day", TimeZone: "UTC",
			GroupBy: []string{"device_id", "parameter_id"}, Functions: []string{"min", "max", "last"}})
		if err != nil {
			return samaccess.Translate(ctx, err)
		}
		for _, g := range agg.Groups {
			if g.Bucket == nil || g.ParameterID == nil || g.Min == nil || g.Max == nil || g.Last == nil {
				continue
			}
			out = append(out, views.DayStat{Source: sourceKey(g.DeviceID), ParameterID: *g.ParameterID, Day: g.Bucket.UTC().Format("2006-01-02"),
				Min: *g.Min, Max: *g.Max, Last: g.Last.Value, LastAt: g.Last.At})
		}
		return nil
	})
	return out, err
}

// hourStats are each source's values of each parameter by hour.
func hourStats(ctx context.Context, as *client.Client, q client.ObservationQuery, from, until time.Time, series int) ([]views.HourStat, error) {
	var out []views.HourStat
	err := spans(from, until, time.Hour, series, func(start, end time.Time) error {
		q.From, q.To = &start, &end
		agg, err := as.Aggregate(ctx, client.Aggregation{ObservationQuery: q, Bucket: "hour", TimeZone: "UTC",
			GroupBy: []string{"device_id", "parameter_id"}, Functions: []string{"count", "last"}})
		if err != nil {
			return samaccess.Translate(ctx, err)
		}
		for _, g := range agg.Groups {
			if g.Bucket == nil || g.ParameterID == nil || g.Last == nil {
				continue
			}
			h := views.HourStat{Source: sourceKey(g.DeviceID), ParameterID: *g.ParameterID, Hour: *g.Bucket, Last: g.Last.Value, LastAt: g.Last.At}
			if g.Count != nil {
				h.Count = *g.Count
			}
			out = append(out, h)
		}
		return nil
	})
	return out, err
}

// publicValues are the newest value of each parameter the registry holds
// from each of the classroom's sources.
func publicValues(data stationData, sources map[string]Source, param views.Lookup) []views.Value {
	var out []views.Value
	for _, l := range data.latest {
		if _, ok := sources[sourceKey(l.DeviceID)]; !ok || param(l.ParameterID) == nil {
			continue
		}
		out = append(out, views.Value{ParameterID: l.ParameterID, Value: l.Value, ObservedAt: l.ObservedAt, Source: sourceKey(l.DeviceID)})
	}
	return out
}
