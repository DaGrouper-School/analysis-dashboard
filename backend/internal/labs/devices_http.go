package labs

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
)

const (
	msgNoDevice       = "Device not found."
	msgRegister       = "Only facilitators and the institution head can register devices."
	msgRemove         = "You cannot remove this device."
	msgType           = "Pick a device type from the registry."
	msgIdentifierUsed = "A device with this identifier is already registered for this classroom."
)

// DeviceView is the front end's Device. Its SAM source ID shows only to
// those who see the classroom's code, which it carries.
type DeviceView struct {
	ID           uuid.UUID `json:"id"`
	LabID        uuid.UUID `json:"labId"`
	LabName      string    `json:"labName"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Identifier   string    `json:"identifier"`
	SAMSourceID  *string   `json:"samSourceId"`
	RegisteredAt time.Time `json:"registeredAt"`
}

func view(d Device, labName string, c classrooms.Classroom, can roles.Permissions) DeviceView {
	v := DeviceView{ID: d.ID, LabID: d.LabID, LabName: labName, Name: d.Name, Type: d.Type, Identifier: d.Identifier, RegisteredAt: d.RegisteredAt}
	if can.SeeCode {
		s := SourceID(c.Code, d.Identifier)
		v.SAMSourceID = &s
	}
	return v
}

func (m *Module) listDevices(w http.ResponseWriter, r *http.Request) error {
	c, _, can, err := m.classroom(r, r.PathValue("classroomId"))
	if err != nil {
		return err
	}
	labs, err := ForClassroom(r.Context(), m.db, c.ID)
	if err != nil {
		return err
	}
	names := map[uuid.UUID]string{}
	for _, l := range labs {
		names[l.ID] = l.Name
	}
	list, err := ClassroomDevices(r.Context(), m.db, c.ID)
	if err != nil {
		return err
	}
	out := make([]DeviceView, len(list))
	for i, d := range list {
		out[i] = view(d, names[d.LabID], c, can)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) registerDevice(w http.ResponseWriter, r *http.Request) error {
	l, c, _, can, err := m.lab(r)
	if err != nil {
		return err
	}
	if !can.ManageDevices {
		return httpx.Forbidden(msgRegister)
	}
	var in DeviceInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	t, err := m.types(r.Context(), in.Type)
	if err != nil {
		return err
	}
	if t == nil {
		return httpx.FieldError(httpx.CodeInvalidInput, "type", msgType)
	}
	if err := httpx.CheckFields(CheckDevice(in, t.Manual)); err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	d, err := RegisterDevice(r.Context(), m.db, m.now(), l, u.ID, in, t.Manual)
	if errors.Is(err, ErrIdentifierTaken) {
		return httpx.FieldError(httpx.CodeConflict, "identifier", msgIdentifierUsed)
	}
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, view(d, l.Name, c, can))
	return nil
}

func (m *Module) removeDevice(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("deviceId"))
	if err != nil {
		return httpx.NotFound(msgNoDevice)
	}
	d, err := GetDevice(r.Context(), m.db, id)
	if errors.Is(err, ErrNoDevice) || err == nil && d.Removed {
		return httpx.NotFound(msgNoDevice)
	}
	if err != nil {
		return err
	}
	_, _, can, err := m.classroom(r, d.ClassroomID.String())
	if httpx.IsCode(err, httpx.CodeNotFound) {
		return httpx.NotFound(msgNoDevice)
	}
	if err != nil {
		return err
	}
	if !can.ManageDevices {
		return httpx.Forbidden(msgRemove)
	}
	u, _ := accounts.CurrentUser(r.Context())
	if err := RemoveDevice(r.Context(), m.db, m.now(), d, u.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
