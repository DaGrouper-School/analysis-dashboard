package publicsite

import (
	"context"
	"errors"
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
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// A parameter page: one value of one classroom over the season up to its
// latest, read from SAM as the member when the caller is one, so they see
// values still in review, and as anonymous otherwise.

const (
	msgNoClassroom = "Classroom not found."
	msgNoValues    = "This classroom has no released values for that."
)

// maxRawPoints bounds the values a page reads one by one: the last day's
// readings, or a season of values that are not numbers.
const maxRawPoints = 5000

// DomainRef is a domain as a page names it.
type DomainRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// Rain is a rain page's reference gauge and tolerance.
type Rain struct {
	ReferenceSourceID *string `json:"referenceSourceId"`
	Tolerance         float64 `json:"tolerance"`
}

// Compared is another public classroom with values of the same parameter.
type Compared struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	PlaceName *string   `json:"placeName"`
	Country   *string   `json:"country"`
}

// Terms are a classroom's licence and attribution, for every downloaded
// row.
type Terms struct {
	Licence      string   `json:"licence"`
	LicenceURL   *string  `json:"licenceUrl"`
	Attribution  string   `json:"attribution"`
	Restrictions []string `json:"restrictions"`
}

// ParameterPage is the front end's PublicParameterPage.
type ParameterPage struct {
	Content         Content            `json:"content"`
	Classroom       Classroom          `json:"classroom"`
	Parameter       registry.Parameter `json:"parameter"`
	Domain          *DomainRef         `json:"domain"`
	Note            *string            `json:"note"`
	Series          []Series           `json:"series"`
	Rain            *Rain              `json:"rain"`
	ReleasedThrough *time.Time         `json:"releasedThrough"`
	InReview        bool               `json:"inReview"`
	Compare         []Compared         `json:"compare"`
	DeviceTypes     []string           `json:"deviceTypes"`
	Terms           Terms              `json:"terms"`
	Member          bool               `json:"member"`
}

// reader is how a page reads a classroom from SAM: as the caller when
// this service's record shows them a member of it, as anonymous otherwise.
// Membership only: a platform admin who is not a member reads as a
// visitor.
func (m *Module) reader(r *http.Request, c classrooms.Classroom) (*client.Client, bool, error) {
	ctx := r.Context()
	cur, ok := accounts.CurrentUser(ctx)
	if !ok {
		return m.sam.Anonymous(), false, nil
	}
	role, err := classrooms.RoleOf(ctx, m.db, c.ID, cur.ID)
	if err != nil || role == "" {
		return m.sam.Anonymous(), false, err
	}
	u, err := accounts.Get(ctx, m.db, cur.ID)
	if err != nil {
		return nil, false, err
	}
	as, err := m.sam.User(samaccess.Person{ParticipantID: u.ParticipantID, Registered: u.Principal != nil})
	if err != nil {
		return nil, false, samaccess.Translate(ctx, err)
	}
	return as, true, nil
}

// shownClassroom is the classroom in the path as the caller may see it:
// not found when it does not exist, or, for a visitor, when it does not
// show its data publicly, whatever SAM still lets anonymous read.
func (m *Module) shownClassroom(r *http.Request) (classrooms.Classroom, *client.Client, bool, error) {
	id, err := uuid.Parse(r.PathValue("classroomId"))
	if err != nil {
		return classrooms.Classroom{}, nil, false, httpx.NotFound(msgNoClassroom)
	}
	c, err := classrooms.Get(r.Context(), m.db, id)
	if errors.Is(err, classrooms.ErrNoClassroom) {
		return c, nil, false, httpx.NotFound(msgNoClassroom)
	}
	if err != nil {
		return c, nil, false, err
	}
	as, member, err := m.reader(r, c)
	return c, as, member, err
}

