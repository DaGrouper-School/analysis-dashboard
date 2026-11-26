package labs

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// The raw data table: a classroom's values read from SAM as the member,
// a page at a time, with the total and the filters it can offer from
// SAM's summary.

// FieldSource is the source ID of field observations, which have no
// device.
const FieldSource = "field"

// The raw table's pages: the most rows one holds, and the furthest offset
// SAM takes.
const (
	MaxRawPage   = 500
	MaxRawOffset = 10_000
)

// RawQuery is the front end's RawQuery.
type RawQuery struct {
	ClassroomID string `json:"classroomId"`
	From        string `json:"from"`
	To          string `json:"to"`
	ParameterID string `json:"parameterId"`
	SourceID    string `json:"sourceId"`
	LabID       string `json:"labId"`
	Sort        string `json:"sort"`
	Order       string `json:"order"`
}

// RawRow is the front end's RawRow.
type RawRow struct {
	ID             string    `json:"id"`
	ObservedAt     time.Time `json:"observedAt"`
	ReceivedAt     time.Time `json:"receivedAt"`
	ParameterID    string    `json:"parameterId"`
	Value          any       `json:"value"`
	SourceID       string    `json:"sourceId"`
	SourceName     string    `json:"sourceName"`
	SourceKind     string    `json:"sourceKind"`
	LabID          uuid.UUID `json:"labId"`
	SiteName       string    `json:"siteName"`
	Location       Point     `json:"location"`
	LocationSource string    `json:"locationSource"`
	ContributorID  *int64    `json:"contributorId"`
}

// RawSource is a source the table can be filtered by.
type RawSource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// RawSite is a site the table can be filtered by: a lab.
type RawSite struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// RawDays are the first and last day of the values.
type RawDays struct {
	First string `json:"first"`
	Last  string `json:"last"`
}

// RawOptions are the filters the table can offer.
type RawOptions struct {
	ParameterIDs []string    `json:"parameterIds"`
	Sources      []RawSource `json:"sources"`
	Sites        []RawSite   `json:"sites"`
	Days         *RawDays    `json:"days"`
}

// RawPage is the front end's RawPage.
type RawPage struct {
	ClassroomID   uuid.UUID          `json:"classroomId"`
	ClassroomName string             `json:"classroomName"`
	Rows          []RawRow           `json:"rows"`
	Total         int64              `json:"total"`
	Page          int                `json:"page"`
	PageSize      int                `json:"pageSize"`
	Terms         settings.DataTerms `json:"terms"`
	Options       RawOptions         `json:"options"`
}

// rawSorts are SAM's sort keys for the table's columns. SAM sorts these by
// ID, so a sort by parameter, source or site groups rows rather than
// ordering them by their labels.
var rawSorts = map[string]string{"time": "observed_at", "parameter": "parameter_id", "source": "device_id", "site": "space_id"}

const day = "2006-01-02"

// rawScope is what the query picks in SAM's terms: the classroom's
// devices and labs, by SAM's IDs.
type rawScope struct {
	classroom classrooms.Classroom
	labs      map[uuid.UUID]Lab
	bySpace   map[uuid.UUID]Lab
	devices   map[uuid.UUID]Device
	bySAM     map[uuid.UUID]Device
}

// filters are the query's filters, as SAM's: those the query sets, minus
// any named in without.
func (s rawScope) filters(q RawQuery, without ...string) (client.ObservationQuery, map[string]string) {
	out := client.ObservationQuery{SpaceID: *s.classroom.SpaceID}
	f := map[string]string{}
	use := func(name, v string) bool { return v != "" && !slices.Contains(without, name) }
	if use("labId", q.LabID) {
		id, err := uuid.Parse(q.LabID)
		l, ok := s.labs[id]
		switch {
		case err != nil || !ok:
			f["labId"] = "Pick one of the classroom's sites."
		case l.SpaceID != nil:
			out.SpaceID = *l.SpaceID
		}
	}
	if use("sourceId", q.SourceID) {
		if q.SourceID == FieldSource {
			out.Field = true
		} else if id, err := uuid.Parse(q.SourceID); err != nil || s.devices[id].SAMDeviceID == nil {
			f["sourceId"] = "Pick one of the classroom's sources."
		} else {
			out.DeviceIDs = []uuid.UUID{*s.devices[id].SAMDeviceID}
		}
	}
	if use("parameterId", q.ParameterID) {
		out.ParameterIDs = []string{q.ParameterID}
	}
	if use("from", q.From) {
		if t, err := time.Parse(day, q.From); err != nil {
			f["from"] = "Pick a day."
		} else {
			out.From = &t
		}
	}
	if use("to", q.To) {
		// SAM's range excludes its end, so the day after the last one.
		if t, err := time.Parse(day, q.To); err != nil {
			f["to"] = "Pick a day."
		} else {
			end := t.AddDate(0, 0, 1)
			out.To = &end
		}
	}
	return out, f
}

