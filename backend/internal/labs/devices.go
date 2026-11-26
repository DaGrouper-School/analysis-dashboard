package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Device is a device registered to a lab.
type Device struct {
	ID           uuid.UUID
	LabID        uuid.UUID
	ClassroomID  uuid.UUID
	Name         string
	Type         string
	Identifier   string
	RegisteredAt time.Time
	SAMDeviceID  *uuid.UUID
	SAMVersion   *int
	Removed      bool
}

// DeviceInput is the front end's RegisterDeviceInput.
type DeviceInput struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

// identifierForm is a hardware identifier, as the front end checks it.
var identifierForm = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$`)

// CheckDevice reports what is wrong with a device, by field. A device read
// by hand has no serial, so its identifier is not checked: one is made.
func CheckDevice(in DeviceInput, manual bool) map[string]string {
	f := map[string]string{}
	if utf8.RuneCountInString(strings.TrimSpace(in.Name)) < 3 {
		f["name"] = "Enter a name."
	}
	if !manual && !identifierForm.MatchString(strings.TrimSpace(in.Identifier)) {
		f["identifier"] = "Use 3 to 64 letters, digits, dots, dashes or underscores."
	}
	return f
}

// SourceID is a device's SAM source identity: its classroom's code and its
// identifier.
func SourceID(classroomCode, identifier string) string { return classroomCode + ":" + identifier }

// ErrIdentifierTaken means a live device of the classroom has the
// identifier.
var ErrIdentifierTaken = errors.New("identifier taken")

// ErrNoDevice means there is no such device.
var ErrNoDevice = errors.New("no such device")

// DeviceKind is the outbox kind that registers and retires a device in
// SAM, sent as the person who did it.
const DeviceKind = "device"

func deviceEntity(id uuid.UUID) string { return "device:" + id.String() }

// deviceChange is what SAM is to hold of a device: registered, or retired.
type deviceChange struct {
	Retired bool `json:"retired"`
}

// RegisterDevice stores a device in a lab, registered by by, and sends it
// to SAM as them. A device read by hand gets the next MAN-0001 identifier
// of its classroom; any other's is upper-cased.
func RegisterDevice(ctx context.Context, q db.Querier, now time.Time, l Lab, by uuid.UUID, in DeviceInput, manual bool) (Device, error) {
	d := Device{ID: uuid.New(), LabID: l.ID, ClassroomID: l.ClassroomID, Name: strings.TrimSpace(in.Name), Type: in.Type,
		Identifier: strings.ToUpper(strings.TrimSpace(in.Identifier)), RegisteredAt: now}
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		if manual {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM devices WHERE classroom_id = $1 AND identifier LIKE 'MAN-%'`, l.ClassroomID).Scan(&n); err != nil {
				return err
			}
			d.Identifier = fmt.Sprintf("MAN-%04d", n+1)
		}
		_, err := tx.Exec(ctx, `INSERT INTO devices (id, lab_id, classroom_id, name, type, identifier, registered_by, registered_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, d.ID, l.ID, l.ClassroomID, d.Name, d.Type, d.Identifier, by, now)
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return ErrIdentifierTaken
		}
		if err != nil {
			return err
		}
		_, err = samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: deviceEntity(d.ID), Kind: DeviceKind, Actor: &by, Payload: deviceChange{}})
		return err
	})
	return d, err
}

// RemoveDevice removes a device here at once, and retires it in SAM as by,
// which keeps its readings.
func RemoveDevice(ctx context.Context, q db.Querier, now time.Time, d Device, by uuid.UUID) error {
	return pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE devices SET removed_at = $2, removed_by = $3 WHERE id = $1 AND removed_at IS NULL`, d.ID, now, by)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = samaccess.Enqueue(ctx, tx, now, samaccess.Change{Entity: deviceEntity(d.ID), Kind: DeviceKind, Actor: &by,
			Payload: deviceChange{Retired: true}, TakesAccessAway: true})
		return err
	})
}

const deviceColumns = `id, lab_id, classroom_id, name, type, identifier, registered_at, sam_device_id, sam_version, removed_at IS NOT NULL`

func scanDevice(r pgx.Row) (Device, error) {
	var d Device
	err := r.Scan(&d.ID, &d.LabID, &d.ClassroomID, &d.Name, &d.Type, &d.Identifier, &d.RegisteredAt, &d.SAMDeviceID, &d.SAMVersion, &d.Removed)
	return d, err
}

