package labs

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
)

// Module is the labs module.
type Module struct {
	db       db.Querier
	sessions *accounts.Sessions
	now      func() time.Time
	reg      Registry
	sam      *samaccess.Access
}

// Registry is what the module reads of the registry the front end reads:
// a device type, a reading's values under a type's keys, and a manual
// entry checked against its template.
type Registry interface {
	DeviceType(ctx context.Context, id string) (*registry.DeviceType, error)
	KeyValues(ctx context.Context, typeID string, values map[string]any) (map[string]any, map[string]string, error)
	CheckEntry(ctx context.Context, templateID string, values map[string]any) (registry.Entry, map[string]string, error)
	View(ctx context.Context) (*registry.View, error)
}

// New builds the module. Readings are relayed to SAM through sam, as the
// person recording.
func New(q db.Querier, sessions *accounts.Sessions, now func() time.Time, reg Registry, sam *samaccess.Access) *Module {
	return &Module{db: q, sessions: sessions, now: now, reg: reg, sam: sam}
}

// Routes serves the labs operations of the front end's contract.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	h := func(pattern string, handler httpx.Handler) {
		mux.Handle(pattern, m.sessions.RequireUser(handler))
	}
	h("GET "+prefix+"/classrooms/{classroomId}/labs", m.list)
	h("POST "+prefix+"/classrooms/{classroomId}/labs", m.create)
	h("GET "+prefix+"/labs/{labId}", m.get)
	h("PUT "+prefix+"/labs/{labId}", m.update)
	h("GET "+prefix+"/classrooms/{classroomId}/devices", m.listDevices)
	h("POST "+prefix+"/labs/{labId}/devices", m.registerDevice)
	h("DELETE "+prefix+"/devices/{deviceId}", m.removeDevice)
	h("POST "+prefix+"/devices/{deviceId}/readings", m.uploadReadings)
	h("GET "+prefix+"/devices/{deviceId}/readings", m.deviceReadings)
	h("GET "+prefix+"/me/conditions", m.currentConditions)
	h("GET "+prefix+"/me/observation-targets", m.observationTargets)
	h("POST "+prefix+"/labs/{labId}/observations", m.submitObservation)
	h("POST "+prefix+"/observations/batch", m.syncObservations)
	h("POST "+prefix+"/exports/raw", m.rawValues)
	h("GET "+prefix+"/classrooms/{classroomId}/release", m.releaseReview)
	h("POST "+prefix+"/classrooms/{classroomId}/release", m.releaseData)
	h("GET "+prefix+"/devices/{deviceId}/alerts", m.listDeviceAlerts)
	h("POST "+prefix+"/devices/{deviceId}/alerts", m.createAlert)
	h("DELETE "+prefix+"/devices/{deviceId}/alerts", m.resetAlerts)
	h("PUT "+prefix+"/devices/{deviceId}/alerts/{ruleId}", m.updateAlert)
	h("DELETE "+prefix+"/devices/{deviceId}/alerts/{ruleId}", m.deleteAlert)
}

const (
	msgNoClassroom = "Classroom not found."
	msgNoLab       = "Lab not found."
	msgManage      = "Only facilitators and the institution head can add or change a lab."
)

// classroom is the classroom in the path as the viewer sees it, or 404.
func (m *Module) classroom(r *http.Request, id string) (classrooms.Classroom, roles.Role, roles.Permissions, error) {
	u, _ := accounts.CurrentUser(r.Context())
	cid, err := uuid.Parse(id)
	if err != nil {
		return classrooms.Classroom{}, "", roles.Permissions{}, httpx.NotFound(msgNoClassroom)
	}
	c, role, can, err := classrooms.Visible(r.Context(), m.db, cid, u)
	if errors.Is(err, classrooms.ErrNoClassroom) {
		return c, role, can, httpx.NotFound(msgNoClassroom)
	}
	return c, role, can, err
}

// lab is the lab in the path, its classroom and what the viewer may do;
// a lab in a classroom they may not see is not found.
func (m *Module) lab(r *http.Request) (Lab, classrooms.Classroom, roles.Role, roles.Permissions, error) {
	id, err := uuid.Parse(r.PathValue("labId"))
	if err != nil {
		return Lab{}, classrooms.Classroom{}, "", roles.Permissions{}, httpx.NotFound(msgNoLab)
	}
	l, err := Get(r.Context(), m.db, id)
	if errors.Is(err, ErrNoLab) {
		return l, classrooms.Classroom{}, "", roles.Permissions{}, httpx.NotFound(msgNoLab)
	}
	if err != nil {
		return l, classrooms.Classroom{}, "", roles.Permissions{}, err
	}
	c, role, can, err := m.classroom(r, l.ClassroomID.String())
	if httpx.IsCode(err, httpx.CodeNotFound) {
		err = httpx.NotFound(msgNoLab)
	}
	return l, c, role, can, err
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	c, _, _, err := m.classroom(r, r.PathValue("classroomId"))
	if err != nil {
		return err
	}
	list, err := ForClassroom(r.Context(), m.db, c.ID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

func (m *Module) create(w http.ResponseWriter, r *http.Request) error {
	c, _, can, err := m.classroom(r, r.PathValue("classroomId"))
	if err != nil {
		return err
	}
	if !can.ManageDevices {
		return httpx.Forbidden(msgManage)
	}
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(Check(in)); err != nil {
		return err
	}
	l, err := Create(r.Context(), m.db, m.now(), c.ID, c.Code, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, l)
	return nil
}

func (m *Module) update(w http.ResponseWriter, r *http.Request) error {
	l, c, _, can, err := m.lab(r)
	if err != nil {
		return err
	}
	if !can.ManageDevices {
		return httpx.Forbidden(msgManage)
	}
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(Check(in)); err != nil {
		return err
	}
	if l, err = Update(r.Context(), m.db, m.now(), l, c.Code, in); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, l)
	return nil
}

// Detail is the front end's LabDetail. Readings and observations are added
// by the modules that keep them.
type Detail struct {
	Lab           Lab          `json:"lab"`
	ClassroomName string       `json:"classroomName"`
	Devices       []DeviceView `json:"devices"`
	Readings      []any        `json:"readings"`
	Observations  []any        `json:"observations"`
	CanManage     bool         `json:"canManage"`
	CanRecord     bool         `json:"canRecord"`
}

func (m *Module) get(w http.ResponseWriter, r *http.Request) error {
	l, c, _, can, err := m.lab(r)
	if err != nil {
		return err
	}
	list, err := LabDevices(r.Context(), m.db, l.ID)
	if err != nil {
		return err
	}
	devices := make([]DeviceView, len(list))
	for i, d := range list {
		devices[i] = view(d, l.Name, c, can)
	}
	httpx.WriteJSON(w, http.StatusOK, Detail{Lab: l, ClassroomName: c.Name, Devices: devices, Readings: []any{}, Observations: []any{},
		CanManage: can.ManageDevices, CanRecord: can.RecordReadings && can.SeeUnreleased})
	return nil
}
