package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/consent"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// ReadingInput is the front end's ReadingInput.
type ReadingInput struct {
	ObservedAt string         `json:"observedAt"`
	Values     map[string]any `json:"values"`
	Location   *Point         `json:"location"`
}

// Reading is the front end's Reading.
type Reading struct {
	ID                      uuid.UUID      `json:"id"`
	DeviceID                uuid.UUID      `json:"deviceId"`
	LabID                   uuid.UUID      `json:"labId"`
	ClassroomID             uuid.UUID      `json:"classroomId"`
	ObservedAt              time.Time      `json:"observedAt"`
	ReceivedAt              time.Time      `json:"receivedAt"`
	Values                  map[string]any `json:"values"`
	Location                Point          `json:"location"`
	LocationSource          string         `json:"locationSource"`
	CreditedTo              *string        `json:"creditedTo"`
	RecordedByParticipantID *int64         `json:"recordedByParticipantId"`
}

// The sources of a reading's position.
const (
	FromDevice = "device"
	FromLab    = "lab"
)

// The forms a relayed record comes through.
const (
	FormUpload      = "upload"
	FormObservation = "observation"
	FormBatch       = "batch"
)

// readingAccepted is SAM's status of a reading it took, its values stored
// or already held.
const readingAccepted = "accepted"

// ClockTolerance is how far ahead of this service's clock a record may be
// dated before it counts as dated in the future.
const ClockTolerance = 5 * time.Minute

// SAMFuture is how far ahead SAM takes a value at all.
const SAMFuture = 24 * time.Hour

const (
	msgCannotRecord = "You can read this classroom’s data but not record readings."
	msgNotReady     = "This device is still being set up. Try again in a moment."
	msgNoConsent    = "Your guardian needs to give consent before you can send observations."
	msgWithdrawn    = "Your guardian withdrew their consent, so you cannot send observations. Ask them to give it again."
	msgNotStored    = "None of these readings were accepted."
)

// ContributionBlock is why the user may not send observations, as the
// front end's contributionBlock says it, or "" when they may: a minor needs
// their guardian's given consent, by this service's own record.
func ContributionBlock(u accounts.User) string {
	if u.Minor == nil || !*u.Minor || u.Consent != nil && *u.Consent == accounts.ConsentGiven {
		return ""
	}
	if u.Consent != nil && *u.Consent == accounts.ConsentWithdrawn {
		return msgWithdrawn
	}
	return msgNoConsent
}

// CheckReading reports what is wrong with a reading, by field, as the
// front end's validateReading does.
func CheckReading(in ReadingInput) map[string]string {
	f := map[string]string{}
	if _, err := time.Parse(time.RFC3339, in.ObservedAt); err != nil {
		f["observedAt"] = "Enter when it was measured."
	}
	if len(in.Values) == 0 {
		f["values"] = "A reading needs at least one value."
	}
	if p := in.Location; p != nil && !validPoint(*p) {
		f["location"] = "Enter a valid latitude and longitude, or leave both blank."
	}
	return f
}

// validPoint reports whether p is a position on Earth.
func validPoint(p Point) bool {
	return !math.IsNaN(p.Lat) && !math.IsNaN(p.Lng) && p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180
}

// CheckFuture reports why a record dated at is refused, received now
// under the future_observations action, or "": SAM takes nothing dated
// more than a day ahead, and refuse turns away anything past the clock
// tolerance.
func CheckFuture(at, now time.Time, action string) string {
	switch {
	case at.After(now.Add(SAMFuture)):
		return "It is dated more than a day ahead."
	case action == settings.FutureRefuse && at.After(now.Add(ClockTolerance)):
		return "It is dated in the future."
	}
	return ""
}

// Round is p rounded to decimals places.
func Round(p Point, decimals int) Point {
	f := math.Pow(10, float64(decimals))
	return Point{Lat: math.Round(p.Lat*f) / f, Lng: math.Round(p.Lng*f) / f}
}

// MinorDecimals are the decimal places a minor's positions are rounded to:
// the coarser of the privacy setting now and the rounding their guardian's
// latest consent was given against.
func MinorDecimals(ctx context.Context, q db.Querier, user uuid.UUID) (int, error) {
	d, err := consent.Decimals(ctx, q)
	if err != nil {
		return 0, err
	}
	r, found, err := consent.LatestRecord(ctx, q, user, nil)
	if found && r.Decimals < d {
		d = r.Decimals
	}
	return d, err
}