// GetDevice is the device with id, removed or not, or ErrNoDevice.
func GetDevice(ctx context.Context, q db.Querier, id uuid.UUID) (Device, error) {
	d, err := scanDevice(q.QueryRow(ctx, `SELECT `+deviceColumns+` FROM devices WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNoDevice
	}
	return d, err
}

func devices(ctx context.Context, q db.Querier, where string, arg any) ([]Device, error) {
	rows, err := q.Query(ctx, `SELECT `+deviceColumns+` FROM devices WHERE removed_at IS NULL AND `+where+` ORDER BY registered_at, id`, arg)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Device, error) { return scanDevice(r) })
}

// LabDevices are a lab's live devices, oldest first.
func LabDevices(ctx context.Context, q db.Querier, lab uuid.UUID) ([]Device, error) {
	return devices(ctx, q, `lab_id = $1`, lab)
}

// ClassroomDevices are the live devices of a classroom's labs.
func ClassroomDevices(ctx context.Context, q db.Querier, classroom uuid.UUID) ([]Device, error) {
	return devices(ctx, q, `classroom_id = $1`, classroom)
}

// applyDevice registers a device in its lab's space as the person who
// registered it, with its registry type, its identifier, no key and
// policy flag, or retires it naming the version SAM holds. It waits until
// SAM has the lab's space and, for this tenant's own type, the type, and
// holds the person with their own rights: their membership of the
// classroom, or of the platform admins' group.
func (m *Module) applyDevice(ctx context.Context, a samaccess.Attempt) error {
	var want deviceChange
	if err := json.Unmarshal(a.Payload, &want); err != nil {
		return samaccess.Hold(err)
	}
	id, err := uuid.Parse(strings.TrimPrefix(a.Entity, "device:"))
	if err != nil {
		return samaccess.Hold(err)
	}
	d, err := GetDevice(ctx, m.db, id)
	if err != nil {
		return err
	}
	if want.Retired {
		return m.retire(ctx, a, d)
	}
	if d.SAMDeviceID != nil || d.Removed {
		return nil
	}
	l, err := Get(ctx, m.db, d.LabID)
	if err != nil {
		return err
	}
	if l.SpaceID == nil {
		return samaccess.ErrNotYet
	}
	if waiting, err := samaccess.Waiting(ctx, m.db, registry.DeviceTypeEntity(d.Type)); err != nil || waiting {
		return errors.Join(samaccess.ErrNotYet, err)
	}
	if ok, err := m.actorReady(ctx, d.ClassroomID, a); err != nil || !ok {
		return errors.Join(samaccess.ErrNotYet, err)
	}
	sam, err := a.SAM.RegisterDevice(a.WithKey(ctx, "register"), client.DeviceRegistration{SpaceID: *l.SpaceID, DeviceTypeID: d.Type,
		Identifier: d.Identifier, Policy: "flag"})
	if err != nil {
		return err
	}
	_, err = m.db.Exec(ctx, `UPDATE devices SET sam_device_id = $2, sam_version = $3 WHERE id = $1`, id, sam.ID, sam.Version)
	return err
}

// retire retires a device SAM has, as the person who removed it once SAM
// holds their rights. SAM keeps its readings.
func (m *Module) retire(ctx context.Context, a samaccess.Attempt, d Device) error {
	if d.SAMDeviceID == nil {
		return nil
	}
	if ok, err := m.actorReady(ctx, d.ClassroomID, a); err != nil || !ok {
		return errors.Join(samaccess.ErrNotYet, err)
	}
	err := a.SAM.RetireDevice(a.WithKey(ctx, "retire"), *d.SAMDeviceID, *d.SAMVersion)
	var e *client.Error
	if errors.As(err, &e) && e.Status == 404 {
		return nil
	}
	return err
}

// actorReady reports whether SAM holds the change's person with the rights
// they act on: their membership of the classroom, or the platform admins'.
func (m *Module) actorReady(ctx context.Context, classroom uuid.UUID, a samaccess.Attempt) (bool, error) {
	if a.Actor == nil {
		return false, samaccess.Hold(errors.New("a device change names no person"))
	}
	if in, err := classrooms.InSAM(ctx, m.db, classroom, *a.Actor); err != nil || in {
		return in, err
	}
	u, err := accounts.Get(ctx, m.db, *a.Actor)
	return u.SAMAdmin, err
}
