// Package settingsapi serves platform admins the platform's settings and
// the configuration's change log, which internal/settings keeps.
package settingsapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// Module serves platform admins the settings and the configuration's
// change log.
type Module struct {
	db       db.Querier
	sessions *accounts.Sessions
	now      func() time.Time
}

// New builds the module.
func New(q db.Querier, sessions *accounts.Sessions, now func() time.Time) *Module {
	return &Module{db: q, sessions: sessions, now: now}
}

// Routes serves the settings' and the change log's admin operations,
// which are in the contract's addendum.
func (m *Module) Routes(mux *http.ServeMux, prefix string) {
	admin := func(h httpx.Handler) http.Handler {
		return m.sessions.RequireAdmin("Only platform admins can change the platform's configuration.", h)
	}
	mux.Handle("GET "+prefix+"/admin/settings", admin(m.list))
	mux.Handle("PUT "+prefix+"/admin/settings/{key}", admin(m.put))
	mux.Handle("GET "+prefix+"/admin/configuration-changes", admin(m.changes))
}

// SettingView is one setting as admins see it: nil before the seed sets
// it.
type SettingView struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Seeded    bool            `json:"seeded"`
	UpdatedAt *time.Time      `json:"updatedAt"`
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) error {
	stored, err := settings.All(r.Context(), m.db)
	if err != nil {
		return err
	}
	out := make([]SettingView, 0, len(settings.Keys()))
	for _, k := range settings.Keys() {
		v := SettingView{Key: k.Name, Value: json.RawMessage("null")}
		if e, ok := stored[k.Name]; ok {
			v.Value, v.Seeded, v.UpdatedAt = e.Value, e.Seeded, &e.UpdatedAt
		}
		out = append(out, v)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) put(w http.ResponseWriter, r *http.Request) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBody))
	if err != nil {
		return httpx.New(httpx.CodeInvalidInput, "The request could not be read.")
	}
	u, _ := accounts.CurrentUser(r.Context())
	key := r.PathValue("key")
	_, err = settings.Put(r.Context(), m.db, m.now(), settings.Actor{UserID: &u.ID, Name: u.FullName}, key, raw)
	var invalid *jsoncheck.Invalid
	switch {
	case errors.Is(err, settings.ErrUnknownKey):
		return httpx.NotFound("There is no such setting.")
	case errors.As(err, &invalid):
		return httpx.Invalid(invalid.Fields())
	case err != nil:
		return err
	}
	e, err := settings.Get(r.Context(), m.db, key)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, SettingView{Key: e.Key, Value: e.Value, Seeded: e.Seeded, UpdatedAt: &e.UpdatedAt})
	return nil
}

// ChangesPage is how many changes one page of the log holds.
const ChangesPage = 50

// ChangeView is one entry of the log as admins read it.
type ChangeView struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	ByName  string    `json:"byName"`
	Kind    string    `json:"kind"`
	Subject string    `json:"subject"`
	Summary string    `json:"summary"`
}

// ChangesView is a page of the log, newest first, and the cursor of the
// next page, or nil on the last.
type ChangesView struct {
	Items []ChangeView `json:"items"`
	Next  *string      `json:"next"`
}

func (m *Module) changes(w http.ResponseWriter, r *http.Request) error {
	var before int64
	if c := r.URL.Query().Get("before"); c != "" {
		var err error
		if before, err = strconv.ParseInt(c, 10, 64); err != nil || before < 1 {
			return httpx.FieldError(httpx.CodeInvalidInput, "before", "Use the next cursor of the page before.")
		}
	}
	list, err := settings.ChangesBefore(r.Context(), m.db, before, ChangesPage+1)
	if err != nil {
		return err
	}
	out := ChangesView{Items: []ChangeView{}}
	for i, l := range list {
		if i == ChangesPage {
			next := strconv.FormatInt(list[i-1].ID, 10)
			out.Next = &next
			break
		}
		out.Items = append(out.Items, ChangeView{ID: strconv.FormatInt(l.ID, 10), At: l.At, ByName: l.Actor.Name, Kind: l.Kind, Subject: l.Subject, Summary: l.Summary})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}
