package labs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Reads of readings from SAM, as the viewer, once their membership here
// allows it: SAM reflects a removal only once the outbox delivers it.

// The device readings' page sizes: the default, and the most one request
// may ask for.
const (
	DefaultReadings = 100
	MaxReadings     = 1000
)

// flagFromSite is SAM's flag on a value it placed at its site.
const flagFromSite = "location_from_site"

// Conditions is the front end's CurrentConditions.
type Conditions struct {
	Device        DeviceView `json:"device"`
	ClassroomName string     `json:"classroomName"`
	Reading       Reading    `json:"reading"`
	// Alerts are those the reading trips of the levels in force.
	Alerts []Alert `json:"alerts"`
}

// relayed is what this service kept of a record it relayed.
type relayed struct {
	receivedAt time.Time
	recordedBy *uuid.UUID
	creditUser *uuid.UUID
}

func relayedRecords(ctx context.Context, q db.Querier, ids []uuid.UUID) (map[uuid.UUID]relayed, error) {
	out := map[uuid.UUID]relayed{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT reading_id, received_at, recorded_by, credit_user_id FROM relayed_records WHERE reading_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var r relayed
		if err := rows.Scan(&id, &r.receivedAt, &r.recordedBy, &r.creditUser); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}

// people names the people readings were recorded by or credited to: their
// participant IDs, and the names of those credited, never a minor's.
type people struct {
	ctx   context.Context
	q     db.Querier
	users map[uuid.UUID]*accounts.User
}

func (p *people) get(id uuid.UUID) *accounts.User {
	if u, ok := p.users[id]; ok {
		return u
	}
	u, err := accounts.Get(p.ctx, p.q, id)
	var out *accounts.User
	if err == nil {
		out = &u
	}
	p.users[id] = out
	return out
}

// credit fills in who recorded r and, when they asked for it, its credit.
func (p *people) credit(r *Reading, k relayed, found bool) {
	if !found {
		return
	}
	r.ReceivedAt = k.receivedAt
	if k.recordedBy != nil {
		if u := p.get(*k.recordedBy); u != nil {
			r.RecordedByParticipantID = &u.ParticipantID
		}
	}
	if k.creditUser != nil {
		if u := p.get(*k.creditUser); u != nil && (u.Minor == nil || !*u.Minor) {
			r.CreditedTo = &u.FullName
		}
	}
}

// geometryPoint is a GeoJSON point's position, if it is one.
func geometryPoint(raw json.RawMessage) (Point, bool) {
	var g struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	}
	if json.Unmarshal(raw, &g) != nil || g.Type != "Point" || len(g.Coordinates) < 2 {
		return Point{}, false
	}
	return Point{Lat: g.Coordinates[1], Lng: g.Coordinates[0]}, true
}

// place sets r's position from SAM's: the lab's when SAM took it from the
// site or has none.
func place(r *Reading, l Lab, raw json.RawMessage, fromSite bool) {
	r.Location, r.LocationSource = l.Location, FromLab
	if p, ok := geometryPoint(raw); ok && !fromSite {
		r.Location, r.LocationSource = p, FromDevice
	}
}

// viewer is the client SAM is read through as u.
func (m *Module) viewer(u accounts.User) (*client.Client, error) {
	return m.sam.User(samaccess.Person{ParticipantID: u.ParticipantID, Registered: u.Principal != nil})
}

