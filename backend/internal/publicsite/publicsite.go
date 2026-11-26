// Package publicsite is the public site: the home page and its content,
// read from SAM as its anonymous principal, so SAM's read rules apply to
// what a visitor sees (docs/design/dashboards-and-public-site.md#public-pages).
package publicsite

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/labs"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/views"
)

// CacheFor is how long a classroom's core readings from SAM are kept
// before SAM is asked again.
const CacheFor = time.Minute

// Registry is what the module reads of the registry the front end reads.
type Registry interface {
	View(ctx context.Context) (*registry.View, error)
}

// Module is the public site.
type Module struct {
	db       db.Querier
	reg      Registry
	sam      *samaccess.Access
	sessions *accounts.Sessions
	now      func() time.Time

	mu        sync.Mutex
	cache     map[uuid.UUID]cached
	hasValues map[string]hasValues
}

type cached struct {
	core views.CoreReadings
	at   time.Time
}

// New builds the module. SAM is read as its anonymous principal, and as
// the caller on the pages a member sees more of.
func New(q db.Querier, reg Registry, sam *samaccess.Access, sessions *accounts.Sessions, now func() time.Time) *Module {
	return &Module{db: q, reg: reg, sam: sam, sessions: sessions, now: now, cache: map[uuid.UUID]cached{}, hasValues: map[string]hasValues{}}
}

// Routes serves the public site, which needs no session; a page a member
// sees more of reads theirs when there is one.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	mux.Handle("GET "+prefix+"/public/home", httpx.Handler(m.home))
	mux.Handle("GET "+prefix+"/public/content", httpx.Handler(m.content))
	mux.Handle("GET "+prefix+"/public/classrooms/{classroomId}", m.sessions.OptionalUser(httpx.Handler(m.station)))
	mux.Handle("GET "+prefix+"/public/classrooms/{classroomId}/profile", m.sessions.OptionalUser(httpx.Handler(m.profile)))
	mux.Handle("GET "+prefix+"/public/contributors/{participantId}", httpx.Handler(m.contributor))
	mux.Handle("GET "+prefix+"/public/classrooms/{classroomId}/parameters/{parameterId}", m.sessions.OptionalUser(httpx.Handler(m.parameter)))
}

// Content is the site content this service keeps: the stages and
// regions, in display order. The site's words are the front end's (the
// contract's addendum).
type Content struct {
	Stages  []classrooms.Stage  `json:"stages"`
	Regions []classrooms.Region `json:"regions"`
}

// Classroom is the front end's PublicClassroom: no code, no members,
// positions rounded.
type Classroom struct {
	ID              uuid.UUID          `json:"id"`
	Name            string             `json:"name"`
	InstitutionName *string            `json:"institutionName"`
	PlaceName       *string            `json:"placeName"`
	Country         *string            `json:"country"`
	Location        *labs.Point        `json:"location"`
	StageID         string             `json:"stageId"`
	RegionID        *string            `json:"regionId"`
	Themes          []string           `json:"themes"`
	Core            views.CoreReadings `json:"core"`
}

// Units are the units of the core readings, from the registry.
type Units struct {
	Rain        string `json:"rain"`
	Temperature string `json:"temperature"`
}

// Home is the front end's PublicHome.
type Home struct {
	Content    Content     `json:"content"`
	Classrooms []Classroom `json:"classrooms"`
	Units      Units       `json:"units"`
}

func (m *Module) siteContent(ctx context.Context) (Content, error) {
	stages, err := classrooms.Stages(ctx, m.db)
	if err != nil {
		return Content{}, err
	}
	regions, err := classrooms.Regions(ctx, m.db)
	if stages == nil {
		stages = []classrooms.Stage{}
	}
	if regions == nil {
		regions = []classrooms.Region{}
	}
	return Content{Stages: stages, Regions: regions}, err
}

func (m *Module) content(w http.ResponseWriter, r *http.Request) error {
	c, err := m.siteContent(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, c)
	return nil
}

func coreParameters(ctx context.Context, q db.Querier) (settings.CoreParameters, error) {
	e, err := settings.Get(ctx, q, settings.KeyCoreParameters)
	if err != nil {
		return settings.CoreParameters{}, err
	}
	var p settings.CoreParameters
	return p, json.Unmarshal(e.Value, &p)
}