func (m *Module) rawValues(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		RawQuery
		Page     *int `json:"page"`
		PageSize *int `json:"pageSize"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	f := map[string]string{}
	page, size := 1, 25
	if in.Page != nil {
		page = *in.Page
	}
	if in.PageSize != nil {
		size = *in.PageSize
	}
	switch {
	case size < 1 || size > MaxRawPage:
		f["pageSize"] = fmt.Sprintf("Show 1 to %d rows a page.", MaxRawPage)
	case page < 1:
		f["page"] = "Pages start at 1."
	case (page-1)*size > MaxRawOffset:
		f["page"] = fmt.Sprintf("Only the first %s rows can be paged through. Narrow the filters to see the rest.", "10,000")
	}
	if in.Sort != "" && rawSorts[in.Sort] == "" {
		f["sort"] = "Sort by time, parameter, source or site."
	}
	if in.Order != "" && in.Order != "asc" && in.Order != "desc" {
		f["order"] = "Sort ascending or descending."
	}
	if err := httpx.CheckFields(f); err != nil {
		return err
	}
	s, as, err := m.rawScope(r, in.ClassroomID)
	if err != nil {
		return err
	}
	out := RawPage{ClassroomID: s.classroom.ID, ClassroomName: s.classroom.Name, Rows: []RawRow{}, Page: page, PageSize: size, Terms: s.classroom.Terms}
	if s.classroom.SpaceID == nil {
		// Still being set up in SAM, so it holds nothing yet.
		out.Options = RawOptions{ParameterIDs: []string{}, Sources: []RawSource{}, Sites: []RawSite{}}
		httpx.WriteJSON(w, http.StatusOK, out)
		return nil
	}
	q, f := s.filters(in.RawQuery)
	if err := httpx.CheckFields(f); err != nil {
		return err
	}
	ctx := r.Context()
	if out.Options, err = m.rawOptions(ctx, as, s, in.RawQuery); err != nil {
		return err
	}
	sum, err := as.Summarise(ctx, q)
	if err != nil {
		return samaccess.Translate(ctx, err)
	}
	out.Total = sum.Count
	if out.Rows, err = m.rawRows(ctx, as, s, q, in.RawQuery, page, size); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// rawScope is the classroom the member asks for, its labs and devices,
// and the client SAM is read through as them. Membership only: a platform
// admin who is not a member reads nothing, and a classroom someone may
// not see is not found.
func (m *Module) rawScope(r *http.Request, id string) (rawScope, *client.Client, error) {
	ctx := r.Context()
	c, role, _, err := m.classroom(r, id)
	if err == nil && role == "" {
		err = httpx.NotFound(msgNoClassroom)
	}
	if err != nil {
		return rawScope{}, nil, err
	}
	s := rawScope{classroom: c, labs: map[uuid.UUID]Lab{}, bySpace: map[uuid.UUID]Lab{}, devices: map[uuid.UUID]Device{}, bySAM: map[uuid.UUID]Device{}}
	list, err := ForClassroom(ctx, m.db, c.ID)
	if err != nil {
		return s, nil, err
	}
	for _, l := range list {
		s.labs[l.ID] = l
		if l.SpaceID != nil {
			s.bySpace[*l.SpaceID] = l
		}
	}
	// Removed devices keep their history, so their values are listed too.
	if err := m.allDevices(ctx, c.ID, func(d Device) {
		s.devices[d.ID] = d
		if d.SAMDeviceID != nil {
			s.bySAM[*d.SAMDeviceID] = d
		}
	}); err != nil {
		return s, nil, err
	}
	cur, _ := accounts.CurrentUser(ctx)
	u, err := accounts.Get(ctx, m.db, cur.ID)
	if err != nil {
		return s, nil, err
	}
	as, err := m.viewer(u)
	if err != nil {
		return s, nil, samaccess.Translate(ctx, err)
	}
	return s, as, nil
}

// allDevices calls each with every device of a classroom, removed or not.
func (m *Module) allDevices(ctx context.Context, classroom uuid.UUID, each func(Device)) error {
	rows, err := m.db.Query(ctx, `SELECT `+deviceColumns+` FROM devices WHERE classroom_id = $1`, classroom)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return err
		}
		each(d)
	}
	return rows.Err()
}

// rawOptions are the filters the table offers, narrowing in order, site
// then source then parameter, each offering only what the ones before it
// leave; the days span the values the other filters match.
func (m *Module) rawOptions(ctx context.Context, as *client.Client, s rawScope, q RawQuery) (RawOptions, error) {
	summary := func(without ...string) (client.Summary, error) {
		sq, _ := s.filters(q, without...)
		sum, err := as.Summarise(ctx, sq)
		return sum, samaccess.Translate(ctx, err)
	}
	all := []string{"labId", "sourceId", "parameterId", "from", "to"}
	out := RawOptions{ParameterIDs: []string{}, Sources: []RawSource{}, Sites: []RawSite{}}
	everything, err := summary(all...)
	if err != nil {
		return out, err
	}
	for _, space := range everything.Spaces {
		if l, ok := s.bySpace[space]; ok && !slices.ContainsFunc(out.Sites, func(x RawSite) bool { return x.ID == l.ID }) {
			out.Sites = append(out.Sites, RawSite{ID: l.ID, Name: l.Name})
		}
	}
	inSite, err := summary("sourceId", "parameterId", "from", "to")
	if err != nil {
		return out, err
	}
	for _, id := range inSite.Devices {
		if d, ok := s.bySAM[id]; ok {
			out.Sources = append(out.Sources, RawSource{ID: d.ID.String(), Name: d.Name, Kind: "device"})
		}
	}
	if inSite.FieldValues > 0 {
		out.Sources = append(out.Sources, RawSource{ID: FieldSource, Name: "Field observation", Kind: FieldSource})
	}
	inSource, err := summary("parameterId", "from", "to")
	if err != nil {
		return out, err
	}
	out.ParameterIDs = append(out.ParameterIDs, inSource.Parameters...)
	span, err := summary("from", "to")
	if err != nil {
		return out, err
	}
	if span.First != nil && span.Last != nil {
		out.Days = &RawDays{First: span.First.UTC().Format(day), Last: span.Last.UTC().Format(day)}
	}
	byName := func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }
	slices.SortFunc(out.Sources, func(a, b RawSource) int { return byName(a.Name, b.Name) })
	slices.SortFunc(out.Sites, func(a, b RawSite) int { return byName(a.Name, b.Name) })
	return out, nil
}

// rawRows are one page of the table from SAM's observation list, newest
// first unless the query sorts otherwise.
func (m *Module) rawRows(ctx context.Context, as *client.Client, s rawScope, sq client.ObservationQuery, q RawQuery, page, size int) ([]RawRow, error) {
	sort := q.Sort
	if sort == "" {
		sort = "time"
	}
	order := q.Order
	if order == "" {
		order = "asc"
		if sort == "time" {
			order = "desc"
		}
	}
	sq.Sort, sq.Order = rawSorts[sort], order
	// SAM gives at most 200 a request, so a larger page is read in steps.
	var records []client.ObservationRecord
	for len(records) < size {
		sq.Offset = (page-1)*size + len(records)
		got, err := as.Observations(ctx, sq, client.PageQuery{Limit: min(size-len(records), 200)})
		if err != nil {
			return nil, samaccess.Translate(ctx, err)
		}
		records = append(records, got.Items...)
		if len(got.Items) == 0 || got.NextCursor == "" {
			break
		}
	}
	ids := make([]uuid.UUID, len(records))
	for i, rec := range records {
		ids[i] = rec.ReadingID
	}
	kept, err := relayedRecords(ctx, m.db, ids)
	if err != nil {
		return nil, err
	}
	p := &people{ctx: ctx, q: m.db, users: map[uuid.UUID]*accounts.User{}}
	out := make([]RawRow, 0, len(records))
	for _, rec := range records {
		out = append(out, s.row(rec, kept, p))
	}
	return out, nil
}

// row is one SAM value as a row of the table, its position rounded as the
// classroom's terms say.
func (s rawScope) row(rec client.ObservationRecord, kept map[uuid.UUID]relayed, p *people) RawRow {
	row := RawRow{ID: rec.ReadingID.String() + ":" + rec.ParameterID, ObservedAt: rec.ObservedAt, ReceivedAt: rec.IngestedAt,
		ParameterID: rec.ParameterID, Value: rec.Value, SourceID: FieldSource, SourceName: "Field observation", SourceKind: FieldSource}
	if rec.DeviceID != nil {
		if d, ok := s.bySAM[*rec.DeviceID]; ok {
			row.SourceID, row.SourceName, row.SourceKind = d.ID.String(), d.Name, "device"
		}
	}
	l := s.bySpace[rec.SpaceID]
	row.LabID, row.SiteName = l.ID, l.Name
	var r Reading
	place(&r, l, rec.Location, slices.Contains(rec.Flags, flagFromSite))
	row.Location, row.LocationSource = r.Location, r.LocationSource
	if d := s.classroom.Terms.CoordinateDecimals; d != nil {
		row.Location = Round(row.Location, *d)
	}
	if k, ok := kept[rec.ReadingID]; ok && k.recordedBy != nil {
		if u := p.get(*k.recordedBy); u != nil {
			row.ContributorID = &u.ParticipantID
		}
	}
	return row
}
