package registry

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// edit is one admin's change to the registry, in its transaction, against
// the registry as the change before it left it.
type edit struct {
	ctx  context.Context
	tx   pgx.Tx
	now  time.Time
	by   accounts.User
	view *View
}

const msgAdminsOnly = "Only platform admins can change the registry."

// admin refuses anyone but a platform admin.
func admin(r *http.Request) error {
	if u, _ := accounts.CurrentUser(r.Context()); !u.PlatformAdmin {
		return httpx.Forbidden(msgAdminsOnly)
	}
	return nil
}

// run makes a platform admin's change: in one transaction under the
// registry lock, against the registry merged from the cached copy of
// SAM's without calling SAM, then answered with what answer returns from
// the registry it left, or with nothing when answer is nil.
func (m *Module) run(w http.ResponseWriter, r *http.Request, change func(*edit) error, answer func(*View) (any, error)) error {
	if err := admin(r); err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	ctx := r.Context()
	s, err := m.cache.Copy(ctx)
	if err != nil {
		return err
	}
	err = pgx.BeginFunc(ctx, m.db, func(tx pgx.Tx) error {
		if err := db.LockTx(ctx, tx, db.RegistryLock); err != nil {
			return err
		}
		v, err := merged(ctx, tx, s)
		if err != nil {
			return err
		}
		return change(&edit{ctx: ctx, tx: tx, now: m.now(), by: u, view: v})
	})
	if err != nil {
		return err
	}
	if answer == nil {
		return nil
	}
	v, err := merged(ctx, m.db, s)
	if err != nil {
		return err
	}
	out, err := answer(v)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// merged is the registry from s, this tenant's own entries and the
// overlay.
func merged(ctx context.Context, q db.Querier, s *SAM) (*View, error) {
	own, err := loadOwn(ctx, q)
	if err != nil {
		return nil, err
	}
	o, err := LoadOverlay(ctx, q)
	if err != nil {
		return nil, err
	}
	v := Merge(withOwn(s, own), o)
	v.owns = own
	return v, nil
}

// log records the change with the admin's name at the time.
func (e *edit) log(summary string) error {
	_, err := e.tx.Exec(e.ctx, `INSERT INTO registry_changes (at, actor_id, actor_name, summary) VALUES ($1, $2, $3, $4)`,
		e.now, e.by.ID, e.by.FullName, summary)
	return err
}

// putOwn stores one of this tenant's entries and sends it to SAM.
func (e *edit) putOwn(o Own) error {
	o.UpdatedAt = e.now
	return putOwn(e.ctx, e.tx, e.now, o)
}

// The overlay's rows, as an admin writes them: in place, or at the end of
// the list. An admin's row is never the seed's.

func (e *edit) putDomain(d DomainOverlay) error {
	_, err := e.tx.Exec(e.ctx, `
		INSERT INTO registry_domains (id, label, icon, description, hidden, position, seeded, updated_at)
		VALUES ($1, $2, $3, $4, $5, (SELECT coalesce(max(position) + 1, 0) FROM registry_domains), false, $6)
		ON CONFLICT (id) DO UPDATE SET label = $2, icon = $3, description = $4, hidden = $5, seeded = false, updated_at = $6`,
		d.ID, d.Label, d.Icon, d.Description, d.Hidden, e.now)
	return err
}

func (e *edit) dropDomain(id string) error {
	_, err := e.tx.Exec(e.ctx, `DELETE FROM registry_domains WHERE id = $1`, id)
	return err
}

func (e *edit) putParameter(p ParameterOverlay) error {
	_, err := e.tx.Exec(e.ctx, `
		INSERT INTO registry_parameters (`+parameterColumns+`, position, seeded, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			(SELECT coalesce(max(position) + 1, 0) FROM registry_parameters), false, $13)
		ON CONFLICT (id) DO UPDATE SET label = $2, definition = $3, plain = $4, explanation = $5, domain = $6, unit = $7,
			source = $8, noise = $9, chart = $10, method = $11, inputs = $12, seeded = false, updated_at = $13`,
		p.ID, p.Label, p.Definition, p.Plain, p.Explanation, p.Domain, p.Unit, p.Source, p.Noise, p.Chart, p.Method, nonNil(p.Inputs), e.now)
	return err
}

// relabel renames a value in its overlay row, if it has one.
func (e *edit) relabel(id, label string) error {
	_, err := e.tx.Exec(e.ctx, `UPDATE registry_parameters SET label = $2, seeded = false, updated_at = $3 WHERE id = $1`, id, label, e.now)
	return err
}

func (e *edit) putDeviceType(t DeviceTypeOverlay) error {
	_, err := e.tx.Exec(e.ctx, `
		INSERT INTO registry_device_types (`+deviceTypeColumns+`, position, seeded, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT coalesce(max(position) + 1, 0) FROM registry_device_types), false, $8)
		ON CONFLICT (id) DO UPDATE SET label = $2, description = $3, color = $4, icon = $5, tiles = $6, cards = $7,
			seeded = false, updated_at = $8`,
		t.ID, t.Label, t.Description, t.Color, t.Icon, nonNil(t.Tiles), nonNil(t.Cards), e.now)
	return err
}

func (e *edit) putTemplate(t TemplateOverlay) error {
	_, err := e.tx.Exec(e.ctx, `
		INSERT INTO registry_templates (`+templateColumns+`, position, seeded, updated_at)
		VALUES ($1, $2, $3, $4, $5, (SELECT coalesce(max(position) + 1, 0) FROM registry_templates), false, $6)
		ON CONFLICT (id) DO UPDATE SET label = $2, description = $3, domain = $4, device_type = $5, seeded = false, updated_at = $6`,
		t.ID, t.Label, t.Description, t.Domain, t.DeviceType, e.now)
	return err
}

// parameterRow and domainRow are an entry's overlay row: the one stored,
// or one that shows the entry as it is shown now.
func (e *edit) parameterRow(p *Parameter) (ParameterOverlay, error) {
	row, found, err := overlayRow(e.ctx, e.tx, "registry_parameters", parameterColumns, p.ID, scanParameter)
	if err != nil || found {
		return row, err
	}
	return ParameterOverlay{ID: p.ID, Label: p.Label, Plain: p.Plain, Source: p.Source, Noise: p.Noise, Inputs: []string{}}, nil
}

func (e *edit) domainRow(d Domain) (DomainOverlay, error) {
	row, found, err := overlayRow(e.ctx, e.tx, "registry_domains", `id, label, icon, description, hidden`, d.ID,
		func(r scanner) (o DomainOverlay, err error) {
			return o, r.Scan(&o.ID, &o.Label, &o.Icon, &o.Description, &o.Hidden)
		})
	if err != nil || found {
		return row, err
	}
	return DomainOverlay{ID: d.ID, Label: d.Label, Icon: d.Icon, Description: d.Description}, nil
}

func overlayRow[T any](ctx context.Context, q db.Querier, table, columns, id string, scan func(scanner) (T, error)) (T, bool, error) {
	list, err := rows(ctx, q, `SELECT `+columns+` FROM `+table+` WHERE id = $1`, scan, id)
	if err != nil || len(list) == 0 {
		var zero T
		return zero, false, err
	}
	return list[0], true, nil
}

// Change is one entry of the registry's change log.
type Change struct {
	ID      string `json:"id"`
	At      string `json:"at"`
	ByName  string `json:"byName"`
	Summary string `json:"summary"`
}

// ChangesShown is how many of the newest changes the log shows.
const ChangesShown = 500

func (m *Module) listChanges(w http.ResponseWriter, r *http.Request) error {
	if err := admin(r); err != nil {
		return err
	}
	list, err := rows(r.Context(), m.db, `SELECT id, at, actor_name, summary FROM registry_changes ORDER BY at DESC, id DESC LIMIT $1`,
		func(r scanner) (c Change, err error) {
			var id int64
			var at time.Time
			err = r.Scan(&id, &at, &c.ByName, &c.Summary)
			c.ID, c.At = "chg_"+strconv.FormatInt(id, 10), at.UTC().Format(time.RFC3339Nano)
			return c, err
		}, ChangesShown)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, nonNilList(list))
	return nil
}

func nonNilList[T any](l []T) []T {
	if l == nil {
		return []T{}
	}
	return l
}
