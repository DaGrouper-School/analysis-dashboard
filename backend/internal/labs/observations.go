package labs

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// ObservationInput is the front end's ObservationInput: values by
// parameter, in each parameter's own unit.
type ObservationInput struct {
	TemplateID string         `json:"templateId"`
	LabID      string         `json:"labId"`
	DeviceID   *string        `json:"deviceId"`
	ObservedAt string         `json:"observedAt"`
	Values     map[string]any `json:"values"`
	Location   *Point         `json:"location"`
}

// Observation is the front end's Observation.
type Observation struct {
	ID                      uuid.UUID      `json:"id"`
	LabID                   uuid.UUID      `json:"labId"`
	ClassroomID             uuid.UUID      `json:"classroomId"`
	ObservedAt              time.Time      `json:"observedAt"`
	ReceivedAt              time.Time      `json:"receivedAt"`
	Values                  map[string]any `json:"values"`
	Location                Point          `json:"location"`
	LocationSource          string         `json:"locationSource"`
	CreditedTo              *string        `json:"creditedTo"`
	RecordedByParticipantID *int64         `json:"recordedByParticipantId"`
	TemplateID              string         `json:"templateId"`
	DeviceID                *uuid.UUID     `json:"deviceId"`
}

func observation(r Reading, templateID string, device *uuid.UUID) Observation {
	return Observation{ID: r.ID, LabID: r.LabID, ClassroomID: r.ClassroomID, ObservedAt: r.ObservedAt, ReceivedAt: r.ReceivedAt, Values: r.Values,
		Location: r.Location, LocationSource: r.LocationSource, CreditedTo: r.CreditedTo, RecordedByParticipantID: r.RecordedByParticipantID,
		TemplateID: templateID, DeviceID: device}
}

// Target is the front end's ObservationTarget.
type Target struct {
	Lab           Lab          `json:"lab"`
	ClassroomName string       `json:"classroomName"`
	Devices       []DeviceView `json:"devices"`
}

const (
	msgCannotObserve = "You can read this classroom’s data but not record observations."
	msgNoForm        = "That observation form no longer exists."
	msgCheckFields   = "Check the highlighted fields."
	msgLabNotReady   = "This lab is still being set up. Try again in a moment."
)

// entryNamespace names the IDs of manual entries in SAM.
var entryNamespace = uuid.MustParse("6f3c2a5e-8b1d-4f0a-9c47-2d5e8a1b7c30")

// EntryID is a manual entry's ID in SAM: the same for the same person,
// form, lab, device and time, so a resent entry stores nothing twice.
func EntryID(user uuid.UUID, templateID string, lab uuid.UUID, device *uuid.UUID, at time.Time) uuid.UUID {
	d := ""
	if device != nil {
		d = device.String()
	}
	return uuid.NewSHA1(entryNamespace, []byte(user.String()+"|"+templateID+"|"+lab.String()+"|"+d+"|"+at.UTC().Format(time.RFC3339Nano)))
}

// canRecord reports whether a member may record in a classroom, as the
// lab page's canRecord says.
func canRecord(can roles.Permissions) bool { return can.RecordReadings && can.SeeUnreleased }