func (m *Module) parameter(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	c, as, member, err := m.shownClassroom(r)
	if err != nil {
		return err
	}
	v, err := m.reg.View(ctx)
	if err != nil {
		return err
	}
	p := v.Parameter(r.PathValue("parameterId"))
	if p == nil || c.SpaceID == nil || (!member && !c.Sharing.ShowPublic) {
		return httpx.NotFound(msgNoValues)
	}
	params, err := coreParameters(ctx, m.db)
	if err != nil {
		return err
	}
	ids := rainIDs(p.ID, params.RainDaily)
	sources, types, err := m.sources(ctx, c, v)
	if err != nil {
		return err
	}
	points, err := readSeason(ctx, as, *c.SpaceID, ids, p.ValueType == "number", sources)
	if err != nil {
		return err
	}
	if len(points) == 0 {
		return httpx.NotFound(msgNoValues)
	}
	isRain := slices.Contains(params.RainDaily, p.ID)
	series, reference := seriesOf(points, ids, isRain, func(id string) bool {
		q := v.Parameter(id)
		return q != nil && q.Source == "manual"
	})
	out, err := m.page(ctx, c, v, p, params)
	if err != nil {
		return err
	}
	out.Series, out.Member = series, member
	if isRain {
		out.Rain = &Rain{ReferenceSourceID: reference, Tolerance: params.RainTolerance}
	}
	for _, s := range series {
		if t := types[s.Source.ID]; t != "" && !slices.Contains(out.DeviceTypes, t) {
			out.DeviceTypes = append(out.DeviceTypes, t)
		}
	}
	if member {
		if out.InReview, err = inReview(ctx, as, *c.SpaceID, ids, out.ReleasedThrough); err != nil {
			return err
		}
	}
	if out.Compare, err = m.compare(ctx, c.ID, ids); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// page is what a parameter page holds besides its values.
func (m *Module) page(ctx context.Context, c classrooms.Classroom, v *registry.View, p *registry.Parameter, params settings.CoreParameters) (ParameterPage, error) {
	content, err := m.siteContent(ctx)
	if err != nil {
		return ParameterPage{}, err
	}
	shown, err := m.publicClassroom(ctx, c, params)
	if err != nil {
		return ParameterPage{}, err
	}
	out := ParameterPage{Content: content, Classroom: shown, Parameter: *p, DeviceTypes: []string{}, Compare: []Compared{},
		ReleasedThrough: classrooms.ReleaseCutoff(c, m.now()),
		Terms: Terms{Licence: c.Terms.Licence, LicenceURL: c.Terms.LicenceURL, Attribution: c.Terms.Attribution,
			Restrictions: append([]string{}, c.Terms.Restrictions...)}}
	if p.Domain != nil {
		for _, d := range v.Domains {
			if d.ID == *p.Domain {
				out.Domain = &DomainRef{ID: d.ID, Label: d.Label, Icon: d.Icon}
			}
		}
	}
	list, err := labs.ForClassroom(ctx, m.db, c.ID)
	if err != nil {
		return out, err
	}
	for _, l := range list {
		if note := strings.TrimSpace(l.SiteProfile.ParameterNotes[p.ID]); note != "" {
			out.Note = &note
			break
		}
	}
	return out, nil
}

// sources are the classroom's devices as sources, by SAM's device ID,
// removed ones too, and the registry label of each one's type, by source
// ID. Values no device sent are the field source, under "".
func (m *Module) sources(ctx context.Context, c classrooms.Classroom, v *registry.View) (map[string]Source, map[string]string, error) {
	byType := map[string]registry.DeviceType{}
	for _, t := range v.DeviceTypes {
		byType[t.ID] = t
	}
	out := map[string]Source{"": {ID: labs.FieldSource, Kind: labs.FieldSource}}
	types := map[string]string{}
	err := labs.AllDevices(ctx, m.db, c.ID, func(d labs.Device) {
		if d.SAMDeviceID == nil {
			return
		}
		name := d.Name
		out[d.SAMDeviceID.String()] = Source{ID: d.ID.String(), Name: &name, Kind: "device", Color: byType[d.Type].Color}
		types[d.ID.String()] = byType[d.Type].Label
	})
	return out, types, err
}

// readSeason reads ids' values in space over the season up to the latest
// of them: numbers as each hour's last value from SAM's aggregates, and the
// last day's readings one by one, so a day's chart shows every reading;
// values that are not numbers one by one throughout. A device that is not
// the classroom's is left out.
func readSeason(ctx context.Context, as *client.Client, space uuid.UUID, ids []string, numeric bool, sources map[string]Source) ([]sourced, error) {
	latest, err := as.Latest(ctx, client.ObservationQuery{SpaceID: space, ParameterIDs: ids})
	if notReadable(err) || (err == nil && len(latest) == 0) {
		return nil, nil
	}
	if err != nil {
		return nil, samaccess.Translate(ctx, err)
	}
	newest := latest[0].ObservedAt
	for _, l := range latest {
		if l.ObservedAt.After(newest) {
			newest = l.ObservedAt
		}
	}
	from := season(newest)
	dayAgo := newest.Add(-24 * time.Hour)
	if !numeric {
		dayAgo = from
	}
	var out []sourced
	if numeric && dayAgo.After(from) {
		if out, err = hourly(ctx, as, space, ids, from, dayAgo, len(latest), sources); err != nil {
			return nil, err
		}
	}
	raw, err := readRaw(ctx, as, space, ids, dayAgo, sources)
	return append(out, raw...), err
}

// notReadable is SAM refusing the read: the caller may read nothing there.
func notReadable(err error) bool {
	var ce *client.Error
	return errors.As(err, &ce) && ce.Status == http.StatusForbidden
}

// sourceOf is the source of a value from device, or false for a device
// that is not the classroom's.
func sourceOf(sources map[string]Source, device *uuid.UUID) (Source, bool) {
	key := ""
	if device != nil {
		key = device.String()
	}
	s, ok := sources[key]
	return s, ok
}

// hourly is each hour's last value of ids from from to until, read in
// spans short enough that no aggregate holds more groups than SAM gives:
// series is how many sources and values there are.
func hourly(ctx context.Context, as *client.Client, space uuid.UUID, ids []string, from, until time.Time, series int,
	sources map[string]Source) ([]sourced, error) {
	const maxGroups = 10_000
	span := time.Duration(max(1, maxGroups/max(1, series))) * time.Hour
	var out []sourced
	for start := from; start.Before(until); start = start.Add(span) {
		end := start.Add(span)
		if end.After(until) {
			end = until
		}
		agg, err := as.Aggregate(ctx, client.Aggregation{ObservationQuery: client.ObservationQuery{SpaceID: space, ParameterIDs: ids, From: &start, To: &end},
			Bucket: "hour", TimeZone: "UTC", GroupBy: []string{"device_id", "parameter_id"}, Functions: []string{"last"}})
		if err != nil {
			return nil, samaccess.Translate(ctx, err)
		}
		for _, g := range agg.Groups {
			src, ok := sourceOf(sources, g.DeviceID)
			if !ok || g.ParameterID == nil || g.Last == nil {
				continue
			}
			out = append(out, sourced{source: src, parameterID: *g.ParameterID, Point: Point{ObservedAt: g.Last.At, Value: g.Last.Value}})
		}
	}
	return out, nil
}

// readRaw is every value of ids observed from from, oldest first, up to
// maxRawPoints.
func readRaw(ctx context.Context, as *client.Client, space uuid.UUID, ids []string, from time.Time, sources map[string]Source) ([]sourced, error) {
	q := client.ObservationQuery{SpaceID: space, ParameterIDs: ids, From: &from, Sort: "observed_at", Order: "asc"}
	var out []sourced
	page := client.PageQuery{Limit: 200}
	for len(out) < maxRawPoints {
		got, err := as.Observations(ctx, q, page)
		if err != nil {
			return nil, samaccess.Translate(ctx, err)
		}
		for _, rec := range got.Items {
			if src, ok := sourceOf(sources, rec.DeviceID); ok {
				out = append(out, sourced{source: src, parameterID: rec.ParameterID, Point: Point{ObservedAt: rec.ObservedAt, Value: rec.Value}})
			}
		}
		if got.NextCursor == "" {
			break
		}
		page.Cursor = got.NextCursor
	}
	return out, nil
}

// inReview reports whether values of ids observed after the cutoff wait
// for release, as a member reads them.
func inReview(ctx context.Context, as *client.Client, space uuid.UUID, ids []string, cutoff *time.Time) (bool, error) {
	q := client.ObservationQuery{SpaceID: space, ParameterIDs: ids, Publication: "unpublished"}
	if cutoff != nil {
		after := cutoff.Add(time.Microsecond)
		q.From = &after
	}
	sum, err := as.Summarise(ctx, q)
	if err != nil {
		return false, samaccess.Translate(ctx, err)
	}
	return sum.Count > 0, nil
}

// compare is every other classroom that shows its data publicly and has
// released values of ids, as anonymous reads them, by name. Each answer
// is kept for CacheFor.
func (m *Module) compare(ctx context.Context, except uuid.UUID, ids []string) ([]Compared, error) {
	listed, err := classrooms.List(ctx, m.db, uuid.Nil, true)
	if err != nil {
		return nil, err
	}
	params, err := coreParameters(ctx, m.db)
	if err != nil {
		return nil, err
	}
	out := []Compared{}
	for _, l := range listed {
		if l.ID == except || !l.Sharing.ShowPublic || l.SpaceID == nil {
			continue
		}
		has, err := m.released(ctx, *l.SpaceID, ids)
		if err != nil {
			return nil, err
		}
		if !has {
			continue
		}
		shown, err := m.publicClassroom(ctx, l.Classroom, params)
		if err != nil {
			return nil, err
		}
		out = append(out, Compared{ID: shown.ID, Name: shown.Name, PlaceName: shown.PlaceName, Country: shown.Country})
	}
	slices.SortFunc(out, func(a, b Compared) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, nil
}

// released reports whether anonymous reads values of ids in space, kept
// for CacheFor.
func (m *Module) released(ctx context.Context, space uuid.UUID, ids []string) (bool, error) {
	key := space.String() + "|" + strings.Join(ids, ",")
	m.mu.Lock()
	hit, ok := m.hasValues[key]
	m.mu.Unlock()
	if ok && m.now().Sub(hit.at) < CacheFor {
		return hit.has, nil
	}
	sum, err := m.sam.Anonymous().Summarise(ctx, client.ObservationQuery{SpaceID: space, ParameterIDs: ids})
	switch {
	case notReadable(err):
		sum.Count = 0
	case err != nil:
		return false, samaccess.Translate(ctx, err)
	}
	m.mu.Lock()
	m.hasValues[key] = hasValues{has: sum.Count > 0, at: m.now()}
	m.mu.Unlock()
	return sum.Count > 0, nil
}

type hasValues struct {
	has bool
	at  time.Time
}