// Seq is a reading's sequence number in SAM: its observed time in
// milliseconds, so a resent reading of the device is the same reading.
func Seq(at time.Time) int64 { return at.UnixMilli() }

func futureAction(ctx context.Context, q db.Querier) (string, error) {
	e, err := settings.Get(ctx, q, settings.KeyFuture)
	if errors.Is(err, settings.ErrNotSet) {
		return settings.FutureFlag, nil
	}
	if err != nil {
		return "", err
	}
	var f settings.Future
	return f.Action, json.Unmarshal(e.Value, &f)
}

// prepared is a reading ready for SAM, and what the front end gets back.
type prepared struct {
	relay  map[string]any
	answer Reading
}

func (m *Module) uploadReadings(w http.ResponseWriter, r *http.Request) error {
	d, l, u, err := m.recorder(r)
	if err != nil {
		return err
	}
	var in struct {
		Readings []ReadingInput `json:"readings"`
		Credit   bool           `json:"credit"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	list, err := m.prepare(r.Context(), d, l, u, in.Readings, in.Credit)
	if err != nil {
		return err
	}
	out, err := m.relay(r.Context(), d, u, list, FormUpload)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// recorder is the device in the path, its lab and the user, who may record
// readings in its classroom, as the lab page's canRecord says, and, if a
// minor, has their guardian's consent.
func (m *Module) recorder(r *http.Request) (Device, Lab, accounts.User, error) {
	cur, _ := accounts.CurrentUser(r.Context())
	id, err := uuid.Parse(r.PathValue("deviceId"))
	if err != nil {
		return Device{}, Lab{}, cur, httpx.NotFound(msgNoDevice)
	}
	d, err := GetDevice(r.Context(), m.db, id)
	if errors.Is(err, ErrNoDevice) || err == nil && d.Removed {
		return d, Lab{}, cur, httpx.NotFound(msgNoDevice)
	}
	if err != nil {
		return d, Lab{}, cur, err
	}
	_, _, can, err := m.classroom(r, d.ClassroomID.String())
	if httpx.IsCode(err, httpx.CodeNotFound) {
		return d, Lab{}, cur, httpx.NotFound(msgNoDevice)
	}
	if err != nil {
		return d, Lab{}, cur, err
	}
	// As the lab page's canRecord: a platform admin with no role manages
	// devices but records nothing, and SAM gives them no submit grant.
	if !can.RecordReadings || !can.SeeUnreleased {
		return d, Lab{}, cur, httpx.Forbidden(msgCannotRecord)
	}
	u, err := accounts.Get(r.Context(), m.db, cur.ID)
	if err != nil {
		return d, Lab{}, u, err
	}
	if block := ContributionBlock(u); block != "" {
		return d, Lab{}, u, httpx.Forbidden(block)
	}
	if d.SAMDeviceID == nil {
		return d, Lab{}, u, httpx.New(httpx.CodeConflict, msgNotReady)
	}
	l, err := Get(r.Context(), m.db, d.LabID)
	return d, l, u, err
}

// prepare checks each reading and turns it into what SAM takes: values
// under the device type's keys, a minor's position rounded, and a sequence
// number from its time. The first problem refuses the upload.
func (m *Module) prepare(ctx context.Context, d Device, l Lab, u accounts.User, in []ReadingInput, credit bool) ([]prepared, error) {
	if len(in) == 0 {
		return nil, httpx.FieldError(httpx.CodeInvalidInput, "readings", "Send at least one reading.")
	}
	action, err := futureAction(ctx, m.db)
	if err != nil {
		return nil, err
	}
	decimals := -1
	if u.Minor != nil && *u.Minor {
		if decimals, err = MinorDecimals(ctx, m.db, u.ID); err != nil {
			return nil, err
		}
	}
	now := m.now()
	var creditedTo *string
	if credit && (u.Minor == nil || !*u.Minor) {
		creditedTo = &u.FullName
	}
	base := Reading{DeviceID: d.ID, LabID: l.ID, ClassroomID: l.ClassroomID, Location: l.Location, LocationSource: FromLab,
		CreditedTo: creditedTo, RecordedByParticipantID: &u.ParticipantID}
	out := make([]prepared, len(in))
	for i, rd := range in {
		p, problems, err := m.prepareOne(ctx, d, base, rd, now, action, decimals)
		if err != nil {
			return nil, err
		}
		if len(problems) > 0 {
			fields := slices.Sorted(maps.Keys(problems))
			return nil, &httpx.Error{Code: httpx.CodeInvalidInput, Message: fmt.Sprintf("Reading %d: %s", i+1, problems[fields[0]]), Fields: problems}
		}
		out[i] = p
	}
	return out, nil
}

// prepareOne is one reading ready for SAM, or its problems by field. A
// minor's position is rounded to decimals; -1 keeps it whole.
func (m *Module) prepareOne(ctx context.Context, d Device, base Reading, rd ReadingInput, now time.Time, action string, decimals int) (prepared, map[string]string, error) {
	if f := CheckReading(rd); len(f) > 0 {
		return prepared{}, f, nil
	}
	at, _ := time.Parse(time.RFC3339, rd.ObservedAt)
	if msg := CheckFuture(at, now, action); msg != "" {
		return prepared{}, map[string]string{"observedAt": msg}, nil
	}
	keyed, problems, err := m.reg.KeyValues(ctx, d.Type, rd.Values)
	if err != nil || len(problems) > 0 {
		return prepared{}, problems, err
	}
	answer := base
	answer.ObservedAt, answer.Values = at.UTC(), rd.Values
	relay := map[string]any{"v": 1, "device_id": *d.SAMDeviceID, "seq": Seq(at), "captured_at": at.UTC(), "values": keyed}
	if rd.Location != nil {
		p := *rd.Location
		if decimals >= 0 {
			p = Round(p, decimals)
		}
		answer.Location, answer.LocationSource = p, FromDevice
		relay["location"] = point(p)
	}
	return prepared{relay: relay, answer: answer}, nil, nil
}

// relay sends the readings to SAM as the user, unsigned, which SAM flags
// and records the user as vouching for, and keeps a relayed record of each
// SAM took. Readings SAM rejected are left out and
// logged; when it stored none, the upload is refused.
func (m *Module) relay(ctx context.Context, d Device, u accounts.User, list []prepared, form string) ([]Reading, error) {
	as, err := m.sam.User(samaccess.Person{ParticipantID: u.ParticipantID, Registered: u.Principal != nil})
	if err != nil {
		return nil, samaccess.Translate(ctx, err)
	}
	now := m.now()
	in := client.ReadingsRelay{SentAt: &now, Readings: make([]client.IncomingReading, len(list))}
	for i, p := range list {
		in.Readings[i] = client.IncomingReading{Unsigned: p.relay}
	}
	got, err := as.RelayReadings(ctx, *d.SAMDeviceID, in)
	if err != nil {
		return nil, samaccess.Translate(ctx, err)
	}
	var credit *uuid.UUID
	if list[0].answer.CreditedTo != nil {
		credit = &u.ID
	}
	out := []Reading{}
	for i, res := range got.Readings {
		if i >= len(list) || res.ReadingID == nil || res.Status != readingAccepted {
			slog.InfoContext(ctx, "SAM did not store a reading", "device", d.ID, "status", res.Status, "code", res.Code, "problems", res.Problems)
			continue
		}
		a := list[i].answer
		// To the microsecond, as the database keeps it.
		a.ID, a.ReceivedAt = *res.ReadingID, got.ReceivedAt.Round(time.Microsecond)
		if err := keepRelayed(ctx, m.db, a, form, &u.ID, credit); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, httpx.New(httpx.CodeInvalidInput, msgNotStored)
	}
	return out, nil
}

// keepRelayed records a relayed record once; a resent one keeps its first
// row.
func keepRelayed(ctx context.Context, q db.Querier, r Reading, form string, by, credit *uuid.UUID) error {
	var device *uuid.UUID
	if r.DeviceID != uuid.Nil {
		device = &r.DeviceID
	}
	_, err := q.Exec(ctx, `INSERT INTO relayed_records (reading_id, form, lab_id, device_id, recorded_by, credit_user_id, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (reading_id) DO NOTHING`, r.ID, form, r.LabID, device, by, credit, r.ReceivedAt)
	return err
}
