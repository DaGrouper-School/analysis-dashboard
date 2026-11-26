// Package registry is the registry module: the registry the front end
// reads, built from SAM's registry, which is read as this service and
// cached, and this service's overlay, which presents it
// (docs/design/registry-and-content.md).
package registry

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dagrouper-school/sam/client"
	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// Module is the registry module.
type Module struct {
	db       db.Querier
	cache    *Cache
	sessions *accounts.Sessions
	now      func() time.Time
}

// New builds the module. SAM's registry is read through sam, as this
// service, effective in the tenant's root.
func New(q db.Querier, sam *client.Client, root uuid.UUID, now func() time.Time, sessions *accounts.Sessions) *Module {
	fetch := func(ctx context.Context) (*SAM, error) { return Fetch(ctx, sam, root, now()) }
	return &Module{db: q, cache: NewCache(fetch, now), sessions: sessions, now: now}
}

// Cache is the copy of SAM's registry the module serves from.
func (m *Module) Cache() *Cache { return m.cache }

// Routes serves the registry's operations of the front end's contract:
// reads for everyone signed in, changes for platform admins.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	h := func(pattern string, handler httpx.Handler) {
		mux.Handle(strings.Replace(pattern, " ", " "+prefix+"/registry", 1), m.sessions.RequireUser(handler))
	}
	h("GET /parameters", m.list(func(v *View) any { return v.Parameters }))
	h("GET /domains", m.list(func(v *View) any { return v.Domains }))
	h("GET /device-types", m.list(func(v *View) any { return v.DeviceTypes }))
	h("GET /observation-templates", m.list(func(v *View) any { return v.Templates }))
	h("GET /parameters/{parameterId}/calculation", m.calculation)
	h("GET /alert-defaults", m.listAlertDefaults)

	h = func(pattern string, handler httpx.Handler) {
		guarded := func(w http.ResponseWriter, r *http.Request) error {
			if err := admin(r); err != nil {
				return err
			}
			return handler(w, r)
		}
		mux.Handle(strings.Replace(pattern, " ", " "+prefix+"/registry", 1), m.sessions.RequireUser(httpx.Handler(guarded)))
	}
	h("POST /domains", m.createDomain)
	h("PATCH /domains/{domainId}", m.updateDomain)
	h("DELETE /domains/{domainId}", m.deleteDomain)
	h("POST /device-types", m.createDeviceType)
	h("PATCH /device-types/{deviceTypeId}", m.updateDeviceType)
	h("PATCH /parameters/{parameterId}", m.mapParameter)
	h("PUT /parameters/{parameterId}/chart", m.chartParameter)
	h("PUT /parameters/{parameterId}/explanation", m.explainParameter)
	h("POST /alert-defaults", m.createAlertDefault)
	h("PUT /alert-defaults/{ruleId}", m.updateAlertDefault)
	h("DELETE /alert-defaults/{ruleId}", m.deleteAlertDefault)
	h("GET /changes", m.listChanges)
}

// View is the registry as the front end reads it now.
func (m *Module) View(ctx context.Context) (*View, error) {
	s, err := m.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	return merged(ctx, m.db, s)
}

func (m *Module) list(part func(*View) any) httpx.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		v, err := m.View(r.Context())
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, part(v))
		return nil
	}
}

// CalculationDetails is what "How is this calculated?" shows.
type CalculationDetails struct {
	Calculation
	Parameter    CalculatedValue `json:"parameter"`
	InputDetails []Input         `json:"inputDetails"`
}

// CalculatedValue is the value a calculation is for.
type CalculatedValue struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Unit   string `json:"unit"`
	Source string `json:"source"`
	Plain  string `json:"plain,omitempty"`
}

// Input is one value a calculation uses.
type Input struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Unit  string `json:"unit"`
}

func (m *Module) calculation(w http.ResponseWriter, r *http.Request) error {
	v, err := m.View(r.Context())
	if err != nil {
		return err
	}
	p := v.Parameter(r.PathValue("parameterId"))
	if p == nil {
		return httpx.NotFound("That value is not in the registry.")
	}
	out := CalculationDetails{Calculation: *p.Calculation, InputDetails: []Input{},
		Parameter: CalculatedValue{ID: p.ID, Label: p.Label, Unit: p.Unit, Source: p.Source, Plain: p.Plain}}
	for _, id := range p.Calculation.Inputs {
		in := Input{ID: id, Label: id}
		if q := v.Parameter(id); q != nil {
			in.Label, in.Unit = q.Label, q.Unit
		}
		out.InputDetails = append(out.InputDetails, in)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// DeviceType is the device type with id in the registry as the front end
// reads it, or nil.
func (m *Module) DeviceType(ctx context.Context, id string) (*DeviceType, error) {
	v, err := m.View(ctx)
	if err != nil {
		return nil, err
	}
	return v.deviceType(id), nil
}

// DeviceTypeEntity is the outbox entity of this tenant's own device type
// id: what reaches SAM before a device of the type can.
func DeviceTypeEntity(id string) string { return entity(OwnDeviceType, id) }