func (m *Module) deviceReadings(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("deviceId"))
	if err != nil {
		return httpx.NotFound(msgNoDevice)
	}
	d, err := GetDevice(r.Context(), m.db, id)
	if errors.Is(err, ErrNoDevice) {
		return httpx.NotFound(msgNoDevice)
	}
	if err != nil {
		return err
	}
	if _, _, _, err := m.classroom(r, d.ClassroomID.String()); err != nil {
		if httpx.IsCode(err, httpx.CodeNotFound) {
			return httpx.NotFound(msgNoDevice)
		}
		return err
	}
	// A limit that is not a whole number from 1 is the default, as the
	// contract lists no refusal for it.
	limit := DefaultReadings
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n >= 1 {
		limit = min(n, MaxReadings)
	}
	out := []Reading{}
	if d.SAMDeviceID == nil {
		httpx.WriteJSON(w, http.StatusOK, out)
		return nil
	}
	l, err := Get(r.Context(), m.db, d.LabID)
	if err != nil {
		return err
	}
	cur, _ := accounts.CurrentUser(r.Context())
	u, err := accounts.Get(r.Context(), m.db, cur.ID)
	if err != nil {
		return err
	}
	as, err := m.viewer(u)
	if err != nil {
		return samaccess.Translate(r.Context(), err)
	}
	list, err := readingsOf(r.Context(), as, *l.SpaceID, *d.SAMDeviceID, limit)
	if err != nil {
		return samaccess.Translate(r.Context(), err)
	}
	ids := make([]uuid.UUID, len(list))
	for i, s := range list {
		ids[i] = s.ReadingID
	}
	kept, err := relayedRecords(r.Context(), m.db, ids)
	if err != nil {
		return err
	}
	p := &people{ctx: r.Context(), q: m.db, users: map[uuid.UUID]*accounts.User{}}
	for _, s := range list {
		out = append(out, fromSAM(s, d, l, kept, p))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// readingsOf are a device's readings in SAM, newest first, up to limit.
func readingsOf(ctx context.Context, as *client.Client, space, device uuid.UUID, limit int) ([]client.Reading, error) {
	var out []client.Reading
	q := client.PageQuery{Limit: min(limit, 200)}
	for len(out) < limit {
		page, err := as.Readings(ctx, client.ObservationQuery{SpaceID: space, DeviceIDs: []uuid.UUID{device}}, q)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextCursor == "" || len(page.Items) == 0 {
			break
		}
		q.Cursor = page.NextCursor
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// fromSAM is a SAM reading in the front end's shape: values by parameter,
// in the parameters' own units, as SAM stores them.
func fromSAM(s client.Reading, d Device, l Lab, kept map[uuid.UUID]relayed, p *people) Reading {
	r := Reading{ID: s.ReadingID, DeviceID: d.ID, LabID: d.LabID, ClassroomID: d.ClassroomID, ObservedAt: s.ObservedAt, ReceivedAt: s.ObservedAt,
		Values: map[string]any{}}
	fromSite := false
	for param, v := range s.Values {
		r.Values[param] = v.Value
		fromSite = fromSite || slices.Contains(v.Flags, flagFromSite)
	}
	place(&r, l, s.Location, fromSite)
	k, found := kept[s.ReadingID]
	p.credit(&r, k, found)
	return r
}

func (m *Module) currentConditions(w http.ResponseWriter, r *http.Request) error {
	cur, _ := accounts.CurrentUser(r.Context())
	u, err := accounts.Get(r.Context(), m.db, cur.ID)
	if err != nil {
		return err
	}
	// Membership only: a platform admin sees no classroom's conditions as
	// an admin.
	listed, err := classrooms.List(r.Context(), m.db, u.ID, false)
	if err != nil {
		return err
	}
	out := []Conditions{}
	var as *client.Client
	p := &people{ctx: r.Context(), q: m.db, users: map[uuid.UUID]*accounts.User{}}
	for _, c := range listed {
		if c.Role == "" || c.SpaceID == nil {
			continue
		}
		if as == nil {
			if as, err = m.viewer(u); err != nil {
				return samaccess.Translate(r.Context(), err)
			}
		}
		got, err := m.conditionsIn(r.Context(), as, c.Classroom, c.Role, u, p)
		if err != nil {
			return err
		}
		out = append(out, got...)
	}
	slices.SortFunc(out, func(a, b Conditions) int { return b.Reading.ObservedAt.Compare(a.Reading.ObservedAt) })
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// newestReadings are SAM's latest values grouped by device, keeping each
// device's newest reading's values.
func newestReadings(records []client.ObservationRecord) map[uuid.UUID][]client.ObservationRecord {
	newest := map[uuid.UUID][]client.ObservationRecord{}
	for _, rec := range records {
		if rec.DeviceID == nil {
			continue
		}
		list := newest[*rec.DeviceID]
		switch {
		case len(list) == 0 || rec.ObservedAt.After(list[0].ObservedAt):
			newest[*rec.DeviceID] = []client.ObservationRecord{rec}
		case rec.ReadingID == list[0].ReadingID:
			newest[*rec.DeviceID] = append(list, rec)
		}
	}
	return newest
}

// latestReading is a device's newest reading from SAM's latest values of
// it, in the front end's shape.
func latestReading(d Device, l Lab, list []client.ObservationRecord) Reading {
	first := list[0]
	rd := Reading{ID: first.ReadingID, DeviceID: d.ID, LabID: d.LabID, ClassroomID: d.ClassroomID, ObservedAt: first.ObservedAt,
		ReceivedAt: first.IngestedAt, Values: map[string]any{}}
	fromSite := false
	for _, rec := range list {
		rd.Values[rec.ParameterID] = rec.Value
		fromSite = fromSite || slices.Contains(rec.Flags, flagFromSite)
	}
	place(&rd, l, first.Location, fromSite)
	return rd
}

// conditionsIn are the latest reading of each of a classroom's devices SAM
// holds values of: the values of each device's newest reading.
func (m *Module) conditionsIn(ctx context.Context, as *client.Client, c classrooms.Classroom, role roles.Role, u accounts.User,
	p *people) ([]Conditions, error) {
	devices, err := ClassroomDevices(ctx, m.db, c.ID)
	if err != nil || len(devices) == 0 {
		return nil, err
	}
	bySAM := map[uuid.UUID]Device{}
	var ids []uuid.UUID
	for _, d := range devices {
		if d.SAMDeviceID != nil {
			bySAM[*d.SAMDeviceID], ids = d, append(ids, *d.SAMDeviceID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	records, err := as.Latest(ctx, client.ObservationQuery{SpaceID: *c.SpaceID, DeviceIDs: ids})
	if err != nil {
		return nil, samaccess.Translate(ctx, err)
	}
	newest := newestReadings(records)
	var readingIDs []uuid.UUID
	for _, list := range newest {
		readingIDs = append(readingIDs, list[0].ReadingID)
	}
	kept, err := relayedRecords(ctx, m.db, readingIDs)
	if err != nil {
		return nil, err
	}
	can := roles.PermissionsFor(&roles.Viewer{PlatformAdmin: u.PlatformAdmin}, role, c.Sharing)
	defaults, err := registry.AlertDefaults(ctx, m.db)
	if err != nil {
		return nil, err
	}
	v, err := m.reg.View(ctx)
	if err != nil {
		return nil, err
	}
	labs := map[uuid.UUID]Lab{}
	out := []Conditions{}
	for samID, list := range newest {
		d := bySAM[samID]
		l, ok := labs[d.LabID]
		if !ok {
			if l, err = Get(ctx, m.db, d.LabID); err != nil {
				return nil, err
			}
			labs[d.LabID] = l
		}
		rd := latestReading(d, l, list)
		k, found := kept[rd.ID]
		p.credit(&rd, k, found)
		rules, err := m.rulesFor(ctx, d, defaults, p)
		if err != nil {
			return nil, err
		}
		alerts := AlertsFor(rd.Values, rules, v.Parameter)
		if alerts == nil {
			alerts = []Alert{}
		}
		out = append(out, Conditions{Device: view(d, l.Name, c, can), ClassroomName: c.Name, Reading: rd, Alerts: alerts})
	}
	return out, nil
}