func (m *Module) home(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	content, err := m.siteContent(ctx)
	if err != nil {
		return err
	}
	params, err := coreParameters(ctx, m.db)
	if err != nil {
		return err
	}
	v, err := m.reg.View(ctx)
	if err != nil {
		return err
	}
	unit := func(ids []string) string {
		if len(ids) > 0 {
			if p := v.Parameter(ids[0]); p != nil {
				return p.Unit
			}
		}
		return ""
	}
	listed, err := classrooms.List(ctx, m.db, uuid.Nil, true)
	if err != nil {
		return err
	}
	out := Home{Content: content, Classrooms: make([]Classroom, 0, len(listed)), Units: Units{Rain: unit(params.RainDaily), Temperature: unit(params.AirTemperature)}}
	for _, l := range listed {
		c, err := m.publicClassroom(ctx, l.Classroom, params)
		if err != nil {
			return err
		}
		out.Classrooms = append(out.Classrooms, c)
	}
	slices.SortFunc(out.Classrooms, func(a, b Classroom) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// coarsen is p rounded to decimals places.
func coarsen(p labs.Point, decimals int) *labs.Point {
	f := math.Pow(10, float64(decimals))
	return &labs.Point{Lat: math.Round(p.Lat*f) / f, Lng: math.Round(p.Lng*f) / f}
}

// publicClassroom is c as a visitor sees it. The sharing settings here
// decide first: a classroom that does not show its data publicly has no
// values, whatever SAM still lets anonymous read before the revocation
// reaches it.
func (m *Module) publicClassroom(ctx context.Context, c classrooms.Classroom, params settings.CoreParameters) (Classroom, error) {
	out, err := m.shell(ctx, c)
	if err != nil || !c.Sharing.ShowPublic || c.SpaceID == nil {
		return out, err
	}
	out.Core = m.core(ctx, c, params)
	return out, nil
}

// shell is c as a visitor sees it, with no values: its name, host, place
// and themes.
func (m *Module) shell(ctx context.Context, c classrooms.Classroom) (Classroom, error) {
	out := Classroom{ID: c.ID, Name: c.Name, StageID: c.StageID, RegionID: c.RegionID, Themes: []string{}, Core: views.NoCore()}
	// The host is named only once it has signed the host agreement.
	if c.Profile.HostAgreementSigned {
		out.InstitutionName = &c.InstitutionName
	}
	list, err := labs.ForClassroom(ctx, m.db, c.ID)
	if err != nil {
		return out, err
	}
	if len(list) > 0 {
		first := list[0]
		decimals := roles.PublicDecimals
		if d := c.Terms.CoordinateDecimals; d != nil && *d < decimals {
			decimals = *d
		}
		out.PlaceName, out.Country, out.Location = &first.PlaceName, &first.Country, coarsen(first.Location, decimals)
	}
	for _, l := range list {
		out.Themes = append(out.Themes, l.SiteProfile.ResearchQuestions...)
	}
	return out, nil
}

// core is a classroom's core readings as anonymous reads them, kept for
// CacheFor. A classroom SAM cannot be read for shows none.
func (m *Module) core(ctx context.Context, c classrooms.Classroom, params settings.CoreParameters) views.CoreReadings {
	m.mu.Lock()
	hit, ok := m.cache[c.ID]
	m.mu.Unlock()
	if ok && m.now().Sub(hit.at) < CacheFor {
		return hit.core
	}
	core, err := m.readCore(ctx, *c.SpaceID, params)
	if err != nil {
		slog.WarnContext(ctx, "the public home could not read a classroom's core readings", "classroom", c.ID, "err", err)
		return views.NoCore()
	}
	m.mu.Lock()
	m.cache[c.ID] = cached{core: core, at: m.now()}
	m.mu.Unlock()
	return core
}

// readCore reads a classroom's core readings from SAM as anonymous: its
// latest values, and its rain by day and source over the week up to the
// newest.
func (m *Module) readCore(ctx context.Context, space uuid.UUID, params settings.CoreParameters) (views.CoreReadings, error) {
	as := m.sam.Anonymous()
	records, err := as.Latest(ctx, client.ObservationQuery{SpaceID: space})
	var ce *client.Error
	if errors.As(err, &ce) && ce.Status == http.StatusForbidden {
		return views.NoCore(), nil
	}
	if err != nil {
		return views.CoreReadings{}, err
	}
	latest := make([]views.Latest, 0, len(records))
	for _, r := range records {
		latest = append(latest, views.Latest{ParameterID: r.ParameterID, Value: r.Value, ObservedAt: r.ObservedAt, ReceivedAt: r.IngestedAt})
	}
	if len(latest) == 0 || len(params.RainDaily) == 0 {
		return views.Core(latest, nil, params), nil
	}
	newest := latest[0].ObservedAt
	for _, l := range latest {
		if l.ObservedAt.After(newest) {
			newest = l.ObservedAt
		}
	}
	end := time.Date(newest.UTC().Year(), newest.UTC().Month(), newest.UTC().Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	from := end.AddDate(0, 0, -7)
	agg, err := as.Aggregate(ctx, client.Aggregation{ObservationQuery: client.ObservationQuery{SpaceID: space, ParameterIDs: params.RainDaily, From: &from, To: &end},
		Bucket: "day", TimeZone: "UTC", GroupBy: []string{"device_id", "parameter_id"}, Functions: []string{"max", "last"}})
	if err != nil {
		return views.CoreReadings{}, err
	}
	var rain []views.RainDay
	for _, g := range agg.Groups {
		if g.Bucket == nil || g.ParameterID == nil || g.Max == nil {
			continue
		}
		rd := views.RainDay{Day: g.Bucket.UTC().Format("2006-01-02"), Source: labs.FieldSource, ParameterID: *g.ParameterID, Max: *g.Max}
		if g.DeviceID != nil {
			rd.Source = g.DeviceID.String()
		}
		if g.Last != nil {
			rd.LastAt = g.Last.At
		}
		rain = append(rain, rd)
	}
	return views.Core(latest, rain, params), nil
}