func (m *Module) observationTargets(w http.ResponseWriter, r *http.Request) error {
	cur, _ := accounts.CurrentUser(r.Context())
	listed, err := classrooms.List(r.Context(), m.db, cur.ID, false)
	if err != nil {
		return err
	}
	out := []Target{}
	for _, c := range listed {
		can := roles.PermissionsFor(&roles.Viewer{PlatformAdmin: cur.PlatformAdmin}, c.Role, c.Sharing)
		if c.Role == "" || !canRecord(can) {
			continue
		}
		list, err := ForClassroom(r.Context(), m.db, c.ID)
		if err != nil {
			return err
		}
		devices, err := ClassroomDevices(r.Context(), m.db, c.ID)
		if err != nil {
			return err
		}
		for _, l := range list {
			t := Target{Lab: l, ClassroomName: c.Name, Devices: []DeviceView{}}
			for _, d := range devices {
				if d.LabID == l.ID {
					t.Devices = append(t.Devices, view(d, l.Name, c.Classroom, can))
				}
			}
			out = append(out, t)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) submitObservation(w http.ResponseWriter, r *http.Request) error {
	l, _, _, can, err := m.lab(r)
	if err != nil {
		return err
	}
	if !canRecord(can) {
		return httpx.Forbidden(msgCannotObserve)
	}
	cur, _ := accounts.CurrentUser(r.Context())
	u, err := accounts.Get(r.Context(), m.db, cur.ID)
	if err != nil {
		return err
	}
	if block := ContributionBlock(u); block != "" {
		return httpx.Forbidden(block)
	}
	var in struct {
		ObservationInput
		Credit bool `json:"credit"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	o, _, err := m.Record(r.Context(), l, u, in.ObservationInput, in.Credit, FormObservation, RecordOptions{})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, o)
	return nil
}

// checked is an entry checked here: its time, device, and what the
// registry made of it.
type checked struct {
	at     time.Time
	device *Device
	entry  registry.Entry
}

// check checks an entry against its template and the lab: its time, its
// position, its values, and the device it was read from, which must be one
// of the lab's of the template's type, or none for a field observation.
func (m *Module) check(ctx context.Context, l Lab, in ObservationInput) (checked, error) {
	f := map[string]string{}
	var c checked
	at, err := time.Parse(time.RFC3339, in.ObservedAt)
	if err != nil {
		f["observedAt"] = "Enter when you observed it."
	} else {
		action, err := futureAction(ctx, m.db)
		if err != nil {
			return c, err
		}
		if msg := CheckFuture(at, m.now(), action); msg != "" {
			f["observedAt"] = msg
		}
		c.at = at
	}
	if p := in.Location; p != nil && !validPoint(*p) {
		f["location"] = "That GPS position is not valid."
	}
	e, problems, err := m.reg.CheckEntry(ctx, in.TemplateID, in.Values)
	if errors.Is(err, registry.ErrNoTemplate) {
		return c, httpx.NotFound(msgNoForm)
	}
	if err != nil {
		return c, err
	}
	c.entry = e
	for k, v := range problems {
		f[k] = v
	}
	if d, msg, err := m.entryDevice(ctx, l, e.Template.DeviceType, in.DeviceID); err != nil {
		return c, err
	} else if msg != "" {
		f["deviceId"] = msg
	} else {
		c.device = d
	}
	if len(f) > 0 {
		return c, &httpx.Error{Code: httpx.CodeInvalidInput, Message: msgCheckFields, Fields: f}
	}
	return c, nil
}

// entryDevice is the device an entry was read from, or why it cannot be.
func (m *Module) entryDevice(ctx context.Context, l Lab, deviceType, id *string) (*Device, string, error) {
	if deviceType == nil {
		if id != nil {
			return nil, "A field observation is not read off a device.", nil
		}
		return nil, "", nil
	}
	const pick = "Choose one of this lab’s registered devices of that type."
	if id == nil {
		return nil, pick, nil
	}
	did, err := uuid.Parse(*id)
	if err != nil {
		return nil, pick, nil
	}
	d, err := GetDevice(ctx, m.db, did)
	if errors.Is(err, ErrNoDevice) || err == nil && (d.Removed || d.LabID != l.ID || d.Type != *deviceType) {
		return nil, pick, nil
	}
	return &d, "", err
}

// RecordOptions are how an entry reaches SAM beyond the form: the ID it
// takes there, nil for one made from the entry itself, and the sender's
// clock when it was sent, nil for this service's.
type RecordOptions struct {
	ID     *uuid.UUID
	SentAt *time.Time
}

// Record checks a manual entry made in lab l by u and relays it to SAM as
// them: an entry through a device type's plain template as an unsigned
// reading of the device, any other through SAM's template, under its
// device or in the lab's space. It is kept as a relayed record of form.
// It reports whether SAM held the entry already.
func (m *Module) Record(ctx context.Context, l Lab, u accounts.User, in ObservationInput, credit bool, form string, opts RecordOptions) (Observation, bool, error) {
	c, err := m.check(ctx, l, in)
	if err != nil {
		return Observation{}, false, err
	}
	if l.SpaceID == nil {
		return Observation{}, false, httpx.New(httpx.CodeConflict, msgLabNotReady)
	}
	if c.device != nil && c.device.SAMDeviceID == nil {
		return Observation{}, false, httpx.New(httpx.CodeConflict, msgNotReady)
	}
	if c.entry.Plain {
		list, err := m.prepare(ctx, *c.device, l, u, []ReadingInput{{ObservedAt: in.ObservedAt, Values: in.Values, Location: in.Location}}, credit)
		if err != nil {
			return Observation{}, false, err
		}
		got, held, err := m.relay(ctx, *c.device, u, list, form, opts.SentAt)
		if err != nil {
			return Observation{}, false, err
		}
		return observation(got[0], in.TemplateID, &c.device.ID), held[0], nil
	}
	return m.submitEntry(ctx, l, u, in, c, credit, form, opts)
}

// allDuplicate reports whether SAM held every value already.
func allDuplicate(values []client.ValueResult) bool {
	for _, v := range values {
		if v.Status != valueDuplicate {
			return false
		}
	}
	return len(values) > 0
}

// valueDuplicate is SAM's status of a value it already held.
const valueDuplicate = "duplicate"

func (m *Module) submitEntry(ctx context.Context, l Lab, u accounts.User, in ObservationInput, c checked, credit bool, form string,
	opts RecordOptions) (Observation, bool, error) {
	decimals := -1
	if u.Minor != nil && *u.Minor {
		var err error
		if decimals, err = MinorDecimals(ctx, m.db, u.ID); err != nil {
			return Observation{}, false, err
		}
	}
	var devID, samDevice *uuid.UUID
	if c.device != nil {
		devID, samDevice = &c.device.ID, c.device.SAMDeviceID
	}
	pid := u.ParticipantID
	id := EntryID(u.ID, in.TemplateID, l.ID, devID, c.at)
	if opts.ID != nil {
		id = *opts.ID
	}
	rd := Reading{ID: id, LabID: l.ID, ClassroomID: l.ClassroomID, ObservedAt: c.at.UTC(),
		Values: in.Values, Location: l.Location, LocationSource: FromLab, RecordedByParticipantID: &pid}
	if credit && (u.Minor == nil || !*u.Minor) {
		rd.CreditedTo = &u.FullName
	}
	entry := client.TemplateEntry{ID: rd.ID, TemplateID: in.TemplateID, DeviceID: samDevice, ObservedAt: c.at.UTC(), Values: c.entry.Fields}
	if in.Location != nil {
		p := *in.Location
		if decimals >= 0 {
			p = Round(p, decimals)
		}
		rd.Location, rd.LocationSource = p, FromDevice
		entry.Location = &client.ObservationLocation{Geometry: point(p), Method: "gps"}
	}
	as, err := m.viewer(u)
	if err != nil {
		return Observation{}, false, samaccess.Translate(ctx, err)
	}
	sent := opts.SentAt
	if sent == nil {
		now := m.now()
		sent = &now
	}
	got, err := as.SubmitEntries(ctx, client.EntriesRequest{SpaceID: *l.SpaceID, SentAt: sent, Entries: []client.TemplateEntry{entry}})
	if err != nil {
		return Observation{}, false, samaccess.Translate(ctx, err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Status != readingAccepted {
		return Observation{}, false, entryRefused(ctx, got, c.entry)
	}
	rd.ReceivedAt = got.ReceivedAt.Round(time.Microsecond)
	var creditUser *uuid.UUID
	if rd.CreditedTo != nil {
		creditUser = &u.ID
	}
	if err := keepRelayed(ctx, m.db, rd, form, &u.ID, creditUser); err != nil {
		return Observation{}, false, err
	}
	return observation(rd, in.TemplateID, devID), allDuplicate(got.Entries[0].Values), nil
}

// entryRefused is SAM's refusal of an entry, its problems by field named
// by parameter where SAM names the field.
func entryRefused(ctx context.Context, got client.EntriesOutcome, e registry.Entry) error {
	f := map[string]string{}
	if len(got.Entries) == 1 {
		res := got.Entries[0]
		slog.InfoContext(ctx, "SAM did not store an entry", "status", res.Status, "code", res.Code, "problems", res.Problems)
		for _, p := range res.FieldProblems {
			if param, ok := e.Parameters[p.Field]; ok {
				f[param] = "This value is not accepted."
			}
		}
	}
	if len(f) > 0 {
		return &httpx.Error{Code: httpx.CodeInvalidInput, Message: msgCheckFields, Fields: f}
	}
	return httpx.New(httpx.CodeInvalidInput, "This observation was not accepted.")
}
