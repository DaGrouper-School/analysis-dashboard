// Package registry is the registry module: the registry the front end
// reads, built from SAM's registry, which is read as this service and
// cached, and this service's overlay, which presents it
// (docs/design/registry-and-content.md).
package registry

import (
	"context"
	"net/http"
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
}

// New builds the module. SAM's registry is read through sam, as this
// service, effective in the tenant's root.
func New(q db.Querier, sam *client.Client, root uuid.UUID, now func() time.Time, sessions *accounts.Sessions) *Module {
	fetch := func(ctx context.Context) (*SAM, error) { return Fetch(ctx, sam, root, now()) }
	return &Module{db: q, cache: NewCache(fetch, now), sessions: sessions}
}

// Cache is the copy of SAM's registry the module serves from.
func (m *Module) Cache() *Cache { return m.cache }

// Routes serves the registry's read operations of the front end's
// contract, for everyone signed in.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	read := func(h httpx.Handler) http.Handler { return m.sessions.RequireUser(h) }
	mux.Handle("GET "+prefix+"/registry/parameters", read(m.list(func(v *View) any { return v.Parameters })))
	mux.Handle("GET "+prefix+"/registry/domains", read(m.list(func(v *View) any { return v.Domains })))
	mux.Handle("GET "+prefix+"/registry/device-types", read(m.list(func(v *View) any { return v.DeviceTypes })))
	mux.Handle("GET "+prefix+"/registry/observation-templates", read(m.list(func(v *View) any { return v.Templates })))
	mux.Handle("GET "+prefix+"/registry/parameters/{parameterId}/calculation", read(httpx.Handler(m.calculation)))
}

// View is the registry as the front end reads it now.
func (m *Module) View(ctx context.Context) (*View, error) {
	s, err := m.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	o, err := LoadOverlay(ctx, m.db)
	if err != nil {
		return nil, err
	}
	return Merge(s, o), nil
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
